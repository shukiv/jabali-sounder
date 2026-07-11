// Package poller runs a background health loop over enrolled servers so the
// fleet's status is current without an operator clicking Check, and so each
// probe is recorded as a heartbeat — the basis for status history, trends, and
// alerting (roadmap M1).
package poller

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/remote"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/secrets"
)

const (
	defaultInterval  = 60 * time.Second
	probeTimeout     = 15 * time.Second
	pollConcurrency  = 8
	defaultRetention = 14 // days
	pruneInterval    = time.Hour
)

// Config wires the poller's collaborators.
type Config struct {
	Servers        repository.ServerRepository
	Heartbeats     repository.HeartbeatRepository
	SecretKey      *secrets.Key
	AllowPlaintext bool
	Interval       time.Duration
	// RetentionDays bounds heartbeat history. 0 -> default; negative -> disabled.
	RetentionDays int
	Log           *slog.Logger

	// Probe overrides the real panel health check in tests; nil uses the client.
	Probe func(ctx context.Context, s models.Server) (*remote.CheckResult, error)
}

// Poller periodically checks every non-disabled server and records the outcome.
type Poller struct{ cfg Config }

// New returns a Poller, applying defaults for interval and logger.
func New(cfg Config) *Poller {
	if cfg.Interval <= 0 {
		cfg.Interval = defaultInterval
	}
	if cfg.RetentionDays == 0 {
		cfg.RetentionDays = defaultRetention
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	return &Poller{cfg: cfg}
}

// Run polls immediately, then every interval, until ctx is cancelled.
func (p *Poller) Run(ctx context.Context) {
	p.cfg.Log.Info("health poller started", "interval", p.cfg.Interval, "retention_days", p.cfg.RetentionDays)
	p.PollOnce(ctx)
	p.prune(ctx)
	ticker := time.NewTicker(p.cfg.Interval)
	pruneTicker := time.NewTicker(pruneInterval)
	defer ticker.Stop()
	defer pruneTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			p.cfg.Log.Info("health poller stopped")
			return
		case <-ticker.C:
			p.PollOnce(ctx)
		case <-pruneTicker.C:
			p.prune(ctx)
		}
	}
}

// PollOnce checks every non-disabled server concurrently.
func (p *Poller) PollOnce(ctx context.Context) {
	servers, err := p.cfg.Servers.List(ctx)
	if err != nil {
		p.cfg.Log.Warn("poller: list servers failed", "error", err)
		return
	}
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(pollConcurrency)
	for _, s := range servers {
		if s.Status == models.ServerStatusDisabled {
			continue // operator paused polling for this server
		}
		s := s
		g.Go(func() error {
			p.pollServer(gctx, s)
			return nil
		})
	}
	_ = g.Wait()
}

// prune drops heartbeat history older than the retention window so the poller
// (which writes a row per server per interval) can't grow the table without
// bound. Negative RetentionDays disables it.
func (p *Poller) prune(ctx context.Context) {
	if p.cfg.RetentionDays < 0 {
		return
	}
	cutoff := time.Now().Add(-time.Duration(p.cfg.RetentionDays) * 24 * time.Hour)
	n, err := p.cfg.Heartbeats.PruneOlderThan(ctx, cutoff)
	if err != nil {
		p.cfg.Log.Warn("poller: prune heartbeats failed", "error", err)
		return
	}
	if n > 0 {
		p.cfg.Log.Info("poller: pruned old heartbeats", "deleted", n, "older_than_days", p.cfg.RetentionDays)
	}
}

func (p *Poller) pollServer(ctx context.Context, s models.Server) {
	secret, err := secrets.OpenSecret(p.cfg.SecretKey, s.TokenSecretEnc, p.cfg.AllowPlaintext)
	if err != nil {
		// Stored secret can't be decrypted here — the credential is unusable.
		p.cfg.Log.Warn("poller: decrypt secret failed", "server", s.Name, "error", err)
		_ = p.cfg.Servers.UpdateStatus(ctx, s.ID, s.Status, models.CredentialInvalid)
		p.record(ctx, s, false, s.Version, map[string]any{"error": "decrypt_failed"})
		return
	}

	result, err := p.probe(ctx, s, secret)
	if err != nil {
		p.cfg.Log.Warn("poller: probe failed", "server", s.Name, "error", err)
		_ = p.cfg.Servers.UpdateStatus(ctx, s.ID, models.ServerStatusUnreachable, models.CredentialUnknown)
		p.record(ctx, s, false, s.Version, map[string]any{"error": "probe_failed"})
		return
	}

	status, cred := statusFromCheck(result)
	_ = p.cfg.Servers.UpdateStatus(ctx, s.ID, status, cred)

	version := s.Version
	if result != nil && result.Version != "" {
		version = result.Version
	}
	healthy := result != nil && result.Reachable && result.CredentialValid
	p.record(ctx, s, healthy, version, result)
}

// probe runs the actual health check (or the test override).
func (p *Poller) probe(ctx context.Context, s models.Server, secret string) (*remote.CheckResult, error) {
	if p.cfg.Probe != nil {
		return p.cfg.Probe(ctx, s)
	}
	client := remote.NewClient(s.BaseURL, s.TokenID, secret, s.InsecureSkipVerify)
	cctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	return client.CheckHealth(cctx)
}

func (p *Poller) record(ctx context.Context, s models.Server, healthy bool, version string, details any) {
	b, _ := json.Marshal(details)
	hb := &models.Heartbeat{
		ID:        ids.NewULID(),
		ServerID:  s.ID,
		Healthy:   healthy,
		Version:   version,
		Details:   b,
		CheckedAt: time.Now().UTC(),
	}
	if err := p.cfg.Heartbeats.Record(ctx, hb); err != nil {
		p.cfg.Log.Warn("poller: record heartbeat failed", "server", s.Name, "error", err)
	}
}

// statusFromCheck maps a health-check result to the persisted status pair. It
// mirrors the on-demand Check handler so the poller and manual checks agree.
func statusFromCheck(r *remote.CheckResult) (models.ServerStatus, models.CredentialStatus) {
	if r == nil || !r.Reachable {
		return models.ServerStatusUnreachable, models.CredentialUnknown
	}
	if r.CredentialValid {
		return models.ServerStatusActive, models.CredentialValid
	}
	return models.ServerStatusActive, models.CredentialInvalid
}

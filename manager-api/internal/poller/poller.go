// Package poller runs a background health loop over enrolled servers so the
// fleet's status is current without an operator clicking Check, and so each
// probe is recorded as a heartbeat — the basis for status history, trends, and
// alerting (roadmap M1).
package poller

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/alert"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/remote"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/secrets"
)

const (
	defaultInterval        = 60 * time.Second
	probeTimeout           = 15 * time.Second
	pollConcurrency        = 8
	defaultRetention       = 14 // days
	pruneInterval          = time.Hour
	defaultCertWarn        = 14 // days
	cpuThreshold           = 80.0
	cpuHighDuration        = 60 * time.Second
	escalateAfterDefault   = 15 * time.Minute
	backupStaleDefault     = 7 // days
	remediationFailDefault = 3
	tokenExpiryDefault     = 7 // days
)

// Config wires the poller's collaborators.
type Config struct {
	Servers        repository.ServerRepository
	Heartbeats     repository.HeartbeatRepository
	MetricSamples  repository.MetricSampleRepository
	Sessions       repository.SessionRepository
	SecretKey      *secrets.Key
	AllowPlaintext bool
	Interval       time.Duration
	// RetentionDays bounds heartbeat history. 0 -> default; negative -> disabled.
	RetentionDays int
	// Notifier receives an alert when a server crosses the healthy boundary.
	// nil disables alerting.
	Notifier alert.Notifier
	// CertWarnDays is the TLS-expiry alert threshold. 0 -> default.
	CertWarnDays int
	Log          *slog.Logger

	// Probe overrides the real panel health check in tests; nil uses the client.
	// It returns both the health result and the raw status (for metrics).
	Probe func(ctx context.Context, s models.Server) (*remote.CheckResult, *remote.ServerStatusResp, error)
	// CertProbe overrides the real TLS cert probe in tests; nil uses the client.
	CertProbe func(baseURL string) (time.Time, error)
	// Notifications receives in-app alerts (SND-18). nil disables them.
	Notifications repository.NotificationRepository
	// AlertRules drives metric thresholds (SND-20). nil -> a built-in CPU rule.
	AlertRules repository.AlertRuleRepository
	// Channels are extra delivery destinations (SND-20). nil -> legacy webhook only.
	Channels repository.AlertChannelRepository
	// Maintenance suppresses alerts during planned windows (SND-22). nil -> never.
	Maintenance repository.MaintenanceRepository
	// Muted silences specific (server, kind) alerts (SND-21). nil -> never.
	Muted repository.MutedAlertRepository
	// EscalateAfter re-notifies unacked incidents (SND-21). 0 -> default.
	EscalateAfter time.Duration
	// Backups tracks backup runs to completion + staleness (SND-27). nil disables.
	Backups repository.BackupRepository
	// APITokens drives expiry reminders (SND-28). nil disables them.
	APITokens repository.APITokenRepository
	// TokenExpiryDays is the reminder lead time. 0 -> default 7.
	TokenExpiryDays int
	// BackupStaleDays flags servers whose last successful backup is older than
	// this (or never). 0 -> default; negative -> disabled.
	BackupStaleDays int
	// Audit records automated remediation actions (SND-29). nil -> not audited.
	Audit repository.AuditRepository
	// Remediation enables automatic restart of a server's web service after
	// RemediationFailures consecutive failed checks (SND-29). Off by default.
	Remediation         bool
	RemediationFailures int    // 0 -> default 3
	RemediationService  string // service name to restart; "" -> default
	// Now overrides the clock in tests; nil uses time.Now.
	Now func() time.Time
}

// Poller periodically checks every non-disabled server and records the outcome.
type Poller struct {
	cfg         Config
	breachMu    sync.Mutex
	breachSince map[string]time.Time // key: serverID|metric
	failMu      sync.Mutex
	failStreak  map[string]int  // consecutive failed checks per server (SND-29)
	restarted   map[string]bool // remediation already attempted this outage
}

// New returns a Poller, applying defaults for interval and logger.
func New(cfg Config) *Poller {
	if cfg.Interval <= 0 {
		cfg.Interval = defaultInterval
	}
	if cfg.RetentionDays == 0 {
		cfg.RetentionDays = defaultRetention
	}
	if cfg.CertWarnDays <= 0 {
		cfg.CertWarnDays = defaultCertWarn
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.EscalateAfter <= 0 {
		cfg.EscalateAfter = escalateAfterDefault
	}
	if cfg.BackupStaleDays == 0 {
		cfg.BackupStaleDays = backupStaleDefault
	}
	if cfg.RemediationFailures <= 0 {
		cfg.RemediationFailures = remediationFailDefault
	}
	if cfg.RemediationService == "" {
		cfg.RemediationService = "web"
	}
	if cfg.TokenExpiryDays <= 0 {
		cfg.TokenExpiryDays = tokenExpiryDefault
	}
	return &Poller{
		cfg:         cfg,
		breachSince: map[string]time.Time{},
		failStreak:  map[string]int{},
		restarted:   map[string]bool{},
	}
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
	p.escalate(ctx)
	p.watchBackups(ctx)
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
	} else if n > 0 {
		p.cfg.Log.Info("poller: pruned old heartbeats", "deleted", n, "older_than_days", p.cfg.RetentionDays)
	}
	if p.cfg.MetricSamples != nil {
		if _, err := p.cfg.MetricSamples.PruneOlderThan(ctx, cutoff); err != nil {
			p.cfg.Log.Warn("poller: prune metric samples failed", "error", err)
		}
	}
	if p.cfg.Sessions != nil {
		if _, err := p.cfg.Sessions.PruneExpired(ctx, time.Now()); err != nil {
			p.cfg.Log.Warn("poller: prune sessions failed", "error", err)
		}
	}
	if p.cfg.Notifications != nil {
		if _, err := p.cfg.Notifications.PruneOlderThan(ctx, cutoff); err != nil {
			p.cfg.Log.Warn("poller: prune notifications failed", "error", err)
		}
	}
	if p.cfg.Maintenance != nil {
		if _, err := p.cfg.Maintenance.PruneExpired(ctx, cutoff); err != nil {
			p.cfg.Log.Warn("poller: prune maintenance windows failed", "error", err)
		}
	}
	if p.cfg.Backups != nil {
		if _, err := p.cfg.Backups.PruneOlderThan(ctx, cutoff); err != nil {
			p.cfg.Log.Warn("poller: prune backup runs failed", "error", err)
		}
	}
	p.checkBackupStale(ctx)
	p.checkTokenExpiry(ctx)
}

// checkTokenExpiry raises a one-time reminder for each API token entering its
// expiry window (SND-28). Deduped per token via the notification's server_id.
func (p *Poller) checkTokenExpiry(ctx context.Context) {
	if p.cfg.APITokens == nil || p.cfg.Notifications == nil {
		return
	}
	now := p.cfg.Now()
	before := now.Add(time.Duration(p.cfg.TokenExpiryDays) * 24 * time.Hour)
	toks, err := p.cfg.APITokens.ListExpiring(ctx, now, before)
	if err != nil {
		p.cfg.Log.Warn("poller: list expiring tokens failed", "error", err)
		return
	}
	for _, t := range toks {
		if exists, _ := p.cfg.Notifications.ActiveExists(ctx, t.ID, "token_expiring"); exists {
			continue
		}
		days := int(t.ExpiresAt.Time.Sub(now).Hours() / 24)
		_ = p.cfg.Notifications.Create(ctx, &models.Notification{
			ID: ids.NewULID(), Kind: "token_expiring", ServerID: t.ID, ServerName: t.Name,
			Metric: "api_token", Severity: models.SeverityWarning,
			Message:   fmt.Sprintf("API token %q expires in %d days", t.Name, days),
			CreatedAt: now.UTC(),
		})
	}
}

// clientFor decrypts a server's token and returns a remote client.
func (p *Poller) clientFor(s models.Server) (*remote.Client, error) {
	secret, err := secrets.OpenSecret(p.cfg.SecretKey, s.TokenSecretEnc, p.cfg.AllowPlaintext)
	if err != nil {
		return nil, err
	}
	return remote.NewClient(s.BaseURL, s.TokenID, secret, s.InsecureSkipVerify), nil
}

// backupTerminal maps a panel operation status to a terminal BackupRun status,
// or "" if the operation is still in progress.
func backupTerminal(status string) string {
	switch strings.ToLower(status) {
	case "succeeded", "success", "completed", "done", "ok":
		return models.BackupSucceeded
	case "failed", "error", "errored", "cancelled", "canceled":
		return models.BackupFailed
	default:
		return ""
	}
}

// watchBackups polls the panel operation status for each non-terminal backup run
// and records the outcome; a fresh success resolves any stale-backup incident.
func (p *Poller) watchBackups(ctx context.Context) {
	if p.cfg.Backups == nil {
		return
	}
	runs, err := p.cfg.Backups.NonTerminal(ctx)
	if err != nil {
		p.cfg.Log.Warn("poller: list non-terminal backups failed", "error", err)
		return
	}
	for _, run := range runs {
		if run.OperationID == "" {
			continue // nothing to poll against
		}
		s, err := p.cfg.Servers.FindByID(ctx, run.ServerID)
		if err != nil {
			continue
		}
		client, err := p.clientFor(*s)
		if err != nil {
			continue
		}
		cctx, cancel := context.WithTimeout(ctx, probeTimeout)
		op, _, err := client.OperationStatus(cctx, run.OperationID)
		cancel()
		if err != nil || op == nil {
			continue // transient; try again next pass
		}
		terminal := backupTerminal(op.Status)
		if terminal == "" {
			_ = p.cfg.Backups.UpdateStatus(ctx, run.ID, models.BackupRunning, op.Message, nil)
			continue
		}
		now := p.cfg.Now().UTC()
		_ = p.cfg.Backups.UpdateStatus(ctx, run.ID, terminal, op.Message, &now)
		if terminal == models.BackupSucceeded && p.cfg.Notifications != nil {
			_ = p.cfg.Notifications.ResolveActive(ctx, run.ServerID, "backup_stale")
		}
	}
}

// checkBackupStale opens a notification for any server whose last successful
// backup is older than the threshold (or never), and resolves it otherwise.
func (p *Poller) checkBackupStale(ctx context.Context) {
	if p.cfg.Backups == nil || p.cfg.Notifications == nil || p.cfg.BackupStaleDays < 0 {
		return
	}
	servers, err := p.cfg.Servers.List(ctx)
	if err != nil {
		return
	}
	now := p.cfg.Now()
	maxAge := time.Duration(p.cfg.BackupStaleDays) * 24 * time.Hour
	for _, s := range servers {
		if s.Status == models.ServerStatusDisabled {
			continue
		}
		last, err := p.cfg.Backups.LatestSuccess(ctx, s.ID)
		if err != nil {
			continue
		}
		stale := last == nil || now.Sub(last.StartedAt) > maxAge
		if !stale {
			_ = p.cfg.Notifications.ResolveActive(ctx, s.ID, "backup_stale")
			continue
		}
		if exists, _ := p.cfg.Notifications.ActiveExists(ctx, s.ID, "backup_stale"); exists {
			continue
		}
		if p.suppressed(ctx, s, now) {
			continue
		}
		msg := fmt.Sprintf("no successful backup in over %d days", p.cfg.BackupStaleDays)
		if last == nil {
			msg = "no successful backup on record"
		}
		_ = p.cfg.Notifications.Create(ctx, &models.Notification{
			ID: ids.NewULID(), Kind: "backup_stale", ServerID: s.ID, ServerName: s.Name,
			Metric: "backup", Severity: models.SeverityWarning, Message: msg, CreatedAt: now.UTC(),
		})
	}
}

// maybeRemediate restarts a server's web service after N consecutive failed
// checks, once per outage, gated by config + maintenance windows and audited
// (SND-29). A healthy check resets the streak.
func (p *Poller) maybeRemediate(ctx context.Context, s models.Server, healthy bool, secret string) {
	if !p.cfg.Remediation {
		return
	}
	p.failMu.Lock()
	if healthy {
		delete(p.failStreak, s.ID)
		delete(p.restarted, s.ID)
		p.failMu.Unlock()
		return
	}
	p.failStreak[s.ID]++
	streak := p.failStreak[s.ID]
	already := p.restarted[s.ID]
	p.failMu.Unlock()

	if streak < p.cfg.RemediationFailures || already || secret == "" {
		return
	}
	if p.suppressed(ctx, s, p.cfg.Now()) {
		return // planned maintenance — do not auto-act
	}

	client := remote.NewClient(s.BaseURL, s.TokenID, secret, s.InsecureSkipVerify)
	cctx, cancel := context.WithTimeout(ctx, probeTimeout)
	_, err := client.RestartService(cctx, p.cfg.RemediationService)
	cancel()

	p.failMu.Lock()
	p.restarted[s.ID] = true // one attempt per outage regardless of outcome
	p.failMu.Unlock()

	if err != nil {
		p.cfg.Log.Warn("poller: auto-restart failed", "server", s.Name, "error", err)
		return
	}
	p.cfg.Log.Info("poller: auto-restarted service", "server", s.Name, "service", p.cfg.RemediationService, "after_failures", streak)
	if p.cfg.Audit != nil {
		_ = p.cfg.Audit.Create(ctx, &models.AuditLog{
			ID: ids.NewULID(), Event: "server.remediation.restart", Actor: "system:remediation",
			ServerID: s.ID, ServerName: s.Name, CreatedAt: p.cfg.Now().UTC(),
		})
	}
	if p.cfg.Notifications != nil {
		_ = p.cfg.Notifications.Create(ctx, &models.Notification{
			ID: ids.NewULID(), Kind: "auto_restart", ServerID: s.ID, ServerName: s.Name,
			Metric: "remediation", Severity: models.SeverityWarning,
			Message:   fmt.Sprintf("auto-restarted %q after %d failed checks", p.cfg.RemediationService, streak),
			CreatedAt: p.cfg.Now().UTC(),
		})
	}
}

func (p *Poller) pollServer(ctx context.Context, s models.Server) {
	var (
		status  models.ServerStatus
		cred    models.CredentialStatus
		healthy bool
		version = s.Version
		details any
		message string
		metrics *remote.ServerStatusResp
	)

	secret, err := secrets.OpenSecret(p.cfg.SecretKey, s.TokenSecretEnc, p.cfg.AllowPlaintext)
	switch {
	case err != nil:
		// Stored secret can't be decrypted here — the credential is unusable.
		p.cfg.Log.Warn("poller: decrypt secret failed", "server", s.Name, "error", err)
		status, cred = s.Status, models.CredentialInvalid
		details = map[string]any{"error": "decrypt_failed"}
		message = "stored token secret cannot be decrypted"
	default:
		result, st, perr := p.probe(ctx, s, secret)
		metrics = st
		if perr != nil {
			p.cfg.Log.Warn("poller: probe failed", "server", s.Name, "error", perr)
			status, cred = models.ServerStatusUnreachable, models.CredentialUnknown
			details = map[string]any{"error": "probe_failed"}
			message = "server did not respond"
		} else {
			status, cred = statusFromCheck(result)
			if result != nil && result.Version != "" {
				version = result.Version
			}
			healthy = result != nil && result.Reachable && result.CredentialValid
			details = result
			message = healthMessage(status, cred)
		}
	}

	_ = p.cfg.Servers.UpdateStatus(ctx, s.ID, status, cred)
	p.record(ctx, s, healthy, version, details)
	p.maybeAlert(ctx, s, status, cred, message)
	p.checkCert(ctx, s)
	p.recordMetrics(ctx, s, metrics)
	p.evaluateServiceRules(ctx, s, metrics, p.cfg.Now())
	p.maybeRemediate(ctx, s, healthy, secret)
}

// serviceBad reports whether a panel-reported service status is a problem worth
// alerting on. "unknown"/"healthy" (and empty) never alert; everything else
// (stopped, failed, degraded, critical, down, inactive) does.
func serviceBad(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "healthy", "ok", "running", "unknown":
		return false
	default:
		return true
	}
}

// evaluateServiceRules opens/resolves one incident per managed-server service
// whose panel-reported health is not healthy, gated by the enabled
// "service_down" alert rule and its sustained duration (SND: service-down
// notifications). Deduped per (server, service); dispatched to alert channels.
func (p *Poller) evaluateServiceRules(ctx context.Context, s models.Server, st *remote.ServerStatusResp, now time.Time) {
	if p.cfg.Notifications == nil || st == nil || len(st.Services) == 0 {
		return
	}
	var rule *models.AlertRule
	for _, r := range p.enabledRules(ctx) {
		if r.Metric == "service_down" {
			rc := r
			rule = &rc
			break
		}
	}
	if rule == nil {
		return // rule disabled — no service alerting
	}

	for _, svc := range st.Services {
		if svc.Name == "" {
			continue
		}
		kind := "service_down:" + svc.Name
		key := s.ID + "|svc:" + svc.Name

		if serviceBad(svc.Status) {
			p.breachMu.Lock()
			since, tracking := p.breachSince[key]
			if !tracking {
				p.breachSince[key] = now
				since = now
			}
			p.breachMu.Unlock()
			if now.Sub(since) < time.Duration(rule.DurationSeconds)*time.Second {
				continue // down, but not yet for the required duration
			}
			if exists, _ := p.cfg.Notifications.ActiveExists(ctx, s.ID, kind); exists {
				continue
			}
			if p.suppressed(ctx, s, now) {
				continue
			}
			if muted, _ := p.isMuted(ctx, s.ID, kind); muted {
				continue
			}
			msg := serviceMessage(svc.Name, svc.Status, svc.Reason)
			_ = p.cfg.Notifications.Create(ctx, &models.Notification{
				ID:         ids.NewULID(),
				Kind:       kind,
				ServerID:   s.ID,
				ServerName: s.Name,
				Metric:     "service_down",
				Severity:   rule.Severity,
				Message:    msg,
				CreatedAt:  now.UTC(),
			})
			p.cfg.Log.Info("poller: service incident", "server", s.Name, "service", svc.Name, "status", svc.Status)
			p.dispatch(ctx, alert.Event{
				Kind:       alert.KindDown,
				ServerID:   s.ID,
				ServerName: s.Name,
				BaseURL:    s.BaseURL,
				Status:     string(s.Status),
				Message:    msg,
				At:         now.UTC(),
			}, rule.Severity, &s, now)
			continue
		}

		// Healthy again: clear tracking and resolve any open incident.
		p.breachMu.Lock()
		_, was := p.breachSince[key]
		delete(p.breachSince, key)
		p.breachMu.Unlock()
		if !was {
			continue
		}
		if exists, _ := p.cfg.Notifications.ActiveExists(ctx, s.ID, kind); exists {
			_ = p.cfg.Notifications.ResolveActive(ctx, s.ID, kind)
			p.dispatch(ctx, alert.Event{
				Kind:       alert.KindRecovered,
				ServerID:   s.ID,
				ServerName: s.Name,
				BaseURL:    s.BaseURL,
				Status:     string(s.Status),
				Message:    svc.Name + " is healthy again",
				At:         now.UTC(),
			}, models.SeverityInfo, &s, now)
		}
	}
}

func serviceMessage(name, status, reason string) string {
	msg := fmt.Sprintf("service %s is %s", name, strings.ToLower(status))
	if reason != "" {
		msg += " (" + reason + ")"
	}
	return msg
}

// recordMetrics stores a compact resource-usage sample when the panel reported
// status this poll (roadmap M1: trends).
func (p *Poller) recordMetrics(ctx context.Context, s models.Server, st *remote.ServerStatusResp) {
	if p.cfg.MetricSamples == nil || st == nil {
		return
	}
	snap := st.Snapshot()
	m := &models.MetricSample{
		ID:          ids.NewULID(),
		ServerID:    s.ID,
		CPUPercent:  snap.CPUPercent,
		RAMPercent:  snap.RAMPercent,
		DiskPercent: snap.DiskPercent,
		Load1:       snap.Load1,
		SampledAt:   time.Now().UTC(),
	}
	if err := p.cfg.MetricSamples.Record(ctx, m); err != nil {
		p.cfg.Log.Warn("poller: record metric sample failed", "server", s.Name, "error", err)
	}
	p.evaluateRules(ctx, s, snap, p.cfg.Now())
}

// evaluateRules checks each enabled alert rule against the latest snapshot. A
// value over threshold sustained for the rule's duration opens one incident
// (deduped, severity-tagged), dispatched to every channel that accepts the
// severity; a drop back to/under threshold resolves it (SND-20/21/22).
func (p *Poller) evaluateRules(ctx context.Context, s models.Server, snap remote.MetricSnapshot, now time.Time) {
	if p.cfg.Notifications == nil {
		return
	}
	for _, rule := range p.enabledRules(ctx) {
		val := metricValue(snap, rule.Metric)
		if val == nil {
			continue // panel did not report this metric
		}
		kind := rule.Metric + "_high"
		key := s.ID + "|" + rule.Metric

		if *val > rule.Threshold {
			p.breachMu.Lock()
			since, tracking := p.breachSince[key]
			if !tracking {
				p.breachSince[key] = now
				since = now
			}
			p.breachMu.Unlock()
			if now.Sub(since) < time.Duration(rule.DurationSeconds)*time.Second {
				continue // breaching, but not yet for the required duration
			}
			if exists, _ := p.cfg.Notifications.ActiveExists(ctx, s.ID, kind); exists {
				continue // one active incident per (server, metric)
			}
			if p.suppressed(ctx, s, now) {
				continue // planned maintenance window
			}
			if muted, _ := p.isMuted(ctx, s.ID, kind); muted {
				continue // operator silenced this alert
			}
			msg := breachMessage(rule.Metric, *val, rule.Threshold)
			_ = p.cfg.Notifications.Create(ctx, &models.Notification{
				ID:         ids.NewULID(),
				Kind:       kind,
				ServerID:   s.ID,
				ServerName: s.Name,
				Metric:     rule.Metric,
				Value:      *val,
				Threshold:  rule.Threshold,
				Severity:   rule.Severity,
				Message:    msg,
				CreatedAt:  now.UTC(),
			})
			p.cfg.Log.Info("poller: threshold incident", "server", s.Name, "metric", rule.Metric, "value", *val)
			p.dispatch(ctx, alert.Event{
				Kind:       alert.KindDown,
				ServerID:   s.ID,
				ServerName: s.Name,
				BaseURL:    s.BaseURL,
				Status:     string(s.Status),
				Message:    msg,
				At:         now.UTC(),
			}, rule.Severity, &s, now)
			continue
		}

		// Recovered: clear tracking and resolve any open incident.
		p.breachMu.Lock()
		_, was := p.breachSince[key]
		delete(p.breachSince, key)
		p.breachMu.Unlock()
		if !was {
			continue
		}
		if exists, _ := p.cfg.Notifications.ActiveExists(ctx, s.ID, kind); exists {
			_ = p.cfg.Notifications.ResolveActive(ctx, s.ID, kind)
			p.dispatch(ctx, alert.Event{
				Kind:       alert.KindRecovered,
				ServerID:   s.ID,
				ServerName: s.Name,
				BaseURL:    s.BaseURL,
				Status:     string(s.Status),
				Message:    recoverMessage(rule.Metric, *val),
				At:         now.UTC(),
			}, models.SeverityInfo, &s, now)
		}
	}
}

// enabledRules returns the configured rules, or a built-in CPU rule when no
// rule repository is wired (keeps SND-18 behaviour and existing tests intact).
func (p *Poller) enabledRules(ctx context.Context) []models.AlertRule {
	if p.cfg.AlertRules != nil {
		if rs, err := p.cfg.AlertRules.ListEnabled(ctx); err == nil {
			return rs
		} else {
			p.cfg.Log.Warn("poller: list alert rules failed", "error", err)
		}
	}
	return []models.AlertRule{{
		Metric: "cpu", Threshold: cpuThreshold, DurationSeconds: int(cpuHighDuration.Seconds()),
		Severity: models.SeverityCritical, Enabled: true,
	}}
}

func metricValue(snap remote.MetricSnapshot, metric string) *float64 {
	switch metric {
	case "cpu":
		return snap.CPUPercent
	case "ram":
		return snap.RAMPercent
	case "disk":
		return snap.DiskPercent
	case "load1":
		return snap.Load1
	}
	return nil
}

func breachMessage(metric string, val, thr float64) string {
	if metric == "load1" {
		return fmt.Sprintf("load1 at %.2f (threshold %.2f)", val, thr)
	}
	return fmt.Sprintf("%s at %.0f%% (threshold %.0f%%)", strings.ToUpper(metric), val, thr)
}

func recoverMessage(metric string, val float64) string {
	if metric == "load1" {
		return fmt.Sprintf("load1 back to %.2f", val)
	}
	return fmt.Sprintf("%s back to %.0f%%", strings.ToUpper(metric), val)
}

func (p *Poller) isMuted(ctx context.Context, serverID, kind string) (bool, error) {
	if p.cfg.Muted == nil {
		return false, nil
	}
	return p.cfg.Muted.IsMuted(ctx, serverID, kind)
}

// suppressed reports whether an active maintenance window covers this server.
func (p *Poller) suppressed(ctx context.Context, s models.Server, now time.Time) bool {
	if p.cfg.Maintenance == nil {
		return false
	}
	active, err := p.cfg.Maintenance.ActiveForServer(ctx, s.ID, s.Environment, now)
	if err != nil {
		p.cfg.Log.Warn("poller: maintenance check failed", "server", s.Name, "error", err)
		return false
	}
	return active
}

// dispatch delivers ev to the legacy webhook plus every enabled channel that
// accepts the severity, unless a maintenance window suppresses it.
func (p *Poller) dispatch(ctx context.Context, ev alert.Event, severity string, s *models.Server, now time.Time) {
	if s != nil && p.suppressed(ctx, *s, now) {
		p.cfg.Log.Debug("poller: alert suppressed by maintenance", "server", s.Name, "kind", ev.Kind)
		return
	}
	var notifiers []alert.Notifier
	if p.cfg.Notifier != nil {
		notifiers = append(notifiers, p.cfg.Notifier)
	}
	notifiers = append(notifiers, p.notifiersFor(ctx, severity)...)
	if len(notifiers) == 0 {
		return
	}
	alert.Dispatch(ctx, p.cfg.Log, notifiers, ev)
}

// notifiersFor builds notifiers for every enabled channel whose min_severity
// admits the given severity. Channel config is sealed; it is opened here.
func (p *Poller) notifiersFor(ctx context.Context, severity string) []alert.Notifier {
	if p.cfg.Channels == nil {
		return nil
	}
	chans, err := p.cfg.Channels.ListEnabled(ctx)
	if err != nil {
		p.cfg.Log.Warn("poller: list channels failed", "error", err)
		return nil
	}
	want := models.SeverityRank(severity)
	var out []alert.Notifier
	for _, ch := range chans {
		if models.SeverityRank(ch.MinSeverity) > want {
			continue
		}
		raw, err := secrets.OpenSecret(p.cfg.SecretKey, ch.ConfigEnc, p.cfg.AllowPlaintext)
		if err != nil {
			p.cfg.Log.Warn("poller: open channel config failed", "channel", ch.Name, "error", err)
			continue
		}
		var m map[string]string
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			p.cfg.Log.Warn("poller: decode channel config failed", "channel", ch.Name, "error", err)
			continue
		}
		n, err := alert.BuildNotifier(ch.Type, m, p.cfg.Log)
		if err != nil {
			p.cfg.Log.Warn("poller: build channel failed", "channel", ch.Name, "error", err)
			continue
		}
		out = append(out, n)
	}
	return out
}

// escalate re-notifies incidents left unacked past EscalateAfter, once (SND-21).
func (p *Poller) escalate(ctx context.Context) {
	if p.cfg.Notifications == nil {
		return
	}
	now := p.cfg.Now()
	before := now.Add(-p.cfg.EscalateAfter)
	stale, err := p.cfg.Notifications.UnackedSince(ctx, before, now)
	if err != nil {
		p.cfg.Log.Warn("poller: escalation query failed", "error", err)
		return
	}
	for _, n := range stale {
		p.dispatch(ctx, alert.Event{
			Kind:       alert.KindDown,
			ServerID:   n.ServerID,
			ServerName: n.ServerName,
			Status:     "unacked",
			Message:    "ESCALATION (unacknowledged): " + n.Message,
			At:         now.UTC(),
		}, models.SeverityCritical, nil, now)
		_ = p.cfg.Notifications.MarkEscalated(ctx, n.ID, now)
		p.cfg.Log.Info("poller: escalated incident", "id", n.ID, "server", n.ServerName)
	}
}

// checkCert samples the panel's TLS certificate expiry (best-effort), stores it,
// and alerts once when it first crosses the warning threshold (M1).
func (p *Poller) checkCert(ctx context.Context, s models.Server) {
	exp, err := p.certExpiry(s.BaseURL)
	if err != nil {
		p.cfg.Log.Debug("poller: cert probe failed", "server", s.Name, "error", err)
		return
	}
	_ = p.cfg.Servers.UpdateCertExpiry(ctx, s.ID, &exp)

	now := time.Now()
	warn := time.Duration(p.cfg.CertWarnDays) * 24 * time.Hour
	newWithin := exp.Sub(now) < warn // includes already-expired
	priorWithin := s.CertExpiresAt != nil && s.CertExpiresAt.Sub(now) < warn
	if !newWithin || priorWithin {
		return // not newly within the threshold
	}
	days := int(exp.Sub(now).Hours() / 24)
	msg := "expires in " + strconv.Itoa(days) + " days"
	if days < 0 {
		msg = "has expired"
	}
	p.dispatch(ctx, alert.Event{
		Kind:       alert.KindCertExpiring,
		ServerID:   s.ID,
		ServerName: s.Name,
		BaseURL:    s.BaseURL,
		Status:     string(s.Status),
		Message:    msg,
		At:         now.UTC(),
	}, models.SeverityWarning, &s, now)
	p.cfg.Log.Info("poller: cert alert sent", "server", s.Name, "days", days)
}

func (p *Poller) certExpiry(baseURL string) (time.Time, error) {
	if p.cfg.CertProbe != nil {
		return p.cfg.CertProbe(baseURL)
	}
	return remote.PeerCertNotAfter(baseURL)
}

// maybeAlert fires a notification when a server crosses the healthy boundary:
// healthy -> unhealthy (down) or a known-bad state -> healthy (recovered). It
// stays quiet on transient unknown states and when no notifier is configured, so
// it does not spam on every poll.
func (p *Poller) maybeAlert(ctx context.Context, s models.Server, status models.ServerStatus, cred models.CredentialStatus, message string) {
	if p.cfg.Notifier == nil && p.cfg.Channels == nil {
		return
	}
	priorHealthy := s.Status == models.ServerStatusActive && s.CredentialStatus == models.CredentialValid
	newHealthy := status == models.ServerStatusActive && cred == models.CredentialValid
	priorKnownBad := s.Status == models.ServerStatusUnreachable || s.CredentialStatus == models.CredentialInvalid

	var kind string
	switch {
	case priorHealthy && !newHealthy:
		kind = alert.KindDown
	case priorKnownBad && newHealthy:
		kind = alert.KindRecovered
	default:
		return
	}

	now := time.Now()
	severity := models.SeverityCritical
	if kind == alert.KindRecovered {
		severity = models.SeverityInfo
	}
	p.dispatch(ctx, alert.Event{
		Kind:             kind,
		ServerID:         s.ID,
		ServerName:       s.Name,
		BaseURL:          s.BaseURL,
		Status:           string(status),
		CredentialStatus: string(cred),
		Message:          message,
		At:               now.UTC(),
	}, severity, &s, now)
	p.cfg.Log.Info("poller: alert sent", "server", s.Name, "kind", kind)
}

func healthMessage(status models.ServerStatus, cred models.CredentialStatus) string {
	if status != models.ServerStatusActive {
		return "server unreachable"
	}
	if cred != models.CredentialValid {
		return "automation credential invalid"
	}
	return "healthy"
}

// probe runs the actual health check + metrics fetch (or the test override).
func (p *Poller) probe(ctx context.Context, s models.Server, secret string) (*remote.CheckResult, *remote.ServerStatusResp, error) {
	if p.cfg.Probe != nil {
		return p.cfg.Probe(ctx, s)
	}
	client := remote.NewClient(s.BaseURL, s.TokenID, secret, s.InsecureSkipVerify)
	cctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	return client.CheckWithMetrics(cctx)
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

// Package report generates periodic fleet-summary reports and delivers them to a
// webhook (roadmap M4). Reuses the same JSON+text shape as alerts so Slack/
// Discord/Mattermost incoming webhooks render them.
package report

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

const (
	defaultInterval = 24 * time.Hour
	deliverTimeout  = 10 * time.Second
)

// Config wires the reporter.
type Config struct {
	Servers    repository.ServerRepository
	Interval   time.Duration
	WebhookURL string
	Log        *slog.Logger
}

// Summary is the fleet snapshot delivered in a report.
type Summary struct {
	GeneratedAt       time.Time      `json:"generated_at"`
	Total             int            `json:"total"`
	Active            int            `json:"active"`
	Unreachable       int            `json:"unreachable"`
	Disabled          int            `json:"disabled"`
	CredentialInvalid int            `json:"credential_invalid"`
	ByEnvironment     map[string]int `json:"by_environment"`
	ByVersion         map[string]int `json:"by_version"`
}

// Reporter periodically summarizes the fleet and posts it to a webhook.
type Reporter struct{ cfg Config }

// New returns a Reporter with defaults applied.
func New(cfg Config) *Reporter {
	if cfg.Interval <= 0 {
		cfg.Interval = defaultInterval
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	return &Reporter{cfg: cfg}
}

// Run delivers a report every interval until ctx is cancelled. It does NOT fire
// on start, to avoid a report every restart.
func (r *Reporter) Run(ctx context.Context) {
	r.cfg.Log.Info("fleet reporter started", "interval", r.cfg.Interval)
	ticker := time.NewTicker(r.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			r.cfg.Log.Info("fleet reporter stopped")
			return
		case <-ticker.C:
			if err := r.Once(ctx); err != nil {
				r.cfg.Log.Warn("fleet report delivery failed", "error", err)
			}
		}
	}
}

// Once generates and delivers a single report.
func (r *Reporter) Once(ctx context.Context) error {
	servers, err := r.cfg.Servers.List(ctx)
	if err != nil {
		return fmt.Errorf("report list servers: %w", err)
	}
	return r.deliver(ctx, Summarize(servers, time.Now().UTC()))
}

// Summarize builds a fleet summary from the enrolled servers.
func Summarize(servers []models.Server, now time.Time) Summary {
	sum := Summary{
		GeneratedAt:   now,
		Total:         len(servers),
		ByEnvironment: map[string]int{},
		ByVersion:     map[string]int{},
	}
	for _, s := range servers {
		switch s.Status {
		case models.ServerStatusActive:
			sum.Active++
		case models.ServerStatusUnreachable:
			sum.Unreachable++
		case models.ServerStatusDisabled:
			sum.Disabled++
		}
		if s.CredentialStatus == models.CredentialInvalid {
			sum.CredentialInvalid++
		}
		env := s.Environment
		if env == "" {
			env = "unassigned"
		}
		sum.ByEnvironment[env]++
		v := s.Version
		if v == "" {
			v = "unknown"
		}
		sum.ByVersion[v]++
	}
	return sum
}

// Text renders a human-readable summary for a webhook "text" field.
func (s Summary) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "📊 Jabali Sounder fleet report — %d servers\n", s.Total)
	fmt.Fprintf(&b, "active %d · unreachable %d · disabled %d · invalid-cred %d\n",
		s.Active, s.Unreachable, s.Disabled, s.CredentialInvalid)
	if len(s.ByEnvironment) > 0 {
		fmt.Fprintf(&b, "by env: %s\n", joinCounts(s.ByEnvironment))
	}
	if len(s.ByVersion) > 1 {
		fmt.Fprintf(&b, "versions: %s", joinCounts(s.ByVersion))
	}
	return b.String()
}

func joinCounts(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return m[keys[i]] > m[keys[j]] })
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s %d", k, m[k]))
	}
	return strings.Join(parts, ", ")
}

func (r *Reporter) deliver(ctx context.Context, sum Summary) error {
	payload := struct {
		Summary
		Text string `json:"text"`
	}{Summary: sum, Text: sum.Text()}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("report marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.cfg.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("report request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: deliverTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("report deliver: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("report webhook returned HTTP %d", resp.StatusCode)
	}
	r.cfg.Log.Info("fleet report delivered", "total", sum.Total)
	return nil
}

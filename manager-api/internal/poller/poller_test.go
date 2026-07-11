package poller

import (
	"context"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/alert"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/remote"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

func testRepos(t *testing.T) (repository.ServerRepository, repository.HeartbeatRepository, repository.MetricSampleRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "poll.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	gormDB, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return repository.NewServerRepository(gormDB), repository.NewHeartbeatRepository(gormDB), repository.NewMetricSampleRepository(gormDB)
}

func seed(t *testing.T, repo repository.ServerRepository, status models.ServerStatus) *models.Server {
	t.Helper()
	s := &models.Server{
		ID:               ids.NewULID(),
		Name:             "srv",
		BaseURL:          "https://panel.example:8443",
		TokenID:          "TID",
		TokenSecretEnc:   []byte(hex.EncodeToString([]byte("secret"))),
		Scopes:           models.JSONStringArray{"read:*"},
		Status:           status,
		CredentialStatus: models.CredentialUnknown,
	}
	if err := repo.Create(context.Background(), s); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return s
}

func newPoller(sr repository.ServerRepository, hr repository.HeartbeatRepository, probe func(context.Context, models.Server) (*remote.CheckResult, *remote.ServerStatusResp, error)) *Poller {
	return New(Config{
		Servers:        sr,
		Heartbeats:     hr,
		AllowPlaintext: true, // no key -> hex fallback in tests
		Log:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		Probe:          probe,
	})
}

// TestPollUpdatesStatusAndRecordsHeartbeat: a healthy probe flips status to
// active/valid and records a healthy heartbeat.
func TestPollUpdatesStatusAndRecordsHeartbeat(t *testing.T) {
	sr, hr, _ := testRepos(t)
	s := seed(t, sr, models.ServerStatusUnreachable)

	p := newPoller(sr, hr, func(context.Context, models.Server) (*remote.CheckResult, *remote.ServerStatusResp, error) {
		return &remote.CheckResult{Reachable: true, CredentialValid: true, Version: "v9"}, nil, nil
	})
	p.PollOnce(context.Background())

	got, _ := sr.FindByID(context.Background(), s.ID)
	if got.Status != models.ServerStatusActive || got.CredentialStatus != models.CredentialValid {
		t.Fatalf("status=%s cred=%s, want active/valid", got.Status, got.CredentialStatus)
	}
	hb, err := hr.Latest(context.Background(), s.ID)
	if err != nil || hb == nil {
		t.Fatalf("no heartbeat recorded: %v", err)
	}
	if !hb.Healthy || hb.Version != "v9" {
		t.Fatalf("heartbeat healthy=%v version=%q, want true/v9", hb.Healthy, hb.Version)
	}
}

// TestPollMarksUnreachable: an unreachable result persists unreachable/unknown
// and an unhealthy heartbeat.
func TestPollMarksUnreachable(t *testing.T) {
	sr, hr, _ := testRepos(t)
	s := seed(t, sr, models.ServerStatusActive)

	p := newPoller(sr, hr, func(context.Context, models.Server) (*remote.CheckResult, *remote.ServerStatusResp, error) {
		return &remote.CheckResult{Reachable: false}, nil, nil
	})
	p.PollOnce(context.Background())

	got, _ := sr.FindByID(context.Background(), s.ID)
	if got.Status != models.ServerStatusUnreachable || got.CredentialStatus != models.CredentialUnknown {
		t.Fatalf("status=%s cred=%s, want unreachable/unknown", got.Status, got.CredentialStatus)
	}
	hb, _ := hr.Latest(context.Background(), s.ID)
	if hb == nil || hb.Healthy {
		t.Fatalf("expected unhealthy heartbeat")
	}
}

// TestPollSkipsDisabled: a disabled server is not probed and records nothing.
func TestPollSkipsDisabled(t *testing.T) {
	sr, hr, _ := testRepos(t)
	s := seed(t, sr, models.ServerStatusDisabled)

	var probed bool
	p := newPoller(sr, hr, func(context.Context, models.Server) (*remote.CheckResult, *remote.ServerStatusResp, error) {
		probed = true
		return &remote.CheckResult{Reachable: true, CredentialValid: true}, nil, nil
	})
	p.PollOnce(context.Background())

	if probed {
		t.Fatal("disabled server should not be probed")
	}
	if hb, _ := hr.Latest(context.Background(), s.ID); hb != nil {
		t.Fatal("disabled server should record no heartbeat")
	}
}

// TestPruneRetention drops heartbeats older than the retention window.
func TestPruneRetention(t *testing.T) {
	sr, hr, _ := testRepos(t)
	s := seed(t, sr, models.ServerStatusActive)

	old := &models.Heartbeat{ID: ids.NewULID(), ServerID: s.ID, CheckedAt: time.Now().Add(-48 * time.Hour)}
	fresh := &models.Heartbeat{ID: ids.NewULID(), ServerID: s.ID, CheckedAt: time.Now()}
	if err := hr.Record(context.Background(), old); err != nil {
		t.Fatalf("record old: %v", err)
	}
	if err := hr.Record(context.Background(), fresh); err != nil {
		t.Fatalf("record fresh: %v", err)
	}

	p := newPoller(sr, hr, nil)
	p.cfg.RetentionDays = 1 // keep last 24h
	p.prune(context.Background())

	rows, err := hr.Recent(context.Background(), s.ID, 10)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("after prune want 1 heartbeat, got %d", len(rows))
	}
}

type fakeNotifier struct{ events []alert.Event }

func (f *fakeNotifier) Notify(_ context.Context, ev alert.Event) error {
	f.events = append(f.events, ev)
	return nil
}

// TestAlertOnDownTransition: a healthy server going unreachable fires one "down".
func TestAlertOnDownTransition(t *testing.T) {
	sr, hr, _ := testRepos(t)
	s := seed(t, sr, models.ServerStatusActive)
	// seed() sets credential unknown; make it valid so prior is "healthy".
	_ = sr.UpdateStatus(context.Background(), s.ID, models.ServerStatusActive, models.CredentialValid)

	fn := &fakeNotifier{}
	p := New(Config{
		Servers: sr, Heartbeats: hr, AllowPlaintext: true, Notifier: fn,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Probe: func(context.Context, models.Server) (*remote.CheckResult, *remote.ServerStatusResp, error) {
			return &remote.CheckResult{Reachable: false}, nil, nil
		},
	})
	p.PollOnce(context.Background())

	if len(fn.events) != 1 || fn.events[0].Kind != alert.KindDown {
		t.Fatalf("want 1 down alert, got %+v", fn.events)
	}
}

// TestAlertOnRecovery: a known-bad server becoming healthy fires one "recovered".
func TestAlertOnRecovery(t *testing.T) {
	sr, hr, _ := testRepos(t)
	s := seed(t, sr, models.ServerStatusUnreachable) // priorKnownBad

	fn := &fakeNotifier{}
	p := New(Config{
		Servers: sr, Heartbeats: hr, AllowPlaintext: true, Notifier: fn,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Probe: func(context.Context, models.Server) (*remote.CheckResult, *remote.ServerStatusResp, error) {
			return &remote.CheckResult{Reachable: true, CredentialValid: true}, nil, nil
		},
	})
	p.PollOnce(context.Background())
	_ = s

	if len(fn.events) != 1 || fn.events[0].Kind != alert.KindRecovered {
		t.Fatalf("want 1 recovered alert, got %+v", fn.events)
	}
}

// TestNoAlertWhenStable: healthy staying healthy fires nothing.
func TestNoAlertWhenStable(t *testing.T) {
	sr, hr, _ := testRepos(t)
	s := seed(t, sr, models.ServerStatusActive)
	_ = sr.UpdateStatus(context.Background(), s.ID, models.ServerStatusActive, models.CredentialValid)

	fn := &fakeNotifier{}
	p := New(Config{
		Servers: sr, Heartbeats: hr, AllowPlaintext: true, Notifier: fn,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Probe: func(context.Context, models.Server) (*remote.CheckResult, *remote.ServerStatusResp, error) {
			return &remote.CheckResult{Reachable: true, CredentialValid: true}, nil, nil
		},
	})
	p.PollOnce(context.Background())

	if len(fn.events) != 0 {
		t.Fatalf("want no alerts when stable, got %+v", fn.events)
	}
}

// TestPollStoresCertExpiry: the poller samples and stores the panel cert expiry.
func TestPollStoresCertExpiry(t *testing.T) {
	sr, hr, _ := testRepos(t)
	s := seed(t, sr, models.ServerStatusActive)
	exp := time.Now().Add(90 * 24 * time.Hour)

	p := New(Config{
		Servers: sr, Heartbeats: hr, AllowPlaintext: true,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Probe: func(context.Context, models.Server) (*remote.CheckResult, *remote.ServerStatusResp, error) {
			return &remote.CheckResult{Reachable: true, CredentialValid: true}, nil, nil
		},
		CertProbe: func(string) (time.Time, error) { return exp, nil },
	})
	p.PollOnce(context.Background())

	got, _ := sr.FindByID(context.Background(), s.ID)
	if got.CertExpiresAt == nil || got.CertExpiresAt.Unix() != exp.Unix() {
		t.Fatalf("cert expiry not stored: %v", got.CertExpiresAt)
	}
}

// TestCertExpiringAlert: a cert within the warning window fires one cert alert.
func TestCertExpiringAlert(t *testing.T) {
	sr, hr, _ := testRepos(t)
	seed(t, sr, models.ServerStatusActive)
	soon := time.Now().Add(3 * 24 * time.Hour) // within default 14d

	fn := &fakeNotifier{}
	p := New(Config{
		Servers: sr, Heartbeats: hr, AllowPlaintext: true, Notifier: fn,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Probe: func(context.Context, models.Server) (*remote.CheckResult, *remote.ServerStatusResp, error) {
			return &remote.CheckResult{Reachable: true, CredentialValid: true}, nil, nil
		},
		CertProbe: func(string) (time.Time, error) { return soon, nil },
	})
	p.PollOnce(context.Background())

	var certAlerts int
	for _, e := range fn.events {
		if e.Kind == alert.KindCertExpiring {
			certAlerts++
		}
	}
	if certAlerts != 1 {
		t.Fatalf("want 1 cert-expiring alert, got %d (%+v)", certAlerts, fn.events)
	}
}

// TestPollRecordsMetricSample: a reachable poll stores a resource-usage sample.
func TestPollRecordsMetricSample(t *testing.T) {
	sr, hr, mr := testRepos(t)
	s := seed(t, sr, models.ServerStatusActive)

	cpu := 42.0
	p := New(Config{
		Servers: sr, Heartbeats: hr, MetricSamples: mr, AllowPlaintext: true,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Probe: func(context.Context, models.Server) (*remote.CheckResult, *remote.ServerStatusResp, error) {
			return &remote.CheckResult{Reachable: true, CredentialValid: true},
				&remote.ServerStatusResp{CPU: &remote.CPUStatusSlice{UsagePercent: cpu}},
				nil
		},
	})
	p.PollOnce(context.Background())

	rows, err := mr.Recent(context.Background(), s.ID, 10)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(rows) != 1 || rows[0].CPUPercent == nil || *rows[0].CPUPercent != cpu {
		t.Fatalf("metric sample not stored correctly: %+v", rows)
	}
}

// notifRepos returns repos including a notification repo sharing one DB.
func notifRepos(t *testing.T) (repository.ServerRepository, repository.HeartbeatRepository, repository.MetricSampleRepository, repository.NotificationRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "poll.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	gormDB, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return repository.NewServerRepository(gormDB), repository.NewHeartbeatRepository(gormDB),
		repository.NewMetricSampleRepository(gormDB), repository.NewNotificationRepository(gormDB)
}

func cpuPoller(sr repository.ServerRepository, hr repository.HeartbeatRepository, mr repository.MetricSampleRepository, nr repository.NotificationRepository, cpu *float64, now *time.Time) *Poller {
	return New(Config{
		Servers: sr, Heartbeats: hr, MetricSamples: mr, Notifications: nr, AllowPlaintext: true,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: func() time.Time { return *now },
		Probe: func(context.Context, models.Server) (*remote.CheckResult, *remote.ServerStatusResp, error) {
			return &remote.CheckResult{Reachable: true, CredentialValid: true},
				&remote.ServerStatusResp{CPU: &remote.CPUStatusSlice{UsagePercent: *cpu}}, nil
		},
	})
}

// TestCPUNotificationSustained: CPU stays >80% for >=60s -> one notification.
func TestCPUNotificationSustained(t *testing.T) {
	sr, hr, mr, nr := notifRepos(t)
	seed(t, sr, models.ServerStatusActive)
	cpu := 95.0
	now := time.Unix(1_700_000_000, 0)
	p := cpuPoller(sr, hr, mr, nr, &cpu, &now)
	ctx := context.Background()

	p.PollOnce(ctx) // t=0: incident starts, not yet sustained
	if n, _ := nr.UnreadCount(ctx); n != 0 {
		t.Fatalf("premature notification at t=0: %d", n)
	}
	now = now.Add(61 * time.Second)
	p.PollOnce(ctx) // t=61s: sustained -> notify
	if n, _ := nr.UnreadCount(ctx); n != 1 {
		t.Fatalf("want 1 notification after 61s, got %d", n)
	}
}

// TestCPUNotificationTransientSpike: a spike that clears before 60s never fires.
func TestCPUNotificationTransientSpike(t *testing.T) {
	sr, hr, mr, nr := notifRepos(t)
	seed(t, sr, models.ServerStatusActive)
	cpu := 95.0
	now := time.Unix(1_700_000_000, 0)
	p := cpuPoller(sr, hr, mr, nr, &cpu, &now)
	ctx := context.Background()

	p.PollOnce(ctx) // start
	now = now.Add(30 * time.Second)
	cpu = 20.0 // recovered before threshold
	p.PollOnce(ctx)
	if n, _ := nr.UnreadCount(ctx); n != 0 {
		t.Fatalf("transient spike should not notify, got %d", n)
	}
}

// TestCPUNotificationDedup: sustained high across many polls -> still one active.
func TestCPUNotificationDedup(t *testing.T) {
	sr, hr, mr, nr := notifRepos(t)
	seed(t, sr, models.ServerStatusActive)
	cpu := 95.0
	now := time.Unix(1_700_000_000, 0)
	p := cpuPoller(sr, hr, mr, nr, &cpu, &now)
	ctx := context.Background()

	p.PollOnce(ctx)
	for i := 0; i < 5; i++ {
		now = now.Add(70 * time.Second)
		p.PollOnce(ctx)
	}
	rows, _ := nr.ListRecent(ctx, 50)
	if len(rows) != 1 {
		t.Fatalf("dedup failed: want 1 notification, got %d", len(rows))
	}
}

// TestCPUNotificationRecoveryAndReincident: recover resolves the incident, and a
// fresh sustained high raises a new one.
func TestCPUNotificationRecoveryAndReincident(t *testing.T) {
	sr, hr, mr, nr := notifRepos(t)
	seed(t, sr, models.ServerStatusActive)
	cpu := 95.0
	now := time.Unix(1_700_000_000, 0)
	p := cpuPoller(sr, hr, mr, nr, &cpu, &now)
	ctx := context.Background()

	p.PollOnce(ctx)
	now = now.Add(61 * time.Second)
	p.PollOnce(ctx) // incident 1
	if exists, _ := nr.ActiveExists(ctx, "", "cpu_high"); !exists {
		// serverID unknown here; check via list instead
	}

	now = now.Add(30 * time.Second)
	cpu = 10.0
	p.PollOnce(ctx) // recovery -> resolve
	rows, _ := nr.ListRecent(ctx, 50)
	if len(rows) != 1 || !rows[0].ResolvedAt.Valid {
		t.Fatalf("recovery should resolve incident 1: %+v", rows)
	}

	// New sustained high -> incident 2.
	cpu = 90.0
	now = now.Add(30 * time.Second)
	p.PollOnce(ctx) // start incident 2
	now = now.Add(61 * time.Second)
	p.PollOnce(ctx) // sustained -> notify
	rows, _ = nr.ListRecent(ctx, 50)
	if len(rows) != 2 {
		t.Fatalf("want 2 notifications after re-incident, got %d", len(rows))
	}
}

// m5Repos returns the full repo set used by the M5 alerting engine.
func m5Repos(t *testing.T) (repository.ServerRepository, repository.HeartbeatRepository, repository.MetricSampleRepository, repository.NotificationRepository, repository.AlertRuleRepository, repository.MaintenanceRepository, repository.MutedAlertRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "poll.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	gormDB, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return repository.NewServerRepository(gormDB), repository.NewHeartbeatRepository(gormDB),
		repository.NewMetricSampleRepository(gormDB), repository.NewNotificationRepository(gormDB),
		repository.NewAlertRuleRepository(gormDB), repository.NewMaintenanceRepository(gormDB),
		repository.NewMutedAlertRepository(gormDB)
}

// TestRuleDrivenThreshold: a lowered CPU rule with zero duration fires on the
// first breaching poll.
func TestRuleDrivenThreshold(t *testing.T) {
	sr, hr, mr, nr, rr, _, _ := m5Repos(t)
	seed(t, sr, models.ServerStatusActive)
	ctx := context.Background()
	if err := rr.EnsureDefaults(ctx, time.Unix(1_700_000_000, 0)); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if err := rr.Update(ctx, &models.AlertRule{Metric: "cpu", Threshold: 50, DurationSeconds: 0, Severity: models.SeverityWarning, Enabled: true}); err != nil {
		t.Fatalf("update rule: %v", err)
	}
	cpu := 60.0
	now := time.Unix(1_700_000_000, 0)
	p := New(Config{
		Servers: sr, Heartbeats: hr, MetricSamples: mr, Notifications: nr, AlertRules: rr,
		AllowPlaintext: true, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: func() time.Time { return now },
		Probe: func(context.Context, models.Server) (*remote.CheckResult, *remote.ServerStatusResp, error) {
			return &remote.CheckResult{Reachable: true, CredentialValid: true},
				&remote.ServerStatusResp{CPU: &remote.CPUStatusSlice{UsagePercent: cpu}}, nil
		},
	})
	p.PollOnce(ctx)
	if n, _ := nr.UnreadCount(ctx); n != 1 {
		t.Fatalf("zero-duration rule should fire on first poll, got %d", n)
	}
	rows, _ := nr.ListRecent(ctx, 10)
	if rows[0].Severity != models.SeverityWarning {
		t.Fatalf("severity = %q", rows[0].Severity)
	}
}

// TestMaintenanceSuppression: an active global window blocks incident creation.
func TestMaintenanceSuppression(t *testing.T) {
	sr, hr, mr, nr, _, mw, _ := m5Repos(t)
	seed(t, sr, models.ServerStatusActive)
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0)
	if err := mw.Create(ctx, &models.MaintenanceWindow{
		ID: ids.NewULID(), ScopeType: "global",
		StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour), CreatedAt: now,
	}); err != nil {
		t.Fatalf("create window: %v", err)
	}
	cpu := 95.0
	cur := now
	p := New(Config{
		Servers: sr, Heartbeats: hr, MetricSamples: mr, Notifications: nr, Maintenance: mw,
		AllowPlaintext: true, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: func() time.Time { return cur },
		Probe: func(context.Context, models.Server) (*remote.CheckResult, *remote.ServerStatusResp, error) {
			return &remote.CheckResult{Reachable: true, CredentialValid: true},
				&remote.ServerStatusResp{CPU: &remote.CPUStatusSlice{UsagePercent: cpu}}, nil
		},
	})
	p.PollOnce(ctx)
	cur = cur.Add(61 * time.Second)
	p.PollOnce(ctx)
	if n, _ := nr.UnreadCount(ctx); n != 0 {
		t.Fatalf("maintenance should suppress incidents, got %d", n)
	}
}

// TestMuteSuppression: a muted (server, kind) blocks incident creation.
func TestMuteSuppression(t *testing.T) {
	sr, hr, mr, nr, _, _, muted := m5Repos(t)
	s := seed(t, sr, models.ServerStatusActive)
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0)
	if err := muted.Mute(ctx, s.ID, "cpu_high", "tester", now); err != nil {
		t.Fatalf("mute: %v", err)
	}
	cpu := 95.0
	cur := now
	p := New(Config{
		Servers: sr, Heartbeats: hr, MetricSamples: mr, Notifications: nr, Muted: muted,
		AllowPlaintext: true, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: func() time.Time { return cur },
		Probe: func(context.Context, models.Server) (*remote.CheckResult, *remote.ServerStatusResp, error) {
			return &remote.CheckResult{Reachable: true, CredentialValid: true},
				&remote.ServerStatusResp{CPU: &remote.CPUStatusSlice{UsagePercent: cpu}}, nil
		},
	})
	p.PollOnce(ctx)
	cur = cur.Add(61 * time.Second)
	p.PollOnce(ctx)
	if n, _ := nr.UnreadCount(ctx); n != 0 {
		t.Fatalf("mute should suppress incidents, got %d", n)
	}
}

// TestEscalation: an unacked incident older than EscalateAfter is re-notified
// once and marked escalated.
func TestEscalation(t *testing.T) {
	sr, hr, mr, nr, _, _, _ := m5Repos(t)
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0)
	if err := nr.Create(ctx, &models.Notification{
		ID: ids.NewULID(), Kind: "cpu_high", ServerID: "S1", ServerName: "srv",
		Metric: "cpu", Value: 95, Threshold: 80, Severity: models.SeverityCritical,
		Message: "CPU at 95% (threshold 80%)", CreatedAt: now.Add(-time.Hour),
	}); err != nil {
		t.Fatalf("seed incident: %v", err)
	}
	fn := &fakeNotifier{}
	p := New(Config{
		Servers: sr, Heartbeats: hr, MetricSamples: mr, Notifications: nr, Notifier: fn,
		EscalateAfter:  15 * time.Minute,
		AllowPlaintext: true, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: func() time.Time { return now },
	})
	p.escalate(ctx)
	if len(fn.events) != 1 {
		t.Fatalf("want 1 escalation event, got %d", len(fn.events))
	}
	if !strings.Contains(fn.events[0].Message, "ESCALATION") {
		t.Fatalf("event message = %q", fn.events[0].Message)
	}
	// Second sweep must not re-escalate (escalated_at now set).
	p.escalate(ctx)
	if len(fn.events) != 1 {
		t.Fatalf("re-escalated: %d events", len(fn.events))
	}
}

// m7Repos returns a fuller repo set for M7 features over one sqlite DB.
func m7Repos(t *testing.T) (*gorm.DB, repository.ServerRepository, repository.HeartbeatRepository, repository.NotificationRepository, repository.BackupRepository, repository.AuditRepository, repository.APITokenRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "m7.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	g, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return g, repository.NewServerRepository(g), repository.NewHeartbeatRepository(g),
		repository.NewNotificationRepository(g), repository.NewBackupRepository(g),
		repository.NewAuditRepository(g), repository.NewAPITokenRepository(g)
}

// seedPanel makes a server whose BaseURL points at a permissive httptest panel.
func seedPanel(t *testing.T, repo repository.ServerRepository, baseURL string, status models.ServerStatus) *models.Server {
	t.Helper()
	s := &models.Server{
		ID:               ids.NewULID(),
		Name:             "panel",
		BaseURL:          baseURL,
		TokenID:          "TID",
		TokenSecretEnc:   []byte(hex.EncodeToString([]byte("secret"))), // AllowPlaintext hex fallback
		Scopes:           models.JSONStringArray{"read:*"},
		Status:           status,
		CredentialStatus: models.CredentialValid,
	}
	if err := repo.Create(context.Background(), s); err != nil {
		t.Fatalf("seed panel: %v", err)
	}
	return s
}

// TestAutoRestartRemediation: after N consecutive failed checks the poller
// restarts the service exactly once, audited, respecting the streak reset.
func TestAutoRestartRemediation(t *testing.T) {
	var restarts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/services/") && strings.HasSuffix(r.URL.Path, "/restart") {
			atomic.AddInt32(&restarts, 1)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	_, sr, hr, nr, _, ar, _ := m7Repos(t)
	seedPanel(t, sr, srv.URL, models.ServerStatusActive)
	ctx := context.Background()

	p := New(Config{
		Servers: sr, Heartbeats: hr, Notifications: nr, Audit: ar,
		AllowPlaintext: true, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Remediation: true, RemediationFailures: 2, RemediationService: "web",
		Probe: func(context.Context, models.Server) (*remote.CheckResult, *remote.ServerStatusResp, error) {
			return &remote.CheckResult{Reachable: false, CredentialValid: false}, nil, nil
		},
	})
	p.PollOnce(ctx) // streak 1 -> no action
	if atomic.LoadInt32(&restarts) != 0 {
		t.Fatalf("premature restart")
	}
	p.PollOnce(ctx) // streak 2 -> restart once
	p.PollOnce(ctx) // still failing -> must NOT restart again
	if got := atomic.LoadInt32(&restarts); got != 1 {
		t.Fatalf("want exactly 1 restart, got %d", got)
	}
	logs, _ := ar.List(ctx, repository.AuditFilter{})
	if len(logs) != 1 || logs[0].Event != "server.remediation.restart" {
		t.Fatalf("remediation not audited: %+v", logs)
	}
}

// TestRemediationDisabledByDefault: without Remediation the poller never acts.
func TestRemediationDisabledByDefault(t *testing.T) {
	var restarts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&restarts, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	_, sr, hr, nr, _, _, _ := m7Repos(t)
	seedPanel(t, sr, srv.URL, models.ServerStatusActive)
	p := New(Config{
		Servers: sr, Heartbeats: hr, Notifications: nr, AllowPlaintext: true,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Probe: func(context.Context, models.Server) (*remote.CheckResult, *remote.ServerStatusResp, error) {
			return &remote.CheckResult{Reachable: false}, nil, nil
		},
	})
	for i := 0; i < 5; i++ {
		p.PollOnce(context.Background())
	}
	if atomic.LoadInt32(&restarts) != 0 {
		t.Fatalf("remediation ran while disabled: %d", restarts)
	}
}

// TestBackupWatcherResolvesToTerminal: a pending run polls to succeeded.
func TestBackupWatcherReachesTerminal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/operations/") {
			_, _ = w.Write([]byte(`{"status":"succeeded","message":"done"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, sr, hr, nr, br, _, _ := m7Repos(t)
	s := seedPanel(t, sr, srv.URL, models.ServerStatusActive)
	ctx := context.Background()
	_ = br.Create(ctx, &models.BackupRun{
		ID: ids.NewULID(), ServerID: s.ID, ServerName: s.Name, OperationID: "op-1",
		Status: models.BackupPending, StartedAt: time.Now(),
	})
	p := New(Config{
		Servers: sr, Heartbeats: hr, Notifications: nr, Backups: br, AllowPlaintext: true,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: func() time.Time { return time.Unix(1_700_000_000, 0) },
	})
	p.watchBackups(ctx)
	runs, _ := br.ListRecent(ctx, 10)
	if len(runs) != 1 || runs[0].Status != models.BackupSucceeded || !runs[0].FinishedAt.Valid {
		t.Fatalf("backup not marked succeeded: %+v", runs)
	}
}

// TestBackupStaleNotification: a server with no successful backup is flagged.
func TestBackupStaleNotification(t *testing.T) {
	_, sr, hr, nr, br, _, _ := m7Repos(t)
	seedPanel(t, sr, "http://127.0.0.1:1", models.ServerStatusActive)
	ctx := context.Background()
	p := New(Config{
		Servers: sr, Heartbeats: hr, Notifications: nr, Backups: br, AllowPlaintext: true,
		BackupStaleDays: 7, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now: func() time.Time { return time.Unix(1_700_000_000, 0) },
	})
	p.checkBackupStale(ctx)
	if n, _ := nr.UnreadCount(ctx); n != 1 {
		t.Fatalf("want 1 stale-backup notification, got %d", n)
	}
	// Idempotent: a second pass does not duplicate.
	p.checkBackupStale(ctx)
	if n, _ := nr.UnreadCount(ctx); n != 1 {
		t.Fatalf("stale-backup deduped failed: %d", n)
	}
}

// TestTokenExpiryReminder: a token nearing expiry raises one reminder.
func TestTokenExpiryReminder(t *testing.T) {
	_, sr, hr, nr, _, _, tr := m7Repos(t)
	ctx := context.Background()
	exp := time.Now().Add(3 * 24 * time.Hour)
	if _, _, err := tr.Mint(ctx, "ci", "owner", &exp); err != nil {
		t.Fatalf("mint: %v", err)
	}
	p := New(Config{
		Servers: sr, Heartbeats: hr, Notifications: nr, APITokens: tr, AllowPlaintext: true,
		TokenExpiryDays: 7, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	p.checkTokenExpiry(ctx)
	p.checkTokenExpiry(ctx) // dedup
	rows, _ := nr.ListRecent(ctx, 10)
	if len(rows) != 1 || rows[0].Kind != "token_expiring" {
		t.Fatalf("want 1 token_expiring notification, got %+v", rows)
	}
}

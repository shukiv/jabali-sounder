package poller

import (
	"context"
	"encoding/hex"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/remote"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

func testRepos(t *testing.T) (repository.ServerRepository, repository.HeartbeatRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "poll.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	gormDB, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return repository.NewServerRepository(gormDB), repository.NewHeartbeatRepository(gormDB)
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

func newPoller(sr repository.ServerRepository, hr repository.HeartbeatRepository, probe func(context.Context, models.Server) (*remote.CheckResult, error)) *Poller {
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
	sr, hr := testRepos(t)
	s := seed(t, sr, models.ServerStatusUnreachable)

	p := newPoller(sr, hr, func(context.Context, models.Server) (*remote.CheckResult, error) {
		return &remote.CheckResult{Reachable: true, CredentialValid: true, Version: "v9"}, nil
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
	sr, hr := testRepos(t)
	s := seed(t, sr, models.ServerStatusActive)

	p := newPoller(sr, hr, func(context.Context, models.Server) (*remote.CheckResult, error) {
		return &remote.CheckResult{Reachable: false}, nil
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
	sr, hr := testRepos(t)
	s := seed(t, sr, models.ServerStatusDisabled)

	var probed bool
	p := newPoller(sr, hr, func(context.Context, models.Server) (*remote.CheckResult, error) {
		probed = true
		return &remote.CheckResult{Reachable: true, CredentialValid: true}, nil
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
	sr, hr := testRepos(t)
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

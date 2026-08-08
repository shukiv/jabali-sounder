package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

func sessionDB(t *testing.T) SessionRepository {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "s.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	g, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return NewSessionRepository(g)
}

func TestSessionActiveSemantics(t *testing.T) {
	r := sessionDB(t)
	ctx := context.Background()
	now := time.Now()
	mk := func(id string, expires time.Time, revoked bool) {
		s := &models.Session{ID: id, AdminID: "adm_1", CreatedAt: now, LastSeenAt: now, ExpiresAt: expires}
		if revoked {
			s.RevokedAt.Time = now
			s.RevokedAt.Valid = true
		}
		if err := r.Create(ctx, s); err != nil {
			t.Fatalf("create %s: %v", id, err)
		}
	}
	mk("live", now.Add(time.Hour), false)
	mk("revoked", now.Add(time.Hour), true)
	mk("expired", now.Add(-time.Minute), false)

	check := func(id string, wantActive bool) {
		active, err := r.Active(ctx, id)
		if err != nil {
			t.Fatalf("Active(%s) unexpected err: %v", id, err)
		}
		if active != wantActive {
			t.Fatalf("Active(%s) = %v, want %v", id, active, wantActive)
		}
	}
	check("live", true)
	check("revoked", false)
	check("expired", false)
	// Missing / empty id: not active, and crucially NOT an error (that path must
	// never be conflated with an infra failure).
	check("does-not-exist", false)
	check("", false)
}

func TestSessionActiveThrottlesLastSeen(t *testing.T) {
	r := sessionDB(t)
	ctx := context.Background()
	now := time.Now()

	// Stale last_seen -> Active stamps it forward.
	stale := &models.Session{ID: "stale", AdminID: "adm_1", CreatedAt: now, LastSeenAt: now.Add(-2 * sessionTouchInterval), ExpiresAt: now.Add(time.Hour)}
	if err := r.Create(ctx, stale); err != nil {
		t.Fatalf("create stale: %v", err)
	}
	if _, err := r.Active(ctx, "stale"); err != nil {
		t.Fatalf("active stale: %v", err)
	}
	after, _ := r.FindByID(ctx, "stale")
	if !after.LastSeenAt.After(now.Add(-sessionTouchInterval)) {
		t.Fatalf("stale last_seen not advanced: %v", after.LastSeenAt)
	}

	// Fresh last_seen -> Active leaves it untouched (throttled).
	fresh := &models.Session{ID: "fresh", AdminID: "adm_1", CreatedAt: now, LastSeenAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := r.Create(ctx, fresh); err != nil {
		t.Fatalf("create fresh: %v", err)
	}
	before, _ := r.FindByID(ctx, "fresh")
	if _, err := r.Active(ctx, "fresh"); err != nil {
		t.Fatalf("active fresh: %v", err)
	}
	afterFresh, _ := r.FindByID(ctx, "fresh")
	if !afterFresh.LastSeenAt.Equal(before.LastSeenAt) {
		t.Fatalf("fresh last_seen changed despite throttle: %v -> %v", before.LastSeenAt, afterFresh.LastSeenAt)
	}
}

package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
)

func polexDB(t *testing.T) PolicyExceptionRepository {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "p.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	g, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return NewPolicyExceptionRepository(g)
}

func TestPolicyExceptionIgnoreRestore(t *testing.T) {
	r := polexDB(t)
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0).UTC()

	// Ignore without expiry -> active.
	if err := r.Ignore(ctx, "S1", "insecure_tls", "accepted risk", nil, "alice", now); err != nil {
		t.Fatalf("ignore: %v", err)
	}
	act, _ := r.ListActive(ctx, now)
	if len(act) != 1 || act[0].Reason != "accepted risk" || act[0].CreatedBy != "alice" {
		t.Fatalf("active after ignore: %+v", act)
	}

	// Re-ignoring the same (server,check) upserts (no duplicate, refreshed reason).
	if err := r.Ignore(ctx, "S1", "insecure_tls", "reviewed again", nil, "bob", now); err != nil {
		t.Fatalf("re-ignore: %v", err)
	}
	act, _ = r.ListActive(ctx, now)
	if len(act) != 1 || act[0].Reason != "reviewed again" || act[0].CreatedBy != "bob" {
		t.Fatalf("upsert: %+v", act)
	}

	// Restore removes it.
	removed, err := r.Restore(ctx, "S1", "insecure_tls")
	if err != nil || !removed {
		t.Fatalf("restore: removed=%v err=%v", removed, err)
	}
	if act, _ := r.ListActive(ctx, now); len(act) != 0 {
		t.Fatalf("still active after restore: %+v", act)
	}
	if removed, _ := r.Restore(ctx, "S1", "insecure_tls"); removed {
		t.Fatal("second restore should report nothing removed")
	}
}

func TestPolicyExceptionExpiry(t *testing.T) {
	r := polexDB(t)
	ctx := context.Background()
	now := time.Unix(1_700_000_000, 0).UTC()
	exp := now.Add(time.Hour)

	if err := r.Ignore(ctx, "S2", "cert_expiring", "renewing soon", &exp, "alice", now); err != nil {
		t.Fatalf("ignore: %v", err)
	}
	// Before expiry: active.
	if act, _ := r.ListActive(ctx, now); len(act) != 1 {
		t.Fatalf("should be active before expiry: %+v", act)
	}
	// After expiry: auto-reactivated (not returned as active).
	if act, _ := r.ListActive(ctx, exp.Add(time.Second)); len(act) != 0 {
		t.Fatalf("expired exception must not be active: %+v", act)
	}
	// Prune removes the expired row.
	n, err := r.PruneExpired(ctx, exp.Add(time.Second))
	if err != nil || n != 1 {
		t.Fatalf("prune: n=%d err=%v", n, err)
	}
}

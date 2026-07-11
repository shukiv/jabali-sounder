package repository

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

func m6repoDB(t *testing.T) (HeartbeatRepository, MetricSampleRepository) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "r.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	g, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return NewHeartbeatRepository(g), NewMetricSampleRepository(g)
}

func TestUptimeSince(t *testing.T) {
	hr, _ := m6repoDB(t)
	ctx := context.Background()
	now := time.Now()
	// 3 healthy + 1 unhealthy in window, 1 healthy outside window.
	seed := func(healthy bool, ago time.Duration) {
		_ = hr.Record(ctx, &models.Heartbeat{ID: ids.NewULID(), ServerID: "S1", Healthy: healthy, CheckedAt: now.Add(-ago)})
	}
	seed(true, time.Hour)
	seed(true, 2*time.Hour)
	seed(true, 3*time.Hour)
	seed(false, 4*time.Hour)
	seed(true, 48*time.Hour) // outside a 24h window
	healthy, total, err := hr.UptimeSince(ctx, "S1", now.Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("uptime: %v", err)
	}
	if total != 4 || healthy != 3 {
		t.Fatalf("want 3/4 in window, got %d/%d", healthy, total)
	}
}

func TestMetricRange(t *testing.T) {
	_, mr := m6repoDB(t)
	ctx := context.Background()
	now := time.Now()
	for i := 0; i < 10; i++ {
		_ = mr.Record(ctx, &models.MetricSample{ID: ids.NewULID(), ServerID: "S1", SampledAt: now.Add(-time.Duration(i) * time.Hour)})
	}
	rows, err := mr.Range(ctx, "S1", now.Add(-5*time.Hour), 0)
	if err != nil {
		t.Fatalf("range: %v", err)
	}
	// samples at 0..5h ago = 6 rows, ascending.
	if len(rows) != 6 {
		t.Fatalf("want 6 in range, got %d", len(rows))
	}
	for i := 1; i < len(rows); i++ {
		if rows[i].SampledAt.Before(rows[i-1].SampledAt) {
			t.Fatal("range not ascending")
		}
	}
}

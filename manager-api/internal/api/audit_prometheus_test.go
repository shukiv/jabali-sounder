package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

func m6DB(t *testing.T) (repository.AuditRepository, repository.ServerRepository, repository.MetricSampleRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "m6.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	gormDB, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return repository.NewAuditRepository(gormDB), repository.NewServerRepository(gormDB), repository.NewMetricSampleRepository(gormDB)
}

func TestAuditListFilterAndCSV(t *testing.T) {
	audit, _, _ := m6DB(t)
	ctx := context.Background()
	for i, ev := range []string{"server.enroll", "server.delete", "server.enroll"} {
		if err := audit.Create(ctx, &models.AuditLog{
			ID: ids.NewULID(), Event: ev, Actor: "alice", ServerName: "panel", SourceIP: "10.0.0.1",
			CreatedAt: time.Now().Add(-time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	r := gin.New()
	r.Use(asRole("operator", "01OP"))
	RegisterAuditRoutes(r.Group("/api/v1"), AuditHandlerConfig{Repo: audit})

	// All events.
	w := do(r, http.MethodGet, "/api/v1/admin/audit", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d", w.Code)
	}
	var out struct {
		Total int `json:"total"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.Total != 3 {
		t.Fatalf("want 3 events, got %d", out.Total)
	}
	// Filter by event.
	w = do(r, http.MethodGet, "/api/v1/admin/audit?event=server.enroll", "")
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.Total != 2 {
		t.Fatalf("want 2 enroll events, got %d", out.Total)
	}
	// CSV export.
	w = do(r, http.MethodGet, "/api/v1/admin/audit.csv", "")
	if w.Code != http.StatusOK || !strings.HasPrefix(w.Body.String(), "time,event,actor") {
		t.Fatalf("csv header wrong: %q", w.Body.String()[:40])
	}
	if strings.Count(w.Body.String(), "\n") < 4 {
		t.Fatalf("csv should have header + 3 rows")
	}
}

func TestPrometheusScrape(t *testing.T) {
	_, servers, metrics := m6DB(t)
	ctx := context.Background()
	s := &models.Server{
		ID: ids.NewULID(), Name: "panel-1", BaseURL: "https://p1:8443", Environment: "prod",
		Status: models.ServerStatusActive, CredentialStatus: models.CredentialValid,
		TokenID: "T", TokenSecretEnc: []byte("x"), Scopes: models.JSONStringArray{"read:*"},
	}
	if err := servers.Create(ctx, s); err != nil {
		t.Fatalf("seed server: %v", err)
	}
	cpu := 42.5
	if err := metrics.Record(ctx, &models.MetricSample{ID: ids.NewULID(), ServerID: s.ID, CPUPercent: &cpu, SampledAt: time.Now()}); err != nil {
		t.Fatalf("seed metric: %v", err)
	}
	r := gin.New()
	r.Use(asRole("viewer", "01V"))
	RegisterPrometheusRoutes(r.Group("/api/v1"), PrometheusHandlerConfig{Servers: servers, MetricSamples: metrics})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/prometheus", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("scrape: %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{
		`jabali_server_up{server="panel-1",id="`,
		`jabali_server_cpu_percent{server="panel-1"`,
		"jabali_fleet_servers_total 1",
		"jabali_fleet_servers_healthy 1",
		"# TYPE jabali_server_up gauge",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("scrape missing %q in:\n%s", want, body)
		}
	}
	if !strings.Contains(body, "42.5") {
		t.Fatalf("cpu value missing: %s", body)
	}
}

func TestAuditServerMutationPersists(t *testing.T) {
	audit, _, _ := m6DB(t)
	r := gin.New()
	r.Use(asRole("bob", "01BOB"))
	r.POST("/x", func(c *gin.Context) {
		auditServerMutation(nil, audit, c, "delete", "SRV1", "panel-x")
		c.Status(http.StatusOK)
	})
	if w := do(r, http.MethodPost, "/x", ""); w.Code != http.StatusOK {
		t.Fatalf("handler: %d", w.Code)
	}
	rows, err := audit.List(context.Background(), repository.AuditFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || rows[0].Event != "server.delete" || rows[0].ServerID != "SRV1" {
		t.Fatalf("audit not persisted: %+v", rows)
	}
}

func TestDownsampleMetrics(t *testing.T) {
	mk := func(n int) []models.MetricSample {
		out := make([]models.MetricSample, n)
		for i := range out {
			out[i] = models.MetricSample{ID: ids.NewULID()}
		}
		return out
	}
	if got := downsampleMetrics(mk(50), 400); len(got) != 50 {
		t.Fatalf("under target unchanged: %d", len(got))
	}
	big := mk(4000)
	got := downsampleMetrics(big, 400)
	if len(got) > 402 {
		t.Fatalf("downsample too large: %d", len(got))
	}
	if got[len(got)-1].ID != big[len(big)-1].ID {
		t.Fatal("last sample must be preserved")
	}
}

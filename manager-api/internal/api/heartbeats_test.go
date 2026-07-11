package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

// TestHeartbeatsEndpoint covers the M1 status-history endpoint: recent
// heartbeats + an uptime summary over the returned window.
func TestHeartbeatsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "hb.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	gormDB, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sr := repository.NewServerRepository(gormDB)
	hr := repository.NewHeartbeatRepository(gormDB)

	srv := &models.Server{
		ID: ids.NewULID(), Name: "srv", BaseURL: "https://p.example:8443",
		TokenID: "T", TokenSecretEnc: []byte("x"), Scopes: models.JSONStringArray{"read:*"},
		Status: models.ServerStatusActive, CredentialStatus: models.CredentialValid,
	}
	if err := sr.Create(context.Background(), srv); err != nil {
		t.Fatalf("seed: %v", err)
	}
	for i, healthy := range []bool{true, true, false} {
		hb := &models.Heartbeat{ID: ids.NewULID(), ServerID: srv.ID, Healthy: healthy, CheckedAt: time.Now().Add(time.Duration(i) * time.Second)}
		if err := hr.Record(context.Background(), hb); err != nil {
			t.Fatalf("record: %v", err)
		}
	}

	r := gin.New()
	RegisterServerRoutes(r.Group("/api/v1"), ServerHandlerConfig{Repo: sr, Heartbeats: hr})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/servers/"+srv.ID+"/heartbeats", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body struct {
		Total  int `json:"total"`
		Uptime struct {
			Healthy int     `json:"healthy"`
			Total   int     `json:"total"`
			Ratio   float64 `json:"ratio"`
		} `json:"uptime"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("json: %v", err)
	}
	if body.Total != 3 || body.Uptime.Healthy != 2 {
		t.Fatalf("total=%d healthy=%d, want 3/2", body.Total, body.Uptime.Healthy)
	}
	if body.Uptime.Ratio < 0.66 || body.Uptime.Ratio > 0.67 {
		t.Fatalf("ratio=%v, want ~0.667", body.Uptime.Ratio)
	}
}

// TestMetricsEndpoint covers the M1 metrics-trends endpoint.
func TestMetricsEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "m.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	gormDB, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sr := repository.NewServerRepository(gormDB)
	mr := repository.NewMetricSampleRepository(gormDB)
	srv := &models.Server{ID: ids.NewULID(), Name: "s", BaseURL: "https://p:8443", TokenID: "T", TokenSecretEnc: []byte("x"), Scopes: models.JSONStringArray{"read:*"}, Status: models.ServerStatusActive, CredentialStatus: models.CredentialValid}
	if err := sr.Create(context.Background(), srv); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cpu := 55.5
	if err := mr.Record(context.Background(), &models.MetricSample{ID: ids.NewULID(), ServerID: srv.ID, CPUPercent: &cpu, SampledAt: time.Now()}); err != nil {
		t.Fatalf("record: %v", err)
	}

	r := gin.New()
	RegisterServerRoutes(r.Group("/api/v1"), ServerHandlerConfig{Repo: sr, MetricSamples: mr})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/servers/"+srv.ID+"/metrics", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var body struct {
		Total int `json:"total"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if body.Total != 1 {
		t.Fatalf("total=%d want 1", body.Total)
	}
}

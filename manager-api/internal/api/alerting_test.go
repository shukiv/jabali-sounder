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
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

func alertingRouter(t *testing.T, role string) (*gin.Engine, AlertingHandlerConfig) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "alerting.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	gormDB, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	rules := repository.NewAlertRuleRepository(gormDB)
	if err := rules.EnsureDefaults(context.Background(), time.Now().UTC()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cfg := AlertingHandlerConfig{
		Rules:          rules,
		Channels:       repository.NewAlertChannelRepository(gormDB),
		Maintenance:    repository.NewMaintenanceRepository(gormDB),
		Muted:          repository.NewMutedAlertRepository(gormDB),
		AllowPlaintext: true, // no key -> hex fallback for sealed config
	}
	r := gin.New()
	r.Use(asRole(role, "01OP"))
	RegisterAlertingRoutes(r.Group("/api/v1"), cfg)
	return r, cfg
}

func TestAlertRulesListAndUpdate(t *testing.T) {
	r, _ := alertingRouter(t, "operator")

	w := do(r, http.MethodGet, "/api/v1/admin/alert-rules", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list rules: %d", w.Code)
	}
	var listed struct {
		Data []map[string]any `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &listed)
	if len(listed.Data) < 3 {
		t.Fatalf("expected seeded rules, got %d", len(listed.Data))
	}

	body := `{"threshold":70,"duration_seconds":30,"severity":"critical","enabled":true}`
	if w := do(r, http.MethodPut, "/api/v1/admin/alert-rules/cpu", body); w.Code != http.StatusOK {
		t.Fatalf("update rule: %d %s", w.Code, w.Body.String())
	}
	// Invalid severity rejected.
	if w := do(r, http.MethodPut, "/api/v1/admin/alert-rules/cpu", `{"severity":"bogus"}`); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad severity: %d", w.Code)
	}
	// Unknown metric -> 404.
	if w := do(r, http.MethodPut, "/api/v1/admin/alert-rules/nope", `{"severity":"warning"}`); w.Code != http.StatusNotFound {
		t.Fatalf("unknown metric: %d", w.Code)
	}
}

func TestAlertRuleUpdateForbiddenForViewer(t *testing.T) {
	r, _ := alertingRouter(t, "viewer")
	if w := do(r, http.MethodPut, "/api/v1/admin/alert-rules/cpu", `{"severity":"warning","enabled":true}`); w.Code != http.StatusForbidden {
		t.Fatalf("viewer should be forbidden, got %d", w.Code)
	}
}

func TestAlertChannelLifecycleAndTest(t *testing.T) {
	// A local webhook endpoint the channel + test-send target.
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r, _ := alertingRouter(t, "operator")

	create := `{"name":"ops","type":"webhook","min_severity":"warning","enabled":true,"config":{"url":"` + srv.URL + `"}}`
	w := do(r, http.MethodPost, "/api/v1/admin/alert-channels", create)
	if w.Code != http.StatusCreated {
		t.Fatalf("create channel: %d %s", w.Code, w.Body.String())
	}
	var ch map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &ch)
	id, _ := ch["id"].(string)
	if id == "" {
		t.Fatalf("no channel id")
	}
	// List must not leak config/secrets.
	w = do(r, http.MethodGet, "/api/v1/admin/alert-channels", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list channels: %d", w.Code)
	}
	if bs := w.Body.String(); strings.Contains(bs, srv.URL) || strings.Contains(bs, "\"config\"") {
		t.Fatalf("channel list leaked config: %s", bs)
	}
	// Test-send hits the webhook.
	if w := do(r, http.MethodPost, "/api/v1/admin/alert-channels/"+id+"/test", ""); w.Code != http.StatusOK {
		t.Fatalf("test channel: %d %s", w.Code, w.Body.String())
	}
	if hits == 0 {
		t.Fatal("test-send did not reach the webhook")
	}
	// Delete.
	if w := do(r, http.MethodDelete, "/api/v1/admin/alert-channels/"+id, ""); w.Code != http.StatusOK {
		t.Fatalf("delete channel: %d", w.Code)
	}

	// Invalid channel (missing url) rejected.
	if w := do(r, http.MethodPost, "/api/v1/admin/alert-channels", `{"name":"x","type":"webhook","config":{}}`); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid channel: %d", w.Code)
	}
}

func TestMaintenanceAndMuteEndpoints(t *testing.T) {
	r, cfg := alertingRouter(t, "operator")

	start := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	end := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	body := `{"scope_type":"global","starts_at":"` + start + `","ends_at":"` + end + `","reason":"upgrade"}`
	if w := do(r, http.MethodPost, "/api/v1/admin/maintenance", body); w.Code != http.StatusCreated {
		t.Fatalf("create maintenance: %d %s", w.Code, w.Body.String())
	}
	active, err := cfg.Maintenance.ActiveForServer(context.Background(), "S1", "prod", time.Now().UTC())
	if err != nil || !active {
		t.Fatalf("global window should be active: active=%v err=%v", active, err)
	}
	// ends before starts -> rejected.
	bad := `{"scope_type":"global","starts_at":"` + end + `","ends_at":"` + start + `"}`
	if w := do(r, http.MethodPost, "/api/v1/admin/maintenance", bad); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad window: %d", w.Code)
	}

	// Mute + unmute.
	if w := do(r, http.MethodPost, "/api/v1/admin/muted", `{"server_id":"S1","kind":"cpu_high"}`); w.Code != http.StatusOK {
		t.Fatalf("mute: %d %s", w.Code, w.Body.String())
	}
	// Re-muting the same (server, kind) must stay idempotent, not 500 (SND-38).
	if w := do(r, http.MethodPost, "/api/v1/admin/muted", `{"server_id":"S1","kind":"cpu_high"}`); w.Code != http.StatusOK {
		t.Fatalf("re-mute should be idempotent: %d %s", w.Code, w.Body.String())
	}
	muted, _ := cfg.Muted.IsMuted(context.Background(), "S1", "cpu_high")
	if !muted {
		t.Fatal("expected muted")
	}
	if w := do(r, http.MethodDelete, "/api/v1/admin/muted?server_id=S1&kind=cpu_high", ""); w.Code != http.StatusOK {
		t.Fatalf("unmute: %d", w.Code)
	}
	muted, _ = cfg.Muted.IsMuted(context.Background(), "S1", "cpu_high")
	if muted {
		t.Fatal("expected unmuted")
	}
}

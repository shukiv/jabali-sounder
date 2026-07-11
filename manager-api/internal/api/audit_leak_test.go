package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestSafeRemoteErrorHidesRawError covers SND-9: the per-server error summary
// must not leak the raw transport error (which can expose internal IPs).
func TestSafeRemoteErrorHidesRawError(t *testing.T) {
	log := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	raw := errors.New("dial tcp 10.0.0.5:8443: connect: connection refused")

	got := safeRemoteError(log, "srv1", "metrics", 503, raw)
	if strings.Contains(got, "10.0.0.5") || strings.Contains(got, "dial tcp") {
		t.Fatalf("summary leaked raw error: %q", got)
	}
	if !strings.Contains(got, "metrics unavailable") || !strings.Contains(got, "503") {
		t.Fatalf("summary not user-safe/informative: %q", got)
	}

	// No HTTP code -> no leak either.
	got2 := safeRemoteError(log, "srv1", "metrics", 0, raw)
	if strings.Contains(got2, "10.0.0.5") {
		t.Fatalf("summary leaked raw error: %q", got2)
	}
}

// TestPrivilegedMutationIsAudited covers SND-10: a server mutation emits a
// structured audit event naming the action and target.
func TestPrivilegedMutationIsAudited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newTestServerRepo(t)
	srv := seedServer(t, repo)

	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	r := gin.New()
	r.Use(asRole("operator", "01OP"))
	RegisterServerRoutes(r.Group("/api/v1"), ServerHandlerConfig{Repo: repo, AllowPlaintext: true, Log: log})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/servers/"+srv.ID+"/disable", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("disable status = %d", w.Code)
	}

	// Find the audit line in the JSON log output.
	var found bool
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		var rec map[string]any
		if json.Unmarshal([]byte(line), &rec) != nil {
			continue
		}
		if rec["msg"] == "audit" && rec["event"] == "server.disable" && rec["server_id"] == srv.ID {
			found = true
			if s, _ := rec["server_name"].(string); s != srv.Name {
				t.Errorf("audit server_name = %q, want %q", s, srv.Name)
			}
		}
	}
	if !found {
		t.Fatalf("no audit event for disable in log:\n%s", buf.String())
	}
	_ = context.Background
}

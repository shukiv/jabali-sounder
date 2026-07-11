package remote

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestActionClient(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/automation/services/denied/restart" {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "scope_denied"})
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "status": "done"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "01TOKEN", "secret", false)

	// Restart hits the right method + path and parses ok.
	res, err := c.RestartService(t.Context(), "nginx")
	if err != nil || !res.OK {
		t.Fatalf("restart: %v result=%+v", err, res)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/v1/automation/services/nginx/restart" {
		t.Fatalf("wrong request: %s %s", gotMethod, gotPath)
	}

	// Cache purge with a body.
	if r, err := c.PurgeCache(t.Context(), "all", ""); err != nil || !r.OK {
		t.Fatalf("purge: %v", err)
	}
	if gotPath != "/api/v1/automation/cache/purge" {
		t.Fatalf("purge path: %s", gotPath)
	}

	// Denied action surfaces the panel error code.
	if _, err := c.RestartService(t.Context(), "denied"); err == nil {
		t.Fatal("expected scope_denied error")
	}
}

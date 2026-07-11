package alert

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestWebhookNotifierPostsJSON(t *testing.T) {
	var received map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := NewWebhook(srv.URL, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ev := Event{Kind: KindDown, ServerID: "01X", ServerName: "panel-1", Status: "unreachable", CredentialStatus: "unknown", Message: "server did not respond", At: time.Now()}
	if err := n.Notify(context.Background(), ev); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if received["kind"] != "down" || received["server_name"] != "panel-1" {
		t.Fatalf("webhook payload wrong: %v", received)
	}
	text, _ := received["text"].(string)
	if !strings.Contains(text, "panel-1") || !strings.Contains(text, "unhealthy") {
		t.Fatalf("text not human-readable: %q", text)
	}
}

func TestWebhookNotifierErrorsOnBadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	n := NewWebhook(srv.URL, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := n.Notify(context.Background(), Event{Kind: KindDown}); err == nil {
		t.Fatal("expected error on 500 webhook response")
	}
}

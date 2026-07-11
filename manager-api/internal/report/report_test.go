package report

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

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

func TestSummarize(t *testing.T) {
	servers := []models.Server{
		{Status: models.ServerStatusActive, CredentialStatus: models.CredentialValid, Environment: "production", Version: "v1"},
		{Status: models.ServerStatusActive, CredentialStatus: models.CredentialInvalid, Environment: "production", Version: "v1"},
		{Status: models.ServerStatusUnreachable, Environment: "staging", Version: "v2"},
		{Status: models.ServerStatusDisabled},
	}
	s := Summarize(servers, time.Now())
	if s.Total != 4 || s.Active != 2 || s.Unreachable != 1 || s.Disabled != 1 || s.CredentialInvalid != 1 {
		t.Fatalf("counts wrong: %+v", s)
	}
	if s.ByEnvironment["production"] != 2 || s.ByEnvironment["unassigned"] != 1 {
		t.Fatalf("env breakdown wrong: %+v", s.ByEnvironment)
	}
	if !strings.Contains(s.Text(), "4 servers") {
		t.Fatalf("text missing summary: %q", s.Text())
	}
}

func TestDeliver(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rep := New(Config{WebhookURL: srv.URL, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err := rep.deliver(context.Background(), Summarize([]models.Server{{Status: models.ServerStatusActive}}, time.Now())); err != nil {
		t.Fatalf("deliver: %v", err)
	}
	if got["total"] != float64(1) || got["text"] == nil {
		t.Fatalf("payload wrong: %v", got)
	}
}

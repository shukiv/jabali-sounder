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
)

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestNtfyNotifierPosts(t *testing.T) {
	var gotBody, gotTitle, gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		gotTitle = r.Header.Get("Title")
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n, err := NewNtfy(map[string]string{"url": srv.URL, "topic": "fleet", "token": "tok_123"}, quietLog())
	if err != nil {
		t.Fatalf("new ntfy: %v", err)
	}
	ev := Event{Kind: KindDown, ServerName: "panel-1", Message: "no response"}
	if err := n.Notify(context.Background(), ev); err != nil {
		t.Fatalf("notify: %v", err)
	}
	if gotPath != "/fleet" {
		t.Fatalf("topic path = %q", gotPath)
	}
	if !strings.Contains(gotBody, "panel-1") {
		t.Fatalf("body = %q", gotBody)
	}
	if !strings.Contains(gotTitle, "panel-1") {
		t.Fatalf("title = %q", gotTitle)
	}
	if gotAuth != "Bearer tok_123" {
		t.Fatalf("auth = %q", gotAuth)
	}
}

func TestNtfyRequiresTopic(t *testing.T) {
	if _, err := NewNtfy(map[string]string{"url": "https://ntfy.sh"}, quietLog()); err == nil {
		t.Fatal("expected error without topic")
	}
}

func TestPagerDutyTriggerAndResolve(t *testing.T) {
	var payloads []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p map[string]any
		_ = json.NewDecoder(r.Body).Decode(&p)
		payloads = append(payloads, p)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	pd, err := NewPagerDuty(map[string]string{"routing_key": "R1"}, quietLog())
	if err != nil {
		t.Fatalf("new pd: %v", err)
	}
	// Point at the test server by overriding the client's transport target.
	pd.client = srv.Client()
	// Rewrite the request URL via a round-tripper wrapper.
	pd.client.Transport = rewriteHost(srv.URL)

	if err := pd.Notify(context.Background(), Event{Kind: KindDown, ServerID: "S1", ServerName: "p"}); err != nil {
		t.Fatalf("trigger: %v", err)
	}
	if err := pd.Notify(context.Background(), Event{Kind: KindRecovered, ServerID: "S1", ServerName: "p"}); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(payloads) != 2 {
		t.Fatalf("want 2 payloads, got %d", len(payloads))
	}
	if payloads[0]["event_action"] != "trigger" || payloads[1]["event_action"] != "resolve" {
		t.Fatalf("actions wrong: %v", payloads)
	}
	if payloads[0]["dedup_key"] != "jabali-sounder:S1" {
		t.Fatalf("dedup key wrong: %v", payloads[0]["dedup_key"])
	}
}

// rewriteHost sends all requests to base instead of their real host.
type rewriteHost string

func (h rewriteHost) RoundTrip(req *http.Request) (*http.Response, error) {
	target := string(h)
	newReq := req.Clone(req.Context())
	// Replace scheme+host with the test server's.
	u := newReq.URL
	trimmed := strings.TrimPrefix(target, "http://")
	u.Scheme = "http"
	u.Host = trimmed
	return http.DefaultTransport.RoundTrip(newReq)
}

func TestBuildNotifierAndDispatch(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	wh, err := BuildNotifier(TypeWebhook, map[string]string{"url": srv.URL}, quietLog())
	if err != nil {
		t.Fatalf("build webhook: %v", err)
	}
	nt, err := BuildNotifier(TypeNtfy, map[string]string{"url": srv.URL, "topic": "t"}, quietLog())
	if err != nil {
		t.Fatalf("build ntfy: %v", err)
	}
	sent := Dispatch(context.Background(), quietLog(), []Notifier{wh, nt}, Event{Kind: KindDown, ServerName: "p"})
	if sent != 2 || hits != 2 {
		t.Fatalf("dispatch sent=%d hits=%d", sent, hits)
	}

	if _, err := BuildNotifier("bogus", nil, quietLog()); err == nil {
		t.Fatal("expected error for unknown type")
	}
}

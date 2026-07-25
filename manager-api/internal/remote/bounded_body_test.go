package remote

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestResponseBodyBounded proves the remote client caps how much of a managed
// panel's response it will read (JAB-113). A hostile or compromised server that
// returns a body larger than maxBody must yield a decode error attributed to
// that server, never an unbounded read that could OOM the control plane.
//
// The oversized body is invalid past the cap point, so a correct (bounded)
// client truncates at maxBody and fails to decode; an unbounded client would
// read the whole multi-megabyte body into memory first.
func TestResponseBodyBounded(t *testing.T) {
	// A JSON object that opens correctly, then a huge run of filler that is
	// far larger than maxBody and never closes the object. Under the cap the
	// decoder sees truncated JSON and errors; without the cap it would read
	// every byte before failing.
	oversized := `{"status":"ok","version":"` + strings.Repeat("A", 4*maxBody) + `"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(oversized))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "01ABCDEFGHJKLMNPQRSTUVWXYZ", "test-secret-32-bytes-fixed-length!", false)

	_, _, err := c.Health(context.Background())
	if err == nil {
		t.Fatalf("expected a decode error from an over-cap response body, got nil (body was read unbounded)")
	}
}

// TestResponseBodyWithinCapStillDecodes guards against the cap being set so low
// that legitimate responses break: a normal, well-under-maxBody body must still
// decode cleanly.
func TestResponseBodyWithinCapStillDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok","version":"1.2.3"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "01ABCDEFGHJKLMNPQRSTUVWXYZ", "test-secret-32-bytes-fixed-length!", false)

	h, _, err := c.Health(context.Background())
	if err != nil {
		t.Fatalf("normal response should decode, got error: %v", err)
	}
	if h.Version != "1.2.3" {
		t.Fatalf("version mismatch: got %q want %q", h.Version, "1.2.3")
	}
}

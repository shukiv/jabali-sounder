package remote

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPeerCertNotAfter(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	exp, err := PeerCertNotAfter(srv.URL)
	if err != nil {
		t.Fatalf("PeerCertNotAfter: %v", err)
	}
	if !exp.After(time.Now()) {
		t.Fatalf("expected a future expiry, got %v", exp)
	}
}

func TestPeerCertNotAfterUnreachable(t *testing.T) {
	if _, err := PeerCertNotAfter("https://127.0.0.1:1/"); err == nil {
		t.Fatal("expected error dialing a closed port")
	}
}

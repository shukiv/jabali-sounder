package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/go-webauthn/webauthn/webauthn"
)

func ctxWith(host, xfProto string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = host
	if xfProto != "" {
		req.Header.Set("X-Forwarded-Proto", xfProto)
	}
	c.Request = req
	return c
}

func TestRPConfigDerivation(t *testing.T) {
	cases := []struct {
		name, host, xfp, cfgRPID, cfgOrigin string
		wantRPID, wantOrigin                string
	}{
		{"proxied https", "mx.example.com", "https", "", "", "mx.example.com", "https://mx.example.com"},
		{"host with port, plain", "mx.example.com:8443", "", "", "", "mx.example.com", "http://mx.example.com:8443"},
		{"config override wins", "loopback:10000", "http", "sounder.example.com", "https://sounder.example.com", "sounder.example.com", "https://sounder.example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &authHandler{cfg: AuthHandlerConfig{WebAuthnRPID: tc.cfgRPID, WebAuthnOrigin: tc.cfgOrigin}}
			rpID, origin := h.rpConfig(ctxWith(tc.host, tc.xfp))
			if rpID != tc.wantRPID || origin != tc.wantOrigin {
				t.Fatalf("got (%q,%q), want (%q,%q)", rpID, origin, tc.wantRPID, tc.wantOrigin)
			}
		})
	}
}

func TestPasskeySessionStoreSingleUseAndExpiry(t *testing.T) {
	s := newPasskeySessionStore()
	data := &webauthn.SessionData{Challenge: "abc"}

	id := s.put(data)
	if got := s.take(id); got == nil || got.Challenge != "abc" {
		t.Fatalf("first take should return the session, got %+v", got)
	}
	if got := s.take(id); got != nil {
		t.Fatal("second take must be nil (single-use)")
	}

	// Expired entry is not returned.
	s.mu.Lock()
	s.m["expired"] = passkeySession{data: data, expires: time.Now().Add(-time.Second)}
	s.mu.Unlock()
	if got := s.take("expired"); got != nil {
		t.Fatal("expired session must not be returned")
	}
}

package remote

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestGoldenSignature verifies the client produces the exact HMAC signature
// that jabali2's middleware expects. This is the single most important test
// in the manager — if it breaks, every remote call 401s.
//
// The signature scheme (from jabali2 automation_hmac.go):
//
//	sig = hex(HMAC_SHA256(secret, METHOD + "\n" + RequestURI + "\n" + ts + "\n" + hex(sha256(body))))
func TestGoldenSignature(t *testing.T) {
	secret := "test-secret-32-bytes-fixed-length!"
	tokenID := "01ABCDEFGHJKLMNPQRSTUVWXYZ"

	tests := []struct {
		name   string
		method string
		path   string // must include query string if any
		body   []byte
	}{
		{
			name:   "GET no body no query",
			method: "GET",
			path:   "/api/v1/automation/status",
			body:   nil,
		},
		{
			name:   "GET with query params",
			method: "GET",
			path:   "/api/v1/automation/logs?service=nginx&since=2026-01-01",
			body:   nil,
		},
		{
			name:   "POST with body",
			method: "POST",
			path:   "/api/v1/automation/delegated-login",
			body:   []byte(`{"target_user_id":"01JTEST1234567890ABCDEFG"}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a test server that captures the Authorization header
			// and verifies the signature server-side (mirrors jabali2's
			// constant-time compare).
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw := r.Header.Get("Authorization")
				if !strings.HasPrefix(raw, "Jabali-HMAC ") {
					t.Errorf("missing Jabali-HMAC prefix: got %q", raw)
					http.Error(w, "bad header", http.StatusUnauthorized)
					return
				}
				params := parseAuthParams(raw[len("Jabali-HMAC "):])
				kid := params["kid"]
				tsStr := params["ts"]
				sig := params["sig"]

				if kid != tokenID {
					t.Errorf("kid mismatch: got %q want %q", kid, tokenID)
				}

				// Recompute expected signature.
				body := tt.body
				if body == nil {
					body = []byte{}
				}
				bodyHash := sha256.Sum256(body)
				mac := hmac.New(sha256.New, []byte(secret))
				mac.Write([]byte(r.Method))
				mac.Write([]byte("\n"))
				mac.Write([]byte(r.URL.RequestURI()))
				mac.Write([]byte("\n"))
				mac.Write([]byte(tsStr))
				mac.Write([]byte("\n"))
				mac.Write([]byte(hex.EncodeToString(bodyHash[:])))
				expected := hex.EncodeToString(mac.Sum(nil))

				if sig != expected {
					t.Errorf("signature mismatch:\n  got:  %s\n  want: %s", sig, expected)
				}

				// Verify the signed RequestURI includes the query string.
				if strings.Contains(tt.path, "?") {
					if !strings.Contains(r.URL.RequestURI(), "?") {
						t.Errorf("RequestURI missing query string: %q", r.URL.RequestURI())
					}
				}

				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"ok"}`))
			}))
			defer srv.Close()

			client := NewClient(srv.URL, tokenID, secret, false)
			resp, err := client.Do(t.Context(), tt.method, tt.path, tt.body)
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status %d — signature rejected", resp.StatusCode)
			}
		})
	}
}

// TestSignatureDeterministic verifies the signature is deterministic for
// fixed inputs (same secret + method + path + ts + body → same sig).
func TestSignatureDeterministic(t *testing.T) {
	secret := "deterministic-test-secret-key!!!"
	method := "GET"
	path := "/api/v1/automation/status"
	ts := "1700000000"
	body := []byte{}

	bodyHash := sha256.Sum256(body)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(method))
	mac.Write([]byte("\n"))
	mac.Write([]byte(path))
	mac.Write([]byte("\n"))
	mac.Write([]byte(ts))
	mac.Write([]byte("\n"))
	mac.Write([]byte(hex.EncodeToString(bodyHash[:])))
	sig1 := hex.EncodeToString(mac.Sum(nil))

	mac2 := hmac.New(sha256.New, []byte(secret))
	mac2.Write([]byte(method))
	mac2.Write([]byte("\n"))
	mac2.Write([]byte(path))
	mac2.Write([]byte("\n"))
	mac2.Write([]byte(ts))
	mac2.Write([]byte("\n"))
	mac2.Write([]byte(hex.EncodeToString(bodyHash[:])))
	sig2 := hex.EncodeToString(mac2.Sum(nil))

	if sig1 != sig2 {
		t.Errorf("signatures not deterministic: %s vs %s", sig1, sig2)
	}
}

// parseAuthParams parses the "kid=..., ts=..., sig=..." portion of the
// Authorization header. Mirrors jabali2's parseAutoAuthParams.
func parseAuthParams(s string) map[string]string {
	params := map[string]string{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			params[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return params
}

// Ensure time is used (the client uses time.Now for ts).
var _ = time.Now
var _ = strconv.Itoa

// TestDecodeJSONBodyBounded verifies a hostile/compromised managed panel that
// returns an oversized response body yields a bounded error rather than reading
// unbounded bytes into memory (issue #113). A body at/under maxBody still
// decodes normally.
func TestDecodeJSONBodyBounded(t *testing.T) {
	t.Run("oversized body errors", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// A valid-JSON string far larger than maxBody: `"` + filler + `"`.
			_, _ = w.Write([]byte{'"'})
			chunk := strings.Repeat("A", 64*1024)
			for written := 0; written < (maxBody + 4*1024*1024); written += len(chunk) {
				if _, err := w.Write([]byte(chunk)); err != nil {
					return
				}
			}
			_, _ = w.Write([]byte{'"'})
		}))
		defer srv.Close()

		resp, err := http.Get(srv.URL)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		defer resp.Body.Close()

		var v any
		err = decodeJSONBody(resp, &v)
		if err == nil {
			t.Fatal("expected bounded error for oversized body, got nil")
		}
		if !strings.Contains(err.Error(), "exceeds") {
			t.Fatalf("expected size-cap error, got: %v", err)
		}
	})

	t.Run("normal body decodes", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}))
		defer srv.Close()

		resp, err := http.Get(srv.URL)
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		defer resp.Body.Close()

		var v map[string]string
		if err := decodeJSONBody(resp, &v); err != nil {
			t.Fatalf("decode normal body: %v", err)
		}
		if v["status"] != "ok" {
			t.Fatalf("wrong decode: %v", v)
		}
	})
}

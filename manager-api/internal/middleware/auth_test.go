package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func init() { gin.SetMode(gin.TestMode) }

// mount builds a minimal engine with one protected route guarded by
// AuthMiddleware(secret) and returns it.
func mount(secret string) *gin.Engine {
	r := gin.New()
	g := r.Group("")
	g.Use(AuthMiddleware(secret))
	g.GET("/protected", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}

// TestAuthFailsClosedOnEmptySecret asserts that when no JWT secret is
// configured, the protected surface is never served unauthenticated — not even
// to a token an attacker forged by HMAC-signing with the empty key (issue #112).
func TestAuthFailsClosedOnEmptySecret(t *testing.T) {
	r := mount("")

	// A token an attacker could forge, signing with the empty key.
	claims := &jwt.RegisteredClaims{
		Subject:   "attacker",
		ID:        "attacker",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	}
	forged, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(""))
	if err != nil {
		t.Fatalf("sign forged token: %v", err)
	}

	cases := []struct{ name, auth string }{
		{"no header", ""},
		{"forged empty-key token", "Bearer " + forged},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			if tc.auth != "" {
				req.Header.Set("Authorization", tc.auth)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("empty-secret guard should 401, got %d (%s)", w.Code, w.Body.String())
			}
		})
	}
}

// TestAuthAcceptsValidTokenWithSecret is the positive control: a properly
// minted token is accepted when a real secret is configured.
func TestAuthAcceptsValidTokenWithSecret(t *testing.T) {
	secret := "a-real-32-byte-secret-for-testing!!"
	r := mount(secret)

	token, _, err := MintToken(secret, "01ADMIN", "admin", time.Hour)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("valid token should 200, got %d (%s)", w.Code, w.Body.String())
	}
}

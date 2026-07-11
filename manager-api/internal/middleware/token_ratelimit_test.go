package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func rlRouter(l *TokenRateLimiter, id string, limit int) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(ctxAdminID, id)
		if limit >= 0 {
			c.Set(ctxTokenRate, limit)
		}
		c.Next()
	})
	r.Use(l.Middleware())
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}

func TestTokenRateLimit(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	l := NewTokenRateLimiter(func() time.Time { return now })
	r := rlRouter(l, "apitoken:T1", 3)
	hit := func() int {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
		return w.Code
	}
	for i := 0; i < 3; i++ {
		if c := hit(); c != http.StatusOK {
			t.Fatalf("req %d within limit should pass, got %d", i+1, c)
		}
	}
	if c := hit(); c != http.StatusTooManyRequests {
		t.Fatalf("4th req should be 429, got %d", c)
	}
	// Next window resets.
	now = now.Add(61 * time.Second)
	if c := hit(); c != http.StatusOK {
		t.Fatalf("after window reset should pass, got %d", c)
	}
}

func TestTokenRateLimitBypass(t *testing.T) {
	l := NewTokenRateLimiter(nil)
	// JWT request (no apitoken prefix) is never limited.
	r := rlRouter(l, "01OP", 1)
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("JWT request must not be rate-limited, got %d", w.Code)
		}
	}
	// Token with limit 0 = unlimited.
	r0 := rlRouter(l, "apitoken:T0", 0)
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		r0.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
		if w.Code != http.StatusOK {
			t.Fatalf("limit 0 must be unlimited, got %d", w.Code)
		}
	}
}

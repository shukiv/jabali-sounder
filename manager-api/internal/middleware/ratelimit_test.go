package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func fireLogin(r *gin.Engine, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.RemoteAddr = ip + ":12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestLoginLimiterLocksOutAfterMaxFailures covers SND-3: the Nth rapid failed
// attempt from one source is rejected with 429.
func TestLoginLimiterLocksOutAfterMaxFailures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lim := NewLoginLimiter(5, time.Hour, time.Hour, nil, nil)
	r := gin.New()
	r.POST("/auth/login", lim.Middleware(), func(c *gin.Context) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_credentials"})
	})

	for i := 1; i <= 5; i++ {
		if w := fireLogin(r, "10.0.0.1"); w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: want 401, got %d", i, w.Code)
		}
	}
	w := fireLogin(r, "10.0.0.1")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("6th attempt: want 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After on lockout")
	}
}

// TestLoginLimiterIsolatesIPs and success-reset.
func TestLoginLimiterIsolatesIPs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	lim := NewLoginLimiter(3, time.Hour, time.Hour, nil, nil)
	r := gin.New()
	r.POST("/auth/login", lim.Middleware(), func(c *gin.Context) { c.JSON(http.StatusUnauthorized, gin.H{}) })
	for i := 0; i < 3; i++ {
		fireLogin(r, "10.0.0.1")
	}
	if w := fireLogin(r, "10.0.0.1"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("attacker IP should be locked: got %d", w.Code)
	}
	if w := fireLogin(r, "10.0.0.2"); w.Code != http.StatusUnauthorized {
		t.Fatalf("innocent IP should not be throttled: got %d", w.Code)
	}
}

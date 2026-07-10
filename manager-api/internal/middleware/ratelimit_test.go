package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// fireLogin sends one request to the limiter-wrapped handler that always
// returns wantStatus, simulating a failed (401) or successful (200) login.
func fireLogin(r *gin.Engine, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.RemoteAddr = ip + ":12345"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestLoginLimiterLocksOutAfterMaxFailures asserts the Nth rapid failed attempt
// from one source is rejected with 429 (issue #110).
func TestLoginLimiterLocksOutAfterMaxFailures(t *testing.T) {
	lim := NewLoginLimiter(5, time.Hour, time.Hour, nil)
	r := gin.New()
	r.POST("/auth/login", lim.Middleware(), func(c *gin.Context) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
	})

	for i := 1; i <= 5; i++ {
		if w := fireLogin(r, "10.0.0.1"); w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: want 401, got %d", i, w.Code)
		}
	}
	// 6th attempt is locked out before the handler runs.
	w := fireLogin(r, "10.0.0.1")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("6th attempt: want 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header on lockout")
	}
}

// TestLoginLimiterIsolatesSourceIPs asserts one IP's lockout does not throttle a
// different source.
func TestLoginLimiterIsolatesSourceIPs(t *testing.T) {
	lim := NewLoginLimiter(3, time.Hour, time.Hour, nil)
	r := gin.New()
	r.POST("/auth/login", lim.Middleware(), func(c *gin.Context) {
		c.JSON(http.StatusUnauthorized, gin.H{})
	})

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

// TestLoginLimiterSuccessClearsFailures asserts a successful login resets the
// per-IP failure counter.
func TestLoginLimiterSuccessClearsFailures(t *testing.T) {
	lim := NewLoginLimiter(3, time.Hour, time.Hour, nil)
	status := http.StatusUnauthorized
	r := gin.New()
	r.POST("/auth/login", lim.Middleware(), func(c *gin.Context) {
		c.JSON(status, gin.H{})
	})

	fireLogin(r, "10.0.0.9") // 1 failure
	fireLogin(r, "10.0.0.9") // 2 failures
	status = http.StatusOK
	if w := fireLogin(r, "10.0.0.9"); w.Code != http.StatusOK {
		t.Fatalf("success: got %d", w.Code)
	}
	status = http.StatusUnauthorized
	// Counter reset — should take a fresh full run of failures to lock out.
	for i := 0; i < 2; i++ {
		if w := fireLogin(r, "10.0.0.9"); w.Code != http.StatusUnauthorized {
			t.Fatalf("post-reset attempt %d: got %d", i, w.Code)
		}
	}
}

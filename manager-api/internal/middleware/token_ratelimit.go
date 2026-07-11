package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const tokenRatePruneThreshold = 4096

// TokenRateLimiter enforces a per-token fixed-window request cap (SND-31). Each
// token's limit travels in the request context (set by AuthMiddleware); a limit
// of 0 means unlimited. The window is one minute.
type TokenRateLimiter struct {
	mu      sync.Mutex
	windows map[string]*rateWindow
	now     func() time.Time
}

type rateWindow struct {
	count       int
	windowStart time.Time
}

// NewTokenRateLimiter builds a limiter. A nil now defaults to time.Now.
func NewTokenRateLimiter(now func() time.Time) *TokenRateLimiter {
	if now == nil {
		now = time.Now
	}
	return &TokenRateLimiter{windows: make(map[string]*rateWindow), now: now}
}

// allow records a hit for tokenID and reports whether it is within limit/min,
// plus seconds until the current window resets.
func (l *TokenRateLimiter) allow(tokenID string, limit int) (bool, int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.prune(now)

	w := l.windows[tokenID]
	if w == nil || now.Sub(w.windowStart) >= time.Minute {
		w = &rateWindow{windowStart: now}
		l.windows[tokenID] = w
	}
	w.count++
	retry := 60 - int(now.Sub(w.windowStart).Seconds())
	if retry < 1 {
		retry = 1
	}
	return w.count <= limit, retry
}

func (l *TokenRateLimiter) prune(now time.Time) {
	if len(l.windows) < tokenRatePruneThreshold {
		return
	}
	for id, w := range l.windows {
		if now.Sub(w.windowStart) >= time.Minute {
			delete(l.windows, id)
		}
	}
}

// Middleware enforces the per-token rate limit. Non-token requests and tokens
// with no limit pass through.
func (l *TokenRateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := AdminID(c)
		if !strings.HasPrefix(id, "apitoken:") {
			c.Next()
			return
		}
		limitV, ok := c.Get(ctxTokenRate)
		limit, _ := limitV.(int)
		if !ok || limit <= 0 {
			c.Next()
			return
		}
		allowed, retry := l.allow(strings.TrimPrefix(id, "apitoken:"), limit)
		if !allowed {
			c.Header("Retry-After", strconv.Itoa(retry))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate_limited"})
			return
		}
		c.Next()
	}
}

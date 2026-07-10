package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Defaults for login throttling (SND-3). The control plane has a single admin
// account, so one target = full compromise; failed logins must be throttled.
const (
	defaultLoginMaxFailures = 5
	defaultLoginLockout     = 15 * time.Minute
	defaultLoginWindow      = 15 * time.Minute
	loginPruneThreshold     = 1024
)

// LoginLimiter throttles repeated failed logins per source IP. It counts only
// failed attempts (HTTP 401 from the wrapped handler); a successful login
// clears the counter. After maxFailures failures within window, the IP is
// locked out for lockout and further attempts get 429 with Retry-After. Failed
// attempts are logged with the source IP.
type LoginLimiter struct {
	mu          sync.Mutex
	attempts    map[string]*ipAttempts
	maxFailures int
	lockout     time.Duration
	window      time.Duration
	now         func() time.Time
	log         *slog.Logger
}

type ipAttempts struct {
	failures    int
	windowStart time.Time
	lockedUntil time.Time
	lastSeen    time.Time
}

// NewLoginLimiter builds a limiter. A nil now defaults to time.Now, a nil log to
// slog.Default; non-positive tuning values fall back to the package defaults.
func NewLoginLimiter(maxFailures int, lockout, window time.Duration, now func() time.Time, log *slog.Logger) *LoginLimiter {
	if maxFailures <= 0 {
		maxFailures = defaultLoginMaxFailures
	}
	if lockout <= 0 {
		lockout = defaultLoginLockout
	}
	if window <= 0 {
		window = defaultLoginWindow
	}
	if now == nil {
		now = time.Now
	}
	if log == nil {
		log = slog.Default()
	}
	return &LoginLimiter{
		attempts:    make(map[string]*ipAttempts),
		maxFailures: maxFailures,
		lockout:     lockout,
		window:      window,
		now:         now,
		log:         log,
	}
}

// Middleware returns a gin handler that blocks locked-out IPs before the login
// handler runs and records the outcome afterward.
func (l *LoginLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := l.now()

		l.mu.Lock()
		l.prune(now)
		if st := l.attempts[ip]; st != nil && now.Before(st.lockedUntil) {
			retry := int(st.lockedUntil.Sub(now).Seconds()) + 1
			l.mu.Unlock()
			l.log.Warn("login blocked: too many failed attempts", "source_ip", ip, "retry_after_s", retry)
			c.Header("Retry-After", strconv.Itoa(retry))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "too_many_attempts",
			})
			return
		}
		l.mu.Unlock()

		c.Next()

		l.record(ip, now, c.Writer.Status())
	}
}

// record updates the per-IP counter based on the handler's response status.
func (l *LoginLimiter) record(ip string, now time.Time, status int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	switch {
	case status == http.StatusUnauthorized:
		st := l.attempts[ip]
		if st == nil {
			st = &ipAttempts{windowStart: now}
			l.attempts[ip] = st
		}
		if now.Sub(st.windowStart) > l.window {
			st.failures = 0
			st.windowStart = now
		}
		st.failures++
		st.lastSeen = now
		l.log.Warn("failed login attempt", "source_ip", ip, "failures", st.failures)
		if st.failures >= l.maxFailures {
			st.lockedUntil = now.Add(l.lockout)
			st.failures = 0
			st.windowStart = now
			l.log.Warn("login locked out", "source_ip", ip, "lockout_s", int(l.lockout.Seconds()))
		}
	case status < 400:
		// Successful auth clears any accumulated failures for this IP.
		delete(l.attempts, ip)
	default:
		if st := l.attempts[ip]; st != nil {
			st.lastSeen = now
		}
	}
}

// prune drops stale entries so a flood of distinct source IPs can't grow the map
// without bound. Runs only once the map is large enough to matter. Caller holds
// the mutex.
func (l *LoginLimiter) prune(now time.Time) {
	if len(l.attempts) < loginPruneThreshold {
		return
	}
	for ip, st := range l.attempts {
		if now.After(st.lockedUntil) && now.Sub(st.lastSeen) > l.window {
			delete(l.attempts, ip)
		}
	}
}

package middleware

import (
	"sync"
	"time"
)

// Per-account login backoff (SND-30). Complements the per-IP LoginLimiter: an
// attacker rotating source IPs against one username is slowed by a capped delay
// keyed on the account, and repeated failures raise a one-time alert. There is
// deliberately NO hard per-account lockout — the control plane can have a single
// admin, so a lockout would let an attacker deny the legitimate operator access.
const (
	defaultAccountAlertThreshold = 5
	accountBackoffStep           = 400 * time.Millisecond
	accountBackoffCap            = 3 * time.Second
	accountPruneThreshold        = 1024
)

// AccountFailureTracker tracks failed logins per username within a window.
type AccountFailureTracker struct {
	mu        sync.Mutex
	accounts  map[string]*acctState
	window    time.Duration
	threshold int
	now       func() time.Time
}

type acctState struct {
	failures    int
	windowStart time.Time
	alerted     bool
	lastSeen    time.Time
}

// NewAccountFailureTracker builds a tracker. Non-positive values use defaults.
func NewAccountFailureTracker(threshold int, window time.Duration, now func() time.Time) *AccountFailureTracker {
	if threshold <= 0 {
		threshold = defaultAccountAlertThreshold
	}
	if window <= 0 {
		window = defaultLoginWindow
	}
	if now == nil {
		now = time.Now
	}
	return &AccountFailureTracker{
		accounts:  make(map[string]*acctState),
		window:    window,
		threshold: threshold,
		now:       now,
	}
}

// RecordFailure counts a failed attempt for username and returns the backoff
// delay the caller should apply before responding, plus alert=true exactly once
// when the failure count first reaches the threshold within the window. An empty
// username is ignored (returns 0, false).
func (t *AccountFailureTracker) RecordFailure(username string) (delay time.Duration, alert bool) {
	if username == "" {
		return 0, false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.now()
	t.prune(now)

	st := t.accounts[username]
	if st == nil || now.Sub(st.windowStart) > t.window {
		st = &acctState{windowStart: now}
		t.accounts[username] = st
	}
	st.failures++
	st.lastSeen = now

	// Capped linear backoff after the first failure.
	if n := st.failures - 1; n > 0 {
		delay = time.Duration(n) * accountBackoffStep
		if delay > accountBackoffCap {
			delay = accountBackoffCap
		}
	}
	if st.failures >= t.threshold && !st.alerted {
		st.alerted = true
		alert = true
	}
	return delay, alert
}

// RecordSuccess clears the counter for a username on a successful login.
func (t *AccountFailureTracker) RecordSuccess(username string) {
	if username == "" {
		return
	}
	t.mu.Lock()
	delete(t.accounts, username)
	t.mu.Unlock()
}

// prune drops stale entries. Caller holds the mutex.
func (t *AccountFailureTracker) prune(now time.Time) {
	if len(t.accounts) < accountPruneThreshold {
		return
	}
	for u, st := range t.accounts {
		if now.Sub(st.lastSeen) > t.window {
			delete(t.accounts, u)
		}
	}
}

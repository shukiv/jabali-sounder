package middleware

import (
	"testing"
	"time"
)

func TestAccountBackoffAndAlert(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return now }
	tr := NewAccountFailureTracker(3, 15*time.Minute, clock)

	// First failure: no delay, no alert.
	if d, a := tr.RecordFailure("alice"); d != 0 || a {
		t.Fatalf("1st failure: delay=%v alert=%v", d, a)
	}
	// Second: one step of backoff, no alert yet.
	if d, a := tr.RecordFailure("alice"); d != accountBackoffStep || a {
		t.Fatalf("2nd failure: delay=%v alert=%v", d, a)
	}
	// Third reaches threshold -> alert once.
	if d, a := tr.RecordFailure("alice"); d != 2*accountBackoffStep || !a {
		t.Fatalf("3rd failure: delay=%v alert=%v", d, a)
	}
	// Fourth: still delaying, but alert does not fire again.
	if _, a := tr.RecordFailure("alice"); a {
		t.Fatal("alert must fire only once per window")
	}
	// A different account is tracked independently.
	if _, a := tr.RecordFailure("bob"); a {
		t.Fatal("bob should not be alerted from alice's failures")
	}
}

func TestAccountBackoffCapAndReset(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	tr := NewAccountFailureTracker(100, 15*time.Minute, func() time.Time { return now })
	var last time.Duration
	for i := 0; i < 50; i++ {
		last, _ = tr.RecordFailure("x")
	}
	if last != accountBackoffCap {
		t.Fatalf("backoff should cap at %v, got %v", accountBackoffCap, last)
	}
	// Success clears the counter -> next failure starts fresh (no delay).
	tr.RecordSuccess("x")
	if d, _ := tr.RecordFailure("x"); d != 0 {
		t.Fatalf("after success, first failure should have no delay, got %v", d)
	}
}

func TestAccountBackoffWindowRollover(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return now }
	tr := NewAccountFailureTracker(3, 10*time.Minute, clock)
	tr.RecordFailure("a")
	tr.RecordFailure("a")
	now = now.Add(11 * time.Minute) // window elapsed
	if d, a := tr.RecordFailure("a"); d != 0 || a {
		t.Fatalf("after window rollover, counter resets: delay=%v alert=%v", d, a)
	}
}

func TestAccountBackoffIgnoresEmpty(t *testing.T) {
	tr := NewAccountFailureTracker(1, time.Minute, nil)
	if d, a := tr.RecordFailure(""); d != 0 || a {
		t.Fatal("empty username must be ignored")
	}
}

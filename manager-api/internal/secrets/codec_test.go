package secrets

import (
	"errors"
	"testing"
)

// TestSealOpenNoKeyGuard covers SND-6: without a key the plaintext fallback is
// only available when explicitly allowed, and round-trips when it is.
func TestSealOpenNoKeyGuard(t *testing.T) {
	if _, err := SealSecret(nil, "secret", false); !errors.Is(err, ErrNoKey) {
		t.Fatalf("SealSecret(nil, ..., false) = %v, want ErrNoKey", err)
	}
	if _, err := OpenSecret(nil, []byte("00"), false); !errors.Is(err, ErrNoKey) {
		t.Fatalf("OpenSecret(nil, ..., false) = %v, want ErrNoKey", err)
	}

	enc, err := SealSecret(nil, "secret", true)
	if err != nil {
		t.Fatalf("SealSecret(nil, ..., true): %v", err)
	}
	got, err := OpenSecret(nil, enc, true)
	if err != nil {
		t.Fatalf("OpenSecret(nil, ..., true): %v", err)
	}
	if got != "secret" {
		t.Fatalf("round-trip = %q, want %q", got, "secret")
	}
}

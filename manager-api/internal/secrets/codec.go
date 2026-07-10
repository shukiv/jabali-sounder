package secrets

import (
	"encoding/hex"
	"fmt"
)

// ErrNoKey is returned by SealSecret/OpenSecret when no encryption key is
// present and the plaintext fallback is not permitted (SND-6).
var ErrNoKey = fmt.Errorf("no encryption key and plaintext fallback disabled")

// SealSecret encrypts a token secret for storage. With a key it AES-GCM seals
// the plaintext. Without a key it falls back to hex-encoded plaintext ONLY when
// allowPlaintext is true (dev); otherwise it returns ErrNoKey so production
// never silently stores credentials in the clear. This is the single place the
// hex fallback lives (SND-6).
func SealSecret(key *Key, plaintext string, allowPlaintext bool) ([]byte, error) {
	if key != nil {
		return key.Seal([]byte(plaintext))
	}
	if !allowPlaintext {
		return nil, ErrNoKey
	}
	return []byte(hex.EncodeToString([]byte(plaintext))), nil
}

// OpenSecret reverses SealSecret: AES-GCM open with a key, else hex-decode the
// plaintext fallback when allowPlaintext is true, else ErrNoKey.
func OpenSecret(key *Key, data []byte, allowPlaintext bool) (string, error) {
	if key != nil {
		plaintext, err := key.Open(data)
		if err != nil {
			return "", fmt.Errorf("open secret: %w", err)
		}
		return string(plaintext), nil
	}
	if !allowPlaintext {
		return "", ErrNoKey
	}
	decoded, err := hex.DecodeString(string(data))
	if err != nil {
		return "", fmt.Errorf("hex decode: %w", err)
	}
	return string(decoded), nil
}

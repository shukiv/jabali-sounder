// Package totp implements RFC 6238 time-based one-time passwords (TOTP over
// HMAC-SHA1, 6 digits, 30s period) for admin two-factor auth. Hand-rolled to
// avoid a dependency; compatible with Google Authenticator, Aegis, 1Password.
package totp

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6238 mandates HMAC-SHA1 for authenticator compatibility
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	period = 30
	digits = 6
)

var enc = base32.StdEncoding.WithPadding(base32.NoPadding)

// GenerateSecret returns a new base32-encoded 20-byte secret.
func GenerateSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("totp secret: %w", err)
	}
	return enc.EncodeToString(b), nil
}

// Code returns the 6-digit code for a secret at time t.
func Code(secret string, t time.Time) (string, error) {
	key, err := enc.DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return "", fmt.Errorf("totp decode secret: %w", err)
	}
	return hotp(key, uint64(t.Unix())/period), nil
}

// Validate reports whether code is valid for secret at t, allowing ±1 step of
// clock skew. Comparison is constant-time.
func Validate(secret, code string, t time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != digits {
		return false
	}
	key, err := enc.DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil {
		return false
	}
	counter := uint64(t.Unix()) / period
	for _, skew := range []int64{0, -1, 1} {
		want := hotp(key, uint64(int64(counter)+skew))
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

// OtpauthURL builds the otpauth:// URI for a QR code.
func OtpauthURL(issuer, account, secret string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprintf("%d", digits))
	q.Set("period", fmt.Sprintf("%d", period))
	return "otpauth://totp/" + label + "?" + q.Encode()
}

func hotp(key []byte, counter uint64) string {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)
	h := hmac.New(sha1.New, key)
	h.Write(buf)
	sum := h.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	val := (uint32(sum[off]&0x7f) << 24) |
		(uint32(sum[off+1]) << 16) |
		(uint32(sum[off+2]) << 8) |
		uint32(sum[off+3])
	return fmt.Sprintf("%0*d", digits, val%1_000_000)
}

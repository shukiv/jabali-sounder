package totp

import (
	"encoding/base32"
	"testing"
	"time"
)

// RFC 6238 SHA1 test vector: secret "12345678901234567890", T=59 -> 8-digit
// 94287082; the 6-digit code is the last six, 287082.
func TestRFC6238Vector(t *testing.T) {
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))
	got, err := Code(secret, time.Unix(59, 0))
	if err != nil {
		t.Fatalf("Code: %v", err)
	}
	if got != "287082" {
		t.Fatalf("code = %q, want 287082", got)
	}
}

func TestValidateWithinWindow(t *testing.T) {
	secret, err := GenerateSecret()
	if err != nil {
		t.Fatalf("GenerateSecret: %v", err)
	}
	now := time.Now()
	code, _ := Code(secret, now)

	if !Validate(secret, code, now) {
		t.Fatal("current code should validate")
	}
	// One step earlier still accepted (skew tolerance).
	if !Validate(secret, code, now.Add(30*time.Second)) {
		t.Fatal("code should validate one step later (skew)")
	}
	// Two steps away should not.
	if Validate(secret, code, now.Add(90*time.Second)) {
		t.Fatal("code should not validate two steps away")
	}
	if Validate(secret, "000000", now) && code != "000000" {
		t.Fatal("wrong code should not validate")
	}
}

func TestOtpauthURL(t *testing.T) {
	u := OtpauthURL("Jabali Sounder", "admin", "ABC")
	if u == "" || u[:15] != "otpauth://totp/" {
		t.Fatalf("bad otpauth url: %q", u)
	}
}

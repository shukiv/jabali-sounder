package repository

import "testing"

func TestTokenIPAllowed(t *testing.T) {
	if !TokenIPAllowed(nil, "1.2.3.4") {
		t.Fatal("empty allowlist must permit any IP")
	}
	if !TokenIPAllowed([]string{"1.2.3.4"}, "1.2.3.4") {
		t.Fatal("exact IP must match")
	}
	if TokenIPAllowed([]string{"1.2.3.4"}, "1.2.3.5") {
		t.Fatal("non-listed IP must be denied")
	}
	if !TokenIPAllowed([]string{"10.0.0.0/8"}, "10.11.12.13") {
		t.Fatal("CIDR must match")
	}
	if TokenIPAllowed([]string{"10.0.0.0/8"}, "11.0.0.1") {
		t.Fatal("outside CIDR must be denied")
	}
	if !TokenIPAllowed([]string{"192.168.1.0/24", "203.0.113.7"}, "203.0.113.7") {
		t.Fatal("mixed list must match exact IP")
	}
}

func TestTokenScopeGrantsAll(t *testing.T) {
	if !TokenScopeGrantsAll(nil) {
		t.Fatal("empty grants all")
	}
	if !TokenScopeGrantsAll([]string{"read:*"}) {
		t.Fatal("read:* grants all")
	}
	if TokenScopeGrantsAll([]string{"fleet"}) {
		t.Fatal("specific scope does not grant all")
	}
}

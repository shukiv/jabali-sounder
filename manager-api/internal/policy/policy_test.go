package policy

import (
	"testing"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

func has(vs []Violation, serverID, check string) bool {
	for _, v := range vs {
		if v.ServerID == serverID && v.Check == check {
			return true
		}
	}
	return false
}

func TestEvaluate(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	expired := now.Add(-24 * time.Hour)
	soon := now.Add(3 * 24 * time.Hour)
	servers := []models.Server{
		{ID: "A", Name: "a", Status: models.ServerStatusActive, CredentialStatus: models.CredentialValid, Version: "v1", InsecureSkipVerify: true},
		{ID: "B", Name: "b", Status: models.ServerStatusActive, CredentialStatus: models.CredentialValid, Version: "v1"},
		{ID: "C", Name: "c", Status: models.ServerStatusActive, CredentialStatus: models.CredentialValid, Version: "v1"},
		{ID: "D", Name: "d", Status: models.ServerStatusUnreachable, CredentialStatus: models.CredentialInvalid, Version: "v2", CertExpiresAt: &expired},
		{ID: "E", Name: "e", Status: models.ServerStatusActive, CredentialStatus: models.CredentialValid, Version: "v1", CertExpiresAt: &soon},
		{ID: "F", Name: "f", Status: models.ServerStatusDisabled, InsecureSkipVerify: true}, // skipped
	}
	vs := Evaluate(servers, now, Options{CertWarnDays: 14})

	if !has(vs, "A", CheckInsecureTLS) {
		t.Fatal("A insecure_tls expected")
	}
	if !has(vs, "D", CheckCredentialInvalid) || !has(vs, "D", CheckUnreachable) || !has(vs, "D", CheckCertExpiring) {
		t.Fatal("D should have invalid-cred + unreachable + expired-cert")
	}
	if !has(vs, "D", CheckVersionDrift) {
		t.Fatal("D (v2) drifts from majority v1")
	}
	if has(vs, "B", CheckVersionDrift) {
		t.Fatal("B is on majority, no drift")
	}
	if !has(vs, "E", CheckCertExpiring) {
		t.Fatal("E cert expiring soon expected")
	}
	// Disabled server F must be skipped entirely.
	for _, v := range vs {
		if v.ServerID == "F" {
			t.Fatal("disabled server must not be evaluated")
		}
	}
}

func TestMajorityTieNoDrift(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	servers := []models.Server{
		{ID: "A", Status: models.ServerStatusActive, Version: "v1"},
		{ID: "B", Status: models.ServerStatusActive, Version: "v2"},
	}
	vs := Evaluate(servers, now, Options{})
	for _, v := range vs {
		if v.Check == CheckVersionDrift {
			t.Fatal("a tie has no majority -> no version drift")
		}
	}
}

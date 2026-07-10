package api

import (
	"net/http"
	"testing"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/remote"
)

// TestEnrollmentGate covers the rule that a server is only enrolled when it is
// reachable AND its automation credentials are valid — a reachable panel that
// rejects the token must NOT be added to the managed list.
func TestEnrollmentGate(t *testing.T) {
	cases := []struct {
		name   string
		check  *remote.CheckResult
		status int
		code   string
		ok     bool
	}{
		{"unreachable", &remote.CheckResult{Reachable: false}, http.StatusUnprocessableEntity, "server_unreachable", false},
		{"nil result", nil, http.StatusUnprocessableEntity, "server_unreachable", false},
		{"reachable bad token", &remote.CheckResult{Reachable: true, CredentialValid: false}, http.StatusUnprocessableEntity, "invalid_credentials", false},
		{"reachable valid token", &remote.CheckResult{Reachable: true, CredentialValid: true}, http.StatusCreated, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, code, ok := enrollmentGate(tc.check)
			if status != tc.status || code != tc.code || ok != tc.ok {
				t.Fatalf("enrollmentGate = (%d, %q, %v), want (%d, %q, %v)", status, code, ok, tc.status, tc.code, tc.ok)
			}
		})
	}
}

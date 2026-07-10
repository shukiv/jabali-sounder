package api

import "testing"

// TestValidatePublicTarget covers SND-4: private/loopback/link-local/CGNAT/IPv6
// internal addresses are rejected; public addresses pass; allowPrivate bypasses.
func TestValidatePublicTarget(t *testing.T) {
	reject := []string{
		"https://127.0.0.1:8443",
		"https://10.0.0.5:8443",
		"https://192.168.1.10:8443",
		"https://172.16.4.4:8443",
		"https://169.254.1.1:8443",
		"https://100.64.0.1:8443",
		"https://[::1]:8443",
		"https://0.0.0.0:8443",
	}
	for _, u := range reject {
		if err := validatePublicTarget(u, false); err == nil {
			t.Errorf("validatePublicTarget(%q, false) = nil, want rejection", u)
		}
	}

	if err := validatePublicTarget("https://8.8.8.8:8443", false); err != nil {
		t.Errorf("public IP should pass: %v", err)
	}
	// allowPrivate bypasses the guard (internal-panel deployments).
	if err := validatePublicTarget("https://127.0.0.1:8443", true); err != nil {
		t.Errorf("allowPrivate should bypass: %v", err)
	}
}

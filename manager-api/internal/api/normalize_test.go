package api

import "testing"

// TestNormalizePanelBaseURLPreservesPort covers issue #109: a user-supplied
// port must be preserved, not silently rewritten to 8443. Absent a port, 8443
// is the default.
func TestNormalizePanelBaseURLPreservesPort(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"scheme custom port", "https://panel.example.com:9000", "https://panel.example.com:9000"},
		{"scheme default port", "https://panel.example.com", "https://panel.example.com:8443"},
		{"scheme explicit 8443", "https://panel.example.com:8443", "https://panel.example.com:8443"},
		{"bare host custom port", "panel.example.com:9000", "https://panel.example.com:9000"},
		{"bare host no port", "panel.example.com", "https://panel.example.com:8443"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizePanelBaseURL(tc.in)
			if err != nil {
				t.Fatalf("normalizePanelBaseURL(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("normalizePanelBaseURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizePanelBaseURLRejectsBad(t *testing.T) {
	bad := []string{"", "http://panel.example.com", "https://panel.example.com/admin", "panel.example.com/x"}
	for _, in := range bad {
		if _, err := normalizePanelBaseURL(in); err == nil {
			t.Errorf("normalizePanelBaseURL(%q) should have errored", in)
		}
	}
}

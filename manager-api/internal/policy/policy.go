// Package policy evaluates enrolled servers against fleet compliance rules and
// reports drift (SND-32): weak TLS, invalid credentials, unreachability, cert
// expiry, and version drift from the fleet majority. It reads only attributes
// Sounder already tracks, so it needs no extra panel calls.
package policy

import (
	"fmt"
	"sort"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

// Check identifiers.
const (
	CheckInsecureTLS       = "insecure_tls"
	CheckCredentialInvalid = "credential_invalid"
	CheckUnreachable       = "unreachable"
	CheckCertExpiring      = "cert_expiring"
	CheckVersionDrift      = "version_drift"
)

// Violation is a single policy breach for a server.
type Violation struct {
	ServerID   string `json:"server_id"`
	ServerName string `json:"server_name"`
	Check      string `json:"check"`
	Severity   string `json:"severity"`
	Message    string `json:"message"`
}

// Options tunes evaluation.
type Options struct {
	CertWarnDays int // default 14
}

// Evaluate returns all policy violations across the given servers. Disabled
// servers are skipped (an operator deliberately paused them).
func Evaluate(servers []models.Server, now time.Time, opts Options) []Violation {
	if opts.CertWarnDays <= 0 {
		opts.CertWarnDays = 14
	}
	majority := majorityVersion(servers)
	var out []Violation
	for _, s := range servers {
		if s.Status == models.ServerStatusDisabled {
			continue
		}
		if s.InsecureSkipVerify {
			out = append(out, v(s, CheckInsecureTLS, models.SeverityCritical,
				"TLS certificate verification is disabled for this panel"))
		}
		if s.CredentialStatus == models.CredentialInvalid {
			out = append(out, v(s, CheckCredentialInvalid, models.SeverityCritical,
				"automation credential is invalid"))
		}
		if s.Status == models.ServerStatusUnreachable {
			out = append(out, v(s, CheckUnreachable, models.SeverityWarning,
				"server is unreachable"))
		}
		if s.CertExpiresAt != nil {
			days := int(s.CertExpiresAt.Sub(now).Hours() / 24)
			switch {
			case days < 0:
				out = append(out, v(s, CheckCertExpiring, models.SeverityCritical, "TLS certificate has expired"))
			case days < opts.CertWarnDays:
				out = append(out, v(s, CheckCertExpiring, models.SeverityWarning,
					fmt.Sprintf("TLS certificate expires in %d days", days)))
			}
		}
		if majority != "" && s.Version != "" && s.Version != majority {
			out = append(out, v(s, CheckVersionDrift, models.SeverityInfo,
				fmt.Sprintf("version %s differs from fleet majority %s", s.Version, majority)))
		}
	}
	return out
}

func v(s models.Server, check, severity, msg string) Violation {
	return Violation{ServerID: s.ID, ServerName: s.Name, Check: check, Severity: severity, Message: msg}
}

// majorityVersion returns the most common non-empty version, or "" if there is
// no clear majority (tie or no data).
func majorityVersion(servers []models.Server) string {
	counts := map[string]int{}
	for _, s := range servers {
		if s.Status == models.ServerStatusDisabled || s.Version == "" {
			continue
		}
		counts[s.Version]++
	}
	if len(counts) == 0 {
		return ""
	}
	type kv struct {
		version string
		n       int
	}
	list := make([]kv, 0, len(counts))
	for ver, n := range counts {
		list = append(list, kv{ver, n})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].n != list[j].n {
			return list[i].n > list[j].n
		}
		return list[i].version < list[j].version
	})
	if len(list) > 1 && list[0].n == list[1].n {
		return "" // tie -> no majority to drift from
	}
	return list[0].version
}

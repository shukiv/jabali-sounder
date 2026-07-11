package remote

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"time"
)

const certDialTimeout = 10 * time.Second

// PeerCertNotAfter dials the managed panel's TLS port and returns the leaf
// certificate's expiry. Verification is skipped on purpose — we only want the
// dates, and panels commonly use self-signed certs (roadmap M1: cert-expiry).
func PeerCertNotAfter(baseURL string) (time.Time, error) {
	u, err := url.Parse(baseURL)
	if err != nil || u.Hostname() == "" {
		return time.Time{}, fmt.Errorf("cert: invalid base url")
	}
	host := u.Host
	if u.Port() == "" {
		host = net.JoinHostPort(u.Hostname(), "443")
	}
	conn, err := tls.DialWithDialer(
		&net.Dialer{Timeout: certDialTimeout},
		"tcp", host,
		&tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // reading cert dates only; self-signed panels are expected
			ServerName:         u.Hostname(),
		},
	)
	if err != nil {
		return time.Time{}, fmt.Errorf("cert dial: %w", err)
	}
	defer func() { _ = conn.Close() }()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return time.Time{}, fmt.Errorf("cert: no peer certificate")
	}
	return certs[0].NotAfter, nil
}

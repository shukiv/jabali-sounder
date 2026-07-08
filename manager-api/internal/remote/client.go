// Package remote provides the outbound client that calls each managed Jabali
// Panel server's automation API with HMAC request signing.
//
// The signature scheme is defined by jabali2's automation_hmac.go middleware:
//
//	Authorization: Jabali-HMAC kid=<token-id>, ts=<unix>, sig=<hex>
//	sig = hex(HMAC_SHA256(secret, METHOD + "\n" + RequestURI + "\n" + ts + "\n" + hex(sha256(BODY))))
//
// where RequestURI is the full request URI including the raw query string
// (path + "?" + rawquery). Signing only the path would 401 any request
// carrying query params.
package remote

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	defaultTimeout = 10 * time.Second
	maxBody        = 1 << 20 // 1 MiB — mirrors jabali2's autoMaxBody
)

// insecureSkipVerify controls TLS certificate verification for outbound calls
// to managed servers. Default false (verify certs). HMAC authenticates
// requests but NOT responses, so skipping verification exposes the data plane
// to MITM — only enable via config for panels using self-signed certs.
// Configured once at startup via SetInsecureSkipVerify before any client is
// built, so it needs no synchronization.
var insecureSkipVerify = false

// SetInsecureSkipVerify sets the TLS verification policy for clients created
// afterwards. Call once at startup from config.
func SetInsecureSkipVerify(v bool) { insecureSkipVerify = v }

// Client is an authenticated HTTP client for one managed Jabali server.
type Client struct {
	baseURL string
	tokenID string
	secret  string
	http    *http.Client
}

// NewClient returns a remote client for the given server.
func NewClient(baseURL, tokenID, secret string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		tokenID: tokenID,
		secret:  secret,
		http: &http.Client{
			Timeout: defaultTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: insecureSkipVerify, //nolint:gosec // opt-in via [remote].insecure_skip_verify for self-signed panels; default false
				},
			},
		},
	}
}

// Do signs and sends an HTTP request to the managed server.
// pathAndQuery must start with "/" and include the raw query string if any
// (e.g. "/api/v1/automation/logs?service=nginx&since=2026-01-01").
func (c *Client) Do(ctx context.Context, method, pathAndQuery string, body []byte) (*http.Response, error) {
	url := c.baseURL + pathAndQuery

	var bodyReader io.Reader
	if body != nil {
		bodyReader = strings.NewReader(string(body))
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("remote: new request: %w", err)
	}

	// Sign the request. The signed string is METHOD + "\n" + RequestURI + "\n" + ts + "\n" + hex(sha256(body)).
	// RequestURI = path + "?" + rawquery (req.URL.RequestURI()).
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	bodyHash := sha256.Sum256(body)
	mac := hmac.New(sha256.New, []byte(c.secret))
	mac.Write([]byte(method))
	mac.Write([]byte("\n"))
	mac.Write([]byte(req.URL.RequestURI()))
	mac.Write([]byte("\n"))
	mac.Write([]byte(ts))
	mac.Write([]byte("\n"))
	mac.Write([]byte(hex.EncodeToString(bodyHash[:])))
	sig := hex.EncodeToString(mac.Sum(nil))

	req.Header.Set("Authorization", fmt.Sprintf("Jabali-HMAC kid=%s, ts=%s, sig=%s", c.tokenID, ts, sig))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.http.Do(req)
}

// Get is a convenience wrapper for GET requests with no body.
func (c *Client) Get(ctx context.Context, pathAndQuery string) (*http.Response, error) {
	return c.Do(ctx, http.MethodGet, pathAndQuery, nil)
}

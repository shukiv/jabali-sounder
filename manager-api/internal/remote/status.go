package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// HealthResp is the /health endpoint response from a managed server.
type HealthResp struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

// AutomationStatusResp is the /api/v1/automation/status response.
type AutomationStatusResp struct {
	Healthy bool   `json:"healthy"`
	Time    string `json:"time"`
}

// Health calls GET /health on the managed server. Does NOT require
// authentication — it's the unauthenticated liveness probe.
func (c *Client) Health(ctx context.Context) (*HealthResp, int, error) {
	resp, err := c.Get(ctx, "/health")
	if err != nil {
		return nil, 0, fmt.Errorf("health: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("health: HTTP %d", resp.StatusCode)
	}
	var h HealthResp
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("health decode: %w", err)
	}
	return &h, resp.StatusCode, nil
}

// AutomationStatus calls GET /api/v1/automation/status (HMAC-signed).
// Returns the status + HTTP code (200 = valid credentials, 401 = invalid).
func (c *Client) AutomationStatus(ctx context.Context) (*AutomationStatusResp, int, error) {
	resp, err := c.Get(ctx, "/api/v1/automation/status")
	if err != nil {
		return nil, 0, fmt.Errorf("automation status: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("automation status: HTTP %d", resp.StatusCode)
	}
	var s AutomationStatusResp
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("automation status decode: %w", err)
	}
	return &s, resp.StatusCode, nil
}

// CheckHealth performs both /health and /automation/status, returning a
// combined result suitable for the health loop + dashboard.
func (c *Client) CheckHealth(ctx context.Context) (*CheckResult, error) {
	result := &CheckResult{}

	h, hcode, err := c.Health(ctx)
	if err != nil {
		result.Reachable = false
		result.HealthError = err.Error()
		result.HealthCode = hcode
		return result, nil //nolint:nilerr // reachable=false is a result, not an error
	}
	result.Reachable = true
	result.HealthCode = hcode
	result.Version = h.Version

	s, scode, err := c.AutomationStatus(ctx)
	if err != nil {
		result.CredentialValid = false
		result.StatusError = err.Error()
		result.StatusCode = scode
		return result, nil
	}
	result.CredentialValid = true
	result.StatusCode = scode
	result.Healthy = s.Healthy

	return result, nil
}

// CheckResult is the combined health-check outcome for one server.
type CheckResult struct {
	Reachable       bool   `json:"reachable"`
	Healthy         bool   `json:"healthy"`
	CredentialValid bool   `json:"credential_valid"`
	Version         string `json:"version"`
	HealthCode      int    `json:"health_code"`
	StatusCode      int    `json:"status_code"`
	HealthError     string `json:"health_error,omitempty"`
	StatusError     string `json:"status_error,omitempty"`
}

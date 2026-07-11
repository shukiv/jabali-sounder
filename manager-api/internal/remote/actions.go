package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// ActionResult is the common envelope returned by write-automation endpoints.
type ActionResult struct {
	OK          bool   `json:"ok"`
	OperationID string `json:"operation_id,omitempty"`
	Status      string `json:"status,omitempty"`
	Message     string `json:"message,omitempty"`
	Error       string `json:"error,omitempty"`
}

// OperationStatus is the state of an async write operation (e.g. a backup).
type OperationStatus struct {
	Status     string `json:"status"`
	Message    string `json:"message,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
}

// Capabilities describes which write actions + scopes a managed panel supports.
type Capabilities struct {
	Version string   `json:"version"`
	Actions []string `json:"actions"`
	Scopes  []string `json:"scopes"`
}

const automationBase = "/api/v1/automation"

// post signs+sends a POST and decodes the ActionResult. A non-2xx status yields
// an error carrying the panel's error code when present.
func (c *Client) post(ctx context.Context, path string, body any) (*ActionResult, error) {
	var raw []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("action marshal: %w", err)
		}
		raw = b
	}
	resp, err := c.Do(ctx, http.MethodPost, path, raw)
	if err != nil {
		return nil, fmt.Errorf("action request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result ActionResult
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if resp.StatusCode >= 300 {
		msg := result.Error
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return &result, fmt.Errorf("action rejected: %s", msg)
	}
	return &result, nil
}

// RestartService restarts a managed service (write:services).
func (c *Client) RestartService(ctx context.Context, name string) (*ActionResult, error) {
	return c.post(ctx, automationBase+"/services/"+url.PathEscape(name)+"/restart", nil)
}

// SetUserEnabled disables or enables a user (write:users).
func (c *Client) SetUserEnabled(ctx context.Context, userID string, enabled bool) (*ActionResult, error) {
	verb := "disable"
	if enabled {
		verb = "enable"
	}
	return c.post(ctx, automationBase+"/users/"+url.PathEscape(userID)+"/"+verb, nil)
}

// SetDomainSuspended suspends or unsuspends a domain (write:domains).
func (c *Client) SetDomainSuspended(ctx context.Context, domainID string, suspended bool) (*ActionResult, error) {
	verb := "unsuspend"
	if suspended {
		verb = "suspend"
	}
	return c.post(ctx, automationBase+"/domains/"+url.PathEscape(domainID)+"/"+verb, nil)
}

// PurgeCache purges cache; scope is "all" or "domain" (write:cache).
func (c *Client) PurgeCache(ctx context.Context, scope, domain string) (*ActionResult, error) {
	body := map[string]string{"scope": scope}
	if domain != "" {
		body["domain"] = domain
	}
	return c.post(ctx, automationBase+"/cache/purge", body)
}

// CreateBackup triggers a backup; returns a 202 with an operation_id (write:backups).
func (c *Client) CreateBackup(ctx context.Context, targets []string) (*ActionResult, error) {
	return c.post(ctx, automationBase+"/backups", map[string]any{"targets": targets})
}

// OperationStatus polls an async operation (e.g. a backup).
func (c *Client) OperationStatus(ctx context.Context, opID string) (*OperationStatus, int, error) {
	resp, err := c.Get(ctx, automationBase+"/operations/"+url.PathEscape(opID))
	if err != nil {
		return nil, 0, fmt.Errorf("operation status: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("operation status: HTTP %d", resp.StatusCode)
	}
	var op OperationStatus
	if err := json.NewDecoder(resp.Body).Decode(&op); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("operation status decode: %w", err)
	}
	return &op, resp.StatusCode, nil
}

// Capabilities returns the panel's supported write actions + scopes.
func (c *Client) Capabilities(ctx context.Context) (*Capabilities, int, error) {
	resp, err := c.Get(ctx, automationBase+"/capabilities")
	if err != nil {
		return nil, 0, fmt.Errorf("capabilities: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("capabilities: HTTP %d", resp.StatusCode)
	}
	var caps Capabilities
	if err := json.NewDecoder(resp.Body).Decode(&caps); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("capabilities decode: %w", err)
	}
	return &caps, resp.StatusCode, nil
}

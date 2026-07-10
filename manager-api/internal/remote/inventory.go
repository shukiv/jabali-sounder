package remote

import (
	"context"
	"fmt"
	"net/http"
)

// Domain is a thinned domain record from a managed server's automation API.
type Domain struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	UserID    string `json:"user_id"`
	IsEnabled bool   `json:"is_enabled"`
}

// DomainListResp is the {data,total} envelope from /api/v1/automation/domains.
type DomainListResp struct {
	Data  []Domain `json:"data"`
	Total int      `json:"total"`
}

// Domains calls GET /api/v1/automation/domains on the managed server.
func (c *Client) Domains(ctx context.Context) (*DomainListResp, int, error) {
	resp, err := c.Get(ctx, "/api/v1/automation/domains")
	if err != nil {
		return nil, 0, fmt.Errorf("domains: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("domains: HTTP %d", resp.StatusCode)
	}
	var result DomainListResp
	if err := decodeJSONBody(resp, &result); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("domains decode: %w", err)
	}
	return &result, resp.StatusCode, nil
}

// User is a thinned user record from a managed server's automation API.
type User struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Username  string `json:"username"`
	PackageID string `json:"package_id"`
	IsAdmin   bool   `json:"is_admin"`
}

// UserListResp is the {data,total} envelope from /api/v1/automation/users.
type UserListResp struct {
	Data  []User `json:"data"`
	Total int    `json:"total"`
}

// Users calls GET /api/v1/automation/users on the managed server.
func (c *Client) Users(ctx context.Context) (*UserListResp, int, error) {
	resp, err := c.Get(ctx, "/api/v1/automation/users")
	if err != nil {
		return nil, 0, fmt.Errorf("users: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("users: HTTP %d", resp.StatusCode)
	}
	var result UserListResp
	if err := decodeJSONBody(resp, &result); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("users decode: %w", err)
	}
	return &result, resp.StatusCode, nil
}

// Application is a thinned application record from a managed server's automation API.
type Application struct {
	ID       string `json:"id"`
	AppType  string `json:"app_type"`
	DomainID string `json:"domain_id"`
	Status   string `json:"status"`
}

// ApplicationListResp is the {data,total} envelope from /api/v1/automation/applications.
type ApplicationListResp struct {
	Data  []Application `json:"data"`
	Total int           `json:"total"`
}

// Applications calls GET /api/v1/automation/applications on the managed server.
func (c *Client) Applications(ctx context.Context) (*ApplicationListResp, int, error) {
	resp, err := c.Get(ctx, "/api/v1/automation/applications")
	if err != nil {
		return nil, 0, fmt.Errorf("applications: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("applications: HTTP %d", resp.StatusCode)
	}
	var result ApplicationListResp
	if err := decodeJSONBody(resp, &result); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("applications decode: %w", err)
	}
	return &result, resp.StatusCode, nil
}

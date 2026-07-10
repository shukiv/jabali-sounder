package remote

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Mailbox is a thinned mailbox record from a managed server's automation API.
type Mailbox struct {
	ID             string `json:"id"`
	DomainID       string `json:"domain_id"`
	Email          string `json:"email"`
	DisplayName    string `json:"display_name"`
	QuotaBytes     uint64 `json:"quota_bytes"`
	IsDisabled     bool   `json:"is_disabled"`
	LastUsageBytes uint64 `json:"last_usage_bytes"`
	LastUsageAt    string `json:"last_usage_at,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	DomainName     string `json:"domain_name"`
	OwnerUserID    string `json:"owner_user_id"`
	UserUsername   string `json:"user_username"`
}

// MailboxListResp is the list envelope from /api/v1/automation/mail/mailboxes.
type MailboxListResp struct {
	Data  []Mailbox `json:"data"`
	Total int       `json:"total"`
}

// Mailboxes calls GET /api/v1/automation/mail/mailboxes on the managed server.
//
// jabali2's automation endpoint (JAB-77) emits a thin shape with DIFFERENT
// field names than Sounder's output contract: {email, domain, owner,
// quota_bytes, last_usage_bytes, disabled}. Decode that wire shape, then map it
// onto Mailbox so the Sounder API keeps exposing domain_name/user_username/
// is_disabled to the UI.
func (c *Client) Mailboxes(ctx context.Context) (*MailboxListResp, int, error) {
	resp, err := c.Get(ctx, "/api/v1/automation/mail/mailboxes")
	if err != nil {
		return nil, 0, fmt.Errorf("mailboxes: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("mailboxes: HTTP %d", resp.StatusCode)
	}
	var wire struct {
		Data []struct {
			Email          string `json:"email"`
			Domain         string `json:"domain"`
			Owner          string `json:"owner"`
			QuotaBytes     uint64 `json:"quota_bytes"`
			LastUsageBytes uint64 `json:"last_usage_bytes"`
			Disabled       bool   `json:"disabled"`
		} `json:"data"`
		Total int `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("mailboxes decode: %w", err)
	}
	result := MailboxListResp{Total: wire.Total, Data: make([]Mailbox, 0, len(wire.Data))}
	for _, m := range wire.Data {
		result.Data = append(result.Data, Mailbox{
			Email:          m.Email,
			DomainName:     m.Domain,
			UserUsername:   m.Owner,
			QuotaBytes:     m.QuotaBytes,
			LastUsageBytes: m.LastUsageBytes,
			IsDisabled:     m.Disabled,
		})
	}
	return &result, resp.StatusCode, nil
}

// MailGroup is a thinned mail group record from a managed server's automation API.
type MailGroup struct {
	ID             string `json:"id"`
	DomainID       string `json:"domain_id"`
	LocalPart      string `json:"local_part"`
	Email          string `json:"email"`
	DisplayName    string `json:"display_name"`
	Description    string `json:"description"`
	GroupKind      string `json:"group_kind"`
	HasMailbox     bool   `json:"has_mailbox"`
	HasCalendar    bool   `json:"has_calendar"`
	HasAddressbook bool   `json:"has_addressbook"`
	HasFiles       bool   `json:"has_files"`
	InternalOnly   bool   `json:"internal_only"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	DomainName     string `json:"domain_name"`
	OwnerUserID    string `json:"owner_user_id"`
	UserUsername   string `json:"user_username"`
	MemberCount    int64  `json:"member_count"`
}

// MailGroupListResp is the list envelope from /api/v1/automation/mail/groups.
type MailGroupListResp struct {
	Data  []MailGroup `json:"data"`
	Total int         `json:"total"`
}

// MailGroups calls GET /api/v1/automation/mail/groups on the managed server.
func (c *Client) MailGroups(ctx context.Context) (*MailGroupListResp, int, error) {
	resp, err := c.Get(ctx, "/api/v1/automation/mail/groups")
	if err != nil {
		return nil, 0, fmt.Errorf("mail groups: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("mail groups: HTTP %d", resp.StatusCode)
	}
	var result MailGroupListResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("mail groups decode: %w", err)
	}
	return &result, resp.StatusCode, nil
}

// MailForwarder is a mailbox-level forwarder record.
type MailForwarder struct {
	ID           string `json:"id"`
	MailboxID    string `json:"mailbox_id"`
	MailboxEmail string `json:"mailbox_email"`
	DomainID     string `json:"domain_id"`
	DomainName   string `json:"domain_name"`
	Type         string `json:"type"`
	LocalPart    string `json:"local_part,omitempty"`
	Target       string `json:"target"`
	KeepCopy     bool   `json:"keep_copy"`
	Enabled      bool   `json:"enabled"`
	CreatedAt    string `json:"created_at"`
}

// MailForwarderListResp is the list envelope from /api/v1/automation/mail/forwarders.
type MailForwarderListResp struct {
	Data  []MailForwarder `json:"data"`
	Total int             `json:"total"`
}

// MailForwarders calls GET /api/v1/automation/mail/forwarders on the managed server.
func (c *Client) MailForwarders(ctx context.Context) (*MailForwarderListResp, int, error) {
	resp, err := c.Get(ctx, "/api/v1/automation/mail/forwarders")
	if err != nil {
		return nil, 0, fmt.Errorf("mail forwarders: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("mail forwarders: HTTP %d", resp.StatusCode)
	}
	var result MailForwarderListResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("mail forwarders decode: %w", err)
	}
	return &result, resp.StatusCode, nil
}

// DomainForwarder is a domain-scoped forwarder record.
type DomainForwarder struct {
	ID         string `json:"id"`
	DomainID   string `json:"domain_id"`
	DomainName string `json:"domain_name"`
	Type       string `json:"type"`
	LocalPart  string `json:"local_part"`
	Target     string `json:"target"`
	Enabled    bool   `json:"enabled"`
	ManagedBy  string `json:"managed_by"`
	CreatedAt  string `json:"created_at"`
}

// DomainForwarderListResp is the list envelope from /api/v1/automation/mail/domain-forwarders.
type DomainForwarderListResp struct {
	Data  []DomainForwarder `json:"data"`
	Total int               `json:"total"`
}

// DomainForwarders calls GET /api/v1/automation/mail/domain-forwarders on the managed server.
func (c *Client) DomainForwarders(ctx context.Context) (*DomainForwarderListResp, int, error) {
	resp, err := c.Get(ctx, "/api/v1/automation/mail/domain-forwarders")
	if err != nil {
		return nil, 0, fmt.Errorf("domain forwarders: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("domain forwarders: HTTP %d", resp.StatusCode)
	}
	var result DomainForwarderListResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("domain forwarders decode: %w", err)
	}
	return &result, resp.StatusCode, nil
}

// MailAutoresponder is an autoresponder record from a managed server.
type MailAutoresponder struct {
	MailboxID    string  `json:"mailbox_id"`
	MailboxEmail string  `json:"mailbox_email,omitempty"`
	DomainID     string  `json:"domain_id,omitempty"`
	DomainName   string  `json:"domain_name,omitempty"`
	Enabled      bool    `json:"enabled"`
	FromDate     *string `json:"from_date"`
	ToDate       *string `json:"to_date"`
	Subject      *string `json:"subject"`
	TextBody     *string `json:"text_body"`
	HTMLBody     *string `json:"html_body"`
	UpdatedAt    string  `json:"updated_at"`
}

// MailAutoresponderListResp is the list envelope from /api/v1/automation/mail/autoresponders.
type MailAutoresponderListResp struct {
	Data  []MailAutoresponder `json:"data"`
	Total int                 `json:"total"`
}

// MailAutoresponders calls GET /api/v1/automation/mail/autoresponders on the managed server.
func (c *Client) MailAutoresponders(ctx context.Context) (*MailAutoresponderListResp, int, error) {
	resp, err := c.Get(ctx, "/api/v1/automation/mail/autoresponders")
	if err != nil {
		return nil, 0, fmt.Errorf("mail autoresponders: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("mail autoresponders: HTTP %d", resp.StatusCode)
	}
	var result MailAutoresponderListResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("mail autoresponders decode: %w", err)
	}
	return &result, resp.StatusCode, nil
}

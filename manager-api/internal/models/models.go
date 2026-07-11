// Package models defines GORM structs, one per table.
package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

// ServerStatus enumerates enrollment status.
type ServerStatus string

const (
	ServerStatusActive      ServerStatus = "active"
	ServerStatusDisabled    ServerStatus = "disabled"
	ServerStatusUnreachable ServerStatus = "unreachable"
)

// CredentialStatus tracks whether the stored automation token still authenticates.
type CredentialStatus string

const (
	CredentialValid   CredentialStatus = "valid"
	CredentialInvalid CredentialStatus = "invalid"
	CredentialUnknown CredentialStatus = "unknown"
)

// Server is an enrolled Jabali Panel server managed by this control plane.
type Server struct {
	ID                 string           `gorm:"column:id;type:char(26);primaryKey" json:"id"`
	Name               string           `gorm:"column:name;type:varchar(200);not null;uniqueIndex" json:"name"`
	BaseURL            string           `gorm:"column:base_url;type:varchar(500);not null" json:"base_url"`
	TokenID            string           `gorm:"column:token_id;type:char(26);not null;uniqueIndex" json:"token_id"`
	TokenSecretEnc     []byte           `gorm:"column:token_secret_enc;type:blob" json:"-"`
	Scopes             JSONStringArray  `gorm:"column:scopes;type:text;serializer:json" json:"scopes"`
	Tags               JSONStringArray  `gorm:"column:tags;type:text;serializer:json" json:"tags"`
	InsecureSkipVerify bool             `gorm:"column:insecure_skip_verify;type:tinyint(1);not null;default:0" json:"insecure_skip_verify"`
	Version            string           `gorm:"column:version;type:varchar(50)" json:"version"`
	Capabilities       JSONStringArray  `gorm:"column:capabilities;type:text;serializer:json" json:"capabilities"`
	HealthURL          string           `gorm:"column:health_url;type:varchar(500)" json:"health_url"`
	Status             ServerStatus     `gorm:"column:status;type:varchar(32);not null" json:"status"`
	CredentialStatus   CredentialStatus `gorm:"column:credential_status;type:varchar(32);not null" json:"credential_status"`
	LastHeartbeatAt    sql.NullTime     `gorm:"column:last_heartbeat_at" json:"-"`
	LastCheckedAt      sql.NullTime     `gorm:"column:last_checked_at" json:"-"`
	CertExpiresAt      *time.Time       `gorm:"column:cert_expires_at" json:"cert_expires_at,omitempty"`
	CreatedAt          time.Time        `gorm:"column:created_at" json:"created_at"`
	UpdatedAt          time.Time        `gorm:"column:updated_at" json:"updated_at"`
	DisabledAt         sql.NullTime     `gorm:"column:disabled_at" json:"-"`
}

func (Server) TableName() string { return "servers" }

// Heartbeat is a recorded health check result for an enrolled server.
type Heartbeat struct {
	ID        string          `gorm:"column:id;type:char(26);primaryKey" json:"id"`
	ServerID  string          `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	Healthy   bool            `gorm:"column:healthy;type:tinyint(1)" json:"healthy"`
	Version   string          `gorm:"column:version;type:varchar(50)" json:"version"`
	Details   json.RawMessage `gorm:"column:details;type:json" json:"details"`
	CheckedAt time.Time       `gorm:"column:checked_at" json:"checked_at"`
}

func (Heartbeat) TableName() string { return "heartbeats" }

// MetricSample is a compact time-series sample of a managed server's
// resource usage, captured by the health poller (roadmap M1: trends).
type MetricSample struct {
	ID          string    `gorm:"column:id;type:char(26);primaryKey" json:"id"`
	ServerID    string    `gorm:"column:server_id;type:char(26);not null;index" json:"server_id"`
	CPUPercent  *float64  `gorm:"column:cpu_percent" json:"cpu_percent,omitempty"`
	RAMPercent  *float64  `gorm:"column:ram_percent" json:"ram_percent,omitempty"`
	DiskPercent *float64  `gorm:"column:disk_percent" json:"disk_percent,omitempty"`
	Load1       *float64  `gorm:"column:load1" json:"load1,omitempty"`
	SampledAt   time.Time `gorm:"column:sampled_at;index" json:"sampled_at"`
}

func (MetricSample) TableName() string { return "metric_samples" }

// Role is a Sounder operator's permission level (M3: RBAC).
type Role string

const (
	RoleViewer   Role = "viewer"   // read-only
	RoleOperator Role = "operator" // read + mutate servers
	RoleOwner    Role = "owner"    // operator + manage operators
)

// Rank orders roles for comparison; higher is more privileged.
func (r Role) Rank() int {
	switch r {
	case RoleOwner:
		return 3
	case RoleOperator:
		return 2
	case RoleViewer:
		return 1
	default:
		return 0
	}
}

// AtLeast reports whether r is at least as privileged as min.
func (r Role) AtLeast(min Role) bool { return r.Rank() >= min.Rank() }

// Valid reports whether r is a known role.
func (r Role) Valid() bool { return r.Rank() > 0 }

// Admin is a manager-side administrator who can log in and manage servers.
type Admin struct {
	ID            string    `gorm:"column:id;type:char(26);primaryKey" json:"id"`
	Username      string    `gorm:"column:username;type:varchar(100);not null;uniqueIndex" json:"username"`
	PasswordHash  string    `gorm:"column:password_hash;type:varchar(255);not null" json:"-"`
	Role          Role      `gorm:"column:role;type:varchar(20);not null;default:owner" json:"role"`
	TOTPSecretEnc []byte    `gorm:"column:totp_secret_enc;type:blob" json:"-"`
	TOTPEnabled   bool      `gorm:"column:totp_enabled;type:tinyint(1);not null;default:0" json:"two_factor_enabled"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Admin) TableName() string { return "admins" }

// Session is a server-side record of an issued login token, so sessions can be
// listed and revoked (M3). The JWT carries the session id; AuthMiddleware
// rejects revoked/expired sessions.
type Session struct {
	ID         string       `gorm:"column:id;type:char(26);primaryKey" json:"id"`
	AdminID    string       `gorm:"column:admin_id;type:char(26);not null;index" json:"admin_id"`
	UserAgent  string       `gorm:"column:user_agent;type:varchar(400)" json:"user_agent"`
	IP         string       `gorm:"column:ip;type:varchar(64)" json:"ip"`
	CreatedAt  time.Time    `gorm:"column:created_at" json:"created_at"`
	LastSeenAt time.Time    `gorm:"column:last_seen_at" json:"last_seen_at"`
	ExpiresAt  time.Time    `gorm:"column:expires_at" json:"expires_at"`
	RevokedAt  sql.NullTime `gorm:"column:revoked_at" json:"-"`
}

func (Session) TableName() string { return "sessions" }

// APIToken is a read-only credential for external tooling to call Sounder's
// read endpoints without the SPA login (M4). Only the secret hash is stored;
// the token grants viewer role.
type APIToken struct {
	ID         string       `gorm:"column:id;type:char(26);primaryKey" json:"id"`
	Name       string       `gorm:"column:name;type:varchar(200);not null" json:"name"`
	SecretHash string       `gorm:"column:secret_hash;type:char(64);not null" json:"-"`
	CreatedBy  string       `gorm:"column:created_by;type:char(26)" json:"created_by"`
	CreatedAt  time.Time    `gorm:"column:created_at" json:"created_at"`
	LastUsedAt sql.NullTime `gorm:"column:last_used_at" json:"-"`
	ExpiresAt  sql.NullTime `gorm:"column:expires_at" json:"-"`
	RevokedAt  sql.NullTime `gorm:"column:revoked_at" json:"-"`
}

func (APIToken) TableName() string { return "api_tokens" }

// JSONStringArray is a []string stored as JSON in a column.
type JSONStringArray []string

func (a *JSONStringArray) Scan(src any) error {
	if src == nil {
		*a = JSONStringArray{}
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return nil
	}
	return json.Unmarshal(b, a)
}

func (a JSONStringArray) Value() (any, error) {
	if a == nil {
		a = JSONStringArray{}
	}
	b, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

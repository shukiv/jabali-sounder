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

// Admin is a manager-side administrator who can log in and manage servers.
type Admin struct {
	ID           string    `gorm:"column:id;type:char(26);primaryKey" json:"id"`
	Username     string    `gorm:"column:username;type:varchar(100);not null;uniqueIndex" json:"username"`
	PasswordHash string    `gorm:"column:password_hash;type:varchar(255);not null" json:"-"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (Admin) TableName() string { return "admins" }

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

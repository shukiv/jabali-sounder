package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

// tokenPrefix identifies a Sounder API token in an Authorization header.
const tokenPrefix = "snd_"

// APITokenRepository stores read-only API tokens (M4).
type APITokenRepository interface {
	// Mint creates a token and returns the one-time plaintext plus the record.
	Mint(ctx context.Context, name, createdBy string, expiresAt *time.Time) (string, *models.APIToken, error)
	List(ctx context.Context) ([]models.APIToken, error)
	Revoke(ctx context.Context, id string) error
	// Rotate issues a new secret for an existing token (same id/name/expiry),
	// invalidating the old secret; returns the new one-time plaintext.
	Rotate(ctx context.Context, id string) (string, *models.APIToken, error)
	// ListExpiring returns non-revoked tokens whose expiry falls in (now, before].
	ListExpiring(ctx context.Context, now, before time.Time) ([]models.APIToken, error)
	// Validate parses+verifies a presented token; returns the record or nil.
	Validate(ctx context.Context, presented string) *models.APIToken
}

type apiTokenRepo struct{ db *gorm.DB }

// NewAPITokenRepository returns a GORM-backed APITokenRepository.
func NewAPITokenRepository(db *gorm.DB) APITokenRepository {
	return &apiTokenRepo{db: db}
}

// FormatToken builds the presented token "snd_<id>_<secret>".
func FormatToken(id, secret string) string { return tokenPrefix + id + "_" + secret }

// HasTokenPrefix reports whether s looks like a Sounder API token.
func HasTokenPrefix(s string) bool { return strings.HasPrefix(s, tokenPrefix) }

func (r *apiTokenRepo) Mint(ctx context.Context, name, createdBy string, expiresAt *time.Time) (string, *models.APIToken, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("api token secret: %w", err)
	}
	secret := hex.EncodeToString(buf)
	sum := sha256.Sum256([]byte(secret))
	tok := &models.APIToken{
		ID:         ids.NewULID(),
		Name:       name,
		SecretHash: hex.EncodeToString(sum[:]),
		CreatedBy:  createdBy,
		CreatedAt:  time.Now(),
	}
	if expiresAt != nil {
		tok.ExpiresAt = sql.NullTime{Time: *expiresAt, Valid: true}
	}
	if err := r.db.WithContext(ctx).Create(tok).Error; err != nil {
		return "", nil, fmt.Errorf("api token create: %w", err)
	}
	return FormatToken(tok.ID, secret), tok, nil
}

func (r *apiTokenRepo) List(ctx context.Context) ([]models.APIToken, error) {
	var rows []models.APIToken
	if err := r.db.WithContext(ctx).
		Where("revoked_at IS NULL").
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("api token list: %w", err)
	}
	return rows, nil
}

func (r *apiTokenRepo) Revoke(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Model(&models.APIToken{}).
		Where("id = ?", id).
		Update("revoked_at", time.Now()).Error; err != nil {
		return fmt.Errorf("api token revoke: %w", err)
	}
	return nil
}

func (r *apiTokenRepo) Validate(ctx context.Context, presented string) *models.APIToken {
	if !HasTokenPrefix(presented) {
		return nil
	}
	parts := strings.SplitN(strings.TrimPrefix(presented, tokenPrefix), "_", 2)
	if len(parts) != 2 {
		return nil
	}
	id, secret := parts[0], parts[1]

	var tok models.APIToken
	if err := r.db.WithContext(ctx).First(&tok, "id = ?", id).Error; err != nil {
		return nil
	}
	if tok.RevokedAt.Valid || (tok.ExpiresAt.Valid && !tok.ExpiresAt.Time.After(time.Now())) {
		return nil
	}
	sum := sha256.Sum256([]byte(secret))
	if subtle.ConstantTimeCompare([]byte(hex.EncodeToString(sum[:])), []byte(tok.SecretHash)) != 1 {
		return nil
	}
	_ = r.db.WithContext(ctx).Model(&models.APIToken{}).Where("id = ?", id).Update("last_used_at", time.Now()).Error
	return &tok
}

func (r *apiTokenRepo) Rotate(ctx context.Context, id string) (string, *models.APIToken, error) {
	var tok models.APIToken
	if err := r.db.WithContext(ctx).First(&tok, "id = ? AND revoked_at IS NULL", id).Error; err != nil {
		return "", nil, fmt.Errorf("api token rotate lookup: %w", err)
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("api token secret: %w", err)
	}
	secret := hex.EncodeToString(buf)
	sum := sha256.Sum256([]byte(secret))
	if err := r.db.WithContext(ctx).Model(&models.APIToken{}).Where("id = ?", id).
		Updates(map[string]any{"secret_hash": hex.EncodeToString(sum[:]), "last_used_at": nil}).Error; err != nil {
		return "", nil, fmt.Errorf("api token rotate update: %w", err)
	}
	tok.SecretHash = hex.EncodeToString(sum[:])
	tok.LastUsedAt = sql.NullTime{}
	return FormatToken(id, secret), &tok, nil
}

func (r *apiTokenRepo) ListExpiring(ctx context.Context, now, before time.Time) ([]models.APIToken, error) {
	var rows []models.APIToken
	if err := r.db.WithContext(ctx).
		Where("revoked_at IS NULL AND expires_at IS NOT NULL AND expires_at > ? AND expires_at <= ?", now, before).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("api token list expiring: %w", err)
	}
	return rows, nil
}

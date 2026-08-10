package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

// WebAuthnCredentialRepository stores registered passkeys (SND-109).
type WebAuthnCredentialRepository interface {
	// ListByAdmin returns an admin's passkeys, newest first.
	ListByAdmin(ctx context.Context, adminID string) ([]models.WebAuthnCredential, error)
	Create(ctx context.Context, cred *models.WebAuthnCredential) error
	// Delete removes a passkey by id, scoped to its owner. Reports whether a row
	// was removed (false = not found / not owned).
	Delete(ctx context.Context, id, adminID string) (bool, error)
	// FindByCredentialID resolves a passkey by its raw credential id (login).
	FindByCredentialID(ctx context.Context, credentialID []byte) (*models.WebAuthnCredential, error)
	// UpdateData refreshes the stored credential blob (sign count) and last-used.
	UpdateData(ctx context.Context, id string, data []byte, lastUsed time.Time) error
}

type webauthnCredRepo struct{ db *gorm.DB }

// NewWebAuthnCredentialRepository returns a GORM-backed repository.
func NewWebAuthnCredentialRepository(db *gorm.DB) WebAuthnCredentialRepository {
	return &webauthnCredRepo{db: db}
}

func (r *webauthnCredRepo) ListByAdmin(ctx context.Context, adminID string) ([]models.WebAuthnCredential, error) {
	var rows []models.WebAuthnCredential
	if err := r.db.WithContext(ctx).
		Where("admin_id = ?", adminID).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("webauthn list: %w", err)
	}
	return rows, nil
}

func (r *webauthnCredRepo) Create(ctx context.Context, cred *models.WebAuthnCredential) error {
	if err := r.db.WithContext(ctx).Create(cred).Error; err != nil {
		return fmt.Errorf("webauthn create: %w", err)
	}
	return nil
}

func (r *webauthnCredRepo) Delete(ctx context.Context, id, adminID string) (bool, error) {
	res := r.db.WithContext(ctx).
		Where("id = ? AND admin_id = ?", id, adminID).
		Delete(&models.WebAuthnCredential{})
	if res.Error != nil {
		return false, fmt.Errorf("webauthn delete: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

func (r *webauthnCredRepo) FindByCredentialID(ctx context.Context, credentialID []byte) (*models.WebAuthnCredential, error) {
	var cred models.WebAuthnCredential
	if err := r.db.WithContext(ctx).First(&cred, "credential_id = ?", credentialID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("webauthn find: %w", err)
	}
	return &cred, nil
}

func (r *webauthnCredRepo) UpdateData(ctx context.Context, id string, data []byte, lastUsed time.Time) error {
	if err := r.db.WithContext(ctx).Model(&models.WebAuthnCredential{}).
		Where("id = ?", id).
		Updates(map[string]any{"data": data, "last_used_at": lastUsed}).Error; err != nil {
		return fmt.Errorf("webauthn update: %w", err)
	}
	return nil
}

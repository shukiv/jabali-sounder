// Package repository provides data access behind standard interfaces.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

// ErrNotFound is the sentinel for record-not-found, wrapping GORM's.
var ErrNotFound = errors.New("repository: not found")

// ServerRepository is the interface for server enrollment data access.
type ServerRepository interface {
	Create(ctx context.Context, s *models.Server) error
	FindByID(ctx context.Context, id string) (*models.Server, error)
	List(ctx context.Context) ([]models.Server, error)
	Update(ctx context.Context, s *models.Server) error
	UpdateStatus(ctx context.Context, id string, status models.ServerStatus, credStatus models.CredentialStatus) error
	// UpdateCertExpiry stores the managed panel TLS cert expiry (poller).
	UpdateCertExpiry(ctx context.Context, id string, expiresAt *time.Time) error
	// Delete hard-removes a server row (heartbeats cascade on MariaDB).
	Delete(ctx context.Context, id string) error
}

type serverRepo struct{ db *gorm.DB }

// NewServerRepository returns a GORM-backed ServerRepository.
func NewServerRepository(db *gorm.DB) ServerRepository {
	return &serverRepo{db: db}
}

func (r *serverRepo) Create(ctx context.Context, s *models.Server) error {
	if err := r.db.WithContext(ctx).Create(s).Error; err != nil {
		return fmt.Errorf("server create: %w", err)
	}
	return nil
}

func (r *serverRepo) FindByID(ctx context.Context, id string) (*models.Server, error) {
	var s models.Server
	if err := r.db.WithContext(ctx).First(&s, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("server find by id: %w", err)
	}
	normalizeServerCollections(&s)
	return &s, nil
}

func (r *serverRepo) List(ctx context.Context) ([]models.Server, error) {
	var rows []models.Server
	if err := r.db.WithContext(ctx).Order("name ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("server list: %w", err)
	}
	for i := range rows {
		normalizeServerCollections(&rows[i])
	}
	return rows, nil
}

func normalizeServerCollections(s *models.Server) {
	if s.Tags == nil {
		s.Tags = models.JSONStringArray{}
	}
}

func (r *serverRepo) Update(ctx context.Context, s *models.Server) error {
	if err := r.db.WithContext(ctx).Save(s).Error; err != nil {
		return fmt.Errorf("server update: %w", err)
	}
	return nil
}

func (r *serverRepo) UpdateStatus(ctx context.Context, id string, status models.ServerStatus, credStatus models.CredentialStatus) error {
	res := r.db.WithContext(ctx).Model(&models.Server{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":            status,
			"credential_status": credStatus,
		})
	if res.Error != nil {
		return fmt.Errorf("server update status: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *serverRepo) UpdateCertExpiry(ctx context.Context, id string, expiresAt *time.Time) error {
	if err := r.db.WithContext(ctx).Model(&models.Server{}).
		Where("id = ?", id).
		Update("cert_expires_at", expiresAt).Error; err != nil {
		return fmt.Errorf("server update cert expiry: %w", err)
	}
	return nil
}

func (r *serverRepo) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&models.Server{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("server delete: %w", err)
	}
	return nil
}

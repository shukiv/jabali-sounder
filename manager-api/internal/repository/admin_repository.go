package repository

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

// AdminRepository provides data access for admin users.
type AdminRepository interface {
	FindByUsername(ctx context.Context, username string) (*models.Admin, error)
	Count(ctx context.Context) (int64, error)
	Create(ctx context.Context, a *models.Admin) error
	Update(ctx context.Context, a *models.Admin) error
}

type adminRepo struct{ db *gorm.DB }

// NewAdminRepository returns a GORM-backed AdminRepository.
func NewAdminRepository(db *gorm.DB) AdminRepository {
	return &adminRepo{db: db}
}

func (r *adminRepo) FindByUsername(ctx context.Context, username string) (*models.Admin, error) {
	var a models.Admin
	if err := r.db.WithContext(ctx).First(&a, "username = ?", username).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("admin find by username: %w", err)
	}
	return &a, nil
}

func (r *adminRepo) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.Admin{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("admin count: %w", err)
	}
	return count, nil
}

func (r *adminRepo) Create(ctx context.Context, a *models.Admin) error {
	if err := r.db.WithContext(ctx).Create(a).Error; err != nil {
		return fmt.Errorf("admin create: %w", err)
	}
	return nil
}

func (r *adminRepo) Update(ctx context.Context, a *models.Admin) error {
	if err := r.db.WithContext(ctx).Save(a).Error; err != nil {
		return fmt.Errorf("admin update: %w", err)
	}
	return nil
}

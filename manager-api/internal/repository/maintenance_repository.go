package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

// MaintenanceRepository stores alert-suppression windows (SND-22).
type MaintenanceRepository interface {
	List(ctx context.Context) ([]models.MaintenanceWindow, error)
	Create(ctx context.Context, w *models.MaintenanceWindow) error
	Delete(ctx context.Context, id string) error
	// ActiveForServer reports whether any window currently covers the server
	// (by id), its environment, or the whole fleet.
	ActiveForServer(ctx context.Context, serverID, environment string, now time.Time) (bool, error)
	PruneExpired(ctx context.Context, before time.Time) (int64, error)
}

type maintenanceRepo struct{ db *gorm.DB }

// NewMaintenanceRepository returns a GORM-backed MaintenanceRepository.
func NewMaintenanceRepository(db *gorm.DB) MaintenanceRepository {
	return &maintenanceRepo{db: db}
}

func (r *maintenanceRepo) List(ctx context.Context) ([]models.MaintenanceWindow, error) {
	var rows []models.MaintenanceWindow
	if err := r.db.WithContext(ctx).Order("starts_at DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("maintenance list: %w", err)
	}
	return rows, nil
}

func (r *maintenanceRepo) Create(ctx context.Context, w *models.MaintenanceWindow) error {
	if err := r.db.WithContext(ctx).Create(w).Error; err != nil {
		return fmt.Errorf("maintenance create: %w", err)
	}
	return nil
}

func (r *maintenanceRepo) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&models.MaintenanceWindow{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("maintenance delete: %w", err)
	}
	return nil
}

func (r *maintenanceRepo) ActiveForServer(ctx context.Context, serverID, environment string, now time.Time) (bool, error) {
	q := r.db.WithContext(ctx).Model(&models.MaintenanceWindow{}).
		Where("starts_at <= ? AND ends_at >= ?", now, now).
		Where(
			r.db.Where("scope_type = ?", "global").
				Or("scope_type = ? AND scope_value = ?", "server", serverID).
				Or("scope_type = ? AND scope_value = ?", "environment", environment),
		)
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false, fmt.Errorf("maintenance active: %w", err)
	}
	return count > 0, nil
}

func (r *maintenanceRepo) PruneExpired(ctx context.Context, before time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Where("ends_at < ?", before).Delete(&models.MaintenanceWindow{})
	if res.Error != nil {
		return 0, fmt.Errorf("maintenance prune: %w", res.Error)
	}
	return res.RowsAffected, nil
}

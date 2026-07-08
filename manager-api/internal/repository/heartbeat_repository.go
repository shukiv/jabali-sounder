package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

// HeartbeatRepository stores health check results.
type HeartbeatRepository interface {
	Record(ctx context.Context, hb *models.Heartbeat) error
	Latest(ctx context.Context, serverID string) (*models.Heartbeat, error)
	Recent(ctx context.Context, serverID string, n int) ([]models.Heartbeat, error)
}

type heartbeatRepo struct{ db *gorm.DB }

// NewHeartbeatRepository returns a GORM-backed HeartbeatRepository.
func NewHeartbeatRepository(db *gorm.DB) HeartbeatRepository {
	return &heartbeatRepo{db: db}
}

func (r *heartbeatRepo) Record(ctx context.Context, hb *models.Heartbeat) error {
	if err := r.db.WithContext(ctx).Create(hb).Error; err != nil {
		return fmt.Errorf("heartbeat record: %w", err)
	}
	// Also stamp the server's last_heartbeat_at.
	if err := r.db.WithContext(ctx).Model(&models.Server{}).
		Where("id = ?", hb.ServerID).
		Update("last_heartbeat_at", hb.CheckedAt).Error; err != nil {
		return fmt.Errorf("heartbeat stamp server: %w", err)
	}
	return nil
}

func (r *heartbeatRepo) Latest(ctx context.Context, serverID string) (*models.Heartbeat, error) {
	var hb models.Heartbeat
	if err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("checked_at DESC").
		First(&hb).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("heartbeat latest: %w", err)
	}
	return &hb, nil
}

func (r *heartbeatRepo) Recent(ctx context.Context, serverID string, n int) ([]models.Heartbeat, error) {
	var rows []models.Heartbeat
	if err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("checked_at DESC").
		Limit(n).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("heartbeat recent: %w", err)
	}
	return rows, nil
}

// Ensure the time import is used.
var _ = time.Now

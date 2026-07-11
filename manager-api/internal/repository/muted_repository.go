package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

// MutedAlertRepository silences specific (server, kind) alerts (SND-21).
type MutedAlertRepository interface {
	List(ctx context.Context) ([]models.MutedAlert, error)
	IsMuted(ctx context.Context, serverID, kind string) (bool, error)
	Mute(ctx context.Context, serverID, kind, by string, now time.Time) error
	Unmute(ctx context.Context, serverID, kind string) error
}

type mutedRepo struct{ db *gorm.DB }

// NewMutedAlertRepository returns a GORM-backed MutedAlertRepository.
func NewMutedAlertRepository(db *gorm.DB) MutedAlertRepository {
	return &mutedRepo{db: db}
}

func (r *mutedRepo) List(ctx context.Context) ([]models.MutedAlert, error) {
	var rows []models.MutedAlert
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("muted list: %w", err)
	}
	return rows, nil
}

func (r *mutedRepo) IsMuted(ctx context.Context, serverID, kind string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.MutedAlert{}).
		Where("server_id = ? AND kind = ?", serverID, kind).Count(&count).Error; err != nil {
		return false, fmt.Errorf("muted check: %w", err)
	}
	return count > 0, nil
}

func (r *mutedRepo) Mute(ctx context.Context, serverID, kind, by string, now time.Time) error {
	m := &models.MutedAlert{ID: ids.NewULID(), ServerID: serverID, Kind: kind, CreatedBy: by, CreatedAt: now}
	err := r.db.WithContext(ctx).Create(m).Error
	if err != nil && errors.Is(err, gorm.ErrDuplicatedKey) {
		return nil // already muted; idempotent
	}
	if err != nil {
		return fmt.Errorf("mute: %w", err)
	}
	return nil
}

func (r *mutedRepo) Unmute(ctx context.Context, serverID, kind string) error {
	if err := r.db.WithContext(ctx).
		Where("server_id = ? AND kind = ?", serverID, kind).
		Delete(&models.MutedAlert{}).Error; err != nil {
		return fmt.Errorf("unmute: %w", err)
	}
	return nil
}

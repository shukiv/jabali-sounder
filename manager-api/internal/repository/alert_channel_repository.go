package repository

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

// AlertChannelRepository stores alert delivery destinations (SND-20). ConfigEnc
// is sealed by the caller; this repo is storage-only.
type AlertChannelRepository interface {
	List(ctx context.Context) ([]models.AlertChannel, error)
	ListEnabled(ctx context.Context) ([]models.AlertChannel, error)
	Get(ctx context.Context, id string) (*models.AlertChannel, error)
	Create(ctx context.Context, c *models.AlertChannel) error
	Update(ctx context.Context, c *models.AlertChannel) error
	Delete(ctx context.Context, id string) error
}

type alertChannelRepo struct{ db *gorm.DB }

// NewAlertChannelRepository returns a GORM-backed AlertChannelRepository.
func NewAlertChannelRepository(db *gorm.DB) AlertChannelRepository {
	return &alertChannelRepo{db: db}
}

func (r *alertChannelRepo) List(ctx context.Context) ([]models.AlertChannel, error) {
	var rows []models.AlertChannel
	if err := r.db.WithContext(ctx).Order("created_at ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("alert channel list: %w", err)
	}
	return rows, nil
}

func (r *alertChannelRepo) ListEnabled(ctx context.Context) ([]models.AlertChannel, error) {
	var rows []models.AlertChannel
	if err := r.db.WithContext(ctx).Where("enabled = ?", true).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("alert channel list enabled: %w", err)
	}
	return rows, nil
}

func (r *alertChannelRepo) Get(ctx context.Context, id string) (*models.AlertChannel, error) {
	var c models.AlertChannel
	if err := r.db.WithContext(ctx).First(&c, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("alert channel get: %w", err)
	}
	return &c, nil
}

func (r *alertChannelRepo) Create(ctx context.Context, c *models.AlertChannel) error {
	if err := r.db.WithContext(ctx).Create(c).Error; err != nil {
		return fmt.Errorf("alert channel create: %w", err)
	}
	return nil
}

// Update persists name/type/min_severity/enabled and, when non-nil, config_enc.
func (r *alertChannelRepo) Update(ctx context.Context, c *models.AlertChannel) error {
	fields := map[string]any{
		"name":         c.Name,
		"type":         c.Type,
		"min_severity": c.MinSeverity,
		"enabled":      c.Enabled,
	}
	if c.ConfigEnc != nil {
		fields["config_enc"] = c.ConfigEnc
	}
	res := r.db.WithContext(ctx).Model(&models.AlertChannel{}).Where("id = ?", c.ID).Updates(fields)
	if res.Error != nil {
		return fmt.Errorf("alert channel update: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("alert channel update: not found")
	}
	return nil
}

func (r *alertChannelRepo) Delete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&models.AlertChannel{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("alert channel delete: %w", err)
	}
	return nil
}

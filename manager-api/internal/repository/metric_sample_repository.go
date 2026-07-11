package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

// MetricSampleRepository stores resource-usage time-series samples.
type MetricSampleRepository interface {
	Record(ctx context.Context, m *models.MetricSample) error
	Recent(ctx context.Context, serverID string, n int) ([]models.MetricSample, error)
	PruneOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

type metricSampleRepo struct{ db *gorm.DB }

// NewMetricSampleRepository returns a GORM-backed MetricSampleRepository.
func NewMetricSampleRepository(db *gorm.DB) MetricSampleRepository {
	return &metricSampleRepo{db: db}
}

func (r *metricSampleRepo) Record(ctx context.Context, m *models.MetricSample) error {
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return fmt.Errorf("metric sample record: %w", err)
	}
	return nil
}

func (r *metricSampleRepo) Recent(ctx context.Context, serverID string, n int) ([]models.MetricSample, error) {
	var rows []models.MetricSample
	if err := r.db.WithContext(ctx).
		Where("server_id = ?", serverID).
		Order("sampled_at DESC").
		Limit(n).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("metric sample recent: %w", err)
	}
	return rows, nil
}

func (r *metricSampleRepo) PruneOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Where("sampled_at < ?", cutoff).Delete(&models.MetricSample{})
	if res.Error != nil {
		return 0, fmt.Errorf("metric sample prune: %w", res.Error)
	}
	return res.RowsAffected, nil
}

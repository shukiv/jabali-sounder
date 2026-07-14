package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

// AlertRuleRepository stores fleet-wide metric thresholds (SND-20).
type AlertRuleRepository interface {
	List(ctx context.Context) ([]models.AlertRule, error)
	ListEnabled(ctx context.Context) ([]models.AlertRule, error)
	Update(ctx context.Context, r *models.AlertRule) error
	EnsureDefaults(ctx context.Context, now time.Time) error
}

type alertRuleRepo struct{ db *gorm.DB }

// NewAlertRuleRepository returns a GORM-backed AlertRuleRepository.
func NewAlertRuleRepository(db *gorm.DB) AlertRuleRepository {
	return &alertRuleRepo{db: db}
}

func (r *alertRuleRepo) List(ctx context.Context) ([]models.AlertRule, error) {
	var rows []models.AlertRule
	if err := r.db.WithContext(ctx).Order("metric ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("alert rule list: %w", err)
	}
	return rows, nil
}

func (r *alertRuleRepo) ListEnabled(ctx context.Context) ([]models.AlertRule, error) {
	var rows []models.AlertRule
	if err := r.db.WithContext(ctx).Where("enabled = ?", true).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("alert rule list enabled: %w", err)
	}
	return rows, nil
}

// Update persists threshold/duration/severity/enabled for an existing metric
// rule, keyed by metric (rules are created only via EnsureDefaults).
func (r *alertRuleRepo) Update(ctx context.Context, rule *models.AlertRule) error {
	rule.UpdatedAt = time.Now().UTC()
	res := r.db.WithContext(ctx).Model(&models.AlertRule{}).
		Where("metric = ?", rule.Metric).
		Updates(map[string]any{
			"threshold":        rule.Threshold,
			"duration_seconds": rule.DurationSeconds,
			"severity":         rule.Severity,
			"enabled":          rule.Enabled,
			"updated_at":       rule.UpdatedAt,
		})
	if res.Error != nil {
		return fmt.Errorf("alert rule update: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("alert rule update: unknown metric %q", rule.Metric)
	}
	return nil
}

// defaultRules preserves the pre-M5 behaviour (CPU>80% for 60s) and adds
// sensible RAM/disk/load defaults, disabled where noisy by default.
func defaultRules(now time.Time) []models.AlertRule {
	mk := func(metric string, thr float64, dur int, sev string, on bool) models.AlertRule {
		return models.AlertRule{
			ID: ids.NewULID(), Metric: metric, Threshold: thr, DurationSeconds: dur,
			Severity: sev, Enabled: on, CreatedAt: now, UpdatedAt: now,
		}
	}
	return []models.AlertRule{
		mk("cpu", 80, 60, models.SeverityCritical, true),
		mk("ram", 90, 120, models.SeverityWarning, true),
		mk("disk", 90, 0, models.SeverityWarning, true),
		mk("load1", 8, 120, models.SeverityWarning, false),
		// Any managed-server service reported not-healthy for 2 min (SND: service-down).
		mk("service_down", 0, 120, models.SeverityCritical, true),
	}
}

// EnsureDefaults inserts any missing default rule (idempotent, keyed by metric).
func (r *alertRuleRepo) EnsureDefaults(ctx context.Context, now time.Time) error {
	for _, rule := range defaultRules(now) {
		var count int64
		if err := r.db.WithContext(ctx).Model(&models.AlertRule{}).
			Where("metric = ?", rule.Metric).Count(&count).Error; err != nil {
			return fmt.Errorf("alert rule ensure count: %w", err)
		}
		if count > 0 {
			continue
		}
		if err := r.db.WithContext(ctx).Create(&rule).Error; err != nil {
			return fmt.Errorf("alert rule ensure create: %w", err)
		}
	}
	return nil
}

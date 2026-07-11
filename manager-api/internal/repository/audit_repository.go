package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

// AuditFilter narrows an audit query. Zero values mean "no constraint".
type AuditFilter struct {
	Actor    string
	Event    string
	ServerID string
	Since    time.Time
	Limit    int
	Offset   int
}

// AuditRepository persists and queries privileged-mutation records (SND-24).
type AuditRepository interface {
	Create(ctx context.Context, a *models.AuditLog) error
	List(ctx context.Context, f AuditFilter) ([]models.AuditLog, error)
	Count(ctx context.Context, f AuditFilter) (int64, error)
	PruneOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

type auditRepo struct{ db *gorm.DB }

// NewAuditRepository returns a GORM-backed AuditRepository.
func NewAuditRepository(db *gorm.DB) AuditRepository {
	return &auditRepo{db: db}
}

func (r *auditRepo) Create(ctx context.Context, a *models.AuditLog) error {
	if err := r.db.WithContext(ctx).Create(a).Error; err != nil {
		return fmt.Errorf("audit create: %w", err)
	}
	return nil
}

func (r *auditRepo) applyFilter(q *gorm.DB, f AuditFilter) *gorm.DB {
	if f.Actor != "" {
		q = q.Where("actor = ?", f.Actor)
	}
	if f.Event != "" {
		q = q.Where("event = ?", f.Event)
	}
	if f.ServerID != "" {
		q = q.Where("server_id = ?", f.ServerID)
	}
	if !f.Since.IsZero() {
		q = q.Where("created_at >= ?", f.Since)
	}
	return q
}

func (r *auditRepo) List(ctx context.Context, f AuditFilter) ([]models.AuditLog, error) {
	q := r.applyFilter(r.db.WithContext(ctx).Model(&models.AuditLog{}), f).Order("created_at DESC")
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	if f.Offset > 0 {
		q = q.Offset(f.Offset)
	}
	var rows []models.AuditLog
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("audit list: %w", err)
	}
	return rows, nil
}

func (r *auditRepo) Count(ctx context.Context, f AuditFilter) (int64, error) {
	var n int64
	if err := r.applyFilter(r.db.WithContext(ctx).Model(&models.AuditLog{}), f).Count(&n).Error; err != nil {
		return 0, fmt.Errorf("audit count: %w", err)
	}
	return n, nil
}

func (r *auditRepo) PruneOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Where("created_at < ?", cutoff).Delete(&models.AuditLog{})
	if res.Error != nil {
		return 0, fmt.Errorf("audit prune: %w", res.Error)
	}
	return res.RowsAffected, nil
}

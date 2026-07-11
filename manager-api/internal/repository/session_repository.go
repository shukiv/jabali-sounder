package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

// SessionRepository stores issued-login records for listing + revocation (M3).
type SessionRepository interface {
	Create(ctx context.Context, sess *models.Session) error
	FindByID(ctx context.Context, id string) (*models.Session, error)
	ListActiveByAdmin(ctx context.Context, adminID string) ([]models.Session, error)
	Revoke(ctx context.Context, id string) error
	// Active reports whether a session is usable (exists, not revoked, not
	// expired) and stamps last_seen_at. Used by AuthMiddleware.
	Active(ctx context.Context, id string) bool
	PruneExpired(ctx context.Context, now time.Time) (int64, error)
}

type sessionRepo struct{ db *gorm.DB }

// NewSessionRepository returns a GORM-backed SessionRepository.
func NewSessionRepository(db *gorm.DB) SessionRepository {
	return &sessionRepo{db: db}
}

func (r *sessionRepo) Create(ctx context.Context, sess *models.Session) error {
	if err := r.db.WithContext(ctx).Create(sess).Error; err != nil {
		return fmt.Errorf("session create: %w", err)
	}
	return nil
}

func (r *sessionRepo) FindByID(ctx context.Context, id string) (*models.Session, error) {
	var sess models.Session
	if err := r.db.WithContext(ctx).First(&sess, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("session find: %w", err)
	}
	return &sess, nil
}

func (r *sessionRepo) ListActiveByAdmin(ctx context.Context, adminID string) ([]models.Session, error) {
	var rows []models.Session
	if err := r.db.WithContext(ctx).
		Where("admin_id = ? AND revoked_at IS NULL AND expires_at > ?", adminID, time.Now()).
		Order("last_seen_at DESC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("session list: %w", err)
	}
	return rows, nil
}

func (r *sessionRepo) Revoke(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Model(&models.Session{}).
		Where("id = ?", id).
		Update("revoked_at", time.Now()).Error; err != nil {
		return fmt.Errorf("session revoke: %w", err)
	}
	return nil
}

func (r *sessionRepo) Active(ctx context.Context, id string) bool {
	if id == "" {
		return false
	}
	var sess models.Session
	if err := r.db.WithContext(ctx).First(&sess, "id = ?", id).Error; err != nil {
		return false
	}
	if sess.RevokedAt.Valid || !sess.ExpiresAt.After(time.Now()) {
		return false
	}
	_ = r.db.WithContext(ctx).Model(&models.Session{}).Where("id = ?", id).Update("last_seen_at", time.Now()).Error
	return true
}

func (r *sessionRepo) PruneExpired(ctx context.Context, now time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Where("expires_at < ?", now).Delete(&models.Session{})
	if res.Error != nil {
		return 0, fmt.Errorf("session prune: %w", res.Error)
	}
	return res.RowsAffected, nil
}

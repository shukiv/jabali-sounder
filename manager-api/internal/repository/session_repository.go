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
	// RevokeAllForAdmin revokes every active session belonging to an admin.
	// Used by local password reset so an old token cannot outlive the reset.
	RevokeAllForAdmin(ctx context.Context, adminID string) (int64, error)
	// Active reports whether a session is usable (exists, not revoked, not
	// expired) and stamps last_seen_at. Used by AuthMiddleware. A non-nil error
	// signals an infrastructure failure (e.g. a transient SQLite lock), NOT an
	// inactive session; callers must not treat it as a logout.
	Active(ctx context.Context, id string) (bool, error)
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

func (r *sessionRepo) RevokeAllForAdmin(ctx context.Context, adminID string) (int64, error) {
	res := r.db.WithContext(ctx).Model(&models.Session{}).
		Where("admin_id = ? AND revoked_at IS NULL", adminID).
		Update("revoked_at", time.Now())
	if res.Error != nil {
		return 0, fmt.Errorf("session revoke all: %w", res.Error)
	}
	return res.RowsAffected, nil
}

// sessionTouchInterval throttles last_seen_at writes. Stamping it on every
// request amplified write load and, on SQLite, drove the lock contention that
// randomly logged users out (#5).
const sessionTouchInterval = time.Minute

func (r *sessionRepo) Active(ctx context.Context, id string) (bool, error) {
	if id == "" {
		return false, nil
	}
	var sess models.Session
	if err := r.db.WithContext(ctx).First(&sess, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		// Transient/infra error (e.g. SQLite "database is locked"). Surface it so
		// the middleware fails soft (503) instead of forcing a logout.
		return false, fmt.Errorf("session active: %w", err)
	}
	if sess.RevokedAt.Valid || !sess.ExpiresAt.After(time.Now()) {
		return false, nil
	}
	if time.Since(sess.LastSeenAt) > sessionTouchInterval {
		_ = r.db.WithContext(ctx).Model(&models.Session{}).Where("id = ?", id).Update("last_seen_at", time.Now()).Error
	}
	return true, nil
}

func (r *sessionRepo) PruneExpired(ctx context.Context, now time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Where("expires_at < ?", now).Delete(&models.Session{})
	if res.Error != nil {
		return 0, fmt.Errorf("session prune: %w", res.Error)
	}
	return res.RowsAffected, nil
}

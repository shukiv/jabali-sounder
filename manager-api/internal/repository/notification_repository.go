package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

// NotificationRepository stores in-app fleet notifications (SND-18).
type NotificationRepository interface {
	Create(ctx context.Context, n *models.Notification) error
	ListRecent(ctx context.Context, limit int) ([]models.Notification, error)
	UnreadCount(ctx context.Context) (int64, error)
	MarkRead(ctx context.Context, id string) error
	MarkAllRead(ctx context.Context) error
	ActiveExists(ctx context.Context, serverID, kind string) (bool, error)
	ResolveActive(ctx context.Context, serverID, kind string) error
	PruneOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
	// Incident operations (SND-21).
	ListActive(ctx context.Context) ([]models.Notification, error)
	Ack(ctx context.Context, id, by string, now time.Time) error
	Snooze(ctx context.Context, id string, until time.Time) error
	// UnackedSince returns open, un-acked, un-snoozed incidents created at or
	// before `before` that have not yet been escalated — for escalation sweeps.
	UnackedSince(ctx context.Context, before, now time.Time) ([]models.Notification, error)
	MarkEscalated(ctx context.Context, id string, now time.Time) error
}

type notificationRepo struct{ db *gorm.DB }

// NewNotificationRepository returns a GORM-backed NotificationRepository.
func NewNotificationRepository(db *gorm.DB) NotificationRepository {
	return &notificationRepo{db: db}
}

func (r *notificationRepo) Create(ctx context.Context, n *models.Notification) error {
	if err := r.db.WithContext(ctx).Create(n).Error; err != nil {
		return fmt.Errorf("notification create: %w", err)
	}
	return nil
}

func (r *notificationRepo) ListRecent(ctx context.Context, limit int) ([]models.Notification, error) {
	var rows []models.Notification
	if err := r.db.WithContext(ctx).Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("notification list: %w", err)
	}
	return rows, nil
}

func (r *notificationRepo) UnreadCount(ctx context.Context) (int64, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&models.Notification{}).Where("read_at IS NULL").Count(&n).Error; err != nil {
		return 0, fmt.Errorf("notification unread count: %w", err)
	}
	return n, nil
}

func (r *notificationRepo) MarkRead(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Model(&models.Notification{}).Where("id = ? AND read_at IS NULL", id).Update("read_at", time.Now()).Error; err != nil {
		return fmt.Errorf("notification mark read: %w", err)
	}
	return nil
}

func (r *notificationRepo) MarkAllRead(ctx context.Context) error {
	if err := r.db.WithContext(ctx).Model(&models.Notification{}).Where("read_at IS NULL").Update("read_at", time.Now()).Error; err != nil {
		return fmt.Errorf("notification mark all read: %w", err)
	}
	return nil
}

func (r *notificationRepo) ActiveExists(ctx context.Context, serverID, kind string) (bool, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("server_id = ? AND kind = ? AND resolved_at IS NULL", serverID, kind).
		Count(&n).Error; err != nil {
		return false, fmt.Errorf("notification active exists: %w", err)
	}
	return n > 0, nil
}

func (r *notificationRepo) ResolveActive(ctx context.Context, serverID, kind string) error {
	if err := r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("server_id = ? AND kind = ? AND resolved_at IS NULL", serverID, kind).
		Update("resolved_at", time.Now()).Error; err != nil {
		return fmt.Errorf("notification resolve: %w", err)
	}
	return nil
}

func (r *notificationRepo) PruneOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Where("created_at < ? AND resolved_at IS NOT NULL", cutoff).Delete(&models.Notification{})
	if res.Error != nil {
		return 0, fmt.Errorf("notification prune: %w", res.Error)
	}
	return res.RowsAffected, nil
}

func (r *notificationRepo) ListActive(ctx context.Context) ([]models.Notification, error) {
	var rows []models.Notification
	if err := r.db.WithContext(ctx).
		Where("resolved_at IS NULL").
		Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("notification list active: %w", err)
	}
	return rows, nil
}

func (r *notificationRepo) Ack(ctx context.Context, id, by string, now time.Time) error {
	res := r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("id = ? AND acked_at IS NULL", id).
		Updates(map[string]any{"acked_at": now, "acked_by": by, "read_at": now})
	if res.Error != nil {
		return fmt.Errorf("notification ack: %w", res.Error)
	}
	return nil
}

func (r *notificationRepo) Snooze(ctx context.Context, id string, until time.Time) error {
	if err := r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("id = ?", id).
		Update("snoozed_until", until).Error; err != nil {
		return fmt.Errorf("notification snooze: %w", err)
	}
	return nil
}

func (r *notificationRepo) UnackedSince(ctx context.Context, before, now time.Time) ([]models.Notification, error) {
	var rows []models.Notification
	if err := r.db.WithContext(ctx).
		Where("resolved_at IS NULL AND acked_at IS NULL AND escalated_at IS NULL AND created_at <= ?", before).
		Where("snoozed_until IS NULL OR snoozed_until < ?", now).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("notification unacked since: %w", err)
	}
	return rows, nil
}

func (r *notificationRepo) MarkEscalated(ctx context.Context, id string, now time.Time) error {
	if err := r.db.WithContext(ctx).Model(&models.Notification{}).
		Where("id = ?", id).
		Update("escalated_at", now).Error; err != nil {
		return fmt.Errorf("notification mark escalated: %w", err)
	}
	return nil
}

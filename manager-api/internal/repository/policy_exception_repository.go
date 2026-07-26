package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

// PolicyExceptionRepository stores acknowledged compliance exceptions (SND-107).
type PolicyExceptionRepository interface {
	// ListActive returns exceptions that have not expired as of `now`.
	ListActive(ctx context.Context, now time.Time) ([]models.PolicyException, error)
	// Ignore upserts an exception for (serverID, check). A nil expiresAt never
	// expires. Re-ignoring updates the reason/expiry/actor.
	Ignore(ctx context.Context, serverID, check, reason string, expiresAt *time.Time, by string, now time.Time) error
	// Restore removes the exception for (serverID, check). Returns true if one
	// existed.
	Restore(ctx context.Context, serverID, check string) (bool, error)
	// PruneExpired deletes exceptions whose expiry has passed (housekeeping).
	PruneExpired(ctx context.Context, now time.Time) (int64, error)
}

type policyExceptionRepo struct{ db *gorm.DB }

// NewPolicyExceptionRepository returns a GORM-backed PolicyExceptionRepository.
func NewPolicyExceptionRepository(db *gorm.DB) PolicyExceptionRepository {
	return &policyExceptionRepo{db: db}
}

func (r *policyExceptionRepo) ListActive(ctx context.Context, now time.Time) ([]models.PolicyException, error) {
	var rows []models.PolicyException
	if err := r.db.WithContext(ctx).
		Where("expires_at IS NULL OR expires_at > ?", now.UTC()).
		Order("created_at DESC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("policy exception list: %w", err)
	}
	return rows, nil
}

func (r *policyExceptionRepo) Ignore(ctx context.Context, serverID, check, reason string, expiresAt *time.Time, by string, now time.Time) error {
	exp := sql.NullTime{}
	if expiresAt != nil {
		exp = sql.NullTime{Time: expiresAt.UTC(), Valid: true}
	}
	row := models.PolicyException{
		ID: ids.NewULID(), ServerID: serverID, Check: check, Reason: reason,
		ExpiresAt: exp, CreatedBy: by, CreatedAt: now.UTC(),
	}
	// Upsert on the (server_id, check_name) unique index: re-ignoring refreshes
	// the reason/expiry/actor rather than failing.
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "server_id"}, {Name: "check_name"}},
		DoUpdates: clause.AssignmentColumns([]string{"reason", "expires_at", "created_by", "created_at"}),
	}).Create(&row).Error; err != nil {
		return fmt.Errorf("policy exception ignore: %w", err)
	}
	return nil
}

func (r *policyExceptionRepo) Restore(ctx context.Context, serverID, check string) (bool, error) {
	res := r.db.WithContext(ctx).
		Where("server_id = ? AND check_name = ?", serverID, check).
		Delete(&models.PolicyException{})
	if res.Error != nil {
		return false, fmt.Errorf("policy exception restore: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

func (r *policyExceptionRepo) PruneExpired(ctx context.Context, now time.Time) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("expires_at IS NOT NULL AND expires_at <= ?", now.UTC()).
		Delete(&models.PolicyException{})
	if res.Error != nil {
		return 0, fmt.Errorf("policy exception prune: %w", res.Error)
	}
	return res.RowsAffected, nil
}

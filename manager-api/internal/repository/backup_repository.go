package repository

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

// BackupRepository stores backup-run history (SND-27).
type BackupRepository interface {
	Create(ctx context.Context, b *models.BackupRun) error
	ListRecent(ctx context.Context, limit int) ([]models.BackupRun, error)
	ListByServer(ctx context.Context, serverID string, limit int) ([]models.BackupRun, error)
	// NonTerminal returns runs still pending/running (for the status watcher).
	NonTerminal(ctx context.Context) ([]models.BackupRun, error)
	// LatestSuccess returns the most recent succeeded run for a server, or nil.
	LatestSuccess(ctx context.Context, serverID string) (*models.BackupRun, error)
	UpdateStatus(ctx context.Context, id, status, message string, finishedAt *time.Time) error
	PruneOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

type backupRepo struct{ db *gorm.DB }

// NewBackupRepository returns a GORM-backed BackupRepository.
func NewBackupRepository(db *gorm.DB) BackupRepository {
	return &backupRepo{db: db}
}

func (r *backupRepo) Create(ctx context.Context, b *models.BackupRun) error {
	if err := r.db.WithContext(ctx).Create(b).Error; err != nil {
		return fmt.Errorf("backup create: %w", err)
	}
	return nil
}

func (r *backupRepo) ListRecent(ctx context.Context, limit int) ([]models.BackupRun, error) {
	var rows []models.BackupRun
	if err := r.db.WithContext(ctx).Order("started_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("backup list recent: %w", err)
	}
	return rows, nil
}

func (r *backupRepo) ListByServer(ctx context.Context, serverID string, limit int) ([]models.BackupRun, error) {
	var rows []models.BackupRun
	if err := r.db.WithContext(ctx).Where("server_id = ?", serverID).
		Order("started_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("backup list by server: %w", err)
	}
	return rows, nil
}

func (r *backupRepo) NonTerminal(ctx context.Context) ([]models.BackupRun, error) {
	var rows []models.BackupRun
	if err := r.db.WithContext(ctx).
		Where("status IN ?", []string{models.BackupPending, models.BackupRunning}).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("backup non-terminal: %w", err)
	}
	return rows, nil
}

func (r *backupRepo) LatestSuccess(ctx context.Context, serverID string) (*models.BackupRun, error) {
	var b models.BackupRun
	err := r.db.WithContext(ctx).
		Where("server_id = ? AND status = ?", serverID, models.BackupSucceeded).
		Order("started_at DESC").First(&b).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("backup latest success: %w", err)
	}
	return &b, nil
}

func (r *backupRepo) UpdateStatus(ctx context.Context, id, status, message string, finishedAt *time.Time) error {
	fields := map[string]any{"status": status, "message": message}
	if finishedAt != nil {
		fields["finished_at"] = *finishedAt
	}
	if err := r.db.WithContext(ctx).Model(&models.BackupRun{}).Where("id = ?", id).Updates(fields).Error; err != nil {
		return fmt.Errorf("backup update: %w", err)
	}
	return nil
}

func (r *backupRepo) PruneOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	res := r.db.WithContext(ctx).
		Where("started_at < ? AND status IN ?", cutoff, []string{models.BackupSucceeded, models.BackupFailed}).
		Delete(&models.BackupRun{})
	if res.Error != nil {
		return 0, fmt.Errorf("backup prune: %w", res.Error)
	}
	return res.RowsAffected, nil
}

package api

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

// BackupHandlerConfig wires the backup-history endpoints (SND-27).
type BackupHandlerConfig struct {
	Repo repository.BackupRepository
	Log  *slog.Logger
}

// RegisterBackupRoutes mounts /api/v1/admin/backups (any authed role).
func RegisterBackupRoutes(g *gin.RouterGroup, cfg BackupHandlerConfig) {
	if cfg.Repo == nil {
		return
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	h := &backupHandler{cfg: cfg}
	g.GET("/admin/backups", h.list)
}

type backupHandler struct{ cfg BackupHandlerConfig }

func (h *backupHandler) list(c *gin.Context) {
	limit := 100
	if n, err := strconv.Atoi(c.DefaultQuery("limit", "100")); err == nil && n > 0 && n <= 500 {
		limit = n
	}
	var (
		runs []models.BackupRun
		err  error
	)
	if serverID := c.Query("server_id"); serverID != "" {
		runs, err = h.cfg.Repo.ListByServer(c.Request.Context(), serverID, limit)
	} else {
		runs, err = h.cfg.Repo.ListRecent(c.Request.Context(), limit)
	}
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	out := make([]gin.H, 0, len(runs))
	for _, r := range runs {
		row := gin.H{
			"id":           r.ID,
			"server_id":    r.ServerID,
			"server_name":  r.ServerName,
			"operation_id": r.OperationID,
			"status":       r.Status,
			"message":      r.Message,
			"triggered_by": r.TriggeredBy,
			"started_at":   r.StartedAt,
			"finished_at":  nil,
		}
		if r.FinishedAt.Valid {
			row["finished_at"] = r.FinishedAt.Time
		}
		out = append(out, row)
	}
	c.JSON(http.StatusOK, gin.H{"data": out, "total": len(out)})
}

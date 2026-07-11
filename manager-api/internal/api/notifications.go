package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

// NotificationHandlerConfig wires the in-app notification endpoints (SND-18).
type NotificationHandlerConfig struct {
	Repo repository.NotificationRepository
	Log  *slog.Logger
}

// RegisterNotificationRoutes mounts /api/v1/admin/notifications (any authed role).
func RegisterNotificationRoutes(g *gin.RouterGroup, cfg NotificationHandlerConfig) {
	if cfg.Repo == nil {
		return
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	h := &notificationHandler{cfg: cfg}
	n := g.Group("/admin/notifications")
	n.GET("", h.list)
	n.POST("/:id/read", h.markRead)
	n.POST("/read-all", h.markAllRead)
}

type notificationHandler struct{ cfg NotificationHandlerConfig }

func (h *notificationHandler) list(c *gin.Context) {
	rows, err := h.cfg.Repo.ListRecent(c.Request.Context(), 50)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	unread, err := h.cfg.Repo.UnreadCount(c.Request.Context())
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, n := range rows {
		out = append(out, gin.H{
			"id":          n.ID,
			"kind":        n.Kind,
			"server_id":   n.ServerID,
			"server_name": n.ServerName,
			"metric":      n.Metric,
			"value":       n.Value,
			"threshold":   n.Threshold,
			"message":     n.Message,
			"created_at":  n.CreatedAt,
			"read":        n.ReadAt.Valid,
			"resolved":    n.ResolvedAt.Valid,
		})
	}
	c.JSON(http.StatusOK, gin.H{"data": out, "total": len(out), "unread_count": unread})
}

func (h *notificationHandler) markRead(c *gin.Context) {
	if err := h.cfg.Repo.MarkRead(c.Request.Context(), c.Param("id")); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *notificationHandler) markAllRead(c *gin.Context) {
	if err := h.cfg.Repo.MarkAllRead(c.Request.Context()); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

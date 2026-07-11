package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/middleware"
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
	n.POST("/:id/ack", h.ack)
	n.POST("/:id/snooze", h.snooze)
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
			"severity":    n.Severity,
			"created_at":  n.CreatedAt,
			"read":        n.ReadAt.Valid,
			"resolved":    n.ResolvedAt.Valid,
			"acked":       n.AckedAt.Valid,
			"acked_by":    n.AckedBy,
			"snoozed":     n.SnoozedUntil.Valid && n.SnoozedUntil.Time.After(n.CreatedAt),
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

func (h *notificationHandler) ack(c *gin.Context) {
	if err := h.cfg.Repo.Ack(c.Request.Context(), c.Param("id"), middleware.AdminUsername(c), time.Now().UTC()); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type snoozeRequest struct {
	Minutes int `json:"minutes"`
}

func (h *notificationHandler) snooze(c *gin.Context) {
	var req snoozeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}
	if req.Minutes <= 0 || req.Minutes > 7*24*60 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "minutes must be 1..10080"})
		return
	}
	until := time.Now().UTC().Add(time.Duration(req.Minutes) * time.Minute)
	if err := h.cfg.Repo.Snooze(c.Request.Context(), c.Param("id"), until); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "snoozed_until": until})
}

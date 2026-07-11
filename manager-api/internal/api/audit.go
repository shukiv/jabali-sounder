package api

import (
	"encoding/csv"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

// AuditHandlerConfig wires the audit-log viewer (SND-24).
type AuditHandlerConfig struct {
	Repo repository.AuditRepository
	Log  *slog.Logger
}

// RegisterAuditRoutes mounts /api/v1/admin/audit (any authed role — the trail
// is read-only and useful to every operator).
func RegisterAuditRoutes(g *gin.RouterGroup, cfg AuditHandlerConfig) {
	if cfg.Repo == nil {
		return
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	h := &auditHandler{cfg: cfg}
	g.GET("/admin/audit", h.list)
	g.GET("/admin/audit.csv", h.exportCSV)
}

type auditHandler struct{ cfg AuditHandlerConfig }

const auditMaxLimit = 500

// parseFilter builds an AuditFilter from query params (actor, event, server_id,
// since as RFC3339 or "<n>d" day-window, plus limit/offset).
func parseFilter(c *gin.Context, defaultLimit int) repository.AuditFilter {
	f := repository.AuditFilter{
		Actor:    c.Query("actor"),
		Event:    c.Query("event"),
		ServerID: c.Query("server_id"),
		Limit:    defaultLimit,
	}
	if s := c.Query("since"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			f.Since = t
		} else if days, err := strconv.Atoi(s); err == nil && days > 0 {
			f.Since = time.Now().Add(-time.Duration(days) * 24 * time.Hour)
		}
	}
	if n, err := strconv.Atoi(c.Query("limit")); err == nil && n > 0 {
		f.Limit = n
	}
	if f.Limit > auditMaxLimit {
		f.Limit = auditMaxLimit
	}
	if n, err := strconv.Atoi(c.Query("offset")); err == nil && n > 0 {
		f.Offset = n
	}
	return f
}

func (h *auditHandler) list(c *gin.Context) {
	f := parseFilter(c, 100)
	rows, err := h.cfg.Repo.List(c.Request.Context(), f)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	total, err := h.cfg.Repo.Count(c.Request.Context(), f)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": total, "limit": f.Limit, "offset": f.Offset})
}

func (h *auditHandler) exportCSV(c *gin.Context) {
	f := parseFilter(c, auditMaxLimit)
	rows, err := h.cfg.Repo.List(c.Request.Context(), f)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=jabali-sounder-audit.csv")
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"time", "event", "actor", "actor_id", "server_id", "server_name", "source_ip", "request_id"})
	for _, a := range rows {
		_ = w.Write([]string{
			a.CreatedAt.Format(time.RFC3339),
			csvSafe(a.Event),
			csvSafe(a.Actor),
			csvSafe(a.ActorID),
			csvSafe(a.ServerID),
			csvSafe(a.ServerName),
			csvSafe(a.SourceIP),
			csvSafe(a.RequestID),
		})
	}
	w.Flush()
}

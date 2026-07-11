package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

// DashboardHandlerConfig wires the dashboard endpoint.
type DashboardHandlerConfig struct {
	Repo          repository.ServerRepository
	HeartbeatRepo repository.HeartbeatRepository
	Log           *slog.Logger
}

// RegisterDashboardRoutes mounts GET /api/v1/admin/dashboard.
func RegisterDashboardRoutes(g *gin.RouterGroup, cfg DashboardHandlerConfig) {
	if cfg.Repo == nil {
		return
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	h := &dashboardHandler{cfg: cfg}
	g.GET("/admin/dashboard", h.get)
}

type dashboardHandler struct{ cfg DashboardHandlerConfig }

func (h *dashboardHandler) get(c *gin.Context) {
	servers, err := h.cfg.Repo.List(c.Request.Context())
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}

	type dashboardEntry struct {
		ID               string `json:"id"`
		Name             string `json:"name"`
		BaseURL          string `json:"base_url"`
		Status           string `json:"status"`
		CredentialStatus string `json:"credential_status"`
		Version          string `json:"version"`
		Environment      string `json:"environment"`
		Healthy          bool   `json:"healthy"`
	}

	entries := make([]dashboardEntry, 0, len(servers))
	for _, s := range servers {
		healthy := s.Status == "active" && s.CredentialStatus == "valid"
		entries = append(entries, dashboardEntry{
			ID:               s.ID,
			Name:             s.Name,
			BaseURL:          s.BaseURL,
			Status:           string(s.Status),
			CredentialStatus: string(s.CredentialStatus),
			Version:          s.Version,
			Environment:      s.Environment,
			Healthy:          healthy,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      entries,
		"total":     len(entries),
		"page":      1,
		"page_size": len(entries),
		"sla":       h.fleetSLA(c, servers),
	})
}

// fleetSLA computes per-server uptime over the last 7 days plus a fleet average
// (across servers that have heartbeat data) for the dashboard SLA card (SND-26).
func (h *dashboardHandler) fleetSLA(c *gin.Context, servers []models.Server) gin.H {
	const windowDays = 7
	out := gin.H{"window_days": windowDays, "fleet_ratio": nil, "servers": []gin.H{}}
	if h.cfg.HeartbeatRepo == nil {
		return out
	}
	since := time.Now().Add(-windowDays * 24 * time.Hour)
	perServer := make([]gin.H, 0, len(servers))
	var sum float64
	var counted int
	for _, s := range servers {
		healthy, total, err := h.cfg.HeartbeatRepo.UptimeSince(c.Request.Context(), s.ID, since)
		if err != nil || total == 0 {
			perServer = append(perServer, gin.H{"id": s.ID, "name": s.Name, "ratio": nil})
			continue
		}
		ratio := float64(healthy) / float64(total)
		sum += ratio
		counted++
		perServer = append(perServer, gin.H{"id": s.ID, "name": s.Name, "ratio": ratio})
	}
	out["servers"] = perServer
	if counted > 0 {
		out["fleet_ratio"] = sum / float64(counted)
	}
	return out
}

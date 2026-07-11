package api

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

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
	})
}

package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/middleware"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/remote"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

// registerServerActionRoutes mounts the M2 write/remediation routes. All are
// operator+ (gated by op); reads (capabilities/operation status) are viewer.
func (h *serverHandler) registerActionRoutes(servers *gin.RouterGroup, op gin.HandlerFunc) {
	servers.GET("/:id/capabilities", h.actCapabilities)
	servers.GET("/:id/operations/:opid", h.actOperationStatus)
	act := servers.Group("/:id/actions", op)
	act.POST("/restart-service", h.actRestartService)
	act.POST("/user", h.actSetUser)
	act.POST("/domain", h.actSetDomain)
	act.POST("/purge-cache", h.actPurgeCache)
	act.POST("/backup", h.actBackup)
}

const actionTimeout = 20 * time.Second

// clientForAction loads the server, decrypts its token, and returns a client.
func (h *serverHandler) clientForAction(c *gin.Context) (*remote.Client, *models.Server, bool) {
	s, err := h.cfg.Repo.FindByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return nil, nil, false
		}
		failInternal(c, h.cfg.Log, err)
		return nil, nil, false
	}
	secret, err := h.decryptSecret(s)
	if err != nil {
		failCode(c, h.cfg.Log, http.StatusUnprocessableEntity, "credential_error", err)
		return nil, nil, false
	}
	return remote.NewClient(s.BaseURL, s.TokenID, secret, s.InsecureSkipVerify), s, true
}

// respondAction maps a write-action outcome to the API response.
func (h *serverHandler) respondAction(c *gin.Context, s *models.Server, action string, res *remote.ActionResult, err error) {
	if err != nil {
		code := "action_failed"
		if res != nil && res.Error != "" {
			code = res.Error
		}
		h.cfg.Log.Warn("server action failed", "server", s.Name, "action", action, "error", err)
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": code})
		return
	}
	auditServerMutation(h.cfg.Log, h.cfg.Audit, c, "action."+action, s.ID, s.Name)
	c.JSON(http.StatusOK, res)
}

type restartServiceRequest struct {
	Name string `json:"name" binding:"required"`
}

func (h *serverHandler) actRestartService(c *gin.Context) {
	var req restartServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}
	client, s, ok := h.clientForAction(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), actionTimeout)
	defer cancel()
	res, err := client.RestartService(ctx, req.Name)
	h.respondAction(c, s, "restart_service", res, err)
}

type userActionRequest struct {
	UserID  string `json:"user_id" binding:"required"`
	Enabled bool   `json:"enabled"`
}

func (h *serverHandler) actSetUser(c *gin.Context) {
	var req userActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}
	client, s, ok := h.clientForAction(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), actionTimeout)
	defer cancel()
	res, err := client.SetUserEnabled(ctx, req.UserID, req.Enabled)
	h.respondAction(c, s, "set_user", res, err)
}

type domainActionRequest struct {
	DomainID  string `json:"domain_id" binding:"required"`
	Suspended bool   `json:"suspended"`
}

func (h *serverHandler) actSetDomain(c *gin.Context) {
	var req domainActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}
	client, s, ok := h.clientForAction(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), actionTimeout)
	defer cancel()
	res, err := client.SetDomainSuspended(ctx, req.DomainID, req.Suspended)
	h.respondAction(c, s, "set_domain", res, err)
}

type purgeCacheRequest struct {
	Scope  string `json:"scope"`
	Domain string `json:"domain"`
}

func (h *serverHandler) actPurgeCache(c *gin.Context) {
	var req purgeCacheRequest
	_ = c.ShouldBindJSON(&req)
	if req.Scope == "" {
		req.Scope = "all"
	}
	client, s, ok := h.clientForAction(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), actionTimeout)
	defer cancel()
	res, err := client.PurgeCache(ctx, req.Scope, req.Domain)
	h.respondAction(c, s, "purge_cache", res, err)
}

type backupRequest struct {
	Targets []string `json:"targets"`
}

func (h *serverHandler) actBackup(c *gin.Context) {
	var req backupRequest
	_ = c.ShouldBindJSON(&req)
	client, s, ok := h.clientForAction(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), actionTimeout)
	defer cancel()
	res, err := client.CreateBackup(ctx, req.Targets)
	// Record the backup run so it can be tracked to completion and surfaced in
	// the Backups view (SND-27). Best-effort; never blocks the response.
	if err == nil && res != nil && h.cfg.Backups != nil {
		status := models.BackupPending
		if res.Status != "" {
			status = res.Status
		}
		_ = h.cfg.Backups.Create(c.Request.Context(), &models.BackupRun{
			ID:          ids.NewULID(),
			ServerID:    s.ID,
			ServerName:  s.Name,
			OperationID: res.OperationID,
			Status:      status,
			Message:     res.Message,
			TriggeredBy: middleware.AdminUsername(c),
			StartedAt:   time.Now().UTC(),
		})
	}
	h.respondAction(c, s, "backup", res, err)
}

func (h *serverHandler) actOperationStatus(c *gin.Context) {
	client, _, ok := h.clientForAction(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), actionTimeout)
	defer cancel()
	op, code, err := client.OperationStatus(ctx, c.Param("opid"))
	if err != nil {
		failCode(c, h.cfg.Log, http.StatusBadGateway, "operation_unavailable", err)
		_ = code
		return
	}
	c.JSON(http.StatusOK, op)
}

// actCapabilities fetches the panel's supported write actions and stores them on
// the server row so the UI can show only available actions.
func (h *serverHandler) actCapabilities(c *gin.Context) {
	client, s, ok := h.clientForAction(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), actionTimeout)
	defer cancel()
	caps, _, err := client.Capabilities(ctx)
	if err != nil {
		failCode(c, h.cfg.Log, http.StatusBadGateway, "capabilities_unavailable", err)
		return
	}
	if len(caps.Actions) > 0 {
		s.Capabilities = models.JSONStringArray(caps.Actions)
		_ = h.cfg.Repo.Update(c.Request.Context(), s)
	}
	c.JSON(http.StatusOK, caps)
}

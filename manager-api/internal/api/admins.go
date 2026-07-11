package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/middleware"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

// AdminHandlerConfig wires the operator-management endpoints (M3: RBAC).
type AdminHandlerConfig struct {
	AdminRepo repository.AdminRepository
	Log       *slog.Logger
}

// RegisterAdminRoutes mounts /api/v1/admin/admins, owner-only. Managing
// operators is the most privileged action, so the whole group requires owner.
func RegisterAdminRoutes(g *gin.RouterGroup, cfg AdminHandlerConfig) {
	if cfg.AdminRepo == nil {
		return
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	h := &adminHandler{cfg: cfg}
	admins := g.Group("/admin/admins", middleware.RequireRole(models.RoleOwner))
	admins.GET("", h.list)
	admins.POST("", h.create)
	admins.PATCH("/:id", h.update)
	admins.DELETE("/:id", h.remove)
}

type adminHandler struct{ cfg AdminHandlerConfig }

func adminView(a models.Admin) gin.H {
	return gin.H{"id": a.ID, "username": a.Username, "role": a.Role, "created_at": a.CreatedAt}
}

func (h *adminHandler) list(c *gin.Context) {
	admins, err := h.cfg.AdminRepo.List(c.Request.Context())
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	out := make([]gin.H, 0, len(admins))
	for _, a := range admins {
		out = append(out, adminView(a))
	}
	c.JSON(http.StatusOK, gin.H{"data": out, "total": len(out)})
}

type createAdminRequest struct {
	Username string      `json:"username" binding:"required"`
	Password string      `json:"password" binding:"required"`
	Role     models.Role `json:"role" binding:"required"`
}

func (h *adminHandler) create(c *gin.Context) {
	var req createAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || len(req.Username) > 100 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "username must be 1-100 chars"})
		return
	}
	if len(req.Password) < 8 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "password must be at least 8 characters"})
		return
	}
	if !req.Role.Valid() {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid role"})
		return
	}

	admin, err := NewAdmin(req.Username, req.Password, req.Role)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	if err := h.cfg.AdminRepo.Create(c.Request.Context(), admin); err != nil {
		if strings.Contains(err.Error(), "Duplicate") || strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "UNIQUE") {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "username already exists"})
			return
		}
		failInternal(c, h.cfg.Log, err)
		return
	}
	auditAdmin(h.cfg.Log, c, "create", admin.ID, admin.Username)
	c.JSON(http.StatusCreated, adminView(*admin))
}

type updateAdminRequest struct {
	Role     *models.Role `json:"role"`
	Password *string      `json:"password"`
}

func (h *adminHandler) update(c *gin.Context) {
	admin, err := h.cfg.AdminRepo.FindByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		failInternal(c, h.cfg.Log, err)
		return
	}

	var req updateAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}

	if req.Role != nil {
		if !req.Role.Valid() {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid role"})
			return
		}
		// Never demote the last owner — that would lock out operator management.
		if admin.Role == models.RoleOwner && *req.Role != models.RoleOwner {
			if last, lerr := h.isLastOwner(c); lerr != nil {
				failInternal(c, h.cfg.Log, lerr)
				return
			} else if last {
				c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "cannot demote the last owner"})
				return
			}
		}
		admin.Role = *req.Role
	}
	if req.Password != nil {
		if len(*req.Password) < 8 {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "password must be at least 8 characters"})
			return
		}
		hash, herr := HashPassword(*req.Password)
		if herr != nil {
			failInternal(c, h.cfg.Log, herr)
			return
		}
		admin.PasswordHash = hash
	}

	if err := h.cfg.AdminRepo.Update(c.Request.Context(), admin); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	auditAdmin(h.cfg.Log, c, "update", admin.ID, admin.Username)
	c.JSON(http.StatusOK, adminView(*admin))
}

func (h *adminHandler) remove(c *gin.Context) {
	id := c.Param("id")
	admin, err := h.cfg.AdminRepo.FindByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		failInternal(c, h.cfg.Log, err)
		return
	}
	if id == middleware.AdminID(c) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "cannot delete yourself"})
		return
	}
	if admin.Role == models.RoleOwner {
		if last, lerr := h.isLastOwner(c); lerr != nil {
			failInternal(c, h.cfg.Log, lerr)
			return
		} else if last {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "cannot delete the last owner"})
			return
		}
	}
	if err := h.cfg.AdminRepo.Delete(c.Request.Context(), id); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	auditAdmin(h.cfg.Log, c, "delete", admin.ID, admin.Username)
	c.JSON(http.StatusOK, gin.H{"id": id, "deleted": true})
}

func (h *adminHandler) isLastOwner(c *gin.Context) (bool, error) {
	n, err := h.cfg.AdminRepo.CountByRole(c.Request.Context(), models.RoleOwner)
	if err != nil {
		return false, err
	}
	return n <= 1, nil
}

// auditAdmin records a privileged operator-management action (M3).
func auditAdmin(log *slog.Logger, c *gin.Context, action, targetID, targetUser string) {
	if log == nil {
		log = slog.Default()
	}
	log.Info("audit",
		"event", "admin."+action,
		"actor", middleware.AdminUsername(c),
		"actor_id", middleware.AdminID(c),
		"target_id", targetID,
		"target_user", targetUser,
		"source_ip", c.ClientIP(),
		"request_id", middleware.GetRequestID(c),
	)
}

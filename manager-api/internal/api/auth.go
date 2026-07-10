package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/middleware"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

// AuthHandlerConfig wires the auth endpoints.
type AuthHandlerConfig struct {
	AdminRepo repository.AdminRepository
	JWTSecret string
	JWTTTL    time.Duration
	Log       *slog.Logger
	// Login throttle (SND-3); <=0 uses limiter defaults.
	LoginMaxFailures int
	LoginLockout     time.Duration
	LoginWindow      time.Duration
}

// RegisterAuthRoutes mounts POST /api/v1/auth/login + GET /api/v1/auth/me.
func RegisterAuthRoutes(g *gin.RouterGroup, cfg AuthHandlerConfig) {
	if cfg.AdminRepo == nil || cfg.JWTSecret == "" {
		return
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	h := &authHandler{cfg: cfg}
	auth := g.Group("/auth")
	// Throttle brute force against the sole admin password (SND-3).
	loginLimiter := middleware.NewLoginLimiter(cfg.LoginMaxFailures, cfg.LoginLockout, cfg.LoginWindow, nil, cfg.Log)
	auth.POST("/login", loginLimiter.Middleware(), h.login)
	auth.GET("/setup", h.setupStatus)
	auth.POST("/setup", h.setup)
	auth.GET("/me", middleware.AuthMiddleware(cfg.JWTSecret), h.me)
	auth.POST("/change-password", middleware.AuthMiddleware(cfg.JWTSecret), h.changePassword)
}

type authHandler struct{ cfg AuthHandlerConfig }

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type setupRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *authHandler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}

	// SND-2: every failure mode below returns the SAME opaque message so the
	// unauthenticated login path cannot be used to enumerate usernames or
	// distinguish missing-admin vs bad-password vs internal error.
	req.Username = strings.TrimSpace(req.Username)
	admin, err := h.cfg.AdminRepo.FindByUsername(c.Request.Context(), req.Username)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_credentials"})
			return
		}
		failInternal(c, h.cfg.Log, err)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_credentials"})
		return
	}

	token, expiresAt, err := middleware.MintToken(h.cfg.JWTSecret, admin.ID, admin.Username, h.cfg.JWTTTL)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"expires_at": expiresAt,
		"admin": gin.H{
			"id":       admin.ID,
			"username": admin.Username,
		},
	})
}

func (h *authHandler) me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"id":       middleware.AdminID(c),
		"username": middleware.AdminUsername(c),
	})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
}

// changePassword updates the authenticated admin's password after verifying
// their current one.
func (h *authHandler) changePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}
	if len(req.NewPassword) < 8 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "new password must be at least 8 characters"})
		return
	}

	admin, err := h.cfg.AdminRepo.FindByUsername(c.Request.Context(), middleware.AdminUsername(c))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_session"})
			return
		}
		failInternal(c, h.cfg.Log, err)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "current_password_incorrect"})
		return
	}

	hash, err := HashPassword(req.NewPassword)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	admin.PasswordHash = hash
	if err := h.cfg.AdminRepo.Update(c.Request.Context(), admin); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *authHandler) setupStatus(c *gin.Context) {
	count, err := h.cfg.AdminRepo.Count(c.Request.Context())
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"available": count == 0})
}

func (h *authHandler) setup(c *gin.Context) {
	count, err := h.cfg.AdminRepo.Count(c.Request.Context())
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "setup_already_completed"})
		return
	}

	var req setupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "username is required"})
		return
	}
	if len(req.Password) < 8 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "password must be at least 8 characters"})
		return
	}

	admin, err := NewAdmin(req.Username, req.Password)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	if err := h.cfg.AdminRepo.CreateFirst(c.Request.Context(), admin); err != nil {
		if errors.Is(err, repository.ErrSetupCompleted) {
			c.JSON(http.StatusConflict, gin.H{"error": "setup_already_completed"})
			return
		}
		failInternal(c, h.cfg.Log, err)
		return
	}

	token, expiresAt, err := middleware.MintToken(h.cfg.JWTSecret, admin.ID, admin.Username, h.cfg.JWTTTL)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"token":      token,
		"expires_at": expiresAt,
		"admin": gin.H{
			"id":       admin.ID,
			"username": admin.Username,
		},
	})
}

// HashPassword returns a bcrypt hash of the password. Used by the CLI
// admin set-password command.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// NewAdmin creates an Admin model with a bcrypt-hashed password.
// Used by the CLI admin set-password command.
func NewAdmin(username, password string) (*models.Admin, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	return &models.Admin{
		ID:           ids.NewULID(),
		Username:     username,
		PasswordHash: hash,
	}, nil
}

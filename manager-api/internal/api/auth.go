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
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/secrets"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/totp"
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
	// SecretKey seals the TOTP secret (M3: 2FA); AllowPlaintext mirrors servers.
	SecretKey      *secrets.Key
	AllowPlaintext bool
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
	auth.POST("/2fa/setup", middleware.AuthMiddleware(cfg.JWTSecret), h.setup2FA)
	auth.POST("/2fa/activate", middleware.AuthMiddleware(cfg.JWTSecret), h.activate2FA)
	auth.POST("/2fa/disable", middleware.AuthMiddleware(cfg.JWTSecret), h.disable2FA)
}

type authHandler struct{ cfg AuthHandlerConfig }

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	TOTPCode string `json:"totp_code"`
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

	// Second factor (M3: 2FA). Password is correct; require a valid TOTP code
	// when the account has 2FA enabled. Without a code, tell the client to
	// prompt for one (no token issued).
	if admin.TOTPEnabled {
		if strings.TrimSpace(req.TOTPCode) == "" {
			c.JSON(http.StatusOK, gin.H{"two_factor_required": true})
			return
		}
		secret, serr := secrets.OpenSecret(h.cfg.SecretKey, admin.TOTPSecretEnc, h.cfg.AllowPlaintext)
		if serr != nil {
			failInternal(c, h.cfg.Log, serr)
			return
		}
		if !totp.Validate(secret, req.TOTPCode, time.Now()) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_credentials"})
			return
		}
	}

	// Fail closed: an admin with a missing/corrupt role gets the LEAST privilege
	// (viewer), never owner. Existing admins are set to owner by migration 000008.
	role := admin.Role
	if !role.Valid() {
		h.cfg.Log.Warn("login: admin has invalid role, defaulting to viewer", "username", admin.Username)
		role = models.RoleViewer
	}
	token, expiresAt, err := middleware.MintToken(h.cfg.JWTSecret, admin.ID, admin.Username, role, h.cfg.JWTTTL)
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
			"role":     role,
		},
	})
}

func (h *authHandler) me(c *gin.Context) {
	twoFactor := false
	if a, err := h.cfg.AdminRepo.FindByUsername(c.Request.Context(), middleware.AdminUsername(c)); err == nil {
		twoFactor = a.TOTPEnabled
	}
	c.JSON(http.StatusOK, gin.H{
		"id":                 middleware.AdminID(c),
		"username":           middleware.AdminUsername(c),
		"role":               middleware.AdminRole(c),
		"two_factor_enabled": twoFactor,
	})
}

// currentAdmin loads the authenticated admin by username (from the JWT).
func (h *authHandler) currentAdmin(c *gin.Context) (*models.Admin, bool) {
	a, err := h.cfg.AdminRepo.FindByUsername(c.Request.Context(), middleware.AdminUsername(c))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid_session"})
		} else {
			failInternal(c, h.cfg.Log, err)
		}
		return nil, false
	}
	return a, true
}

// setup2FA generates a pending TOTP secret and returns the otpauth URL for a QR
// code. Not enabled until activate2FA confirms a valid code.
func (h *authHandler) setup2FA(c *gin.Context) {
	admin, ok := h.currentAdmin(c)
	if !ok {
		return
	}
	secret, err := totp.GenerateSecret()
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	enc, err := secrets.SealSecret(h.cfg.SecretKey, secret, h.cfg.AllowPlaintext)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	admin.TOTPSecretEnc = enc
	admin.TOTPEnabled = false
	if err := h.cfg.AdminRepo.Update(c.Request.Context(), admin); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"secret":      secret,
		"otpauth_url": totp.OtpauthURL("Jabali Sounder", admin.Username, secret),
	})
}

type codeRequest struct {
	Code string `json:"code" binding:"required"`
}

// activate2FA enables 2FA after verifying a code against the pending secret.
func (h *authHandler) activate2FA(c *gin.Context) {
	var req codeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}
	admin, ok := h.currentAdmin(c)
	if !ok {
		return
	}
	if len(admin.TOTPSecretEnc) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "no_pending_2fa"})
		return
	}
	secret, err := secrets.OpenSecret(h.cfg.SecretKey, admin.TOTPSecretEnc, h.cfg.AllowPlaintext)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	if !totp.Validate(secret, req.Code, time.Now()) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_code"})
		return
	}
	admin.TOTPEnabled = true
	if err := h.cfg.AdminRepo.Update(c.Request.Context(), admin); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"two_factor_enabled": true})
}

type disable2FARequest struct {
	Password string `json:"password" binding:"required"`
	Code     string `json:"code" binding:"required"`
}

// disable2FA turns off 2FA after verifying the current password AND a code.
func (h *authHandler) disable2FA(c *gin.Context) {
	var req disable2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}
	admin, ok := h.currentAdmin(c)
	if !ok {
		return
	}
	if !admin.TOTPEnabled {
		c.JSON(http.StatusOK, gin.H{"two_factor_enabled": false})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "current_password_incorrect"})
		return
	}
	secret, err := secrets.OpenSecret(h.cfg.SecretKey, admin.TOTPSecretEnc, h.cfg.AllowPlaintext)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	if !totp.Validate(secret, req.Code, time.Now()) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_code"})
		return
	}
	admin.TOTPSecretEnc = nil
	admin.TOTPEnabled = false
	if err := h.cfg.AdminRepo.Update(c.Request.Context(), admin); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"two_factor_enabled": false})
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

	admin, err := NewAdmin(req.Username, req.Password, models.RoleOwner)
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

	token, expiresAt, err := middleware.MintToken(h.cfg.JWTSecret, admin.ID, admin.Username, models.RoleOwner, h.cfg.JWTTTL)
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
			"role":     models.RoleOwner,
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
func NewAdmin(username, password string, role models.Role) (*models.Admin, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	return &models.Admin{
		ID:           ids.NewULID(),
		Username:     username,
		PasswordHash: hash,
		Role:         role,
	}, nil
}

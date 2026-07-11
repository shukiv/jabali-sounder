package api

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/middleware"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

// APITokenHandlerConfig wires the read-only API-token endpoints (M4).
type APITokenHandlerConfig struct {
	Repo repository.APITokenRepository
	Log  *slog.Logger
}

// RegisterAPITokenRoutes mounts /api/v1/admin/api-tokens (operator+).
func RegisterAPITokenRoutes(g *gin.RouterGroup, cfg APITokenHandlerConfig) {
	if cfg.Repo == nil {
		return
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	h := &apiTokenHandler{cfg: cfg}
	t := g.Group("/admin/api-tokens", middleware.RequireRole(models.RoleOperator))
	t.GET("", h.list)
	t.POST("", h.mint)
	t.DELETE("/:id", h.revoke)
	t.POST("/:id/rotate", h.rotate)
}

type apiTokenHandler struct{ cfg APITokenHandlerConfig }

func (h *apiTokenHandler) list(c *gin.Context) {
	tokens, err := h.cfg.Repo.List(c.Request.Context())
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	out := make([]gin.H, 0, len(tokens))
	for _, tk := range tokens {
		item := gin.H{"id": tk.ID, "name": tk.Name, "created_at": tk.CreatedAt, "scopes": []string(tk.Scopes), "allowed_ips": []string(tk.AllowedIPs), "rate_limit_per_min": tk.RateLimitPerMin}
		if tk.LastUsedAt.Valid {
			item["last_used_at"] = tk.LastUsedAt.Time
		}
		if tk.ExpiresAt.Valid {
			item["expires_at"] = tk.ExpiresAt.Time
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, gin.H{"data": out, "total": len(out)})
}

type mintTokenRequest struct {
	Name            string   `json:"name" binding:"required"`
	ExpiresInDays   int      `json:"expires_in_days"`
	Scopes          []string `json:"scopes"`
	AllowedIPs      []string `json:"allowed_ips"`
	RateLimitPerMin int      `json:"rate_limit_per_min"`
}

func (h *apiTokenHandler) mint(c *gin.Context) {
	var req mintTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 200 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "name must be 1-200 chars"})
		return
	}
	var expires *time.Time
	if req.ExpiresInDays > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresInDays) * 24 * time.Hour)
		expires = &t
	}

	scopes, ok := normalizeScopes(req.Scopes)
	if !ok {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "unknown scope"})
		return
	}
	if bad := invalidIP(req.AllowedIPs); bad != "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid ip/cidr: " + bad})
		return
	}
	if req.RateLimitPerMin < 0 || req.RateLimitPerMin > 100000 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "rate_limit_per_min out of range"})
		return
	}

	plaintext, tok, err := h.cfg.Repo.Mint(c.Request.Context(), req.Name, middleware.AdminID(c), expires, scopes, req.AllowedIPs, req.RateLimitPerMin)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	h.cfg.Log.Info("audit", "event", "api_token.create", "actor", middleware.AdminUsername(c),
		"token_id", tok.ID, "token_name", tok.Name, "request_id", middleware.GetRequestID(c))
	// The plaintext token is shown exactly once.
	c.JSON(http.StatusCreated, gin.H{"id": tok.ID, "name": tok.Name, "token": plaintext, "created_at": tok.CreatedAt})
}

func (h *apiTokenHandler) revoke(c *gin.Context) {
	if err := h.cfg.Repo.Revoke(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		failInternal(c, h.cfg.Log, err)
		return
	}
	h.cfg.Log.Info("audit", "event", "api_token.revoke", "actor", middleware.AdminUsername(c),
		"token_id", c.Param("id"), "request_id", middleware.GetRequestID(c))
	c.JSON(http.StatusOK, gin.H{"id": c.Param("id"), "revoked": true})
}

func (h *apiTokenHandler) rotate(c *gin.Context) {
	plaintext, tok, err := h.cfg.Repo.Rotate(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		failInternal(c, h.cfg.Log, err)
		return
	}
	// Rotation issues a fresh working credential — audit it like create/revoke so
	// the action is traceable (invalidates the previous secret immediately).
	h.cfg.Log.Info("audit", "event", "api_token.rotate", "actor", middleware.AdminUsername(c),
		"token_id", tok.ID, "token_name", tok.Name, "request_id", middleware.GetRequestID(c))
	c.JSON(http.StatusOK, gin.H{"id": tok.ID, "name": tok.Name, "token": plaintext})
}

// normalizeScopes validates requested scopes against the known vocabulary.
// Empty input means full read access (returns nil scopes = read:*).
func normalizeScopes(req []string) ([]string, bool) {
	if len(req) == 0 {
		return nil, true
	}
	valid := map[string]bool{}
	for _, s := range middleware.TokenScopeNames() {
		valid[s] = true
	}
	out := make([]string, 0, len(req))
	for _, s := range req {
		if !valid[s] {
			return nil, false
		}
		out = append(out, s)
	}
	return out, true
}

// invalidIP returns the first entry that is not a valid IP or CIDR, or "".
func invalidIP(entries []string) string {
	for _, e := range entries {
		if net.ParseIP(e) != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(e); err == nil {
			continue
		}
		return e
	}
	return ""
}

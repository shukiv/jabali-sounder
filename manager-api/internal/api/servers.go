package api

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/remote"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/secrets"
)

// ServerHandlerConfig wires the server enrollment endpoints.
type ServerHandlerConfig struct {
	Repo      repository.ServerRepository
	SecretKey *secrets.Key
	Log       *slog.Logger
}

// RegisterServerRoutes mounts /api/v1/admin/servers.
func RegisterServerRoutes(g *gin.RouterGroup, cfg ServerHandlerConfig) {
	if cfg.Repo == nil {
		// Enrollment disabled when no DB — mount nothing.
		return
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	h := &serverHandler{cfg: cfg}
	servers := g.Group("/admin/servers")
	servers.GET("", h.list)
	servers.POST("", h.create)
	servers.GET("/:id", h.detail)
	servers.PATCH("/:id", h.update)
	servers.DELETE("/:id", h.remove)
	servers.POST("/:id/check", h.checkHealth)
}

type serverHandler struct{ cfg ServerHandlerConfig }

type createServerRequest struct {
	Name        string   `json:"name" binding:"required"`
	BaseURL     string   `json:"base_url" binding:"required"`
	TokenID     string   `json:"token_id" binding:"required"`
	TokenSecret string   `json:"token_secret" binding:"required"`
	Scopes      []string `json:"scopes"`
	// InsecureSkipVerify skips TLS cert verification for this panel (self-signed).
	InsecureSkipVerify bool `json:"insecure_skip_verify"`
}

func (h *serverHandler) create(c *gin.Context) {
	var req createServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json", "detail": err.Error()})
		return
	}

	baseURL, err := normalizePanelBaseURL(req.BaseURL)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid panel hostname"})
		return
	}

	// Validate name not empty / not too long.
	req.Name = strings.TrimSpace(req.Name)
	if len(req.Name) == 0 || len(req.Name) > 200 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "name must be 1-200 chars"})
		return
	}

	// Probe /health before enrolling — fail fast on unreachable.
	client := remote.NewClient(baseURL, req.TokenID, req.TokenSecret, req.InsecureSkipVerify)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	healthResp, hcode, err := client.Health(ctx)
	if err != nil || hcode != http.StatusOK {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":  "server_unreachable",
			"detail": fmt.Sprintf("GET /health failed: %v (HTTP %d)", err, hcode),
		})
		return
	}

	// Encrypt the token secret.
	var secretEnc []byte
	if h.cfg.SecretKey != nil {
		secretEnc, err = h.cfg.SecretKey.Seal([]byte(req.TokenSecret))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "encrypt token secret: " + err.Error()})
			return
		}
	} else {
		// No key — store hex-encoded plaintext (dev only).
		secretEnc = []byte(hex.EncodeToString([]byte(req.TokenSecret)))
	}

	scopes := req.Scopes
	if scopes == nil {
		scopes = []string{remote.ScopeReadAll}
	}

	server := &models.Server{
		ID:                 ids.NewULID(),
		Name:               req.Name,
		BaseURL:            baseURL,
		TokenID:            req.TokenID,
		TokenSecretEnc:     secretEnc,
		Scopes:             models.JSONStringArray(scopes),
		InsecureSkipVerify: req.InsecureSkipVerify,
		Version:            healthResp.Version,
		HealthURL:          baseURL + "/health",
		Status:             models.ServerStatusActive,
		CredentialStatus:   models.CredentialUnknown,
	}

	if err := h.cfg.Repo.Create(c.Request.Context(), server); err != nil {
		// Check for duplicate name.
		if strings.Contains(err.Error(), "Duplicate") || strings.Contains(err.Error(), "duplicate") {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "duplicate name or token_id"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create server: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, server)
}

func (h *serverHandler) list(c *gin.Context) {
	servers, err := h.cfg.Repo.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":      servers,
		"total":     len(servers),
		"page":      1,
		"page_size": len(servers),
	})
}

func (h *serverHandler) detail(c *gin.Context) {
	s, err := h.cfg.Repo.FindByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, s)
}

type updateServerRequest struct {
	Name               *string   `json:"name"`
	BaseURL            *string   `json:"base_url"`
	Scopes             *[]string `json:"scopes"`
	InsecureSkipVerify *bool     `json:"insecure_skip_verify"`
}

func (h *serverHandler) update(c *gin.Context) {
	s, err := h.cfg.Repo.FindByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var req updateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json", "detail": err.Error()})
		return
	}

	if req.Name != nil {
		s.Name = strings.TrimSpace(*req.Name)
	}
	if req.BaseURL != nil {
		baseURL, err := normalizePanelBaseURL(*req.BaseURL)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid panel hostname"})
			return
		}
		s.BaseURL = baseURL
		s.HealthURL = s.BaseURL + "/health"
	}
	if req.Scopes != nil {
		s.Scopes = models.JSONStringArray(*req.Scopes)
	}
	if req.InsecureSkipVerify != nil {
		s.InsecureSkipVerify = *req.InsecureSkipVerify
	}

	if err := h.cfg.Repo.Update(c.Request.Context(), s); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "update: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, s)
}

func normalizePanelBaseURL(raw string) (string, error) {
	input := strings.TrimSpace(raw)
	if input == "" {
		return "", fmt.Errorf("empty hostname")
	}

	if strings.Contains(input, "://") {
		u, err := url.Parse(input)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" {
			return "", fmt.Errorf("invalid https panel URL")
		}
		if u.Path != "" && u.Path != "/" {
			return "", fmt.Errorf("panel URL must not include a path")
		}
		return "https://" + net.JoinHostPort(u.Hostname(), "8443"), nil
	}

	if strings.ContainsAny(input, "/?#") {
		return "", fmt.Errorf("hostname must not include URL path, query, or fragment")
	}
	host := input
	if strings.Contains(input, ":") {
		u, err := url.Parse("//" + input)
		if err == nil && u.Hostname() != "" {
			host = u.Hostname()
		}
	}
	if strings.TrimSpace(host) == "" {
		return "", fmt.Errorf("empty hostname")
	}
	return "https://" + net.JoinHostPort(host, "8443"), nil
}

func (h *serverHandler) remove(c *gin.Context) {
	id := c.Param("id")
	s, err := h.cfg.Repo.FindByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Soft-delete: set status to disabled.
	s.Status = models.ServerStatusDisabled
	if err := h.cfg.Repo.Update(c.Request.Context(), s); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "disable: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": id, "disabled": true})
}

// checkHealth probes the server's /health + /automation/status on demand.
func (h *serverHandler) checkHealth(c *gin.Context) {
	s, err := h.cfg.Repo.FindByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Decrypt token secret.
	secretStr, err := h.decryptSecret(s)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "decrypt: " + err.Error()})
		return
	}

	client := remote.NewClient(s.BaseURL, s.TokenID, secretStr, s.InsecureSkipVerify)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	result, err := client.CheckHealth(ctx)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "check failed: " + err.Error()})
		return
	}

	// Update server status + credential_status.
	status := models.ServerStatusActive
	if !result.Reachable {
		status = models.ServerStatusUnreachable
	}
	credStatus := models.CredentialUnknown
	if result.Reachable {
		if result.CredentialValid {
			credStatus = models.CredentialValid
		} else {
			credStatus = models.CredentialInvalid
		}
	}
	_ = h.cfg.Repo.UpdateStatus(c.Request.Context(), s.ID, status, credStatus)

	// Update version if we got it.
	if result.Version != "" && result.Version != s.Version {
		s.Version = result.Version
		_ = h.cfg.Repo.Update(c.Request.Context(), s)
	}

	c.JSON(http.StatusOK, result)
}

// decryptSecret decrypts the stored token secret, or hex-decodes it (dev fallback).
func (h *serverHandler) decryptSecret(s *models.Server) (string, error) {
	if h.cfg.SecretKey != nil {
		plaintext, err := h.cfg.SecretKey.Open(s.TokenSecretEnc)
		if err != nil {
			return "", fmt.Errorf("open secret: %w", err)
		}
		return string(plaintext), nil
	}
	// Dev fallback — hex-encoded plaintext.
	decoded, err := hex.DecodeString(string(s.TokenSecretEnc))
	if err != nil {
		return "", fmt.Errorf("hex decode: %w", err)
	}
	return string(decoded), nil
}

// Ensure json import is used.
var _ = json.Marshal

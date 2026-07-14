package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/middleware"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/remote"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/secrets"
)

// ServerHandlerConfig wires the server enrollment endpoints.
type ServerHandlerConfig struct {
	Repo          repository.ServerRepository
	Heartbeats    repository.HeartbeatRepository
	MetricSamples repository.MetricSampleRepository
	Audit         repository.AuditRepository
	Backups       repository.BackupRepository
	SecretKey     *secrets.Key
	Log           *slog.Logger
	// AllowPrivateTargets disables the SSRF guard (SND-4).
	AllowPrivateTargets bool
	// AllowPlaintext permits the dev hex-plaintext token fallback (SND-6).
	AllowPlaintext bool
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
	op := middleware.RequireRole(models.RoleOperator)
	servers := g.Group("/admin/servers")
	servers.GET("", h.list)
	servers.POST("", op, h.create)
	servers.GET("/:id", h.detail)
	servers.PATCH("/:id", op, h.update)
	servers.DELETE("/:id", op, h.remove)
	servers.POST("/:id/disable", op, h.disable)
	servers.POST("/:id/enable", op, h.enable)
	servers.POST("/:id/check", h.checkHealth)
	servers.GET("/:id/heartbeats", h.heartbeats)
	servers.GET("/:id/metrics", h.metrics)
	h.registerActionRoutes(servers, op)
}

type serverHandler struct{ cfg ServerHandlerConfig }

type createServerRequest struct {
	Name        string   `json:"name" binding:"required"`
	BaseURL     string   `json:"base_url" binding:"required"`
	TokenID     string   `json:"token_id" binding:"required"`
	TokenSecret string   `json:"token_secret" binding:"required"`
	Scopes      []string `json:"scopes"`
	Tags        []string `json:"tags"`
	Environment string   `json:"environment"`
	// InsecureSkipVerify skips TLS cert verification for this panel (self-signed).
	InsecureSkipVerify bool `json:"insecure_skip_verify"`
}

func (h *serverHandler) create(c *gin.Context) {
	var req createServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}

	baseURL, err := normalizePanelBaseURL(req.BaseURL)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid panel hostname"})
		return
	}
	if err := h.validateTargetHost(baseURL); err != nil {
		failCode(c, h.cfg.Log, http.StatusUnprocessableEntity, "target_not_allowed", err)
		return
	}

	// Validate name not empty / not too long.
	req.Name = strings.TrimSpace(req.Name)
	if len(req.Name) == 0 || len(req.Name) > 200 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "name must be 1-200 chars"})
		return
	}
	tags, err := normalizeServerTags(req.Tags)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_tags"})
		return
	}

	// Verify reachability AND that the automation credentials actually work
	// before enrolling. Probing only /health (unauthenticated) let servers with
	// a bad token get added and then show as active/invalid; instead reject the
	// enrollment outright if the token is rejected.
	client := remote.NewClient(baseURL, req.TokenID, req.TokenSecret, req.InsecureSkipVerify)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	check, err := client.CheckHealth(ctx)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	if status, code, ok := enrollmentGate(check); !ok {
		detail := fmt.Sprintf("GET /health failed (HTTP %d)", check.HealthCode)
		if code == "invalid_credentials" {
			detail = fmt.Sprintf("automation token was rejected by the panel (HTTP %d)", check.StatusCode)
		}
		c.JSON(status, gin.H{"error": code, "detail": detail})
		return
	}

	// Encrypt the token secret (SND-6: single fallback location).
	secretEnc, err := h.encryptSecret(req.TokenSecret)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
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
		Tags:               models.JSONStringArray(tags),
		Environment:        normalizeEnvironment(req.Environment),
		InsecureSkipVerify: req.InsecureSkipVerify,
		Version:            check.Version,
		HealthURL:          baseURL + "/health",
		Status:             models.ServerStatusActive,
		CredentialStatus:   models.CredentialValid,
	}

	if err := h.cfg.Repo.Create(c.Request.Context(), server); err != nil {
		// Check for duplicate name.
		if strings.Contains(err.Error(), "Duplicate") || strings.Contains(err.Error(), "duplicate") {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "duplicate name or token_id"})
			return
		}
		failInternal(c, h.cfg.Log, err)
		return
	}

	auditServerMutation(h.cfg.Log, h.cfg.Audit, c, "enroll", server.ID, server.Name)
	c.JSON(http.StatusCreated, server)
}

// enrollmentGate decides whether a health-check result permits enrolling a
// server. A server must be reachable AND present valid automation credentials —
// a panel that only answers /health but rejects the token is not enrolled, so
// broken credentials never land in the managed list. ok=false -> reject with the
// returned status + opaque code.
func enrollmentGate(check *remote.CheckResult) (status int, code string, ok bool) {
	switch {
	case check == nil || !check.Reachable:
		return http.StatusUnprocessableEntity, "server_unreachable", false
	case !check.CredentialValid:
		return http.StatusUnprocessableEntity, "invalid_credentials", false
	default:
		return http.StatusCreated, "", true
	}
}

func (h *serverHandler) list(c *gin.Context) {
	servers, err := h.cfg.Repo.List(c.Request.Context())
	if err != nil {
		failInternal(c, h.cfg.Log, err)
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
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, s)
}

type updateServerRequest struct {
	Name               *string   `json:"name"`
	BaseURL            *string   `json:"base_url"`
	Scopes             *[]string `json:"scopes"`
	Tags               *[]string `json:"tags"`
	Environment        *string   `json:"environment"`
	InsecureSkipVerify *bool     `json:"insecure_skip_verify"`
	TokenID            *string   `json:"token_id"`
	TokenSecret        *string   `json:"token_secret"`
}

func (h *serverHandler) update(c *gin.Context) {
	s, err := h.cfg.Repo.FindByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		failInternal(c, h.cfg.Log, err)
		return
	}

	var req updateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
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
		if err := h.validateTargetHost(baseURL); err != nil {
			failCode(c, h.cfg.Log, http.StatusUnprocessableEntity, "target_not_allowed", err)
			return
		}
		s.BaseURL = baseURL
		s.HealthURL = s.BaseURL + "/health"
	}
	if req.Scopes != nil {
		s.Scopes = models.JSONStringArray(*req.Scopes)
	}
	if req.Tags != nil {
		tags, err := normalizeServerTags(*req.Tags)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid_tags"})
			return
		}
		s.Tags = models.JSONStringArray(tags)
	}
	if req.Environment != nil {
		s.Environment = normalizeEnvironment(*req.Environment)
	}
	if req.InsecureSkipVerify != nil {
		s.InsecureSkipVerify = *req.InsecureSkipVerify
	}
	// Token credential edits. Changing either invalidates the known-good
	// credential status until the next health check re-validates it.
	if req.TokenID != nil {
		if tid := strings.TrimSpace(*req.TokenID); tid != "" {
			s.TokenID = tid
			s.CredentialStatus = models.CredentialUnknown
		}
	}
	if req.TokenSecret != nil {
		if ts := strings.TrimSpace(*req.TokenSecret); ts != "" {
			enc, err := h.encryptSecret(ts)
			if err != nil {
				failInternal(c, h.cfg.Log, err)
				return
			}
			s.TokenSecretEnc = enc
			s.CredentialStatus = models.CredentialUnknown
		}
	}

	if err := h.cfg.Repo.Update(c.Request.Context(), s); err != nil {
		if strings.Contains(err.Error(), "Duplicate") || strings.Contains(err.Error(), "duplicate") {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "duplicate name or token_id"})
			return
		}
		failInternal(c, h.cfg.Log, err)
		return
	}
	auditServerMutation(h.cfg.Log, h.cfg.Audit, c, "update", s.ID, s.Name)
	c.JSON(http.StatusOK, s)
}

// encryptSecret seals a plaintext token secret; the hex-plaintext dev fallback
// lives solely in secrets.SealSecret (SND-6).
func (h *serverHandler) encryptSecret(plaintext string) ([]byte, error) {
	return secrets.SealSecret(h.cfg.SecretKey, plaintext, h.cfg.AllowPlaintext)
}

// validateTargetHost guards against SSRF (SND-4) using the handler's config.
func (h *serverHandler) validateTargetHost(baseURL string) error {
	return validatePublicTarget(baseURL, h.cfg.AllowPrivateTargets)
}

// validatePublicTarget resolves the panel host and rejects private, loopback,
// link-local, unspecified, CGNAT, or multicast addresses unless private targets
// are explicitly allowed (SND-4). Shared by enrollment, update, and import.
func validatePublicTarget(baseURL string, allowPrivate bool) error {
	if allowPrivate {
		return nil
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("parse target url: %w", err)
	}
	host := u.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		if !isPublicIP(ip) {
			return fmt.Errorf("target address %s is not public", host)
		}
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return fmt.Errorf("target host %q does not resolve", host)
	}
	for _, ip := range ips {
		if !isPublicIP(ip) {
			return fmt.Errorf("target host %q resolves to a non-public address", host)
		}
	}
	return nil
}

// isPublicIP reports whether ip is a globally routable unicast address.
func isPublicIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil && ip4[0] == 100 && ip4[1]&0xc0 == 64 {
		return false // 100.64.0.0/10 CGNAT
	}
	return true
}

const (
	maxServerTags      = 20
	maxServerTagLength = 40
)

var serverTagPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// normalizeServerTags applies the server-tag contract at the API boundary.
// normalizeEnvironment lowercases + trims a server's environment label. Empty
// is allowed (unassigned). Same character contract as a tag.
func normalizeEnvironment(raw string) string {
	env := strings.ToLower(strings.TrimSpace(raw))
	if len(env) > maxServerTagLength || !serverTagPattern.MatchString(env) {
		return ""
	}
	return env
}

func normalizeServerTags(input []string) ([]string, error) {
	tags := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, raw := range input {
		tag := strings.ToLower(strings.TrimSpace(raw))
		if tag == "" {
			return nil, fmt.Errorf("tags must not contain empty values")
		}
		if len(tag) > maxServerTagLength {
			return nil, fmt.Errorf("tag %q must be at most %d characters", tag, maxServerTagLength)
		}
		if !serverTagPattern.MatchString(tag) {
			return nil, fmt.Errorf("tag %q must start with a letter or number and contain only letters, numbers, dots, underscores, or hyphens", tag)
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
		if len(tags) > maxServerTags {
			return nil, fmt.Errorf("tags must contain at most %d values", maxServerTags)
		}
	}
	return tags, nil
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

// remove hard-deletes a server (heartbeats cascade). This is irreversible;
// to keep a server but stop polling it, use disable instead.
func (h *serverHandler) remove(c *gin.Context) {
	id := c.Param("id")
	srv, err := h.cfg.Repo.FindByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		failInternal(c, h.cfg.Log, err)
		return
	}
	if err := h.cfg.Repo.Delete(c.Request.Context(), id); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	auditServerMutation(h.cfg.Log, h.cfg.Audit, c, "delete", srv.ID, srv.Name)
	c.JSON(http.StatusOK, gin.H{"id": id, "deleted": true})
}

func (h *serverHandler) disable(c *gin.Context) { h.setStatus(c, models.ServerStatusDisabled) }
func (h *serverHandler) enable(c *gin.Context)  { h.setStatus(c, models.ServerStatusActive) }

// setStatus flips a server between active and disabled, preserving its stored
// credentials and credential_status.
func (h *serverHandler) setStatus(c *gin.Context, status models.ServerStatus) {
	s, err := h.cfg.Repo.FindByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		failInternal(c, h.cfg.Log, err)
		return
	}
	s.Status = status
	if err := h.cfg.Repo.Update(c.Request.Context(), s); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	action := "enable"
	if status == models.ServerStatusDisabled {
		action = "disable"
	}
	auditServerMutation(h.cfg.Log, h.cfg.Audit, c, action, s.ID, s.Name)
	c.JSON(http.StatusOK, s)
}

// heartbeats returns recent health-check history for a server plus an uptime
// summary over the returned window (roadmap M1: status history).
func (h *serverHandler) heartbeats(c *gin.Context) {
	id := c.Param("id")
	if _, err := h.cfg.Repo.FindByID(c.Request.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		failInternal(c, h.cfg.Log, err)
		return
	}

	limit := 50
	if n, err := strconv.Atoi(c.DefaultQuery("limit", "50")); err == nil {
		limit = n
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 500 {
		limit = 500
	}

	if h.cfg.Heartbeats == nil {
		c.JSON(http.StatusOK, gin.H{"data": []any{}, "total": 0, "uptime": gin.H{"healthy": 0, "total": 0, "ratio": 0}})
		return
	}
	rows, err := h.cfg.Heartbeats.Recent(c.Request.Context(), id, limit)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	healthy := 0
	for _, hb := range rows {
		if hb.Healthy {
			healthy++
		}
	}
	ratio := 0.0
	if len(rows) > 0 {
		ratio = float64(healthy) / float64(len(rows))
	}

	// Uptime over a longer window (default 7 days) computed across all stored
	// heartbeats, not just the returned page — this is the SLA figure (SND-26).
	windowDays := 7
	if n, err := strconv.Atoi(c.DefaultQuery("uptime_window_days", "7")); err == nil && n > 0 && n <= 365 {
		windowDays = n
	}
	uptimeWindow := gin.H{"healthy": 0, "total": 0, "ratio": 0, "window_days": windowDays}
	wHealthy, wTotal, err := h.cfg.Heartbeats.UptimeSince(c.Request.Context(), id, time.Now().Add(-time.Duration(windowDays)*24*time.Hour))
	if err == nil && wTotal > 0 {
		uptimeWindow = gin.H{
			"healthy": wHealthy, "total": wTotal,
			"ratio": float64(wHealthy) / float64(wTotal), "window_days": windowDays,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  rows,
		"total": len(rows),
		"uptime": gin.H{
			"healthy": healthy,
			"total":   len(rows),
			"ratio":   ratio,
		},
		"uptime_window": uptimeWindow,
	})
}

// metrics returns recent resource-usage samples for a server (roadmap M1:
// trends). Samples are captured by the background poller.
func (h *serverHandler) metrics(c *gin.Context) {
	id := c.Param("id")
	if _, err := h.cfg.Repo.FindByID(c.Request.Context(), id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		failInternal(c, h.cfg.Log, err)
		return
	}

	limit := 100
	if n, err := strconv.Atoi(c.DefaultQuery("limit", "100")); err == nil {
		limit = n
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 1000 {
		limit = 1000
	}

	if h.cfg.MetricSamples == nil {
		c.JSON(http.StatusOK, gin.H{"data": []any{}, "total": 0})
		return
	}

	// A `range` (1h|6h|24h|7d|30d) selects a time window and downsamples to a
	// chart-friendly point count; otherwise fall back to the most-recent N.
	if window, ok := parseRange(c.Query("range")); ok {
		since := time.Now().Add(-window)
		rows, err := h.cfg.MetricSamples.Range(c.Request.Context(), id, since, 20000)
		if err != nil {
			failInternal(c, h.cfg.Log, err)
			return
		}
		rows = downsampleMetrics(rows, 400)
		c.JSON(http.StatusOK, gin.H{"data": rows, "total": len(rows), "range": c.Query("range")})
		return
	}

	rows, err := h.cfg.MetricSamples.Recent(c.Request.Context(), id, limit)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": len(rows)})
}

// parseRange maps a range token to a duration.
func parseRange(r string) (time.Duration, bool) {
	switch r {
	case "1h":
		return time.Hour, true
	case "6h":
		return 6 * time.Hour, true
	case "24h":
		return 24 * time.Hour, true
	case "7d":
		return 7 * 24 * time.Hour, true
	case "30d":
		return 30 * 24 * time.Hour, true
	}
	return 0, false
}

// downsampleMetrics strides an ordered sample slice down to at most `target`
// points, always keeping the last sample so the chart ends at "now".
func downsampleMetrics(rows []models.MetricSample, target int) []models.MetricSample {
	if target < 1 || len(rows) <= target {
		return rows
	}
	stride := (len(rows) + target - 1) / target
	out := make([]models.MetricSample, 0, target+1)
	for i := 0; i < len(rows); i += stride {
		out = append(out, rows[i])
	}
	if last := rows[len(rows)-1]; len(out) == 0 || out[len(out)-1].ID != last.ID {
		out = append(out, last)
	}
	return out
}

// checkHealth probes the server's /health + /automation/status on demand.
func (h *serverHandler) checkHealth(c *gin.Context) {
	s, err := h.cfg.Repo.FindByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		failInternal(c, h.cfg.Log, err)
		return
	}

	// Decrypt token secret. A failure here almost always means the stored
	// secret was encrypted by a different install (e.g. imported settings) —
	// surface a clear, actionable message instead of a raw crypto error.
	secretStr, err := h.decryptSecret(s)
	if err != nil {
		// Can't decrypt the stored secret -> the credential is unusable.
		// Persist that so the table stops showing it as valid.
		_ = h.cfg.Repo.UpdateStatus(c.Request.Context(), s.ID, s.Status, models.CredentialInvalid)
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": "stored token secret can't be decrypted here — edit the server and re-enter the token secret",
		})
		return
	}

	client := remote.NewClient(s.BaseURL, s.TokenID, secretStr, s.InsecureSkipVerify)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	result, err := client.CheckHealth(ctx)
	if err != nil {
		failCode(c, h.cfg.Log, http.StatusServiceUnavailable, "check_failed", err)
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

	// Best-effort: refresh the panel's write capabilities so the UI shows only
	// supported actions (M2). Non-fatal on failure.
	if result.Reachable && result.CredentialValid {
		if caps, _, cerr := client.Capabilities(ctx); cerr == nil && len(caps.Actions) > 0 {
			s.Capabilities = models.JSONStringArray(caps.Actions)
			_ = h.cfg.Repo.Update(c.Request.Context(), s)
		}
	}

	c.JSON(http.StatusOK, result)
}

// decryptSecret decrypts the stored token secret via the shared codec (SND-6).
func (h *serverHandler) decryptSecret(s *models.Server) (string, error) {
	return secrets.OpenSecret(h.cfg.SecretKey, s.TokenSecretEnc, h.cfg.AllowPlaintext)
}

// Ensure json import is used.
var _ = json.Marshal

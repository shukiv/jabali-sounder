package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/alert"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/middleware"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/secrets"
)

// AlertingHandlerConfig wires the M5 alerting endpoints: rules, channels,
// maintenance windows, and mutes.
type AlertingHandlerConfig struct {
	Rules          repository.AlertRuleRepository
	Channels       repository.AlertChannelRepository
	Maintenance    repository.MaintenanceRepository
	Muted          repository.MutedAlertRepository
	SecretKey      *secrets.Key
	AllowPlaintext bool
	Log            *slog.Logger
}

var (
	validChannelTypes = map[string]bool{
		alert.TypeWebhook: true, alert.TypeNtfy: true, alert.TypeSMTP: true, alert.TypePagerDuty: true,
	}
	validSeverities = map[string]bool{
		models.SeverityInfo: true, models.SeverityWarning: true, models.SeverityCritical: true,
	}
	validScopeTypes = map[string]bool{"global": true, "environment": true, "server": true}
)

// RegisterAlertingRoutes mounts /api/v1/admin/alert-* and maintenance/muted.
// Reads are any-role; mutations require operator+.
func RegisterAlertingRoutes(g *gin.RouterGroup, cfg AlertingHandlerConfig) {
	if cfg.Rules == nil {
		return
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	h := &alertingHandler{cfg: cfg}
	op := middleware.RequireRole(models.RoleOperator)

	g.GET("/admin/alert-rules", h.listRules)
	g.PUT("/admin/alert-rules/:metric", op, h.updateRule)

	g.GET("/admin/alert-channels", h.listChannels)
	g.POST("/admin/alert-channels", op, h.createChannel)
	g.PUT("/admin/alert-channels/:id", op, h.updateChannel)
	g.DELETE("/admin/alert-channels/:id", op, h.deleteChannel)
	g.POST("/admin/alert-channels/:id/test", op, h.testChannel)

	g.GET("/admin/maintenance", h.listMaintenance)
	g.POST("/admin/maintenance", op, h.createMaintenance)
	g.DELETE("/admin/maintenance/:id", op, h.deleteMaintenance)

	g.GET("/admin/muted", h.listMuted)
	g.POST("/admin/muted", op, h.mute)
	g.DELETE("/admin/muted", op, h.unmute)
}

type alertingHandler struct{ cfg AlertingHandlerConfig }

// --- rules ------------------------------------------------------------------

func (h *alertingHandler) listRules(c *gin.Context) {
	rows, err := h.cfg.Rules.List(c.Request.Context())
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": len(rows)})
}

type updateRuleRequest struct {
	Threshold       float64 `json:"threshold"`
	DurationSeconds int     `json:"duration_seconds"`
	Severity        string  `json:"severity"`
	Enabled         bool    `json:"enabled"`
}

func (h *alertingHandler) updateRule(c *gin.Context) {
	var req updateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}
	if !validSeverities[req.Severity] {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid severity"})
		return
	}
	if req.DurationSeconds < 0 || req.Threshold < 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "threshold and duration must be non-negative"})
		return
	}
	rule := &models.AlertRule{
		Metric:          c.Param("metric"),
		Threshold:       req.Threshold,
		DurationSeconds: req.DurationSeconds,
		Severity:        req.Severity,
		Enabled:         req.Enabled,
	}
	if err := h.cfg.Rules.Update(c.Request.Context(), rule); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown metric"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --- channels ---------------------------------------------------------------

func channelView(ch models.AlertChannel) gin.H {
	return gin.H{
		"id": ch.ID, "name": ch.Name, "type": ch.Type,
		"min_severity": ch.MinSeverity, "enabled": ch.Enabled, "created_at": ch.CreatedAt,
	}
}

func (h *alertingHandler) listChannels(c *gin.Context) {
	if h.cfg.Channels == nil {
		c.JSON(http.StatusOK, gin.H{"data": []gin.H{}, "total": 0})
		return
	}
	rows, err := h.cfg.Channels.List(c.Request.Context())
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, ch := range rows {
		out = append(out, channelView(ch))
	}
	c.JSON(http.StatusOK, gin.H{"data": out, "total": len(out)})
}

type channelRequest struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	MinSeverity string            `json:"min_severity"`
	Enabled     bool              `json:"enabled"`
	Config      map[string]string `json:"config"`
}

// validateChannel checks fields and that the config actually builds a notifier.
func (h *alertingHandler) validateChannel(req *channelRequest) (string, bool) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 120 {
		return "name must be 1-120 chars", false
	}
	if !validChannelTypes[req.Type] {
		return "invalid channel type", false
	}
	if req.MinSeverity == "" {
		req.MinSeverity = models.SeverityWarning
	}
	if !validSeverities[req.MinSeverity] {
		return "invalid min_severity", false
	}
	if _, err := alert.BuildNotifier(req.Type, req.Config, h.cfg.Log); err != nil {
		return err.Error(), false
	}
	return "", true
}

func (h *alertingHandler) sealConfig(cfg map[string]string) ([]byte, error) {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	return secrets.SealSecret(h.cfg.SecretKey, string(raw), h.cfg.AllowPlaintext)
}

func (h *alertingHandler) createChannel(c *gin.Context) {
	if h.cfg.Channels == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "channels unavailable"})
		return
	}
	var req channelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}
	if msg, ok := h.validateChannel(&req); !ok {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": msg})
		return
	}
	enc, err := h.sealConfig(req.Config)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	ch := &models.AlertChannel{
		ID: ids.NewULID(), Name: req.Name, Type: req.Type, ConfigEnc: enc,
		MinSeverity: req.MinSeverity, Enabled: req.Enabled, CreatedAt: time.Now().UTC(),
	}
	if err := h.cfg.Channels.Create(c.Request.Context(), ch); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusCreated, channelView(*ch))
}

func (h *alertingHandler) updateChannel(c *gin.Context) {
	if h.cfg.Channels == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "channels unavailable"})
		return
	}
	var req channelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}
	if msg, ok := h.validateChannel(&req); !ok {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": msg})
		return
	}
	ch := &models.AlertChannel{
		ID: c.Param("id"), Name: req.Name, Type: req.Type,
		MinSeverity: req.MinSeverity, Enabled: req.Enabled,
	}
	// Re-seal config only when the client supplied one (empty -> keep existing).
	if len(req.Config) > 0 {
		enc, err := h.sealConfig(req.Config)
		if err != nil {
			failInternal(c, h.cfg.Log, err)
			return
		}
		ch.ConfigEnc = enc
	}
	if err := h.cfg.Channels.Update(c.Request.Context(), ch); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *alertingHandler) deleteChannel(c *gin.Context) {
	if h.cfg.Channels == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "channels unavailable"})
		return
	}
	if err := h.cfg.Channels.Delete(c.Request.Context(), c.Param("id")); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *alertingHandler) testChannel(c *gin.Context) {
	if h.cfg.Channels == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "channels unavailable"})
		return
	}
	ch, err := h.cfg.Channels.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
		return
	}
	raw, err := secrets.OpenSecret(h.cfg.SecretKey, ch.ConfigEnc, h.cfg.AllowPlaintext)
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	var cfgMap map[string]string
	if err := json.Unmarshal([]byte(raw), &cfgMap); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	n, err := alert.BuildNotifier(ch.Type, cfgMap, h.cfg.Log)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	ev := alert.Event{
		Kind: alert.KindRecovered, ServerName: "Sounder test",
		Message: "test alert from Jabali Sounder", At: time.Now().UTC(),
	}
	if err := n.Notify(c.Request.Context(), ev); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --- maintenance ------------------------------------------------------------

func (h *alertingHandler) listMaintenance(c *gin.Context) {
	if h.cfg.Maintenance == nil {
		c.JSON(http.StatusOK, gin.H{"data": []gin.H{}, "total": 0})
		return
	}
	rows, err := h.cfg.Maintenance.List(c.Request.Context())
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": len(rows)})
}

type maintenanceRequest struct {
	ScopeType  string    `json:"scope_type"`
	ScopeValue string    `json:"scope_value"`
	StartsAt   time.Time `json:"starts_at"`
	EndsAt     time.Time `json:"ends_at"`
	Reason     string    `json:"reason"`
}

func (h *alertingHandler) createMaintenance(c *gin.Context) {
	if h.cfg.Maintenance == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "maintenance unavailable"})
		return
	}
	var req maintenanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}
	if !validScopeTypes[req.ScopeType] {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid scope_type"})
		return
	}
	if req.ScopeType != "global" && strings.TrimSpace(req.ScopeValue) == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "scope_value required for this scope"})
		return
	}
	if !req.EndsAt.After(req.StartsAt) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "ends_at must be after starts_at"})
		return
	}
	w := &models.MaintenanceWindow{
		ID: ids.NewULID(), ScopeType: req.ScopeType, ScopeValue: strings.TrimSpace(req.ScopeValue),
		StartsAt: req.StartsAt.UTC(), EndsAt: req.EndsAt.UTC(),
		Reason: strings.TrimSpace(req.Reason), CreatedBy: middleware.AdminUsername(c), CreatedAt: time.Now().UTC(),
	}
	if err := h.cfg.Maintenance.Create(c.Request.Context(), w); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusCreated, w)
}

func (h *alertingHandler) deleteMaintenance(c *gin.Context) {
	if h.cfg.Maintenance == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "maintenance unavailable"})
		return
	}
	if err := h.cfg.Maintenance.Delete(c.Request.Context(), c.Param("id")); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --- muted ------------------------------------------------------------------

func (h *alertingHandler) listMuted(c *gin.Context) {
	if h.cfg.Muted == nil {
		c.JSON(http.StatusOK, gin.H{"data": []gin.H{}, "total": 0})
		return
	}
	rows, err := h.cfg.Muted.List(c.Request.Context())
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": len(rows)})
}

type muteRequest struct {
	ServerID string `json:"server_id"`
	Kind     string `json:"kind"`
}

func (h *alertingHandler) mute(c *gin.Context) {
	if h.cfg.Muted == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "mute unavailable"})
		return
	}
	var req muteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}
	if req.ServerID == "" || req.Kind == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "server_id and kind required"})
		return
	}
	if err := h.cfg.Muted.Mute(c.Request.Context(), req.ServerID, req.Kind, middleware.AdminUsername(c), time.Now().UTC()); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *alertingHandler) unmute(c *gin.Context) {
	if h.cfg.Muted == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "mute unavailable"})
		return
	}
	serverID := c.Query("server_id")
	kind := c.Query("kind")
	if serverID == "" || kind == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "server_id and kind required"})
		return
	}
	if err := h.cfg.Muted.Unmute(c.Request.Context(), serverID, kind); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/middleware"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/policy"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

// PolicyHandlerConfig wires the compliance/drift endpoint (SND-32) plus the
// ignore/restore exceptions (SND-107).
type PolicyHandlerConfig struct {
	Repo         repository.ServerRepository
	Exceptions   repository.PolicyExceptionRepository
	Audit        repository.AuditRepository
	CertWarnDays int
	Log          *slog.Logger
}

// RegisterPolicyRoutes mounts /api/v1/admin/policy. Reads are open to any authed
// role; ignoring/restoring a violation requires operator+.
func RegisterPolicyRoutes(g *gin.RouterGroup, cfg PolicyHandlerConfig) {
	if cfg.Repo == nil {
		return
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	h := &policyHandler{cfg: cfg}
	g.GET("/admin/policy", h.get)
	op := middleware.RequireRole(models.RoleOperator)
	g.POST("/admin/policy/exceptions", op, h.ignore)
	g.DELETE("/admin/policy/exceptions", op, h.restore)
}

type policyHandler struct{ cfg PolicyHandlerConfig }

// violationView is a policy.Violation plus its ignore metadata for the UI.
type violationView struct {
	policy.Violation
	Ignored   bool   `json:"ignored"`
	Reason    string `json:"reason,omitempty"`
	IgnoredBy string `json:"ignored_by,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
	IgnoredAt string `json:"ignored_at,omitempty"`
}

func exKey(serverID, check string) string { return serverID + "|" + check }

func (h *policyHandler) get(c *gin.Context) {
	servers, err := h.cfg.Repo.List(c.Request.Context())
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	now := time.Now()
	violations := policy.Evaluate(servers, now, policy.Options{CertWarnDays: h.cfg.CertWarnDays})

	// Active (non-expired) exceptions, keyed by server+check.
	exByKey := map[string]models.PolicyException{}
	if h.cfg.Exceptions != nil {
		if exs, err := h.cfg.Exceptions.ListActive(c.Request.Context(), now); err == nil {
			for _, e := range exs {
				exByKey[exKey(e.ServerID, e.Check)] = e
			}
		} else {
			h.cfg.Log.Warn("policy: list exceptions failed", "error", err)
		}
	}

	active := make([]violationView, 0, len(violations))
	ignored := make([]violationView, 0)
	byCheck := map[string]int{}
	offenders := map[string]bool{}
	critical := 0
	for _, vi := range violations {
		if ex, ok := exByKey[exKey(vi.ServerID, vi.Check)]; ok {
			view := violationView{Violation: vi, Ignored: true, Reason: ex.Reason, IgnoredBy: ex.CreatedBy, IgnoredAt: ex.CreatedAt.UTC().Format(time.RFC3339)}
			if ex.ExpiresAt.Valid {
				view.ExpiresAt = ex.ExpiresAt.Time.UTC().Format(time.RFC3339)
			}
			ignored = append(ignored, view)
			continue // excluded from active risk + counts
		}
		active = append(active, violationView{Violation: vi})
		byCheck[vi.Check]++
		offenders[vi.ServerID] = true
		if vi.Severity == models.SeverityCritical {
			critical++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"violations":      active,
		"ignored":         ignored,
		"total":           len(active),
		"critical":        critical,
		"by_check":        byCheck,
		"servers_total":   len(servers),
		"servers_at_risk": len(offenders),
	})
}

type ignoreRequest struct {
	ServerID  string `json:"server_id"`
	Check     string `json:"check"`
	Reason    string `json:"reason"`
	ExpiresAt string `json:"expires_at"` // RFC3339 or YYYY-MM-DD; empty = never
}

var knownChecks = map[string]bool{
	policy.CheckInsecureTLS: true, policy.CheckCredentialInvalid: true,
	policy.CheckUnreachable: true, policy.CheckCertExpiring: true, policy.CheckVersionDrift: true,
}

func (h *policyHandler) ignore(c *gin.Context) {
	if h.cfg.Exceptions == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "exceptions unavailable"})
		return
	}
	var req ignoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}
	if req.ServerID == "" || !knownChecks[req.Check] {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "server_id and a valid check are required"})
		return
	}
	if len(req.Reason) == 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "a reason is required"})
		return
	}
	if len(req.Reason) > 400 {
		req.Reason = req.Reason[:400]
	}
	expiresAt, err := parseExpiry(req.ExpiresAt)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid expires_at"})
		return
	}
	if err := h.cfg.Exceptions.Ignore(c.Request.Context(), req.ServerID, req.Check, req.Reason, expiresAt, middleware.AdminUsername(c), time.Now().UTC()); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	auditEvent(h.cfg.Log, h.cfg.Audit, c, "policy.ignore", req.ServerID, req.Check)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *policyHandler) restore(c *gin.Context) {
	if h.cfg.Exceptions == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "exceptions unavailable"})
		return
	}
	serverID := c.Query("server_id")
	check := c.Query("check")
	if serverID == "" || check == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "server_id and check required"})
		return
	}
	if _, err := h.cfg.Exceptions.Restore(c.Request.Context(), serverID, check); err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	auditEvent(h.cfg.Log, h.cfg.Audit, c, "policy.restore", serverID, check)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// parseExpiry accepts an empty string (never expires), a full RFC3339 timestamp,
// or a bare YYYY-MM-DD date (interpreted as end of that UTC day).
func parseExpiry(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t, nil
	}
	if d, err := time.Parse("2006-01-02", s); err == nil {
		end := d.Add(24*time.Hour - time.Second).UTC()
		return &end, nil
	}
	return nil, errBadExpiry
}

var errBadExpiry = &policyError{"invalid expires_at"}

type policyError struct{ msg string }

func (e *policyError) Error() string { return e.msg }

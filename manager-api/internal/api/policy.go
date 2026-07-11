package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/policy"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

// PolicyHandlerConfig wires the compliance/drift endpoint (SND-32).
type PolicyHandlerConfig struct {
	Repo         repository.ServerRepository
	CertWarnDays int
	Log          *slog.Logger
}

// RegisterPolicyRoutes mounts /api/v1/admin/policy (any authed role).
func RegisterPolicyRoutes(g *gin.RouterGroup, cfg PolicyHandlerConfig) {
	if cfg.Repo == nil {
		return
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	h := &policyHandler{cfg: cfg}
	g.GET("/admin/policy", h.get)
}

type policyHandler struct{ cfg PolicyHandlerConfig }

func (h *policyHandler) get(c *gin.Context) {
	servers, err := h.cfg.Repo.List(c.Request.Context())
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}
	violations := policy.Evaluate(servers, time.Now(), policy.Options{CertWarnDays: h.cfg.CertWarnDays})
	byCheck := map[string]int{}
	offenders := map[string]bool{}
	for _, vi := range violations {
		byCheck[vi.Check]++
		offenders[vi.ServerID] = true
	}
	c.JSON(http.StatusOK, gin.H{
		"violations":      violations,
		"total":           len(violations),
		"by_check":        byCheck,
		"servers_total":   len(servers),
		"servers_at_risk": len(offenders),
	})
}

package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/updater"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/version"
)

// VersionHandlerConfig wires the version + update-check endpoint (update
// mechanism). Updater may be nil to disable the GitHub check (current-only).
type VersionHandlerConfig struct {
	Updater *updater.Client
	Log     *slog.Logger
}

// RegisterVersionRoutes mounts /api/v1/version (any authed role).
func RegisterVersionRoutes(g *gin.RouterGroup, cfg VersionHandlerConfig) {
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	h := &versionHandler{cfg: cfg}
	g.GET("/version", h.get)
}

type versionHandler struct{ cfg VersionHandlerConfig }

func (h *versionHandler) get(c *gin.Context) {
	info := version.Current()
	resp := gin.H{
		"version":          info.Version,
		"commit":           info.Commit,
		"date":             info.Date,
		"is_dev":           version.IsDev(),
		"update_available": false,
	}
	if h.cfg.Updater == nil {
		c.JSON(http.StatusOK, resp)
		return
	}
	st, err := h.cfg.Updater.Check(c.Request.Context(), info.Version, time.Now())
	if err != nil {
		// Non-fatal: the current version is always reported; the update check is
		// best-effort (offline, rate-limited, or no releases yet).
		h.cfg.Log.Debug("update check failed", "error", err)
		resp["update_error"] = "update check unavailable"
		c.JSON(http.StatusOK, resp)
		return
	}
	resp["latest"] = st.Latest
	resp["update_available"] = st.UpdateAvailable
	resp["release_url"] = st.ReleaseURL
	resp["published_at"] = st.PublishedAt
	c.JSON(http.StatusOK, resp)
}

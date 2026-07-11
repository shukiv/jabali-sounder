package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/version"
)

// RegisterHealthRoutes wires the /health endpoint.
func RegisterHealthRoutes(r *gin.Engine) {
	r.GET("/health", healthHandler)
}

// healthHandler is an unauthenticated liveness probe. It reports the running
// build so operators can confirm which release is actually serving (SND-33) —
// the version is stamped via -ldflags into internal/version, not a stale local.
func healthHandler(c *gin.Context) {
	info := version.Current()
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": info.Version,
		"commit":  info.Commit,
	})
}

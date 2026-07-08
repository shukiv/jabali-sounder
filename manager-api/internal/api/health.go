package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Version is the build-time version string. Overridden at link time with -ldflags.
var Version = "dev"

// RegisterHealthRoutes wires the /health endpoint.
func RegisterHealthRoutes(r *gin.Engine) {
	r.GET("/health", healthHandler)
}

func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": Version,
	})
}

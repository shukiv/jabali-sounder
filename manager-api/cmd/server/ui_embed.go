//go:build embedui

// Embeds the built SPA into the server binary so a single headless binary
// serves both the API (/api/v1, /health) and the UI (everything else) on one
// port. Built with `-tags embedui` after staging manager-ui/dist into
// manager-api/cmd/server/dist. Without the tag, attachSPA is a no-op
// (ui_noembed.go) and the binary is API-only.
package main

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var uiAssets embed.FS

func attachSPA(engine *gin.Engine) {
	distFS, err := fs.Sub(uiAssets, "dist")
	if err != nil {
		return
	}
	fileServer := http.FileServer(http.FS(distFS))
	engine.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		// API + health are owned by the router; unknown ones stay JSON 404.
		if strings.HasPrefix(p, "/api/") || p == "/health" {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
			return
		}
		// Real asset (js/css/svg/…) — serve it as-is.
		if reqPath := strings.TrimPrefix(p, "/"); reqPath != "" {
			if _, statErr := fs.Stat(distFS, reqPath); statErr == nil {
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
		}
		// Root or unknown route — serve index.html at "/" (SPA routing).
		// Rewriting to "/index.html" would make FileServer 301 back to "/".
		c.Request.URL.Path = "/"
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}

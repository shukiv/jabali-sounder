// Package app wires HTTP routes, middleware, and lifecycle together.
package app

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/api"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/middleware"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/remote"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/secrets"
)

// Deps bundles the collaborators NewWithDeps needs.
type Deps struct {
	Log           *slog.Logger
	ServerRepo    repository.ServerRepository
	HeartbeatRepo repository.HeartbeatRepository
	AdminRepo     repository.AdminRepository
	SecretKey     *secrets.Key
	JWTSecret     string
}

// NewWithDeps creates a Gin engine with all routes mounted.
func NewWithDeps(deps Deps) *gin.Engine {
	if deps.Log == nil {
		deps.Log = slog.Default()
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// /health — unauthenticated liveness probe.
	api.RegisterHealthRoutes(r)

	// /api/v1 — API surface.
	v1 := r.Group("/api/v1")

	// API responses must never be cached. The desktop WebView (and browsers)
	// otherwise serve a stale GET after a mutation, so tables would not reflect
	// enroll/disable/enable/delete until a hard reload.
	v1.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Next()
	})

	// Auth endpoints (login + me) — no auth required for login.
	api.RegisterAuthRoutes(v1, api.AuthHandlerConfig{
		AdminRepo: deps.AdminRepo,
		JWTSecret: deps.JWTSecret,
		JWTTTL:    24 * time.Hour,
	})

	// Protected admin routes — require JWT.
	adminGroup := v1.Group("")
	if deps.JWTSecret != "" {
		adminGroup.Use(middleware.AuthMiddleware(deps.JWTSecret))
	}

	// Server enrollment + dashboard (behind auth).
	api.RegisterServerRoutes(adminGroup, api.ServerHandlerConfig{
		Repo:      deps.ServerRepo,
		SecretKey: deps.SecretKey,
		Log:       deps.Log,
	})

	api.RegisterDashboardRoutes(adminGroup, api.DashboardHandlerConfig{
		Repo:          deps.ServerRepo,
		HeartbeatRepo: deps.HeartbeatRepo,
		Log:           deps.Log,
	})

	api.RegisterInventoryRoutes(adminGroup, api.InventoryHandlerConfig{
		Repo:      deps.ServerRepo,
		SecretKey: deps.SecretKey,
		Log:       deps.Log,
	})

	api.RegisterMonitorRoutes(adminGroup, api.MonitorHandlerConfig{
		Repo:      deps.ServerRepo,
		SecretKey: deps.SecretKey,
		Log:       deps.Log,
	})

	api.RegisterMailRoutes(adminGroup, api.MailHandlerConfig{
		Repo:      deps.ServerRepo,
		SecretKey: deps.SecretKey,
		Log:       deps.Log,
	})

	api.RegisterSettingsRoutes(adminGroup, api.SettingsHandlerConfig{
		Repo:      deps.ServerRepo,
		SecretKey: deps.SecretKey,
		Log:       deps.Log,
	})

	return r
}

// Ensure remote is referenced (used by the health-check handler).
var _ = remote.NewClient

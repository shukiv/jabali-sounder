package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/remote"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/secrets"
)

// InventoryHandlerConfig wires the cross-server inventory endpoints.
type InventoryHandlerConfig struct {
	Repo      repository.ServerRepository
	SecretKey *secrets.Key
	Log       *slog.Logger
	// AllowPlaintext permits the dev hex-plaintext token fallback (SND-6).
	AllowPlaintext bool
}

// RegisterInventoryRoutes mounts GET /api/v1/admin/domains + /api/v1/admin/users.
func RegisterInventoryRoutes(g *gin.RouterGroup, cfg InventoryHandlerConfig) {
	if cfg.Repo == nil {
		return
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	h := &inventoryHandler{cfg: cfg}
	g.GET("/admin/domains", h.domains)
	g.GET("/admin/users", h.users)
}

type inventoryHandler struct{ cfg InventoryHandlerConfig }

// domainRow is a domain tagged with its owning server.
type domainRow struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	UserID     string `json:"user_id"`
	IsEnabled  bool   `json:"is_enabled"`
	ServerID   string `json:"server_id"`
	ServerName string `json:"server_name"`
}

func (h *inventoryHandler) domains(c *gin.Context) {
	servers, err := h.cfg.Repo.List(c.Request.Context())
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}

	results := make([]domainRow, 0)
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(c.Request.Context())
	g.SetLimit(8)

	for _, s := range servers {
		if s.Status != "active" {
			continue
		}
		s := s // capture
		g.Go(func() error {
			secret, err := h.decryptSecret(&s)
			if err != nil {
				h.cfg.Log.Warn("decrypt secret failed", "server", s.Name, "error", err)
				return nil
			}
			client := remote.NewClient(s.BaseURL, s.TokenID, secret, s.InsecureSkipVerify)
			ctx, cancel := context.WithTimeout(gctx, 10*time.Second)
			defer cancel()
			resp, _, err := client.Domains(ctx)
			if err != nil {
				h.cfg.Log.Warn("fetch domains failed", "server", s.Name, "error", err)
				return nil
			}
			mu.Lock()
			for _, d := range resp.Data {
				results = append(results, domainRow{
					ID:         d.ID,
					Name:       d.Name,
					UserID:     d.UserID,
					IsEnabled:  d.IsEnabled,
					ServerID:   s.ID,
					ServerName: s.Name,
				})
			}
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()

	c.JSON(http.StatusOK, gin.H{
		"data":      results,
		"total":     len(results),
		"page":      1,
		"page_size": len(results),
	})
}

// userRow is a user tagged with its owning server.
type userRow struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	Username   string `json:"username"`
	PackageID  string `json:"package_id"`
	IsAdmin    bool   `json:"is_admin"`
	ServerID   string `json:"server_id"`
	ServerName string `json:"server_name"`
}

func (h *inventoryHandler) users(c *gin.Context) {
	servers, err := h.cfg.Repo.List(c.Request.Context())
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}

	results := make([]userRow, 0)
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(c.Request.Context())
	g.SetLimit(8)

	for _, s := range servers {
		if s.Status != "active" {
			continue
		}
		s := s
		g.Go(func() error {
			secret, err := h.decryptSecret(&s)
			if err != nil {
				h.cfg.Log.Warn("decrypt secret failed", "server", s.Name, "error", err)
				return nil
			}
			client := remote.NewClient(s.BaseURL, s.TokenID, secret, s.InsecureSkipVerify)
			ctx, cancel := context.WithTimeout(gctx, 10*time.Second)
			defer cancel()
			resp, _, err := client.Users(ctx)
			if err != nil {
				h.cfg.Log.Warn("fetch users failed", "server", s.Name, "error", err)
				return nil
			}
			mu.Lock()
			for _, u := range resp.Data {
				results = append(results, userRow{
					ID:         u.ID,
					Email:      u.Email,
					Username:   u.Username,
					PackageID:  u.PackageID,
					IsAdmin:    u.IsAdmin,
					ServerID:   s.ID,
					ServerName: s.Name,
				})
			}
			mu.Unlock()
			return nil
		})
	}
	_ = g.Wait()

	c.JSON(http.StatusOK, gin.H{
		"data":      results,
		"total":     len(results),
		"page":      1,
		"page_size": len(results),
	})
}

func (h *inventoryHandler) decryptSecret(s *models.Server) (string, error) {
	return secrets.OpenSecret(h.cfg.SecretKey, s.TokenSecretEnc, h.cfg.AllowPlaintext)
}

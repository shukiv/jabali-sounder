package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/remote"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/secrets"
)

const mailTimeout = 10 * time.Second

// MailHandlerConfig wires the cross-server mail inventory endpoint.
type MailHandlerConfig struct {
	Repo      repository.ServerRepository
	SecretKey *secrets.Key
	Log       *slog.Logger
}

// RegisterMailRoutes mounts GET /api/v1/admin/mail.
func RegisterMailRoutes(g *gin.RouterGroup, cfg MailHandlerConfig) {
	if cfg.Repo == nil {
		return
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	h := &mailHandler{cfg: cfg}
	g.GET("/admin/mail", h.list)
}

type mailHandler struct{ cfg MailHandlerConfig }

type mailSnapshotEntry struct {
	Server           monitorServerRef           `json:"server"`
	Available        bool                       `json:"available"`
	Mailboxes        []remote.Mailbox           `json:"mailboxes"`
	Groups           []remote.MailGroup         `json:"groups"`
	Forwarders       []remote.MailForwarder     `json:"forwarders"`
	DomainForwarders []remote.DomainForwarder   `json:"domain_forwarders"`
	Autoresponders   []remote.MailAutoresponder `json:"autoresponders"`
	Error            string                     `json:"error,omitempty"`
}

func (h *mailHandler) list(c *gin.Context) {
	servers, err := h.cfg.Repo.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list servers: " + err.Error()})
		return
	}

	results := make([]mailSnapshotEntry, len(servers))
	g, gctx := errgroup.WithContext(c.Request.Context())
	g.SetLimit(8)

	for i := range servers {
		i := i
		s := servers[i]
		results[i] = newMailSnapshotEntry(s)
		if s.Status != models.ServerStatusActive {
			results[i].Error = "server is not active"
			continue
		}
		g.Go(func() error {
			results[i] = h.fetchServerMail(gctx, s)
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

func (h *mailHandler) fetchServerMail(ctx context.Context, s models.Server) mailSnapshotEntry {
	entry := newMailSnapshotEntry(s)
	client, err := h.clientForServer(&s)
	if err != nil {
		entry.Error = err.Error()
		return entry
	}

	var (
		mu       sync.Mutex
		errParts []string
	)
	addErr := func(part string) {
		mu.Lock()
		errParts = append(errParts, part)
		mu.Unlock()
	}
	markAvailable := func() {
		mu.Lock()
		entry.Available = true
		mu.Unlock()
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(5)

	g.Go(func() error {
		subCtx, cancel := context.WithTimeout(gctx, mailTimeout)
		defer cancel()
		resp, code, err := client.Mailboxes(subCtx)
		if err != nil {
			addErr(mailRemoteError("mailboxes", code, err))
			return nil
		}
		mu.Lock()
		entry.Available = true
		entry.Mailboxes = resp.Data
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		subCtx, cancel := context.WithTimeout(gctx, mailTimeout)
		defer cancel()
		resp, code, err := client.MailGroups(subCtx)
		if err != nil {
			addErr(mailRemoteError("groups", code, err))
			return nil
		}
		mu.Lock()
		entry.Available = true
		entry.Groups = resp.Data
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		subCtx, cancel := context.WithTimeout(gctx, mailTimeout)
		defer cancel()
		resp, code, err := client.MailForwarders(subCtx)
		if err != nil {
			addErr(mailRemoteError("forwarders", code, err))
			return nil
		}
		mu.Lock()
		entry.Available = true
		entry.Forwarders = resp.Data
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		subCtx, cancel := context.WithTimeout(gctx, mailTimeout)
		defer cancel()
		resp, code, err := client.DomainForwarders(subCtx)
		if err != nil {
			addErr(mailRemoteError("domain forwarders", code, err))
			return nil
		}
		mu.Lock()
		entry.Available = true
		entry.DomainForwarders = resp.Data
		mu.Unlock()
		return nil
	})

	g.Go(func() error {
		subCtx, cancel := context.WithTimeout(gctx, mailTimeout)
		defer cancel()
		resp, code, err := client.MailAutoresponders(subCtx)
		if err != nil {
			addErr(mailRemoteError("autoresponders", code, err))
			return nil
		}
		markAvailable()
		mu.Lock()
		entry.Autoresponders = resp.Data
		mu.Unlock()
		return nil
	})

	_ = g.Wait()
	if len(errParts) > 0 {
		entry.Error = strings.Join(errParts, " · ")
	}
	return entry
}

func newMailSnapshotEntry(s models.Server) mailSnapshotEntry {
	return mailSnapshotEntry{
		Server:           serverRef(s),
		Mailboxes:        []remote.Mailbox{},
		Groups:           []remote.MailGroup{},
		Forwarders:       []remote.MailForwarder{},
		DomainForwarders: []remote.DomainForwarder{},
		Autoresponders:   []remote.MailAutoresponder{},
	}
}

func (h *mailHandler) clientForServer(s *models.Server) (*remote.Client, error) {
	secret, err := (&inventoryHandler{cfg: InventoryHandlerConfig{
		Repo:      h.cfg.Repo,
		SecretKey: h.cfg.SecretKey,
		Log:       h.cfg.Log,
	}}).decryptSecret(s)
	if err != nil {
		return nil, err
	}
	return remote.NewClient(s.BaseURL, s.TokenID, secret), nil
}

func mailRemoteError(part string, code int, err error) string {
	if code > 0 {
		return fmt.Sprintf("%s unavailable: HTTP %d: %v", part, code, err)
	}
	return fmt.Sprintf("%s unavailable: %v", part, err)
}

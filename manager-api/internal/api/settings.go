package api

import (
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/remote"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/secrets"
)

// SettingsHandlerConfig wires import/export endpoints.
type SettingsHandlerConfig struct {
	Repo      repository.ServerRepository
	SecretKey *secrets.Key
	Log       *slog.Logger
	// AllowPrivateTargets disables the SSRF guard on import (SND-4).
	AllowPrivateTargets bool
	// AllowPlaintext permits the dev hex-plaintext token fallback (SND-6).
	AllowPlaintext bool
}

// RegisterSettingsRoutes mounts /api/v1/admin/settings.
func RegisterSettingsRoutes(g *gin.RouterGroup, cfg SettingsHandlerConfig) {
	if cfg.Repo == nil {
		return
	}
	if cfg.Log == nil {
		cfg.Log = slog.Default()
	}
	h := &settingsHandler{cfg: cfg}
	settings := g.Group("/admin/settings")
	settings.GET("/export", h.export)
	settings.POST("/import", h.importSettings)
}

type settingsHandler struct{ cfg SettingsHandlerConfig }

type settingsExport struct {
	Kind       string                 `json:"kind"`
	Version    int                    `json:"version"`
	ExportedAt time.Time              `json:"exported_at"`
	Settings   map[string]any         `json:"settings"`
	Servers    []settingsServerExport `json:"servers"`
}

type settingsServerExport struct {
	ID               string   `json:"id,omitempty"`
	Name             string   `json:"name"`
	BaseURL          string   `json:"base_url"`
	TokenID          string   `json:"token_id"`
	TokenSecret      string   `json:"token_secret,omitempty"`
	TokenSecretEnc   string   `json:"token_secret_enc,omitempty"`
	SecretFormat     string   `json:"secret_format,omitempty"`
	Scopes           []string `json:"scopes"`
	Tags             []string `json:"tags,omitempty"`
	Version          string   `json:"version,omitempty"`
	Capabilities     []string `json:"capabilities,omitempty"`
	HealthURL        string   `json:"health_url,omitempty"`
	Status           string   `json:"status,omitempty"`
	CredentialStatus string   `json:"credential_status,omitempty"`
}

type settingsImportResult struct {
	Imported int      `json:"imported"`
	Updated  int      `json:"updated"`
	Created  int      `json:"created"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors,omitempty"`
}

func (h *settingsHandler) export(c *gin.Context) {
	servers, err := h.cfg.Repo.List(c.Request.Context())
	if err != nil {
		failInternal(c, h.cfg.Log, err)
		return
	}

	out := settingsExport{
		Kind:       "jabali-sounder-settings",
		Version:    1,
		ExportedAt: time.Now().UTC(),
		Settings: map[string]any{
			"default_panel_scheme": "https",
			"default_panel_port":   8443,
		},
		Servers: make([]settingsServerExport, 0, len(servers)),
	}
	for _, server := range servers {
		out.Servers = append(out.Servers, settingsServerExport{
			ID:               server.ID,
			Name:             server.Name,
			BaseURL:          server.BaseURL,
			TokenID:          server.TokenID,
			TokenSecretEnc:   base64.StdEncoding.EncodeToString(server.TokenSecretEnc),
			SecretFormat:     "sounder-local-encrypted",
			Scopes:           []string(server.Scopes),
			Tags:             []string(server.Tags),
			Version:          server.Version,
			Capabilities:     []string(server.Capabilities),
			HealthURL:        server.HealthURL,
			Status:           string(server.Status),
			CredentialStatus: string(server.CredentialStatus),
		})
	}

	c.Header("Content-Disposition", `attachment; filename="jabali-sounder-settings.json"`)
	c.JSON(http.StatusOK, out)
}

func (h *settingsHandler) importSettings(c *gin.Context) {
	var req settingsExport
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed_json"})
		return
	}
	if req.Kind != "" && req.Kind != "jabali-sounder-settings" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "unsupported_settings_export"})
		return
	}

	result := settingsImportResult{}
	for i := range req.Servers {
		item := req.Servers[i]
		if err := h.importServer(c, item, &result); err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, err.Error())
		}
	}

	status := http.StatusOK
	if len(result.Errors) > 0 && result.Imported == 0 {
		status = http.StatusUnprocessableEntity
	}
	c.JSON(status, result)
}

func (h *settingsHandler) importServer(c *gin.Context, item settingsServerExport, result *settingsImportResult) error {
	name := strings.TrimSpace(item.Name)
	if name == "" || len(name) > 200 {
		return fmt.Errorf("%s: name must be 1-200 chars", importServerLabel(item))
	}
	baseURL, err := normalizePanelBaseURL(item.BaseURL)
	if err != nil {
		return fmt.Errorf("%s: invalid panel hostname or URL", name)
	}
	if err := validatePublicTarget(baseURL, h.cfg.AllowPrivateTargets); err != nil {
		return fmt.Errorf("%s: target host is not an allowed (public) address", name)
	}
	if strings.TrimSpace(item.TokenID) == "" {
		return fmt.Errorf("%s: token_id is required", name)
	}

	var existing *models.Server
	if item.ID != "" {
		existing, err = h.cfg.Repo.FindByID(c.Request.Context(), item.ID)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			h.cfg.Log.Error("import: find existing failed", "server", name, "error", err)
			return fmt.Errorf("%s: lookup failed", name)
		}
	}

	secretEnc, err := h.importSecret(item, existing)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	status := models.ServerStatusActive
	if item.Status == string(models.ServerStatusDisabled) {
		status = models.ServerStatusDisabled
	}
	credStatus := models.CredentialUnknown
	if item.CredentialStatus == string(models.CredentialValid) {
		credStatus = models.CredentialValid
	}
	if item.CredentialStatus == string(models.CredentialInvalid) {
		credStatus = models.CredentialInvalid
	}

	// A token secret encrypted by a DIFFERENT Sounder install can't be
	// decrypted with this install's key. Don't store an unusable blob (later
	// panel calls would fail with "message authentication failed") — drop it,
	// mark the credential invalid, and tell the operator to re-enter the
	// secret. The server still imports so it can be edited.
	if len(secretEnc) > 0 && h.cfg.SecretKey != nil && item.TokenSecret == "" {
		if _, oerr := h.cfg.SecretKey.Open(secretEnc); oerr != nil {
			secretEnc = nil
			credStatus = models.CredentialInvalid
			result.Errors = append(result.Errors,
				fmt.Sprintf("%s: token secret is from a different Sounder install and can't be decrypted here — edit the server and re-enter the token secret", name))
		}
	}

	scopes := item.Scopes
	if len(scopes) == 0 {
		scopes = []string{remote.ScopeReadAll}
	}
	tags, err := normalizeServerTags(item.Tags)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}

	server := &models.Server{
		ID:               item.ID,
		Name:             name,
		BaseURL:          baseURL,
		TokenID:          strings.TrimSpace(item.TokenID),
		TokenSecretEnc:   secretEnc,
		Scopes:           models.JSONStringArray(scopes),
		Tags:             models.JSONStringArray(tags),
		Version:          item.Version,
		Capabilities:     models.JSONStringArray(item.Capabilities),
		HealthURL:        baseURL + "/health",
		Status:           status,
		CredentialStatus: credStatus,
	}
	if server.ID == "" {
		server.ID = ids.NewULID()
	}

	if existing != nil {
		server.CreatedAt = existing.CreatedAt
		if err := h.cfg.Repo.Update(c.Request.Context(), server); err != nil {
			h.cfg.Log.Error("import: update failed", "server", name, "error", err)
			return fmt.Errorf("%s: update failed", name)
		}
		result.Updated++
		result.Imported++
		auditServerMutation(h.cfg.Log, c, "import-update", server.ID, server.Name)
		return nil
	}

	if err := h.cfg.Repo.Create(c.Request.Context(), server); err != nil {
		h.cfg.Log.Error("import: create failed", "server", name, "error", err)
		return fmt.Errorf("%s: create failed", name)
	}
	result.Created++
	result.Imported++
	auditServerMutation(h.cfg.Log, c, "import-create", server.ID, server.Name)
	return nil
}

func (h *settingsHandler) importSecret(item settingsServerExport, existing *models.Server) ([]byte, error) {
	if item.TokenSecret != "" {
		return secrets.SealSecret(h.cfg.SecretKey, item.TokenSecret, h.cfg.AllowPlaintext)
	}
	if item.TokenSecretEnc != "" {
		secretEnc, err := base64.StdEncoding.DecodeString(item.TokenSecretEnc)
		if err != nil {
			return nil, fmt.Errorf("encrypted token secret is not valid base64")
		}
		return secretEnc, nil
	}
	if existing != nil {
		return existing.TokenSecretEnc, nil
	}
	return nil, fmt.Errorf("token_secret is required for new servers when no encrypted token is present")
}

func importServerLabel(item settingsServerExport) string {
	if item.Name != "" {
		return item.Name
	}
	if item.ID != "" {
		return item.ID
	}
	return "server"
}

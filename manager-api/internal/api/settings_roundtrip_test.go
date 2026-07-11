package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

// TestSettingsRoundTripPreservesInsecureSkipVerify guards the export/import
// round-trip: a server enrolled with insecure_skip_verify (needed for
// self-signed panels) must NOT lose the flag on import, which would flip it to
// TLS-verify and make a self-signed panel read as unreachable.
func TestSettingsRoundTripPreservesInsecureSkipVerify(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newTestServerRepo(t)
	srv := &models.Server{
		ID:                 ids.NewULID(),
		Name:               "tls-panel",
		BaseURL:            "https://panel.example:8443",
		TokenID:            "TID",
		TokenSecretEnc:     []byte("blob"),
		Scopes:             models.JSONStringArray{"read:*"},
		InsecureSkipVerify: true,
		Status:             models.ServerStatusActive,
		CredentialStatus:   models.CredentialValid,
	}
	if err := repo.Create(context.Background(), srv); err != nil {
		t.Fatalf("seed: %v", err)
	}

	r := gin.New()
	r.Use(asRole("operator", "01OP"))
	RegisterSettingsRoutes(r.Group("/api/v1"), SettingsHandlerConfig{
		Repo: repo, AllowPrivateTargets: true, AllowPlaintext: true,
	})

	// Export and confirm the flag is present + true.
	ew := httptest.NewRecorder()
	r.ServeHTTP(ew, httptest.NewRequest(http.MethodGet, "/api/v1/admin/settings/export", nil))
	if ew.Code != http.StatusOK {
		t.Fatalf("export status = %d", ew.Code)
	}
	var exp settingsExport
	if err := json.Unmarshal(ew.Body.Bytes(), &exp); err != nil {
		t.Fatalf("export json: %v", err)
	}
	if len(exp.Servers) != 1 || !exp.Servers[0].InsecureSkipVerify {
		t.Fatalf("export did not carry insecure_skip_verify: %+v", exp.Servers)
	}

	// Simulate the bug's effect, then re-import the exported settings.
	srv.InsecureSkipVerify = false
	if err := repo.Update(context.Background(), srv); err != nil {
		t.Fatalf("flip: %v", err)
	}
	iw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/settings/import", bytes.NewReader(ew.Body.Bytes()))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(iw, req)
	if iw.Code != http.StatusOK {
		t.Fatalf("import status = %d, body = %s", iw.Code, iw.Body.String())
	}

	got, err := repo.FindByID(context.Background(), srv.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if !got.InsecureSkipVerify {
		t.Fatalf("import dropped insecure_skip_verify (server now verifies TLS)")
	}
}

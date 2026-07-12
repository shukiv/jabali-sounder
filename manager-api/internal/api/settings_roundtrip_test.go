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
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/secrets"
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

// TestExportIncludeSecrets covers the opt-in plaintext-secrets export used to
// migrate servers to another install: operator+ gets the decrypted token, a
// viewer is forbidden, and a default export stays sealed.
func TestExportIncludeSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newTestServerRepo(t)
	enc, err := secrets.SealSecret(nil, "s3cr3t-token", true) // dev hex-plaintext fallback
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	srv := &models.Server{
		ID: ids.NewULID(), Name: "p", BaseURL: "https://p:8443", TokenID: "TID",
		TokenSecretEnc: enc, Scopes: models.JSONStringArray{"read:*"},
		Status: models.ServerStatusActive, CredentialStatus: models.CredentialValid,
	}
	if err := repo.Create(context.Background(), srv); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cfg := SettingsHandlerConfig{Repo: repo, AllowPrivateTargets: true, AllowPlaintext: true}

	get := func(role, url string) *httptest.ResponseRecorder {
		r := gin.New()
		r.Use(asRole(role, "01"+role))
		RegisterSettingsRoutes(r.Group("/api/v1"), cfg)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, url, nil))
		return w
	}

	// Viewer may not export secrets.
	if w := get("viewer", "/api/v1/admin/settings/export?include_secrets=true"); w.Code != http.StatusForbidden {
		t.Fatalf("viewer include_secrets = %d, want 403", w.Code)
	}

	// Operator gets the plaintext token, no sealed blob, format=plaintext.
	w := get("operator", "/api/v1/admin/settings/export?include_secrets=true")
	if w.Code != http.StatusOK {
		t.Fatalf("operator export = %d", w.Code)
	}
	var exp settingsExport
	if err := json.Unmarshal(w.Body.Bytes(), &exp); err != nil {
		t.Fatalf("json: %v", err)
	}
	if len(exp.Servers) != 1 || exp.Servers[0].TokenSecret != "s3cr3t-token" ||
		exp.Servers[0].TokenSecretEnc != "" || exp.Servers[0].SecretFormat != "plaintext" {
		t.Fatalf("plaintext export wrong: %+v", exp.Servers)
	}

	// Default export (no flag) stays sealed.
	w2 := get("operator", "/api/v1/admin/settings/export")
	var exp2 settingsExport
	if err := json.Unmarshal(w2.Body.Bytes(), &exp2); err != nil {
		t.Fatalf("json2: %v", err)
	}
	if exp2.Servers[0].TokenSecret != "" || exp2.Servers[0].TokenSecretEnc == "" {
		t.Fatalf("default export should stay sealed: %+v", exp2.Servers)
	}
}

package api

import (
	"bytes"
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

func newTestServerRepo(t *testing.T) repository.ServerRepository {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	gormDB, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return repository.NewServerRepository(gormDB)
}

func seedServer(t *testing.T, repo repository.ServerRepository) *models.Server {
	t.Helper()
	srv := &models.Server{
		ID:               ids.NewULID(),
		Name:             "test",
		BaseURL:          "https://panel.local:8443",
		TokenID:          "OLDTOKENID",
		TokenSecretEnc:   []byte("old-enc"),
		Scopes:           models.JSONStringArray{"read:*"},
		Status:           models.ServerStatusActive,
		CredentialStatus: models.CredentialValid,
	}
	if err := repo.Create(context.Background(), srv); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return srv
}

func patchServer(t *testing.T, repo repository.ServerRepository, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(asRole("operator", "01OP"))
	// SecretKey nil + AllowPlaintext -> encryptSecret uses the hex fallback, so
	// the test needs no key material and still exercises the persistence path.
	RegisterServerRoutes(r.Group("/api/v1"), ServerHandlerConfig{Repo: repo, AllowPlaintext: true})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/servers/"+id, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestUpdateServerSavesTokenCredentials verifies PATCH persists a new token ID
// and secret, and flips credential_status back to unknown for revalidation.
func TestUpdateServerSavesTokenCredentials(t *testing.T) {
	repo := newTestServerRepo(t)
	srv := seedServer(t, repo)

	w := patchServer(t, repo, srv.ID, `{"token_id":"NEWTOKENID","token_secret":"s3cr3t-value"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	got, err := repo.FindByID(context.Background(), srv.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.TokenID != "NEWTOKENID" {
		t.Errorf("token_id not saved: got %q", got.TokenID)
	}
	if wantEnc := hex.EncodeToString([]byte("s3cr3t-value")); string(got.TokenSecretEnc) != wantEnc {
		t.Errorf("token secret not saved: got %q want %q", got.TokenSecretEnc, wantEnc)
	}
	if got.CredentialStatus != models.CredentialUnknown {
		t.Errorf("credential_status = %q, want unknown", got.CredentialStatus)
	}
}

// TestUpdateServerBlankSecretKeepsCurrent verifies an empty token_secret on
// edit does NOT wipe the stored secret (the "leave blank to keep" behavior).
func TestUpdateServerBlankSecretKeepsCurrent(t *testing.T) {
	repo := newTestServerRepo(t)
	srv := seedServer(t, repo)

	w := patchServer(t, repo, srv.ID, `{"name":"renamed","token_secret":""}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	got, err := repo.FindByID(context.Background(), srv.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Name != "renamed" {
		t.Errorf("name not saved: got %q", got.Name)
	}
	if string(got.TokenSecretEnc) != "old-enc" {
		t.Errorf("blank secret overwrote stored secret: got %q", got.TokenSecretEnc)
	}
	if got.TokenID != "OLDTOKENID" {
		t.Errorf("token_id changed unexpectedly: got %q", got.TokenID)
	}
}

func TestUpdateServerNormalizesTags(t *testing.T) {
	repo := newTestServerRepo(t)
	srv := seedServer(t, repo)

	w := patchServer(t, repo, srv.ID, `{"tags":[" Production ","EU-West","production","db.primary"]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}

	got, err := repo.FindByID(context.Background(), srv.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	want := models.JSONStringArray{"production", "eu-west", "db.primary"}
	if !reflect.DeepEqual(got.Tags, want) {
		t.Errorf("tags = %#v, want %#v", got.Tags, want)
	}
}

func TestUpdateServerRejectsInvalidTags(t *testing.T) {
	repo := newTestServerRepo(t)
	srv := seedServer(t, repo)

	w := patchServer(t, repo, srv.ID, `{"tags":["production team"]}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422, body = %s", w.Code, w.Body.String())
	}

	got, err := repo.FindByID(context.Background(), srv.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if len(got.Tags) != 0 {
		t.Errorf("invalid tags were saved: %#v", got.Tags)
	}
}

func doServerAction(t *testing.T, repo repository.ServerRepository, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(asRole("operator", "01OP"))
	RegisterServerRoutes(r.Group("/api/v1"), ServerHandlerConfig{Repo: repo, AllowPlaintext: true})
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestDeleteServerHardRemoves verifies DELETE actually removes the row.
func TestDeleteServerHardRemoves(t *testing.T) {
	repo := newTestServerRepo(t)
	srv := seedServer(t, repo)

	w := doServerAction(t, repo, http.MethodDelete, "/api/v1/admin/servers/"+srv.ID)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if _, err := repo.FindByID(context.Background(), srv.ID); err == nil {
		t.Fatalf("server still exists after delete")
	}
}

// TestDisableEnableServer verifies the status toggles and the row survives.
func TestDisableEnableServer(t *testing.T) {
	repo := newTestServerRepo(t)
	srv := seedServer(t, repo)

	w := doServerAction(t, repo, http.MethodPost, "/api/v1/admin/servers/"+srv.ID+"/disable")
	if w.Code != http.StatusOK {
		t.Fatalf("disable status = %d, body = %s", w.Code, w.Body.String())
	}
	got, err := repo.FindByID(context.Background(), srv.ID)
	if err != nil {
		t.Fatalf("find after disable: %v", err)
	}
	if got.Status != models.ServerStatusDisabled {
		t.Errorf("status = %q, want disabled", got.Status)
	}

	w = doServerAction(t, repo, http.MethodPost, "/api/v1/admin/servers/"+srv.ID+"/enable")
	if w.Code != http.StatusOK {
		t.Fatalf("enable status = %d, body = %s", w.Code, w.Body.String())
	}
	got, err = repo.FindByID(context.Background(), srv.ID)
	if err != nil {
		t.Fatalf("find after enable: %v", err)
	}
	if got.Status != models.ServerStatusActive {
		t.Errorf("status = %q, want active", got.Status)
	}
}

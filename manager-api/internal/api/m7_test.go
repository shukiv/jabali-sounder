package api

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

func m7apiDB(t *testing.T) (*gin.Engine, repository.APITokenRepository, repository.BackupRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "m7api.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	g, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	r := gin.New()
	r.Use(asRole("operator", "01OP"))
	tr := repository.NewAPITokenRepository(g)
	br := repository.NewBackupRepository(g)
	RegisterAPITokenRoutes(r.Group("/api/v1"), APITokenHandlerConfig{Repo: tr})
	RegisterBackupRoutes(r.Group("/api/v1"), BackupHandlerConfig{Repo: br})
	return r, tr, br
}

func TestAPITokenRotate(t *testing.T) {
	r, tr, _ := m7apiDB(t)
	ctx := context.Background()
	old, tok, err := tr.Mint(ctx, "ci", "01OP", nil, nil, nil)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	// Old token validates before rotation.
	if tr.Validate(ctx, old) == nil {
		t.Fatal("old token should validate before rotate")
	}
	w := do(r, http.MethodPost, "/api/v1/admin/api-tokens/"+tok.ID+"/rotate", "")
	if w.Code != http.StatusOK {
		t.Fatalf("rotate: %d %s", w.Code, w.Body.String())
	}
	var out struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.Token == "" || out.Token == old {
		t.Fatalf("rotate must return a new token, got %q", out.Token)
	}
	// Old invalid, new valid.
	if tr.Validate(ctx, old) != nil {
		t.Fatal("old token must be invalid after rotate")
	}
	if tr.Validate(ctx, out.Token) == nil {
		t.Fatal("new token must validate after rotate")
	}
	// Rotating an unknown id is a clean 404, not a 500.
	if w := do(r, http.MethodPost, "/api/v1/admin/api-tokens/nope/rotate", ""); w.Code != http.StatusNotFound {
		t.Fatalf("rotate unknown id: want 404, got %d", w.Code)
	}
}

func TestBackupsList(t *testing.T) {
	r, _, br := m7apiDB(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_ = br.Create(ctx, &models.BackupRun{
			ID: ids.NewULID(), ServerID: "S1", ServerName: "panel", OperationID: "op",
			Status: models.BackupSucceeded, StartedAt: time.Now().Add(-time.Duration(i) * time.Hour),
		})
	}
	_ = br.Create(ctx, &models.BackupRun{ID: ids.NewULID(), ServerID: "S2", ServerName: "other", Status: models.BackupFailed, StartedAt: time.Now()})

	w := do(r, http.MethodGet, "/api/v1/admin/backups", "")
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d", w.Code)
	}
	var out struct {
		Total int `json:"total"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.Total != 4 {
		t.Fatalf("want 4 backups, got %d", out.Total)
	}
	// Filter by server.
	w = do(r, http.MethodGet, "/api/v1/admin/backups?server_id=S1", "")
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.Total != 3 {
		t.Fatalf("want 3 for S1, got %d", out.Total)
	}
}

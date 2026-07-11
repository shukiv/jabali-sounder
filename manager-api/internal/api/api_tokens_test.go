package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/middleware"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

// TestAPITokenAuth covers the M4 read-only API token: it authenticates as
// viewer (reads allowed, writes 403), and stops working once revoked.
func TestAPITokenAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "tok.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	gormDB, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	repo := repository.NewAPITokenRepository(gormDB)
	plaintext, tok, err := repo.Mint(context.Background(), "ci", "01OWNER", nil)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	check := func(ctx context.Context, token string) (string, string, bool) {
		if tk := repo.Validate(ctx, token); tk != nil {
			return tk.ID, tk.Name, true
		}
		return "", "", false
	}
	r := gin.New()
	g := r.Group("")
	g.Use(middleware.AuthMiddleware("secret-not-empty-000000", nil, check))
	g.GET("/read", func(c *gin.Context) { c.String(http.StatusOK, middleware.AdminRole(c)) })
	g.POST("/write", middleware.RequireRole(models.RoleOperator), func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	get := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/read", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// Valid token -> read as viewer.
	if w := get(plaintext); w.Code != http.StatusOK || w.Body.String() != "viewer" {
		t.Fatalf("read: %d role=%q, want 200/viewer", w.Code, w.Body.String())
	}
	// Cannot write (viewer).
	{
		req := httptest.NewRequest(http.MethodPost, "/write", nil)
		req.Header.Set("Authorization", "Bearer "+plaintext)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Fatalf("write with token: %d, want 403", w.Code)
		}
	}
	// Garbage token -> 401.
	if w := get("snd_bogus_xyz"); w.Code != http.StatusUnauthorized {
		t.Fatalf("garbage token: %d, want 401", w.Code)
	}
	// Revoked -> 401.
	if err := repo.Revoke(context.Background(), tok.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if w := get(plaintext); w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked token: %d, want 401", w.Code)
	}
}

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/middleware"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

func newAuthTestEnv(t *testing.T) (*gin.Engine, repository.AdminRepository, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "auth.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	gormDB, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	repo := repository.NewAdminRepository(gormDB)

	admin, err := NewAdmin("admin", "oldpassword", models.RoleOwner)
	if err != nil {
		t.Fatalf("new admin: %v", err)
	}
	if err := repo.Create(context.Background(), admin); err != nil {
		t.Fatalf("create admin: %v", err)
	}

	secret := "test-jwt-secret"
	r := gin.New()
	RegisterAuthRoutes(r.Group("/api/v1"), AuthHandlerConfig{
		AdminRepo: repo,
		JWTSecret: secret,
		JWTTTL:    time.Hour,
	})
	token, _, err := middleware.MintToken(secret, admin.ID, admin.Username, models.RoleOwner, "01SESSION", time.Hour)
	if err != nil {
		t.Fatalf("mint token: %v", err)
	}
	return r, repo, token
}

func postChangePassword(r *gin.Engine, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestChangePasswordSuccess(t *testing.T) {
	r, repo, token := newAuthTestEnv(t)

	w := postChangePassword(r, token, `{"current_password":"oldpassword","new_password":"newpassword123"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	got, err := repo.FindByUsername(context.Background(), "admin")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte("newpassword123")) != nil {
		t.Errorf("new password not applied")
	}
}

func TestChangePasswordWrongCurrent(t *testing.T) {
	r, _, token := newAuthTestEnv(t)
	w := postChangePassword(r, token, `{"current_password":"WRONG","new_password":"newpassword123"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body = %s", w.Code, w.Body.String())
	}
}

func TestChangePasswordRequiresAuth(t *testing.T) {
	r, _, _ := newAuthTestEnv(t)
	w := postChangePassword(r, "", `{"current_password":"oldpassword","new_password":"newpassword123"}`)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestChangePasswordTooShort(t *testing.T) {
	r, _, token := newAuthTestEnv(t)
	w := postChangePassword(r, token, `{"current_password":"oldpassword","new_password":"short"}`)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422, body = %s", w.Code, w.Body.String())
	}
}

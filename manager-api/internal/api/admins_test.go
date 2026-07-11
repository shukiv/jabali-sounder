package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

func newTestAdminRepo(t *testing.T) repository.AdminRepository {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "admins.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	gormDB, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return repository.NewAdminRepository(gormDB)
}

// asRole injects the authenticated identity a real AuthMiddleware would set.
func asRole(role, id string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("admin_role", role)
		c.Set("admin_id", id)
		c.Set("admin_username", "actor")
		c.Next()
	}
}

func adminRouter(t *testing.T, repo repository.AdminRepository, role, id string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(asRole(role, id))
	RegisterAdminRoutes(r.Group("/api/v1"), AdminHandlerConfig{AdminRepo: repo})
	return r
}

func do(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestAdminCRUDOwnerOnly: only owners reach the admin-management routes.
func TestAdminManagementRequiresOwner(t *testing.T) {
	repo := newTestAdminRepo(t)
	for _, role := range []string{"viewer", "operator"} {
		r := adminRouter(t, repo, role, "01ACTOR")
		if w := do(r, http.MethodGet, "/api/v1/admin/admins", ""); w.Code != http.StatusForbidden {
			t.Fatalf("%s GET admins = %d, want 403", role, w.Code)
		}
	}
	r := adminRouter(t, repo, "owner", "01ACTOR")
	if w := do(r, http.MethodGet, "/api/v1/admin/admins", ""); w.Code != http.StatusOK {
		t.Fatalf("owner GET admins = %d, want 200", w.Code)
	}
}

// TestCreateAndDeleteAdmin + last-owner / self guards.
func TestAdminLifecycleGuards(t *testing.T) {
	repo := newTestAdminRepo(t)
	owner, _ := NewAdmin("owner1", "ownerpass123", models.RoleOwner)
	if err := repo.Create(context.Background(), owner); err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	r := adminRouter(t, repo, "owner", owner.ID)

	// Create a viewer.
	w := do(r, http.MethodPost, "/api/v1/admin/admins", `{"username":"viewer1","password":"viewerpass1","role":"viewer"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create viewer = %d (%s)", w.Code, w.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &created)

	// Cannot delete the last owner.
	if w := do(r, http.MethodDelete, "/api/v1/admin/admins/"+owner.ID, ""); w.Code == http.StatusOK {
		t.Fatalf("deleting last owner should fail, got 200")
	}

	// Cannot delete yourself.
	// (owner is the acting admin id) — covered by the last-owner guard above too;
	// verify explicit self guard with a second owner present.
	owner2, _ := NewAdmin("owner2", "ownerpass456", models.RoleOwner)
	_ = repo.Create(context.Background(), owner2)
	if w := do(r, http.MethodDelete, "/api/v1/admin/admins/"+owner.ID, ""); w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("self-delete should be 422, got %d", w.Code)
	}

	// Can delete the viewer.
	if w := do(r, http.MethodDelete, "/api/v1/admin/admins/"+created.ID, ""); w.Code != http.StatusOK {
		t.Fatalf("delete viewer = %d", w.Code)
	}
}

// TestServerMutationRequiresOperator: a viewer cannot mutate servers.
func TestServerMutationRequiresOperator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newTestServerRepo(t)
	r := gin.New()
	r.Use(asRole("viewer", "01V"))
	RegisterServerRoutes(r.Group("/api/v1"), ServerHandlerConfig{Repo: repo, AllowPlaintext: true})

	if w := do(r, http.MethodPost, "/api/v1/admin/servers/whatever/disable", ""); w.Code != http.StatusForbidden {
		t.Fatalf("viewer disable = %d, want 403", w.Code)
	}
	// A viewer can still read.
	if w := do(r, http.MethodGet, "/api/v1/admin/servers", ""); w.Code != http.StatusOK {
		t.Fatalf("viewer list = %d, want 200", w.Code)
	}
}

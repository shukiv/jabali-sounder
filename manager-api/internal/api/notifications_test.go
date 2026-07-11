package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

// TestNotificationEndpoints covers list/unread_count, mark-read, and read-all
// for the in-app notification API (SND-18).
func TestNotificationEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "notif.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	gormDB, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	repo := repository.NewNotificationRepository(gormDB)
	ctx := t.Context()
	for i := 0; i < 2; i++ {
		if err := repo.Create(ctx, &models.Notification{
			ID: ids.NewULID(), Kind: "cpu_high", ServerID: "S", ServerName: "srv",
			Metric: "cpu", Value: 95, Threshold: 80, Message: "high",
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	r := gin.New()
	g := r.Group("/api/v1")
	RegisterNotificationRoutes(g, NotificationHandlerConfig{Repo: repo})

	do := func(method, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}

	// List: 2 rows, both unread.
	w := do(http.MethodGet, "/api/v1/admin/notifications")
	if w.Code != http.StatusOK {
		t.Fatalf("list status %d", w.Code)
	}
	var listed struct {
		Data        []map[string]any `json:"data"`
		UnreadCount int              `json:"unread_count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(listed.Data) != 2 || listed.UnreadCount != 2 {
		t.Fatalf("want 2 rows/2 unread, got %d/%d", len(listed.Data), listed.UnreadCount)
	}

	// Mark one read.
	id := listed.Data[0]["id"].(string)
	if w := do(http.MethodPost, "/api/v1/admin/notifications/"+id+"/read"); w.Code != http.StatusOK {
		t.Fatalf("mark read status %d", w.Code)
	}
	if n, _ := repo.UnreadCount(ctx); n != 1 {
		t.Fatalf("want 1 unread after mark, got %d", n)
	}

	// Mark all read.
	if w := do(http.MethodPost, "/api/v1/admin/notifications/read-all"); w.Code != http.StatusOK {
		t.Fatalf("read-all status %d", w.Code)
	}
	if n, _ := repo.UnreadCount(ctx); n != 0 {
		t.Fatalf("want 0 unread after read-all, got %d", n)
	}
}

package adminreset

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/api"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "a.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	g, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return g
}

func seedAdmin(t *testing.T, g *gorm.DB, username, password string) *models.Admin {
	t.Helper()
	admin, err := api.NewAdmin(username, password, models.RoleOwner)
	if err != nil {
		t.Fatalf("new admin: %v", err)
	}
	if err := repository.NewAdminRepository(g).Create(context.Background(), admin); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	return admin
}

func TestResetAdminPassword(t *testing.T) {
	g := testDB(t)
	ctx := context.Background()
	admin := seedAdmin(t, g, "admin", "oldpassword1")

	// An active session that must be revoked by the reset.
	sessions := repository.NewSessionRepository(g)
	sess := &models.Session{ID: "sess_1", AdminID: admin.ID, CreatedAt: time.Now(), LastSeenAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)}
	if err := sessions.Create(ctx, sess); err != nil {
		t.Fatalf("create session: %v", err)
	}
	if ok, _ := sessions.Active(ctx, "sess_1"); !ok {
		t.Fatal("session should start active")
	}

	if err := ResetAdminPassword(ctx, g, "admin", "newpassword2", "device"); err != nil {
		t.Fatalf("reset: %v", err)
	}

	// Password now verifies against the new value, not the old.
	got, err := repository.NewAdminRepository(g).FindByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte("newpassword2")) != nil {
		t.Fatal("new password does not verify")
	}
	if bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte("oldpassword1")) == nil {
		t.Fatal("old password still verifies")
	}

	// Existing session was revoked.
	if ok, _ := sessions.Active(ctx, "sess_1"); ok {
		t.Fatal("session should be revoked after reset")
	}

	// Audit entry recorded.
	var n int64
	g.Model(&models.AuditLog{}).Where("event = ? AND actor = ? AND actor_id = ?", "auth.password_reset_local", "device", admin.ID).Count(&n)
	if n != 1 {
		t.Fatalf("expected 1 audit row, got %d", n)
	}
}

func TestResetAdminPasswordRejectsShort(t *testing.T) {
	g := testDB(t)
	ctx := context.Background()
	seedAdmin(t, g, "admin", "oldpassword1")

	if err := ResetAdminPassword(ctx, g, "admin", "short", "device"); !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("want ErrPasswordTooShort, got %v", err)
	}
	// Password unchanged.
	got, _ := repository.NewAdminRepository(g).FindByUsername(ctx, "admin")
	if bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte("oldpassword1")) != nil {
		t.Fatal("password must be unchanged after a rejected reset")
	}
}

func TestResetAdminPasswordMissingAdmin(t *testing.T) {
	g := testDB(t)
	if err := ResetAdminPassword(context.Background(), g, "ghost", "newpassword2", "device"); err == nil {
		t.Fatal("expected error for missing admin")
	}
}

// Package adminreset performs a local, offline admin-password reset for
// lockout recovery. It is used only by processes with direct local access to
// the database — the desktop CLI (shell access) and the mobile device-owner
// reset (the on-device Wails bridge). It is deliberately NOT wired to any gin
// route: the reset must never be reachable over the network, especially on the
// server deployment.
package adminreset

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/api"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/ids"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

// MinPasswordLength mirrors the API's change-password rule so a local reset
// cannot set a weaker password than the app itself allows.
const MinPasswordLength = 8

// ErrPasswordTooShort is returned when the new password is under the minimum.
var ErrPasswordTooShort = fmt.Errorf("password must be at least %d characters", MinPasswordLength)

// ResetAdminPassword sets a new password for the named admin directly in the
// local database, revokes that admin's existing sessions (so a leaked or old
// token cannot outlive the reset), and records a best-effort audit entry.
//
// It requires the admin to already exist — callers that also create a missing
// admin (the desktop CLI) handle that separately. It does NOT clear 2FA: a user
// who lost both their password and their authenticator stays locked out, which
// is tracked as a separate follow-up.
//
// actor labels who performed the reset in the audit log (e.g. "device" for the
// mobile on-device reset, "cli" for the desktop command).
func ResetAdminPassword(ctx context.Context, gormDB *gorm.DB, username, newPassword, actor string) error {
	if len(newPassword) < MinPasswordLength {
		return ErrPasswordTooShort
	}

	admins := repository.NewAdminRepository(gormDB)
	admin, err := admins.FindByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("admin %q not found", username)
		}
		return fmt.Errorf("lookup admin %q: %w", username, err)
	}

	hash, err := api.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	admin.PasswordHash = hash
	if err := admins.Update(ctx, admin); err != nil {
		return fmt.Errorf("update admin: %w", err)
	}

	// Invalidate any session that predates the reset. Best-effort: a failure
	// here must not block recovery, but on success it closes the stolen-device
	// window where an old token would otherwise remain valid for the JWT TTL.
	sessions := repository.NewSessionRepository(gormDB)
	if _, rerr := sessions.RevokeAllForAdmin(ctx, admin.ID); rerr != nil {
		// Non-fatal; the password is already changed. Surface via audit only.
		actor += " (session-revoke failed)"
	}

	// Best-effort audit; recovery must not fail because the audit write did.
	audit := repository.NewAuditRepository(gormDB)
	_ = audit.Create(ctx, &models.AuditLog{
		ID:        ids.NewULID(),
		Event:     "auth.password_reset_local",
		Actor:     actor,
		ActorID:   admin.ID,
		CreatedAt: time.Now().UTC(),
	})
	return nil
}

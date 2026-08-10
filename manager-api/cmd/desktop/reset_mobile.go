//go:build android || ios

package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/adminreset"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
)

// ResetLostPassword lets the device owner set a new admin password when locked
// out of the mobile app. It is the mobile counterpart to the desktop CLI reset:
// both rely on local access as the trust anchor (the OS lock screen on mobile,
// a shell on desktop). This method exists ONLY in the android/ios builds and is
// reachable solely through the on-device Wails runtime binding — mobile has no
// HTTP listener, so it can never be called over the network, and it is absent
// from the desktop and server binaries entirely.
func (b *Bridge) ResetLostPassword(username, newPassword string, resetTwoFactor bool) error {
	u := strings.TrimSpace(username)
	if u == "" {
		u = "admin"
	}

	dataDir, err := appDataDir()
	if err != nil {
		return err
	}
	gormDB, err := db.Open("sqlite", filepath.Join(dataDir, "sounder.db"))
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	return adminreset.ResetAdminPassword(context.Background(), gormDB, u, newPassword, "device", resetTwoFactor)
}

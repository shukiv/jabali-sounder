//go:build android || ios

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// setupTray is a no-op on mobile: there is no system tray, and the window must
// never be hidden to a non-existent tray (SND-85 is desktop-only).
func setupTray(_ *Bridge, _ *application.WebviewWindow) {}

//go:build desktop

package main

import (
	_ "embed"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed tray_icon.png
var trayIcon []byte

// desktopPrefs persists desktop-only UI preferences next to the database.
type desktopPrefs struct {
	MinimizeToTray bool `json:"minimize_to_tray"`
}

func prefsPath() (string, error) {
	dir, err := appDataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "desktop-prefs.json"), nil
}

func loadPrefs() desktopPrefs {
	// Minimize-to-tray defaults on so closing the window keeps monitoring alive.
	p := desktopPrefs{MinimizeToTray: true}
	path, err := prefsPath()
	if err != nil {
		return p
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return p
	}
	_ = json.Unmarshal(data, &p)
	return p
}

func savePrefs(p desktopPrefs) {
	path, err := prefsPath()
	if err != nil {
		return
	}
	if data, err := json.Marshal(p); err == nil {
		_ = os.WriteFile(path, data, 0o600)
	}
}

// setupTray installs the system-tray icon, menu, and close-to-tray lifecycle for
// the desktop build (SND-85). The embedded API + background poller keep running
// while the window is hidden; Quit performs an orderly shutdown. There is a
// single window and a single backend — tray activation restores the existing
// window rather than spawning a second instance.
func setupTray(b *Bridge, window *application.WebviewWindow) {
	app := b.app

	var mu sync.Mutex
	minimizeToTray := loadPrefs().MinimizeToTray
	notifiedHide := false

	show := func() {
		window.Show()
		window.UnMinimise()
		window.Focus()
	}

	tray := app.SystemTray.New()
	tray.SetIcon(trayIcon)
	tray.SetTooltip("Jabali Sounder")

	menu := app.NewMenu()
	menu.Add("Open Jabali Sounder").OnClick(func(*application.Context) { show() })
	menu.AddSeparator()
	status := menu.Add("Monitoring: running in the background")
	status.SetEnabled(false)
	menu.AddSeparator()
	minItem := menu.AddCheckbox("Minimize to tray on close", minimizeToTray)
	minItem.OnClick(func(*application.Context) {
		mu.Lock()
		minimizeToTray = !minimizeToTray
		minItem.SetChecked(minimizeToTray)
		savePrefs(desktopPrefs{MinimizeToTray: minimizeToTray})
		mu.Unlock()
	})
	menu.AddSeparator()
	menu.Add("Quit Jabali Sounder").OnClick(func(*application.Context) { app.Quit() })
	tray.SetMenu(menu)

	// Restore the single existing window on tray activation.
	tray.OnClick(func() { show() })
	tray.OnDoubleClick(func() { show() })

	// Closing the window hides it to the tray (keeping the backend + poller
	// alive) when enabled; otherwise it falls through to a normal quit.
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		mu.Lock()
		hide := minimizeToTray
		firstHide := hide && !notifiedHide
		if firstHide {
			notifiedHide = true
		}
		mu.Unlock()

		if !hide {
			return // normal close -> app quits
		}
		window.Hide()
		e.Cancel()
		if firstHide {
			// First hide: make it discoverable that Sounder is still running.
			tray.SetTooltip("Jabali Sounder — still running. Click to reopen.")
			slog.Info("window hidden to tray; Sounder keeps running in the background")
		}
	})

	app.OnShutdown(func() {
		slog.Info("desktop shutting down: stopping tray, poller, and API")
	})
}

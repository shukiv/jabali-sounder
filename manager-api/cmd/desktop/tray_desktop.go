//go:build desktop

package main

import (
	_ "embed"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// trayHostAvailable reports whether the session bus has a StatusNotifier watcher
// AND a host actually registered with it. On GNOME without an AppIndicator/
// StatusNotifier extension the watcher may be absent or present-without-a-host,
// which means a published tray item is invisible. We must never hide the only
// window in that case (SND-92). Checked live on every close since the host can
// appear or disappear after startup.
func trayHostAvailable() bool {
	conn, err := dbus.SessionBus()
	if err != nil {
		return false
	}
	var has bool
	if err := conn.BusObject().Call(
		"org.freedesktop.DBus.NameHasOwner", 0, "org.kde.StatusNotifierWatcher",
	).Store(&has); err != nil || !has {
		return false
	}
	watcher := conn.Object("org.kde.StatusNotifierWatcher", dbus.ObjectPath("/StatusNotifierWatcher"))
	v, err := watcher.GetProperty("org.kde.StatusNotifierWatcher.IsStatusNotifierHostRegistered")
	if err != nil {
		return false
	}
	registered, ok := v.Value().(bool)
	return ok && registered
}

//go:embed tray_icon.png
var trayIcon []byte

// Windows notification-area icons need an opaque light background to stay legible
// against light/dark taskbar themes (SND-93); Linux/macOS keep the standard asset.
//
//go:embed tray_icon_windows.png
var trayIconWindows []byte

func trayIconBytes() []byte {
	if runtime.GOOS == "windows" {
		return trayIconWindows
	}
	return trayIcon
}

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

	hostAtStart := trayHostAvailable()
	if hostAtStart {
		slog.Info("system tray: usable StatusNotifier host registered")
	} else {
		slog.Warn("system tray: no usable host; close-to-tray will quit instead of hiding the window")
	}

	tray := app.SystemTray.New()
	tray.SetIcon(trayIconBytes())
	tray.SetTooltip("Jabali Sounder")

	menu := app.NewMenu()
	menu.Add("Open Jabali Sounder").OnClick(func(*application.Context) { show() })
	menu.AddSeparator()
	status := menu.Add("Monitoring: running in the background")
	status.SetEnabled(false)
	menu.AddSeparator()
	minItem := menu.AddCheckbox("Minimize to tray on close", minimizeToTray && hostAtStart)
	if !hostAtStart {
		minItem.SetEnabled(false)
		minItem.SetLabel("Minimize to tray (unavailable: no tray host)")
	}
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
		mu.Unlock()

		if !hide {
			slog.Info("desktop close: minimize-to-tray disabled, quitting")
			return // normal close -> app quits
		}
		// SND-92: never hide the only window unless a usable tray host is
		// registered right now — otherwise the app becomes an unreachable
		// background process. Fall through to a normal, visible quit.
		if !trayHostAvailable() {
			slog.Warn("desktop close: no usable system-tray host; quitting instead of hiding")
			return
		}
		window.Hide()
		e.Cancel()
		mu.Lock()
		firstHide := !notifiedHide
		if firstHide {
			notifiedHide = true
		}
		mu.Unlock()
		if firstHide {
			slog.Info("window hidden to tray; Sounder keeps running in the background")
		}
	})

	app.OnShutdown(func() {
		slog.Info("desktop shutting down: stopping tray, poller, and API")
	})
}

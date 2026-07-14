//go:build desktop || android || ios

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wailsapp/wails/v3/pkg/application"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/api"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/app"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/poller"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/secrets"
)

//go:embed all:dist
var assets embed.FS

// webviewGpuPolicy selects the Linux WebKitGTK hardware-acceleration policy.
// Default Never (software) because accelerated compositing breaks mouse-wheel
// scrolling on NVIDIA; override with JABALI_SOUNDER_WEBVIEW_GPU for tuning.
func webviewGpuPolicy() application.WebviewGpuPolicy {
	switch os.Getenv("JABALI_SOUNDER_WEBVIEW_GPU") {
	case "never":
		return application.WebviewGpuPolicyNever
	case "ondemand":
		return application.WebviewGpuPolicyOnDemand
	case "always":
		return application.WebviewGpuPolicyAlways
	}
	// Auto: full acceleration (smooth, working wheel) when a GPU is present,
	// otherwise software rendering so the webview still works on headless/VM/
	// GPU-less hosts. Override with JABALI_SOUNDER_WEBVIEW_GPU.
	if hasGPURenderNode() {
		return application.WebviewGpuPolicyAlways
	}
	return application.WebviewGpuPolicyNever
}

// hasGPURenderNode reports whether the kernel exposes a GPU device (DRM render
// node or an NVIDIA proprietary device), a proxy for accelerated rendering.
func hasGPURenderNode() bool {
	for _, pattern := range []string{"/dev/dri/renderD*", "/dev/dri/card*", "/dev/nvidia[0-9]*"} {
		if m, _ := filepath.Glob(pattern); len(m) > 0 {
			return true
		}
	}
	return false
}

func main() {
	// Lockout recovery: `jabali-sounder-desktop reset-password [username]`
	// sets the admin password against the local SQLite DB, then exits.
	if len(os.Args) > 1 && os.Args[1] == "reset-password" {
		if err := resetPassword(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	// Build the backend lazily on the first asset request rather than up front.
	// On mobile the app's storage path (and thus the DB) is only available once
	// the runtime/activity is ready; deferring construction also lets
	// application.New register the asset handler immediately, so the WebView's
	// initial load of https://wails.localhost/ is served instead of refused.
	handler := newLazyHandler(newDesktopHandler)

	bridge := &Bridge{handler: handler}

	// Wails v3: the Go backend is created with application.New. The same main.go
	// also targets iOS/Android. The combined gin+SPA handler is passed straight
	// in as the asset handler, so /api/v1 and the embedded SPA are served
	// in-process on every platform (no localhost server, no open ports on mobile).
	opts := application.Options{
		Name:        "Jabali Sounder",
		Description: "Central control plane for a sounder of Jabali Panel servers.",
		Services: []application.Service{
			application.NewService(bridge),
		},
		Assets: application.AssetOptions{
			Handler: handler,
		},
	}
	// iOS-only option tweaks (no-op elsewhere); see app_options_*.go.
	modifyOptionsForIOS(&opts)

	wailsApp := application.New(opts)
	bridge.app = wailsApp

	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Jabali Sounder",
		Width:  1280,
		Height: 820,
		// Start maximised (fills the screen); the user can restore/resize.
		// Width/Height above are the restored (un-maximised) size.
		StartState:       application.WindowStateMaximised,
		BackgroundColour: application.NewRGBA(20, 20, 20, 255),
		URL:              "/",
		// Disable the native webview right-click menu (Reload/Inspect/Back) so the
		// desktop app feels native. This is a desktop-only Wails option — the web
		// build served by cmd/server is plain Chrome and keeps its normal menu.
		DefaultContextMenuDisabled: true,
		// Webview hardware-acceleration policy (Linux/WebKitGTK): accelerated
		// when a GPU is present, software otherwise; see webviewGpuPolicy().
		Linux: application.LinuxWindow{
			WebviewGpuPolicy: webviewGpuPolicy(),
		},
	})

	// System-tray icon + close-to-tray lifecycle (desktop only; no-op on mobile).
	setupTray(bridge, window)

	if err := wailsApp.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type desktopHandler struct {
	api    http.Handler
	assets http.Handler
}

func newDesktopHandler() (http.Handler, error) {
	dataDir, err := appDataDir()
	if err != nil {
		return nil, err
	}

	keyPath := filepath.Join(dataDir, "secrets.key")
	if err := ensureRandomFile(keyPath, 32); err != nil {
		return nil, err
	}
	jwtSecret, err := loadOrCreateHexSecret(filepath.Join(dataDir, "jwt.secret"), 32)
	if err != nil {
		return nil, err
	}

	key, err := secrets.LoadKey(keyPath)
	if err != nil {
		return nil, fmt.Errorf("load secret key: %w", err)
	}

	dbPath := filepath.Join(dataDir, "sounder.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		return nil, fmt.Errorf("migrate sqlite: %w", err)
	}
	gormDB, err := db.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	gin.SetMode(gin.ReleaseMode)
	serverRepo := repository.NewServerRepository(gormDB)
	heartbeatRepo := repository.NewHeartbeatRepository(gormDB)
	metricRepo := repository.NewMetricSampleRepository(gormDB)
	sessionRepo := repository.NewSessionRepository(gormDB)
	apiTokenRepo := repository.NewAPITokenRepository(gormDB)
	notifRepo := repository.NewNotificationRepository(gormDB)
	alertRuleRepo := repository.NewAlertRuleRepository(gormDB)
	alertChannelRepo := repository.NewAlertChannelRepository(gormDB)
	maintenanceRepo := repository.NewMaintenanceRepository(gormDB)
	mutedRepo := repository.NewMutedAlertRepository(gormDB)
	auditRepo := repository.NewAuditRepository(gormDB)
	backupRepo := repository.NewBackupRepository(gormDB)
	if err := alertRuleRepo.EnsureDefaults(context.Background(), time.Now().UTC()); err != nil {
		slog.Warn("seed default alert rules failed", "error", err)
	}
	apiEngine := app.NewWithDeps(app.Deps{
		Log:              slog.Default(),
		ServerRepo:       serverRepo,
		HeartbeatRepo:    heartbeatRepo,
		MetricSampleRepo: metricRepo,
		SessionRepo:      sessionRepo,
		APITokenRepo:     apiTokenRepo,
		NotificationRepo: notifRepo,
		AlertRuleRepo:    alertRuleRepo,
		AlertChannelRepo: alertChannelRepo,
		MaintenanceRepo:  maintenanceRepo,
		MutedRepo:        mutedRepo,
		AuditRepo:        auditRepo,
		BackupRepo:       backupRepo,
		AdminRepo:        repository.NewAdminRepository(gormDB),
		SecretKey:        key,
		JWTSecret:        jwtSecret,
		// Desktop is a local admin tool that always has an encryption key and
		// commonly manages panels on the operator's own LAN, so allow private
		// enrollment targets (SND-4) but never the plaintext fallback (SND-6).
		AllowPrivateTargets: true,
	})

	// Background health poller (roadmap M1): keep fleet status current + record
	// heartbeats. Runs for the app's lifetime.
	go poller.New(poller.Config{
		Servers:       serverRepo,
		Heartbeats:    heartbeatRepo,
		MetricSamples: metricRepo,
		Sessions:      sessionRepo,
		Notifications: notifRepo,
		Backups:       backupRepo,
		APITokens:     apiTokenRepo,
		Audit:         auditRepo,
		AlertRules:    alertRuleRepo,
		Channels:      alertChannelRepo,
		Maintenance:   maintenanceRepo,
		Muted:         mutedRepo,
		SecretKey:     key,
		Log:           slog.Default(),
	}).Run(context.Background())

	distFS, err := fs.Sub(assets, "dist")
	if err != nil {
		return nil, fmt.Errorf("desktop assets: %w", err)
	}

	return &desktopHandler{
		api:    apiEngine,
		assets: spaFileServer{fsys: distFS},
	}, nil
}

func (h *desktopHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/health" || strings.HasPrefix(r.URL.Path, "/api/v1/") {
		h.api.ServeHTTP(w, r)
		return
	}
	h.assets.ServeHTTP(w, r)
}

// lazyHandler builds the real backend once, on the first request. If
// construction fails it serves the error (visible in the webview) instead of
// leaving the WebView with a refused connection.
type lazyHandler struct {
	build func() (http.Handler, error)
	once  sync.Once
	h     http.Handler
	err   error
}

func newLazyHandler(build func() (http.Handler, error)) *lazyHandler {
	return &lazyHandler{build: build}
}

func (l *lazyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	l.once.Do(func() { l.h, l.err = l.build() })
	if l.err != nil {
		http.Error(w, "startup failed: "+l.err.Error(), http.StatusInternalServerError)
		return
	}
	l.h.ServeHTTP(w, r)
}

type spaFileServer struct {
	fsys fs.FS
}

func (s spaFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name == "" {
		name = "index.html"
	}
	data, err := fs.ReadFile(s.fsys, name)
	if err != nil {
		// SPA client-side routing: unknown paths fall back to index.html.
		name = "index.html"
		data, err = fs.ReadFile(s.fsys, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	// Serve the bytes directly. http.FileServer would 301-redirect
	// /index.html -> /, which the Android WebViewAssetLoader does not follow
	// (it reports the asset as not found), leaving a blank screen.
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
}

func ensureRandomFile(path string, size int) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return fmt.Errorf("random %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func loadOrCreateHexSecret(path string, size int) (string, error) {
	if data, err := os.ReadFile(path); err == nil {
		return strings.TrimSpace(string(data)), nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", fmt.Errorf("random %s: %w", path, err)
	}
	secret := hex.EncodeToString(data)
	if err := os.WriteFile(path, []byte(secret), 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return secret, nil
}

// appDataDir returns (creating if needed) the per-platform data directory where
// the app stores its SQLite DB, secret key, and JWT secret. The base directory
// is platform-specific — desktop uses os.UserConfigDir, mobile uses the app
// sandbox — see platformDataDir in the datadir_*.go files.
func appDataDir() (string, error) {
	dir, err := platformDataDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create data dir: %w", err)
	}
	return dir, nil
}

// resetPassword sets (or creates) the admin password in the local SQLite DB.
// Reads the new password from $JABALI_RESET_PASSWORD or stdin. Used for
// lockout recovery without wiping the database.
func resetPassword(args []string) error {
	username := "admin"
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		username = strings.TrimSpace(args[0])
	}

	dataDir, err := appDataDir()
	if err != nil {
		return err
	}
	dbPath := filepath.Join(dataDir, "sounder.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		return fmt.Errorf("migrate sqlite: %w", err)
	}
	gormDB, err := db.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	repo := repository.NewAdminRepository(gormDB)

	pw := os.Getenv("JABALI_RESET_PASSWORD")
	if pw == "" {
		fmt.Fprintf(os.Stderr, "New password for %q: ", username)
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		pw = strings.TrimSpace(line)
	}
	if len(pw) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	ctx := context.Background()
	existing, err := repo.FindByUsername(ctx, username)
	switch {
	case err == nil:
		hash, herr := api.HashPassword(pw)
		if herr != nil {
			return fmt.Errorf("hash password: %w", herr)
		}
		existing.PasswordHash = hash
		if uerr := repo.Update(ctx, existing); uerr != nil {
			return fmt.Errorf("update admin: %w", uerr)
		}
		fmt.Fprintf(os.Stderr, "Password updated for admin %q\n", username)
		return nil
	case errors.Is(err, repository.ErrNotFound):
		admin, aerr := api.NewAdmin(username, pw, models.RoleOwner)
		if aerr != nil {
			return fmt.Errorf("create admin: %w", aerr)
		}
		if cerr := repo.Create(ctx, admin); cerr != nil {
			return fmt.Errorf("create admin: %w", cerr)
		}
		fmt.Fprintf(os.Stderr, "Admin %q created\n", username)
		return nil
	default:
		return fmt.Errorf("lookup admin: %w", err)
	}
}

// Bridge exposes native desktop capabilities to the SPA over the Wails runtime.
// The browser <a download> trick does not trigger a save in the WebKit webview,
// so file export goes through a native Save As dialog here instead. In Wails v3
// the SPA calls these via @wailsio/runtime Call.ByName("main.Bridge.<Method>").
type Bridge struct {
	app     *application.App
	handler http.Handler // the gin+SPA handler, for mobile ApiCall
}

// OpenExternal opens a URL in the user's default system browser. The webview
// does not open target="_blank" links itself.
func (b *Bridge) OpenExternal(url string) {
	_ = b.app.Browser.OpenURL(url)
}

// SaveFile opens a native "Save As" dialog seeded with defaultName and writes
// content to the chosen path. Returns the saved path, or "" if the user
// cancelled.
func (b *Bridge) SaveFile(defaultName, content string) (string, error) {
	path, err := b.app.Dialog.SaveFile().
		SetFilename(defaultName).
		CanCreateDirectories(true).
		PromptForSingleSelection()
	if err != nil {
		return "", fmt.Errorf("save dialog: %w", err)
	}
	if path == "" {
		return "", nil // cancelled
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}

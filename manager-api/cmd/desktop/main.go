//go:build desktop

package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/api"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/app"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/secrets"
)

//go:embed all:dist
var assets embed.FS

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

	handler, err := newDesktopHandler()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := wails.Run(&options.App{
		Title:  "Jabali Sounder",
		Width:  1280,
		Height: 820,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: handler,
		},
		BackgroundColour: &options.RGBA{R: 20, G: 20, B: 20, A: 255},
	}); err != nil {
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
	apiEngine := app.NewWithDeps(app.Deps{
		Log:           slog.Default(),
		ServerRepo:    repository.NewServerRepository(gormDB),
		HeartbeatRepo: repository.NewHeartbeatRepository(gormDB),
		AdminRepo:     repository.NewAdminRepository(gormDB),
		SecretKey:     key,
		JWTSecret:     jwtSecret,
	})

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

type spaFileServer struct {
	fsys fs.FS
}

func (s spaFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if _, err := fs.Stat(s.fsys, path); err != nil {
		path = "index.html"
	}
	r.URL.Path = "/" + path
	http.FileServer(http.FS(s.fsys)).ServeHTTP(w, r)
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

// appDataDir returns (creating if needed) the per-user data directory where the
// desktop app stores its SQLite DB, secret key, and JWT secret.
func appDataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	dir := filepath.Join(base, "Jabali Sounder")
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
		admin, aerr := api.NewAdmin(username, pw)
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

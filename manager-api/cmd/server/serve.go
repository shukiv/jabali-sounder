package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/app"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/remote"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/secrets"
)

const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 90 * time.Second
	shutdownTimeout   = 10 * time.Second
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the Jabali Sounder HTTP server",
		RunE:  runServe,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			_, err := initConfig()
			return err
		},
	}
}

func runServe(cmd *cobra.Command, args []string) error {
	cfg := sharedCfg
	log := sharedLog

	log.Info("starting jabali-sounder",
		"addr", cfg.Server.Addr,
		"env", cfg.Server.Env,
	)

	// ---- Secret key ----
	key, err := secrets.LoadKey(cfg.Secrets.KeyFile)
	if err != nil {
		log.Warn("secret key not loaded — token encryption disabled", "error", err)
	} else {
		log.Info("secret key loaded", "path", cfg.Secrets.KeyFile)
	}

	// ---- DB ----
	var serverRepo repository.ServerRepository
	var adminRepo repository.AdminRepository
	if cfg.Database.URL != "" {
		if err := db.Migrate(cfg.Database.Driver, cfg.Database.URL); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		log.Info("migrations up-to-date")
		gormDB, err := db.Open(cfg.Database.Driver, cfg.Database.URL)
		if err != nil {
			return fmt.Errorf("open db: %w", err)
		}
		log.Info("db connected", "driver", cfg.Database.Driver)
		serverRepo = repository.NewServerRepository(gormDB)
		adminRepo = repository.NewAdminRepository(gormDB)
	} else {
		log.Warn("database.url not set; running without DB — enrollment disabled")
	}

	// ---- JWT secret ----
	jwtSecret := cfg.JWT.Secret
	if jwtSecret == "" {
		jwtSecret = envFirst("JABALI_SOUNDER_JWT_SECRET", "JABALI_MANAGER_JWT_SECRET")
	}
	if jwtSecret != "" {
		log.Info("JWT auth enabled")
	} else if cfg.Server.Env == "development" {
		// Dev convenience only: generate a random ephemeral secret so admin
		// routes are still gated, but with an unpredictable key. Sessions do
		// not survive a restart. Never use a hardcoded fallback — it would be
		// a public, forgeable signing key.
		ephemeral := make([]byte, 32)
		if _, err := rand.Read(ephemeral); err != nil {
			return fmt.Errorf("generate ephemeral jwt secret: %w", err)
		}
		jwtSecret = hex.EncodeToString(ephemeral)
		log.Warn("JWT secret not set — generated ephemeral dev secret; sessions reset on restart")
	} else {
		return fmt.Errorf("JWT secret required in %q env: set JABALI_SOUNDER_JWT_SECRET or [jwt].secret", cfg.Server.Env)
	}

	remote.SetInsecureSkipVerify(cfg.Remote.InsecureSkipVerify)
	if cfg.Remote.InsecureSkipVerify {
		log.Warn("TLS verification disabled for outbound panel calls — data plane exposed to MITM; use only for self-signed panels")
	}

	deps := app.Deps{
		Log:        log,
		ServerRepo: serverRepo,
		AdminRepo:  adminRepo,
		SecretKey:  key,
		JWTSecret:  jwtSecret,
	}

	ginEngine := app.NewWithDeps(deps)

	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           ginEngine,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	log.Info("listening", "addr", cfg.Server.Addr)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-sigCh:
		log.Info("shutdown signal received")
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	log.Info("server stopped")
	return nil
}

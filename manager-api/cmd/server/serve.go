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

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/alert"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/app"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/poller"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/secrets"
)

const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 90 * time.Second
	shutdownTimeout   = 10 * time.Second
	maxHeaderBytes    = 1 << 20 // 1 MiB header cap (SND-5)
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

	// ---- Secret key (SND-6) ----
	// Token secrets are AES-GCM sealed with this key. Missing key = plaintext
	// storage, which is never acceptable in production. Fail closed unless the
	// operator explicitly opts into the dev plaintext fallback (and never in
	// production).
	key, err := secrets.LoadKey(cfg.Secrets.KeyFile)
	if err != nil {
		isProd := cfg.Server.Env != "development"
		switch {
		case isProd:
			return fmt.Errorf("encryption key required in %q env but not loaded from %q: %w", cfg.Server.Env, cfg.Secrets.KeyFile, err)
		case !cfg.Secrets.AllowPlaintextFallback:
			return fmt.Errorf("encryption key not loaded from %q: set [secrets].key_file, or enable [secrets].allow_plaintext_fallback for dev: %w", cfg.Secrets.KeyFile, err)
		default:
			log.Warn("secret key not loaded — DEV plaintext token fallback enabled; do NOT use in production", "error", err)
		}
	} else {
		log.Info("secret key loaded", "path", cfg.Secrets.KeyFile)
	}

	// ---- DB ----
	var serverRepo repository.ServerRepository
	var adminRepo repository.AdminRepository
	var heartbeatRepo repository.HeartbeatRepository
	var metricRepo repository.MetricSampleRepository
	var sessionRepo repository.SessionRepository
	var apiTokenRepo repository.APITokenRepository
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
		heartbeatRepo = repository.NewHeartbeatRepository(gormDB)
		metricRepo = repository.NewMetricSampleRepository(gormDB)
		sessionRepo = repository.NewSessionRepository(gormDB)
		apiTokenRepo = repository.NewAPITokenRepository(gormDB)
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

	deps := app.Deps{
		Log:                   log,
		ServerRepo:            serverRepo,
		HeartbeatRepo:         heartbeatRepo,
		MetricSampleRepo:      metricRepo,
		SessionRepo:           sessionRepo,
		APITokenRepo:          apiTokenRepo,
		AdminRepo:             adminRepo,
		SecretKey:             key,
		JWTSecret:             jwtSecret,
		MaxBodyBytes:          cfg.Server.MaxBodyBytes,
		AllowPrivateTargets:   cfg.Server.AllowPrivateTargets,
		AllowPlaintextSecrets: cfg.Secrets.AllowPlaintextFallback,
		LoginMaxFailures:      cfg.Auth.LoginMaxFailures,
		LoginLockout:          time.Duration(cfg.Auth.LoginLockoutSeconds) * time.Second,
		LoginWindow:           time.Duration(cfg.Auth.LoginWindowSeconds) * time.Second,
	}

	ginEngine := app.NewWithDeps(deps)
	attachSPA(ginEngine) // serves the embedded SPA when built with -tags embedui

	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           ginEngine,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	log.Info("listening", "addr", cfg.Server.Addr)

	// Background health poller (roadmap M1): keeps fleet status current and
	// records heartbeats without an operator clicking Check.
	pollCtx, stopPoller := context.WithCancel(context.Background())
	defer stopPoller()
	if cfg.Poller.Enabled && serverRepo != nil && heartbeatRepo != nil {
		var notifier alert.Notifier
		if cfg.Alert.WebhookURL != "" {
			notifier = alert.NewWebhook(cfg.Alert.WebhookURL, log)
			log.Info("health alerts enabled", "channel", "webhook")
		}
		hp := poller.New(poller.Config{
			Servers:        serverRepo,
			Heartbeats:     heartbeatRepo,
			MetricSamples:  metricRepo,
			Sessions:       sessionRepo,
			SecretKey:      key,
			AllowPlaintext: cfg.Secrets.AllowPlaintextFallback,
			Interval:       time.Duration(cfg.Poller.IntervalSeconds) * time.Second,
			RetentionDays:  cfg.Poller.RetentionDays,
			Notifier:       notifier,
			CertWarnDays:   cfg.Poller.CertWarnDays,
			Log:            log,
		})
		go hp.Run(pollCtx)
	} else {
		log.Info("health poller disabled")
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-sigCh:
		log.Info("shutdown signal received")
	}
	stopPoller()

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	log.Info("server stopped")
	return nil
}

package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

var (
	sharedCfg *config
	sharedLog *slog.Logger
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "jabali-sounder",
		Short: "Jabali Sounder — central control plane for multiple Jabali Panel servers",
	}
	root.AddCommand(newServeCmd())
	root.AddCommand(newMigrateCmd())
	root.AddCommand(newAdminCmd())
	return root
}

func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Run database migrations",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "up",
		Short: "Apply all pending migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := initConfig()
			if err != nil {
				return err
			}
			if cfg.Database.URL == "" {
				return fmt.Errorf("database.url not set — set JABALI_SOUNDER_DATABASE_URL or config [database] url")
			}
			return dbMigrateUp(cfg.Database.Driver, cfg.Database.URL)
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "down",
		Short: "Roll back all migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := initConfig()
			if err != nil {
				return err
			}
			if cfg.Database.URL == "" {
				return fmt.Errorf("database.url not set — set JABALI_SOUNDER_DATABASE_URL or config [database] url")
			}
			return dbMigrateDown(cfg.Database.Driver, cfg.Database.URL)
		},
	})
	return cmd
}

func initConfig() (*config, error) {
	cfgPath := envFirst("JABALI_SOUNDER_CONFIG", "JABALI_MANAGER_CONFIG")
	if cfgPath == "" {
		cfgPath = defaultConfigPath
	}
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	sharedCfg = cfg
	sharedLog = newLogger(cfg.Log.Level, cfg.Log.Format)
	return cfg, nil
}

func newLogger(level, format string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler
	if format == "json" {
		h = slog.NewJSONHandler(os.Stderr, opts)
	} else {
		h = slog.NewTextHandler(os.Stderr, opts)
	}
	return slog.New(h)
}

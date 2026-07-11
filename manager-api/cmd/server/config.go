package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

// config is the full sounder configuration, loaded from TOML + env overrides.
type config struct {
	Server   serverConfig   `toml:"server"`
	Log      logConfig      `toml:"log"`
	Database databaseConfig `toml:"database"`
	Secrets  secretsConfig  `toml:"secrets"`
	JWT      jwtConfig      `toml:"jwt"`
	Auth     authConfig     `toml:"auth"`
	Poller   pollerConfig   `toml:"poller"`
}

type serverConfig struct {
	Addr string `toml:"addr"`
	Env  string `toml:"env"`
	// MaxBodyBytes caps the size of any request body (SND-5). 0 -> default.
	MaxBodyBytes int64 `toml:"max_body_bytes"`
	// AllowPrivateTargets permits enrolling panels on private/loopback/
	// link-local IPs (SND-4). Default false blocks SSRF to internal hosts.
	AllowPrivateTargets bool `toml:"allow_private_targets"`
}

type pollerConfig struct {
	// Enabled turns on the background health poller (roadmap M1).
	Enabled bool `toml:"enabled"`
	// IntervalSeconds between full fleet passes. Non-positive -> default.
	IntervalSeconds int `toml:"interval_seconds"`
	// RetentionDays bounds heartbeat history. 0 -> default; negative -> disabled.
	RetentionDays int `toml:"retention_days"`
}

type authConfig struct {
	// Login throttle (SND-3). Non-positive values fall back to defaults.
	LoginMaxFailures    int `toml:"login_max_failures"`
	LoginLockoutSeconds int `toml:"login_lockout_seconds"`
	LoginWindowSeconds  int `toml:"login_window_seconds"`
}

type logConfig struct {
	Level  string `toml:"level"`
	Format string `toml:"format"`
}

type databaseConfig struct {
	Driver string `toml:"driver"`
	URL    string `toml:"url"`
}

type secretsConfig struct {
	KeyFile string `toml:"key_file"`
	// AllowPlaintextFallback permits storing token secrets as hex plaintext
	// when no encryption key is present (SND-6). Dev-only; default false, and
	// production refuses to start without a key regardless.
	AllowPlaintextFallback bool `toml:"allow_plaintext_fallback"`
}

type jwtConfig struct {
	Secret string `toml:"secret"`
}

// Defaults returns the default config values.
func Defaults() config {
	return config{
		Server: serverConfig{
			Addr:         "127.0.0.1:8484",
			Env:          "development",
			MaxBodyBytes: 1 << 20, // 1 MiB
		},
		Auth: authConfig{
			LoginMaxFailures:    5,
			LoginLockoutSeconds: 900,
			LoginWindowSeconds:  900,
		},
		Poller: pollerConfig{
			Enabled:         true,
			IntervalSeconds: 60,
			RetentionDays:   14,
		},
		Log: logConfig{
			Level:  "info",
			Format: "text",
		},
		Database: databaseConfig{
			Driver: "mysql",
			URL:    "",
		},
		Secrets: secretsConfig{
			KeyFile: "/etc/jabali-sounder/secrets.key",
		},
	}
}

// loadConfig reads the TOML file at path (if it exists), applies defaults,
// then applies env-var overrides.
func loadConfig(path string) (*config, error) {
	cfg := Defaults()

	if data, err := os.ReadFile(path); err == nil { //nolint:gosec // operator-controlled config path
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	// Env overrides (highest precedence). JABALI_MANAGER_* remains supported
	// so existing installs can be renamed without breaking their unit files.
	if v := envFirst("JABALI_SOUNDER_ADDR", "JABALI_MANAGER_ADDR"); v != "" {
		cfg.Server.Addr = v
	}
	if v := envFirst("JABALI_SOUNDER_ENV", "JABALI_MANAGER_ENV"); v != "" {
		cfg.Server.Env = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.Log.Level = v
	}
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		cfg.Log.Format = v
	}
	if v := envFirst("JABALI_SOUNDER_DATABASE_URL", "JABALI_MANAGER_DATABASE_URL"); v != "" {
		cfg.Database.URL = v
	}
	if v := envFirst("JABALI_SOUNDER_DATABASE_DRIVER", "JABALI_MANAGER_DATABASE_DRIVER"); v != "" {
		cfg.Database.Driver = v
	}
	if v := envFirst("JABALI_SOUNDER_SECRET_KEY_FILE", "JABALI_MANAGER_SECRET_KEY_FILE"); v != "" {
		cfg.Secrets.KeyFile = v
	}
	if v := envFirst("JABALI_SOUNDER_JWT_SECRET", "JABALI_MANAGER_JWT_SECRET"); v != "" {
		cfg.JWT.Secret = v
	}

	if v := envFirst("JABALI_SOUNDER_ALLOW_PLAINTEXT_FALLBACK"); v != "" {
		cfg.Secrets.AllowPlaintextFallback = truthy(v)
	}
	if v := envFirst("JABALI_SOUNDER_ALLOW_PRIVATE_TARGETS"); v != "" {
		cfg.Server.AllowPrivateTargets = truthy(v)
	}
	if v := envFirst("JABALI_SOUNDER_POLL_ENABLED"); v != "" {
		cfg.Poller.Enabled = truthy(v)
	}
	if v := envFirst("JABALI_SOUNDER_POLL_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Poller.IntervalSeconds = n
		}
	}

	// Normalize: strip trailing slash from URL-like fields.
	cfg.Database.URL = strings.TrimSpace(cfg.Database.URL)
	cfg.Database.Driver = strings.TrimSpace(cfg.Database.Driver)
	if cfg.Database.Driver == "" {
		cfg.Database.Driver = "mysql"
	}

	return &cfg, nil
}

func envFirst(names ...string) string {
	for _, name := range names {
		if v := os.Getenv(name); v != "" {
			return v
		}
	}
	return ""
}

// truthy parses a boolean-ish env value ("1", "true", "yes", "on").
func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

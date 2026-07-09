// Package db provides the GORM open helper + golang-migrate integration.
package db

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"github.com/glebarez/sqlite"
	"github.com/golang-migrate/migrate/v4"
	mysqlmigrate "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open returns a GORM DB connected to the configured database driver.
func Open(dbDriver, dsn string) (*gorm.DB, error) {
	gormCfg := &gorm.Config{
		// Warn level, but don't log ErrRecordNotFound: repositories use
		// "not found" as normal control flow (e.g. admin set-password checks
		// for an existing admin before creating it), so logging it just spams
		// a fake-looking error — noticeably during install.
		Logger: logger.New(log.New(os.Stderr, "", log.LstdFlags), logger.Config{
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
		}),
	}
	var (
		db  *gorm.DB
		err error
	)
	switch dbDriver {
	case "", "mysql", "mariadb":
		db, err = gorm.Open(mysql.Open(dsn), gormCfg)
	case "sqlite", "sqlite3":
		db, err = gorm.Open(sqlite.Open(dsn), gormCfg)
	default:
		return nil, fmt.Errorf("unsupported database driver %q", dbDriver)
	}
	if err != nil {
		return nil, fmt.Errorf("gorm open: %w", err)
	}
	return db, nil
}

// Migrate applies all pending .sql migrations from the migrations directory.
func Migrate(dbDriver, dsn string) error {
	if dbDriver == "sqlite" || dbDriver == "sqlite3" {
		return autoMigrateSQLite(dsn)
	}
	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("sql open: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	driver, err := mysqlmigrate.WithInstance(sqlDB, &mysqlmigrate.Config{})
	if err != nil {
		return fmt.Errorf("migrate driver: %w", err)
	}

	migrationsDir := migrationsDir()
	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationsDir,
		"mysql",
		driver,
	)
	if err != nil {
		return fmt.Errorf("migrate new: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// MigrateDown rolls back all migrations.
func MigrateDown(dbDriver, dsn string) error {
	if dbDriver == "sqlite" || dbDriver == "sqlite3" {
		return fmt.Errorf("sqlite migrations cannot be rolled back automatically")
	}
	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("sql open: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	driver, err := mysqlmigrate.WithInstance(sqlDB, &mysqlmigrate.Config{})
	if err != nil {
		return fmt.Errorf("migrate driver: %w", err)
	}

	migrationsDir := migrationsDir()
	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationsDir,
		"mysql",
		driver,
	)
	if err != nil {
		return fmt.Errorf("migrate new: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

func autoMigrateSQLite(dsn string) error {
	gormDB, err := Open("sqlite", dsn)
	if err != nil {
		return err
	}
	if err := gormDB.AutoMigrate(&models.Server{}, &models.Heartbeat{}, &models.Admin{}); err != nil {
		return fmt.Errorf("sqlite automigrate: %w", err)
	}
	return nil
}

// migrationsDir returns the path to the embedded migrations directory.
// Uses an env override for testing, otherwise the package-relative path.
func migrationsDir() string {
	if v := os.Getenv("JABALI_SOUNDER_MIGRATIONS_DIR"); v != "" {
		return v
	}
	if v := os.Getenv("JABALI_MANAGER_MIGRATIONS_DIR"); v != "" {
		return v
	}
	return "manager-api/internal/db/migrations"
}

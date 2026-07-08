package main

import (
	"fmt"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
)

func dbMigrateUp(driver, dsn string) error {
	if err := db.Migrate(driver, dsn); err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

func dbMigrateDown(driver, dsn string) error {
	if err := db.MigrateDown(driver, dsn); err != nil {
		return fmt.Errorf("migrate down: %w", err)
	}
	return nil
}

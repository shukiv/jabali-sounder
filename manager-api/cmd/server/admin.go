package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/api"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
)

func newAdminCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Manage manager admin users",
	}
	cmd.AddCommand(newAdminSetPasswordCmd())
	return cmd
}

func newAdminSetPasswordCmd() *cobra.Command {
	var username string
	var password string
	cmd := &cobra.Command{
		Use:   "set-password",
		Short: "Create or update the admin password",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := initConfig()
			if err != nil {
				return err
			}
			if cfg.Database.URL == "" {
				return fmt.Errorf("database.url not set")
			}

			if password == "" {
				fmt.Printf("Enter password for admin '%s': ", username)
				pwdBytes, err := readPassword()
				if err != nil {
					return err
				}
				password = strings.TrimSpace(string(pwdBytes))
			}
			if len(password) < 8 {
				return fmt.Errorf("password must be at least 8 characters")
			}

			gormDB, err := db.Open(cfg.Database.Driver, cfg.Database.URL)
			if err != nil {
				return fmt.Errorf("open db: %w", err)
			}
			repo := repository.NewAdminRepository(gormDB)

			existing, err := repo.FindByUsername(cmd.Context(), username)
			if err != nil && err != repository.ErrNotFound {
				return fmt.Errorf("lookup admin: %w", err)
			}

			if existing != nil {
				existing.PasswordHash, err = api.HashPassword(password)
				if err != nil {
					return fmt.Errorf("hash password: %w", err)
				}
				if err := repo.Update(cmd.Context(), existing); err != nil {
					return fmt.Errorf("update admin: %w", err)
				}
				fmt.Println("Admin password updated.")
			} else {
				admin, err := api.NewAdmin(username, password, models.RoleOwner)
				if err != nil {
					return fmt.Errorf("create admin: %w", err)
				}
				if err := repo.Create(cmd.Context(), admin); err != nil {
					return fmt.Errorf("create admin: %w", err)
				}
				fmt.Println("Admin user created.")
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&username, "username", "u", "admin", "admin username")
	cmd.Flags().StringVarP(&password, "password", "p", "", "admin password (if empty, prompts interactively)")
	return cmd
}

// readPassword reads a password from stdin (no echo if terminal).
func readPassword() ([]byte, error) {
	return os.ReadFile("/dev/tty")
}

// Ensure gorm import is used.
var _ = gorm.ErrRecordNotFound

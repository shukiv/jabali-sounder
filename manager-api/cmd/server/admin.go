package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
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
	// Read a single line from the controlling terminal with echo disabled. The
	// previous os.ReadFile("/dev/tty") read to EOF (never returning on Enter) and
	// echoed the password in the clear.
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		// No controlling terminal (piped input): read one line from stdin.
		line, rerr := bufio.NewReader(os.Stdin).ReadString('\n')
		return []byte(strings.TrimRight(line, "\r\n")), rerr
	}
	defer func() { _ = tty.Close() }()
	pw, err := term.ReadPassword(int(tty.Fd()))
	fmt.Fprintln(tty) // newline after the hidden input
	return pw, err
}

// Ensure gorm import is used.
var _ = gorm.ErrRecordNotFound

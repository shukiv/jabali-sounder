package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/updater"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/version"
)

// newUpdateCmd self-updates the installed server binary to the latest GitHub
// release, verifying the download against the release's checksums.txt. It does
// NOT restart the service — the running process keeps executing the old (now
// replaced) binary until a restart, so we print that instruction.
func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Download and install the latest release, replacing this binary",
		RunE: func(cmd *cobra.Command, _ []string) error {
			exePath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("cannot locate the running binary: %w", err)
			}
			if resolved, rerr := filepath.EvalSymlinks(exePath); rerr == nil {
				exePath = resolved
			}

			client := updater.New("")
			ctx, cancel := context.WithTimeout(cmd.Context(), 6*time.Minute)
			defer cancel()

			cmd.Printf("Current version: %s\nChecking for the latest release…\n", version.Version)
			staged, err := client.DownloadAndStage(
				ctx, version.Version, "jabali-sounder-server",
				updater.HostOS(), updater.HostArch(), filepath.Dir(exePath),
			)
			if err != nil {
				return err // includes "already up to date (...)"
			}
			if err := updater.ReplaceExecutable(staged.Path, exePath, runtime.GOOS); err != nil {
				_ = os.Remove(staged.Path)
				return fmt.Errorf("install failed (need write access to %s? try sudo): %w", exePath, err)
			}
			cmd.Printf("Updated %s to %s.\nRestart the service to load it:  systemctl restart jabali-sounder\n", exePath, staged.Version)
			return nil
		},
	}
}

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
	var noRestart bool
	cmd := &cobra.Command{
		Use:          "update",
		Short:        "Download and install the latest release, replacing this binary",
		SilenceUsage: true,
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
			// A replaced binary only takes effect once the running process
			// restarts (until then it serves the old, now-deleted inode). Restart
			// the systemd unit automatically when one manages this install.
			if !noRestart && restartService() {
				cmd.Printf("Updated %s to %s and restarted the jabali-sounder service.\n", exePath, staged.Version)
			} else {
				cmd.Printf("Updated %s to %s.\nRestart the service to load it:  systemctl restart jabali-sounder\n", exePath, staged.Version)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&noRestart, "no-restart", false, "do not restart the systemd service after updating")
	return cmd
}

// restartService restarts the jabali-sounder systemd unit when systemd manages
// an active install, so the freshly installed binary is loaded. Returns false
// (leaving the caller to print the manual instruction) when there is no active
// unit or systemctl is unavailable.
func restartService() bool {
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}
	if err := exec.Command("systemctl", "is-active", "--quiet", "jabali-sounder").Run(); err != nil {
		return false // not managed by an active unit here
	}
	return exec.Command("systemctl", "restart", "jabali-sounder").Run() == nil
}

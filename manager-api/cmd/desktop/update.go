//go:build desktop

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	goruntime "runtime"
	"time"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/updater"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/version"
)

// UpdateResult is the JS-facing outcome of a self-update attempt.
type UpdateResult struct {
	OK               bool   `json:"ok"`
	Message          string `json:"message"`
	InstalledVersion string `json:"installed_version,omitempty"`
}

// InstallUpdate downloads the latest release for this OS/arch, verifies its
// checksum, swaps the running binary, and relaunches the app. Bound to JS as
// window.go.main.Bridge.InstallUpdate.
func (b *Bridge) InstallUpdate() (UpdateResult, error) {
	exePath, err := os.Executable()
	if err != nil {
		return UpdateResult{OK: false, Message: "cannot locate the running binary"}, nil
	}
	exePath, _ = resolveSymlink(exePath)

	client := updater.New("")
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()

	staged, err := client.DownloadAndStage(ctx, version.Version, "jabali-sounder", updater.HostOS(), updater.HostArch(), dirOf(exePath))
	if err != nil {
		return UpdateResult{OK: false, Message: "download failed: " + err.Error()}, nil
	}
	if err := updater.ReplaceExecutable(staged.Path, exePath, goruntime.GOOS); err != nil {
		_ = os.Remove(staged.Path)
		return UpdateResult{OK: false, Message: "install failed: " + err.Error()}, nil
	}

	// Relaunch shortly, giving the frontend time to show the result toast.
	go func() {
		time.Sleep(1200 * time.Millisecond)
		cmd := exec.Command(exePath)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Start(); err != nil {
			fmt.Fprintln(os.Stderr, "relaunch failed:", err)
		}
		os.Exit(0)
	}()

	return UpdateResult{OK: true, Message: "Updated to " + staged.Version + " — restarting…", InstalledVersion: staged.Version}, nil
}

func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == os.PathSeparator {
			return p[:i]
		}
	}
	return "."
}

// resolveSymlink follows a symlink so we replace the real binary, not the link.
func resolveSymlink(p string) (string, error) {
	resolved, err := os.Readlink(p)
	if err != nil {
		return p, nil // not a symlink
	}
	if len(resolved) == 0 || resolved[0] != os.PathSeparator {
		resolved = dirOf(p) + string(os.PathSeparator) + resolved
	}
	return resolved, nil
}

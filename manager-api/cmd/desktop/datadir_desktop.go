//go:build desktop

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// platformDataDir returns the per-user data directory on desktop builds.
func platformDataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("user config dir: %w", err)
	}
	return filepath.Join(base, "Jabali Sounder"), nil
}

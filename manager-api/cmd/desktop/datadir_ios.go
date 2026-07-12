//go:build ios

package main

import (
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// platformDataDir returns the app's Application Support directory on iOS,
// suitable for the SQLite DB and secret material.
func platformDataDir() (string, error) {
	p := application.IOS.StoragePath()
	if p == "" {
		return "", fmt.Errorf("ios storage path unavailable")
	}
	return p, nil
}

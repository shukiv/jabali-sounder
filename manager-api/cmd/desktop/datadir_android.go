//go:build android

package main

import (
	"fmt"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// platformDataDir returns the app's private files directory on Android
// (activity.getFilesDir()), suitable for the SQLite DB and secret material.
func platformDataDir() (string, error) {
	p := application.Android.StoragePath()
	if p == "" {
		return "", fmt.Errorf("android storage path unavailable")
	}
	return p, nil
}

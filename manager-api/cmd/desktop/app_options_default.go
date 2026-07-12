//go:build desktop || android

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// modifyOptionsForIOS is a no-op on non-iOS platforms.
func modifyOptionsForIOS(_ *application.Options) {}

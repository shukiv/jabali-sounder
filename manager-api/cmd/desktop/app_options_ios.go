//go:build ios

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// modifyOptionsForIOS adjusts the application options for iOS.
func modifyOptionsForIOS(opts *application.Options) {
	// Disable the default signal handler on iOS to prevent crashes.
	opts.DisableDefaultSignalHandler = true
}

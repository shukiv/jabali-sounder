//go:build android

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// In c-shared build mode main() is not called automatically; the Android host
// invokes the registered function once the JNI bridge is ready.
func init() {
	application.RegisterAndroidMain(main)
}

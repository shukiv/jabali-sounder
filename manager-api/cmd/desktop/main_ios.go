//go:build ios

package main

import "C"

// WailsIOSMain runs the user's main() (application.New / app.Run). The
// WailsAppDelegate invokes it from didFinishLaunchingWithOptions — after UIKit
// has launched — on a background thread, so the Go runtime never starts
// concurrently with UIApplicationMain.
//
//export WailsIOSMain
func WailsIOSMain() {
	main()
}

//go:build desktop

package main

// shareText is a no-op on desktop, which exports via the native SaveFile dialog.
func shareText(string) {}

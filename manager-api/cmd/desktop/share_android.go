//go:build android

package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// shareText writes the content to a file under the app's files dir (exposed via
// the FileProvider) and opens the Android share sheet with it AS A FILE, so the
// chooser offers "Save to Files/Drive" rather than only text targets. Falls back
// to a plain text share if the file can't be written.
func shareText(content string) {
	base := application.Android.StoragePath()
	if base != "" {
		dir := filepath.Join(base, "exports")
		if err := os.MkdirAll(dir, 0o700); err == nil {
			path := filepath.Join(dir, "jabali-sounder-settings.json")
			if err := os.WriteFile(path, []byte(content), 0o600); err == nil {
				payload, _ := json.Marshal(map[string]string{"file": path, "mimeType": "application/json"})
				application.Android.Share(string(payload))
				return
			}
		}
	}
	payload, _ := json.Marshal(map[string]string{"text": content})
	application.Android.Share(string(payload))
}

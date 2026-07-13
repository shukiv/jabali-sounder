//go:build android

package main

import (
	"encoding/json"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// shareText presents the Android share chooser with the given text.
func shareText(content string) {
	payload, _ := json.Marshal(map[string]string{"text": content})
	application.Android.Share(string(payload))
}

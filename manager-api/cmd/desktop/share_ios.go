//go:build ios

package main

import (
	"encoding/json"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// shareText presents the iOS share sheet with the given text.
func shareText(content string) {
	payload, _ := json.Marshal(map[string]string{"text": content})
	application.IOS.Share(string(payload))
}

//go:build desktop || android || ios

package main

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
)

// APIResult is the JS-facing result of an in-process API request.
type APIResult struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

// ApiCall runs an HTTP request through the in-process backend — the same
// gin+SPA handler the asset server uses — and returns the status + body.
//
// On mobile the WebView's asset loader (Android WebViewAssetLoader.PathHandler /
// iOS scheme handler) cannot convey the request method, headers, or body: every
// request arrives as a bodyless GET with no headers. So the SPA routes every
// /api/v1 call through here via @wailsio/runtime, which carries the full
// payload (method, Authorization header, JSON body).
func (b *Bridge) ApiCall(method, path, headersJSON, body string) (APIResult, error) {
	if method == "" {
		method = "GET"
	}
	req := httptest.NewRequest(strings.ToUpper(method), path, strings.NewReader(body))
	if headersJSON != "" {
		var h map[string]string
		if err := json.Unmarshal([]byte(headersJSON), &h); err == nil {
			for k, v := range h {
				if v != "" {
					req.Header.Set(k, v)
				}
			}
		}
	}
	rec := httptest.NewRecorder()
	b.handler.ServeHTTP(rec, req)
	return APIResult{Status: rec.Code, Body: rec.Body.String()}, nil
}

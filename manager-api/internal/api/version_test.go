package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestVersionEndpointCurrentOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	RegisterVersionRoutes(r.Group("/api/v1"), VersionHandlerConfig{Updater: nil})
	w := do(r, http.MethodGet, "/api/v1/version", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if _, ok := body["version"]; !ok {
		t.Fatalf("missing version field: %v", body)
	}
	if body["update_available"] != false {
		t.Fatalf("update_available should be false without an updater: %v", body["update_available"])
	}
}

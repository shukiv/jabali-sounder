package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestBodyLimit covers SND-5: an over-sized declared body is rejected with 413,
// a body within the cap is served normally.
func TestBodyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BodyLimit(100))
	r.POST("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	big := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(strings.Repeat("A", 200)))
	bw := httptest.NewRecorder()
	r.ServeHTTP(bw, big)
	if bw.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body: got %d, want 413", bw.Code)
	}

	small := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader("hi"))
	sw := httptest.NewRecorder()
	r.ServeHTTP(sw, small)
	if sw.Code != http.StatusOK {
		t.Fatalf("small body: got %d, want 200", sw.Code)
	}
}

package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
)

// A transient session-store error must NOT masquerade as a dead session (which
// would log the user out). The middleware fails soft with 503 so the client
// retries instead of dropping to the login screen (#5).
func TestAuthMiddlewareSessionCheckFailsSoft(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "test-secret-not-empty-0000"
	token, _, err := MintToken(secret, "adm_1", "admin", models.RoleOwner, "sess_1", time.Hour)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}

	cases := []struct {
		name  string
		check SessionCheck
		want  int
	}{
		{"active", func(context.Context, string) (bool, error) { return true, nil }, http.StatusOK},
		{"revoked", func(context.Context, string) (bool, error) { return false, nil }, http.StatusUnauthorized},
		{"infra-error", func(context.Context, string) (bool, error) { return false, errors.New("database is locked") }, http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.Use(AuthMiddleware(secret, tc.check, nil))
			r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			r.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("%s: want %d, got %d", tc.name, tc.want, w.Code)
			}
		})
	}
}

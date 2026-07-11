package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/db"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/middleware"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/models"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/repository"
	"git.jabali-panel.com/shukivaknin/jabali-sounder/manager-api/internal/totp"
)

func newAuthTestRouter(t *testing.T, maxFailures int) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dbPath := filepath.Join(t.TempDir(), "auth.db")
	if err := db.Migrate("sqlite", dbPath); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	gormDB, err := db.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	repo := repository.NewAdminRepository(gormDB)
	admin, err := NewAdmin("admin", "correct-horse-battery", models.RoleOwner)
	if err != nil {
		t.Fatalf("new admin: %v", err)
	}
	if err := repo.Create(context.Background(), admin); err != nil {
		t.Fatalf("seed admin: %v", err)
	}

	r := gin.New()
	r.Use(middleware.RequestID())
	RegisterAuthRoutes(r.Group("/api/v1"), AuthHandlerConfig{
		AdminRepo:        repo,
		JWTSecret:        "test-jwt-secret-not-empty-000000",
		JWTTTL:           time.Hour,
		LoginMaxFailures: maxFailures,
		LoginLockout:     time.Hour,
		LoginWindow:      time.Hour,
		AllowPlaintext:   true,
	})
	return r
}

func postLogin(r *gin.Engine, ip, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = ip + ":5555"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestLoginFailuresAreIndistinguishable covers SND-2: a missing user and a bad
// password return the identical opaque body, so login can't enumerate usernames
// or leak internals.
func TestLoginFailuresAreIndistinguishable(t *testing.T) {
	r := newAuthTestRouter(t, 100)

	missing := postLogin(r, "10.1.0.1", `{"username":"ghost","password":"whatever"}`)
	badpw := postLogin(r, "10.1.0.2", `{"username":"admin","password":"wrong"}`)

	for name, w := range map[string]*httptest.ResponseRecorder{"missing": missing, "badpw": badpw} {
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status = %d, want 401", name, w.Code)
		}
		var body map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatalf("%s: bad json: %v", name, err)
		}
		if body["error"] != "invalid_credentials" {
			t.Errorf("%s: error = %v, want invalid_credentials", name, body["error"])
		}
		if _, leaked := body["detail"]; leaked {
			t.Errorf("%s: response leaked a detail field", name)
		}
	}
	if missing.Body.String() != badpw.Body.String() {
		t.Errorf("missing-user and bad-password responses differ:\n  %s\n  %s", missing.Body.String(), badpw.Body.String())
	}
}

// TestLoginRateLimited covers SND-3 end-to-end: repeated failures from one IP
// eventually return 429.
func TestLoginRateLimited(t *testing.T) {
	r := newAuthTestRouter(t, 3)
	for i := 0; i < 3; i++ {
		postLogin(r, "10.2.0.1", `{"username":"admin","password":"wrong"}`)
	}
	if w := postLogin(r, "10.2.0.1", `{"username":"admin","password":"wrong"}`); w.Code != http.StatusTooManyRequests {
		t.Fatalf("after threshold: status = %d, want 429", w.Code)
	}
}

// TestLoginSucceedsWithCorrectPassword is the positive control.
func TestLoginSucceedsWithCorrectPassword(t *testing.T) {
	r := newAuthTestRouter(t, 5)
	w := postLogin(r, "10.3.0.1", `{"username":"admin","password":"correct-horse-battery"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("valid login: status = %d, body = %s", w.Code, w.Body.String())
	}
}

func postAuth(r *gin.Engine, path, token, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1"+path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.RemoteAddr = "10.9.0.1:5555"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestTwoFactorEnableAndLogin covers the M3 2FA flow: enroll -> activate ->
// login now requires a valid TOTP code.
func TestTwoFactorEnableAndLogin(t *testing.T) {
	r := newAuthTestRouter(t, 100)

	// Login (no 2FA yet) to get a token.
	w := postLogin(r, "10.9.0.1", `{"username":"admin","password":"correct-horse-battery"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("initial login: %d", w.Code)
	}
	var lr struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &lr)
	if lr.Token == "" {
		t.Fatal("no token from initial login")
	}

	// Enroll 2FA.
	sw := postAuth(r, "/auth/2fa/setup", lr.Token, "")
	if sw.Code != http.StatusOK {
		t.Fatalf("2fa setup: %d (%s)", sw.Code, sw.Body.String())
	}
	var setup struct {
		Secret string `json:"secret"`
	}
	_ = json.Unmarshal(sw.Body.Bytes(), &setup)

	code, _ := totp.Code(setup.Secret, time.Now())
	aw := postAuth(r, "/auth/2fa/activate", lr.Token, `{"code":"`+code+`"}`)
	if aw.Code != http.StatusOK {
		t.Fatalf("2fa activate: %d (%s)", aw.Code, aw.Body.String())
	}

	// Login without a code -> challenge, no token.
	cw := postLogin(r, "10.9.0.1", `{"username":"admin","password":"correct-horse-battery"}`)
	var chal map[string]any
	_ = json.Unmarshal(cw.Body.Bytes(), &chal)
	if chal["two_factor_required"] != true || chal["token"] != nil {
		t.Fatalf("expected 2fa challenge, got %v", chal)
	}

	// Login with a fresh code -> token.
	code2, _ := totp.Code(setup.Secret, time.Now())
	fw := postLogin(r, "10.9.0.1", `{"username":"admin","password":"correct-horse-battery","totp_code":"`+code2+`"}`)
	var ok struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(fw.Body.Bytes(), &ok)
	if fw.Code != http.StatusOK || ok.Token == "" {
		t.Fatalf("2fa login should succeed with token, got %d (%s)", fw.Code, fw.Body.String())
	}
}

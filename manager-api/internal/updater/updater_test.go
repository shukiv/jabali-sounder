package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.0.0", "v1.0.1", -1},
		{"1.2.0", "1.2.0", 0},
		{"v2.0.0", "v1.9.9", 1},
		{"v1.2.0-rc1", "v1.2.0", -1}, // prerelease < release
		{"v1.2.0", "v1.2.0-rc1", 1},
		{"0.3.0", "v0.3.0", 0}, // leading v ignored
		{"v1.10.0", "v1.9.0", 1},
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestAssetFor(t *testing.T) {
	assets := []Asset{
		{Name: "jabali-sounder-linux-amd64-0.4.0"},
		{Name: "jabali-sounder-windows-amd64-0.4.0.exe"},
		{Name: "jabali-sounder-macos-arm64-0.4.0"},
		{Name: "jabali-sounder-macos-arm64-0.4.0.dmg"},
		{Name: "jabali-sounder-server-linux-amd64-0.4.0"},
	}
	if a, ok := AssetFor(assets, "jabali-sounder", "linux", "amd64"); !ok || a.Name != "jabali-sounder-linux-amd64-0.4.0" {
		t.Fatalf("linux desktop asset: %+v ok=%v", a, ok)
	}
	if a, ok := AssetFor(assets, "jabali-sounder", "windows", "amd64"); !ok || a.Name != "jabali-sounder-windows-amd64-0.4.0.exe" {
		t.Fatalf("windows asset: %+v ok=%v", a, ok)
	}
	// macOS must pick the raw binary, not the .dmg.
	if a, ok := AssetFor(assets, "jabali-sounder", "darwin", "arm64"); !ok || a.Name != "jabali-sounder-macos-arm64-0.4.0" {
		t.Fatalf("darwin asset: %+v ok=%v", a, ok)
	}
	if a, ok := AssetFor(assets, "jabali-sounder-server", "linux", "amd64"); !ok || a.Name != "jabali-sounder-server-linux-amd64-0.4.0" {
		t.Fatalf("server asset: %+v ok=%v", a, ok)
	}
	if _, ok := AssetFor(assets, "jabali-sounder", "linux", "arm64"); ok {
		t.Fatal("no linux-arm64 asset should match")
	}
}

func githubStub(t *testing.T, tag string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The updater resolves the latest tag from the releases/latest redirect.
		w.Header().Set("Location", "https://github.com/owner/repo/releases/tag/"+tag)
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	c := New("owner/repo")
	// Redirect the client at the stub by overriding its transport.
	c.HTTPClient = &http.Client{Transport: redirect(srv.URL)}
	return c
}

type redirect string

func (r redirect) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = string(r)[len("http://"):]
	return http.DefaultTransport.RoundTrip(req)
}

func TestCheckUpdateAvailable(t *testing.T) {
	c := githubStub(t, "v0.5.0")
	now := time.Unix(1_700_000_000, 0)
	st, err := c.Check(context.Background(), "v0.4.0", now)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !st.UpdateAvailable || st.Latest != "v0.5.0" {
		t.Fatalf("want update to v0.5.0, got %+v", st)
	}
}

func TestCheckUpToDate(t *testing.T) {
	c := githubStub(t, "v0.4.0")
	st, err := c.Check(context.Background(), "v0.4.0", time.Unix(1_700_000_000, 0))
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if st.UpdateAvailable {
		t.Fatalf("should be up to date: %+v", st)
	}
}

func TestCheckDevBuildNeverUpdates(t *testing.T) {
	c := githubStub(t, "v9.9.9")
	st, err := c.Check(context.Background(), "dev", time.Unix(1_700_000_000, 0))
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if st.UpdateAvailable {
		t.Fatal("dev build must not report an update")
	}
}

func TestCheckCaches(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.Header().Set("Location", "https://github.com/owner/repo/releases/tag/v0.5.0")
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()
	c := New("owner/repo")
	c.HTTPClient = &http.Client{Transport: redirect(srv.URL)}
	now := time.Unix(1_700_000_000, 0)
	_, _ = c.Check(context.Background(), "v0.4.0", now)
	_, _ = c.Check(context.Background(), "v0.4.0", now.Add(time.Minute)) // within TTL
	if hits != 1 {
		t.Fatalf("expected 1 fetch (cached), got %d", hits)
	}
}

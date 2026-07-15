package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseChecksums(t *testing.T) {
	in := "abc123  jabali-sounder-linux-amd64-0.4.0\n" +
		"def456 *jabali-sounder-windows-amd64-0.4.0.exe\n" +
		"\n# comment line ignored\n"
	m := ParseChecksums(in)
	if m["jabali-sounder-linux-amd64-0.4.0"] != "abc123" {
		t.Fatalf("linux sum: %v", m)
	}
	if m["jabali-sounder-windows-amd64-0.4.0.exe"] != "def456" {
		t.Fatalf("windows sum (star marker) : %v", m)
	}
}

func TestReplaceExecutableUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix rename semantics")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "app")
	staged := filepath.Join(dir, ".staged")
	_ = os.WriteFile(exe, []byte("old"), 0o755)
	_ = os.WriteFile(staged, []byte("new"), 0o755)
	if err := ReplaceExecutable(staged, exe, "linux"); err != nil {
		t.Fatalf("replace: %v", err)
	}
	got, _ := os.ReadFile(exe)
	if string(got) != "new" {
		t.Fatalf("exe not replaced: %q", got)
	}
}

func TestDownloadAndStageVerifiesChecksum(t *testing.T) {
	payload := []byte("BINARY-CONTENT-v0.6.0")
	sum := sha256.Sum256(payload)
	sumHex := hex.EncodeToString(sum[:])
	assetName := "jabali-sounder-linux-amd64-0.6.0"

	mux := http.NewServeMux()
	mux.HandleFunc("/o/r/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://github.com/o/r/releases/tag/v0.6.0")
		w.WriteHeader(http.StatusFound)
	})
	mux.HandleFunc("/o/r/releases/latest/download/"+assetName, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(payload) })
	mux.HandleFunc("/o/r/releases/latest/download/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(sumHex + "  " + assetName + "\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New("o/r")
	c.HTTPClient = &http.Client{Transport: hostRewrite(srv.URL)}

	stageDir := t.TempDir()
	staged, err := c.DownloadAndStage(context.Background(), "v0.5.0", "jabali-sounder", "linux", "amd64", stageDir)
	if err != nil {
		t.Fatalf("stage: %v", err)
	}
	if staged.Version != "v0.6.0" {
		t.Fatalf("version: %s", staged.Version)
	}
	got, _ := os.ReadFile(staged.Path)
	if string(got) != string(payload) {
		t.Fatalf("staged content wrong")
	}
	fi, _ := os.Stat(staged.Path)
	if runtime.GOOS != "windows" && fi.Mode().Perm()&0o100 == 0 {
		t.Fatalf("staged not executable: %v", fi.Mode())
	}
}

func TestDownloadAndStageRejectsBadChecksum(t *testing.T) {
	assetName := "jabali-sounder-linux-amd64-0.6.0"
	mux := http.NewServeMux()
	mux.HandleFunc("/o/r/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "https://github.com/o/r/releases/tag/v0.6.0")
		w.WriteHeader(http.StatusFound)
	})
	mux.HandleFunc("/o/r/releases/latest/download/"+assetName, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("tampered")) })
	mux.HandleFunc("/o/r/releases/latest/download/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("0000000000000000000000000000000000000000000000000000000000000000  " + assetName + "\n"))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New("o/r")
	c.HTTPClient = &http.Client{Transport: hostRewrite(srv.URL)}
	stageDir := t.TempDir()
	_, err := c.DownloadAndStage(context.Background(), "v0.5.0", "jabali-sounder", "linux", "amd64", stageDir)
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
	// Staged file must be cleaned up on mismatch.
	files, _ := os.ReadDir(stageDir)
	if len(files) != 0 {
		t.Fatalf("staged file left behind: %v", files)
	}
}

// hostRewrite points api.github.com (and the http://REPL/ placeholder) at the
// test server.
type hostRewrite string

func (h hostRewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = string(h)[len("http://"):]
	return http.DefaultTransport.RoundTrip(req)
}

package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const downloadTimeout = 5 * time.Minute

// StagedUpdate is a downloaded, checksum-verified binary ready to swap in.
type StagedUpdate struct {
	Path      string // staged binary on disk (same dir as the target exe)
	Version   string // release tag being installed
	AssetName string
}

// HostOS and HostArch expose the compile-time platform to callers that shadow
// the std `runtime` import (e.g. the Wails desktop entrypoint).
func HostOS() string   { return runtime.GOOS }
func HostArch() string { return runtime.GOARCH }

// DownloadAndStage fetches the latest release's asset for prefix/goos/goarch,
// verifies it against the release's checksums.txt, and writes it (mode 0755)
// next to the target executable. It does NOT replace anything.
func (c *Client) DownloadAndStage(ctx context.Context, current, prefix, goos, goarch, stageDir string) (StagedUpdate, error) {
	rel, err := c.latest(ctx, time.Now())
	if err != nil {
		return StagedUpdate{}, err
	}
	if !isDev(current) && Compare(current, rel.TagName) >= 0 {
		return StagedUpdate{}, fmt.Errorf("already up to date (%s)", current)
	}
	assetName := DownloadAssetName(prefix, goos, goarch, rel.TagName)
	base := "https://github.com/" + c.Repo + "/releases/latest/download"

	sums, err := c.fetchChecksums(ctx, base+"/checksums.txt")
	if err != nil {
		return StagedUpdate{}, err
	}
	want := sums[assetName]
	if want == "" {
		return StagedUpdate{}, fmt.Errorf("no checksum published for %s", assetName)
	}

	staged := filepath.Join(stageDir, ".jabali-sounder-update-"+assetName)
	sum, err := c.downloadTo(ctx, base+"/"+assetName, staged)
	if err != nil {
		return StagedUpdate{}, err
	}
	if !strings.EqualFold(sum, want) {
		_ = os.Remove(staged)
		return StagedUpdate{}, fmt.Errorf("checksum mismatch for %s (got %s, want %s)", assetName, sum, want)
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		_ = os.Remove(staged)
		return StagedUpdate{}, fmt.Errorf("chmod staged: %w", err)
	}
	return StagedUpdate{Path: staged, Version: rel.TagName, AssetName: assetName}, nil
}

// DownloadAssetName returns the release-asset filename for goos/goarch at the
// given release tag, matching the release-workflow naming
// (<prefix>-<os>-<arch>-<version>[.exe]). Every platform publishes a
// version-suffixed asset, so this works via releases/latest/download/<name>.
func DownloadAssetName(prefix, goos, goarch, tag string) string {
	osTag := map[string]string{"darwin": "macos", "windows": "windows", "linux": "linux"}[goos]
	if osTag == "" {
		osTag = goos
	}
	name := prefix + "-" + osTag + "-" + goarch + "-" + strings.TrimPrefix(tag, "v")
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// fetchChecksums downloads and parses the release's checksums.txt (from a
// download-path URL) into a map of filename -> sha256 hex.
func (c *Client) fetchChecksums(ctx context.Context, url string) (map[string]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("checksums request: %w", err)
	}
	req.Header.Set("User-Agent", "jabali-sounder")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("checksums fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("checksums HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("checksums read: %w", err)
	}
	return ParseChecksums(string(body)), nil
}

// ParseChecksums parses "<sha256>  <filename>" lines (as produced by sha256sum).
func ParseChecksums(s string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) != 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*") // sha256sum binary marker
		out[name] = fields[0]
	}
	return out
}

// downloadTo streams url to path and returns the content's sha256 hex.
func (c *Client) downloadTo(ctx context.Context, url, path string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("download request: %w", err)
	}
	req.Header.Set("User-Agent", "jabali-sounder")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("download HTTP %d", resp.StatusCode)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return "", fmt.Errorf("create staged: %w", err)
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), resp.Body); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("download copy: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close staged: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ReplaceExecutable swaps the staged binary in for exePath. On Unix a rename
// over the running binary is safe (the process keeps its open inode). On Windows
// the running exe is first moved aside to <exe>.old.
func ReplaceExecutable(staged, exePath, goos string) error {
	if goos == "windows" {
		old := exePath + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exePath, old); err != nil {
			return fmt.Errorf("move current exe aside: %w", err)
		}
		if err := os.Rename(staged, exePath); err != nil {
			// Best-effort rollback.
			_ = os.Rename(old, exePath)
			return fmt.Errorf("install new exe: %w", err)
		}
		return nil
	}
	if err := os.Rename(staged, exePath); err != nil {
		return fmt.Errorf("replace exe: %w", err)
	}
	return nil
}

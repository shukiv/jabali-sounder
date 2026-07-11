// Package updater checks GitHub for the latest published release and compares it
// to the running build, so Sounder can surface (and, on desktop, install)
// updates. Results are cached to respect GitHub's unauthenticated rate limit.
package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultRepo is the GitHub "owner/name" releases are published to.
const DefaultRepo = "shukiv/jabali-sounder"

const (
	apiTimeout = 10 * time.Second
	cacheTTL   = time.Hour
)

// Asset is a downloadable file attached to a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// Release is the subset of the GitHub release payload we use.
type Release struct {
	TagName     string  `json:"tag_name"`
	HTMLURL     string  `json:"html_url"`
	PublishedAt string  `json:"published_at"`
	Prerelease  bool    `json:"prerelease"`
	Draft       bool    `json:"draft"`
	Assets      []Asset `json:"assets"`
}

// Status is the comparison result surfaced to the API/UI.
type Status struct {
	Current         string  `json:"current"`
	Latest          string  `json:"latest"`
	UpdateAvailable bool    `json:"update_available"`
	ReleaseURL      string  `json:"release_url"`
	PublishedAt     string  `json:"published_at"`
	Assets          []Asset `json:"assets,omitempty"`
	Checked         string  `json:"checked_at"`
}

// Client fetches and caches the latest release for a repo.
type Client struct {
	Repo       string
	HTTPClient *http.Client

	mu       sync.Mutex
	cached   *Release
	cachedAt time.Time
	cacheErr error
}

// New returns a Client for repo (empty -> DefaultRepo).
func New(repo string) *Client {
	if repo == "" {
		repo = DefaultRepo
	}
	return &Client{Repo: repo, HTTPClient: &http.Client{Timeout: apiTimeout}}
}

// latest returns the latest non-draft release, cached for cacheTTL. The clock is
// injected so callers/tests are deterministic.
func (c *Client) latest(ctx context.Context, now time.Time) (*Release, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cached != nil && now.Sub(c.cachedAt) < cacheTTL {
		return c.cached, nil
	}
	rel, err := c.fetchLatest(ctx)
	c.cachedAt = now
	if err != nil {
		c.cacheErr = err
		// Serve a stale cache on transient error rather than nothing.
		if c.cached != nil {
			return c.cached, nil
		}
		return nil, err
	}
	c.cached = rel
	c.cacheErr = nil
	return rel, nil
}

func (c *Client) fetchLatest(ctx context.Context) (*Release, error) {
	url := "https://api.github.com/repos/" + c.Repo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("updater request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "jabali-sounder")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("updater fetch: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no releases published")
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("github returned HTTP %d", resp.StatusCode)
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("updater decode: %w", err)
	}
	return &rel, nil
}

// Check compares the current build against the latest release. A dev/un-stamped
// build never reports an update. On GitHub error it returns a Status with the
// current version and update_available=false, plus the error.
func (c *Client) Check(ctx context.Context, current string, now time.Time) (Status, error) {
	st := Status{Current: current, Checked: now.UTC().Format(time.RFC3339)}
	rel, err := c.latest(ctx, now)
	if err != nil {
		return st, err
	}
	st.Latest = rel.TagName
	st.ReleaseURL = rel.HTMLURL
	st.PublishedAt = rel.PublishedAt
	st.Assets = rel.Assets
	if isDev(current) || rel.Draft {
		return st, nil
	}
	st.UpdateAvailable = Compare(current, rel.TagName) < 0
	return st, nil
}

func isDev(v string) bool { return v == "" || v == "dev" }

// Compare returns -1 if a<b, 0 if equal, +1 if a>b, using a lenient semver order
// (leading "v" ignored; a prerelease like 1.2.0-rc sorts before 1.2.0).
func Compare(a, b string) int {
	an, ap := splitSemver(a)
	bn, bp := splitSemver(b)
	for i := 0; i < 3; i++ {
		if an[i] != bn[i] {
			if an[i] < bn[i] {
				return -1
			}
			return 1
		}
	}
	// Equal numeric core: a release outranks a prerelease.
	switch {
	case ap == "" && bp == "":
		return 0
	case ap == "" && bp != "":
		return 1
	case ap != "" && bp == "":
		return -1
	default:
		return strings.Compare(ap, bp)
	}
}

// splitSemver returns the [major,minor,patch] ints and the prerelease suffix.
func splitSemver(v string) ([3]int, string) {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	pre := ""
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		pre = v[i+1:]
		v = v[:i]
	}
	var out [3]int
	for i, part := range strings.SplitN(v, ".", 3) {
		if i > 2 {
			break
		}
		out[i], _ = strconv.Atoi(strings.TrimSpace(part))
	}
	return out, pre
}

// AssetFor returns the release asset matching goos/goarch, or false. It matches
// the release-workflow naming: jabali-sounder-<os>-<arch>[-<ver>][.exe] for the
// desktop app and jabali-sounder-server-<os>-<arch>[-<ver>] for the server.
func AssetFor(assets []Asset, prefix, goos, goarch string) (Asset, bool) {
	osTag := map[string]string{"darwin": "macos", "windows": "windows", "linux": "linux"}[goos]
	if osTag == "" {
		osTag = goos
	}
	needle := prefix + "-" + osTag + "-" + goarch
	for _, a := range assets {
		name := strings.TrimSuffix(a.Name, ".exe")
		// Accept exact or version-suffixed names, but not the .dmg/.app variants.
		if strings.HasSuffix(a.Name, ".dmg") || strings.HasSuffix(a.Name, ".app") {
			continue
		}
		if name == needle || strings.HasPrefix(name, needle+"-") {
			return a, true
		}
	}
	return Asset{}, false
}

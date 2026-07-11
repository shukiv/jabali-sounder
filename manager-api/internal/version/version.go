// Package version holds build-time identifiers stamped via -ldflags -X. The
// zero (un-stamped) values mark a local/dev build.
package version

// These are overridden at build time, e.g.:
//   -ldflags "-X .../internal/version.Version=v0.4.0 -X ...Commit=abc123 -X ...Date=2026-07-11"
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Info is the structured build identity returned by the /version endpoint.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// Current returns the stamped build identity.
func Current() Info {
	return Info{Version: Version, Commit: Commit, Date: Date}
}

// IsDev reports whether this is an un-stamped local/dev build.
func IsDev() bool {
	return Version == "dev" || Version == ""
}

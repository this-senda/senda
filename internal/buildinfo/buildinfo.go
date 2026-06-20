// Package buildinfo holds build-time metadata injected via -ldflags -X.
package buildinfo

import "runtime/debug"

// Set at build time via -ldflags "-X senda/internal/buildinfo.<Var>=<value>".
// Defaults make `go run`/`go build` without ldflags still produce sane output.
var (
	// Version is the release version (e.g. "1.2.3"). "dev" for local builds.
	Version = "dev"
	// Commit is the short git SHA the binary was built from.
	Commit = ""
	// Date is the build timestamp (RFC3339, UTC).
	Date = ""
)

// Info is the full build metadata surfaced to the UI and `--version` output.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// Get returns the build metadata, falling back to the Go build info embedded by
// the toolchain (module VCS stamp) when ldflags were not supplied.
func Get() Info {
	commit := Commit
	if commit == "" {
		if bi, ok := debug.ReadBuildInfo(); ok {
			for _, s := range bi.Settings {
				if s.Key == "vcs.revision" {
					commit = shortSHA(s.Value)
				}
			}
		}
	}
	return Info{Version: Version, Commit: commit, Date: Date}
}

func shortSHA(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

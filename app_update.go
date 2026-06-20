package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"golang.org/x/mod/semver"

	"senda/internal/buildinfo"
)

// releasesAPI is the GitHub endpoint for the latest published release.
const releasesAPI = "https://api.github.com/repos/this-senda/senda/releases/latest"

// releasesPage is the human-facing releases page opened from the UI.
const releasesPage = "https://github.com/this-senda/senda/releases/latest"

// BuildInfo returns the version/commit/date stamped into this binary so the UI
// can show it in the status bar.
func (a *App) BuildInfo() buildinfo.Info {
	return buildinfo.Get()
}

// UpdateInfo is the result of an update check.
type UpdateInfo struct {
	Current   string `json:"current"`   // running version, e.g. "1.2.3"
	Latest    string `json:"latest"`    // latest release tag, e.g. "1.3.0"
	Available bool   `json:"available"` // true when Latest is newer than Current
	URL       string `json:"url"`       // releases page to open
}

// CheckUpdate queries the GitHub releases API for the latest tag and reports
// whether it is newer than the running build. Dev builds (Version "dev") never
// report an update since they have no comparable version. Network/parse errors
// are swallowed into Available=false — this is a best-effort hint, not critical.
func (a *App) CheckUpdate(ctx context.Context) UpdateInfo {
	current := buildinfo.Version
	info := UpdateInfo{Current: current, URL: releasesPage}
	if current == "" || current == "dev" {
		return info
	}

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, releasesAPI, nil)
	if err != nil {
		return info
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return info
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return info
	}

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return info
	}

	latest := strings.TrimPrefix(payload.TagName, "v")
	info.Latest = latest
	info.Available = isNewer(current, latest)
	return info
}

// isNewer reports whether latest is a strictly higher semver than current.
// Both are bare versions (no leading "v"); invalid/equal versions yield false.
func isNewer(current, latest string) bool {
	cur := "v" + strings.TrimPrefix(current, "v")
	lat := "v" + strings.TrimPrefix(latest, "v")
	if !semver.IsValid(cur) || !semver.IsValid(lat) {
		return false
	}
	return semver.Compare(lat, cur) > 0
}

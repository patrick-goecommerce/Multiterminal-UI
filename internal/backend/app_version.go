package backend

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Version is the application version. It is set at build time via ldflags:
//
//	-ldflags "-X github.com/patrick-goecommerce/Multiterminal-UI/internal/backend.Version=1.5.0"
//
// When not set, it defaults to "dev".
var Version = "dev"

// UpdateInfo holds the result of a GitHub release check.
type UpdateInfo struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
	DownloadURL     string `json:"downloadURL"`
}

// GetAppVersion returns the current application version string.
func (a *App) GetAppVersion() string {
	return Version
}

// CheckForUpdates queries the GitHub releases API and compares the latest
// release tag with the current version.
func (a *App) CheckForUpdates() UpdateInfo {
	info := UpdateInfo{CurrentVersion: Version}

	if Version == "dev" {
		return info
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/patrick-goecommerce/Multiterminal-UI/releases/latest")
	if err != nil {
		return info
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return info
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return info
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	info.LatestVersion = latest
	info.DownloadURL = release.HTMLURL
	info.UpdateAvailable = isNewerVersion(Version, latest)

	return info
}

// normalizeVersion strips a leading "v" and returns the bare semver string.
func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// isNewerVersion returns true if latest is a higher semver than current.
func isNewerVersion(current, latest string) bool {
	if latest == "" {
		return false
	}
	cur := parseSemver(normalizeVersion(current))
	lat := parseSemver(normalizeVersion(latest))
	if cur == nil || lat == nil {
		return false
	}
	for i := 0; i < 3; i++ {
		if lat[i] > cur[i] {
			return true
		}
		if lat[i] < cur[i] {
			return false
		}
	}
	return false
}

// parseSemver splits "1.2.3" into [1, 2, 3]. Returns nil on failure.
func parseSemver(v string) []int {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return nil
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		nums[i] = n
	}
	return nums
}

// VersionTitle returns the window title including the version.
func VersionTitle() string {
	if Version == "dev" {
		return "Multiterminal UI dev"
	}
	return fmt.Sprintf("Multiterminal UI v%s", normalizeVersion(Version))
}

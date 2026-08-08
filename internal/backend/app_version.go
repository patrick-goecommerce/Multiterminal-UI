package backend

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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
	// AssetName/AssetURL/ChecksumURL are only populated when UpdateAvailable is
	// true; they point at the release asset matching the running build variant
	// (portable exe vs. installer) and its .sha256 sidecar, for ApplyUpdate.
	AssetName   string `json:"assetName"`
	AssetURL    string `json:"assetURL"`
	ChecksumURL string `json:"checksumURL"`
}

// updateAsset is a single release asset from the GitHub releases API, as used
// by the update checker (distinct from stt_install.go's ghAsset, which has a
// different shape and is used for unrelated STT model downloads).
type updateAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// updateRelease is the subset of the GitHub release API response the update
// checker needs (distinct from stt_install.go's ghRelease).
type updateRelease struct {
	TagName    string        `json:"tag_name"`
	HTMLURL    string        `json:"html_url"`
	Prerelease bool          `json:"prerelease"`
	Assets     []updateAsset `json:"assets"`
}

const releasesListURL = "https://api.github.com/repos/patrick-goecommerce/Multiterminal-UI/releases?per_page=10"

// GetAppVersion returns the current application version string.
func (a *AppService) GetAppVersion() string {
	return Version
}

// CheckForUpdates queries the GitHub releases API for the configured update
// channel and compares the latest matching release tag with the current
// version. Errors (network, rate-limit, decode) are swallowed and reported as
// "no update available" — this is a best-effort background check, not a
// critical path.
func (a *AppService) CheckForUpdates() UpdateInfo {
	info := UpdateInfo{CurrentVersion: Version}

	if Version == "dev" {
		return info
	}

	channel := a.cfg.UpdateChannel
	if channel == "" {
		channel = "stable"
	}

	release, err := resolveLatestRelease(channel)
	if err != nil {
		return info
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	info.LatestVersion = latest
	info.DownloadURL = release.HTMLURL
	info.UpdateAvailable = isNewerVersion(Version, latest)

	if info.UpdateAvailable {
		info.AssetName, info.AssetURL, info.ChecksumURL = resolveAsset(release, isPortableBuild())
	}

	return info
}

// resolveLatestRelease lists recent GitHub releases and returns the first one
// matching the requested channel ("stable" = non-prerelease, "alpha" =
// prerelease), mirroring the release.yml / release-alpha.yml split.
func resolveLatestRelease(channel string) (*updateRelease, error) {
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Get(releasesListURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github releases: status %d", resp.StatusCode)
	}

	var releases []updateRelease
	// Cap the response size we're willing to decode; the real payload is a
	// few KB, this just guards against a misbehaving/compromised endpoint.
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&releases); err != nil {
		return nil, err
	}

	wantPrerelease := channel == "alpha"
	for i := range releases {
		if releases[i].Prerelease == wantPrerelease {
			return &releases[i], nil
		}
	}
	return nil, fmt.Errorf("no release found for channel %q", channel)
}

// isPortableBuild reports whether the running exe is the portable variant
// (as opposed to an Inno Setup-installed one), based on the established
// asset-naming convention (mtui-portable-*.exe vs. mtui-setup-*.exe → mtui.exe).
func isPortableBuild() bool {
	exe, err := os.Executable()
	if err != nil {
		return true
	}
	return strings.Contains(strings.ToLower(filepath.Base(exe)), "portable")
}

// resolveAsset finds the release asset matching the running build variant and
// its .sha256 checksum sidecar (if present).
func resolveAsset(release *updateRelease, portable bool) (assetName, assetURL, checksumURL string) {
	prefix := "mtui-setup-"
	if portable {
		prefix = "mtui-portable-"
	}
	for _, asset := range release.Assets {
		if strings.HasPrefix(asset.Name, prefix) && strings.HasSuffix(asset.Name, ".exe") {
			assetName = asset.Name
			assetURL = asset.BrowserDownloadURL
			break
		}
	}
	if assetName == "" {
		return "", "", ""
	}
	for _, asset := range release.Assets {
		if asset.Name == assetName+".sha256" {
			checksumURL = asset.BrowserDownloadURL
			break
		}
	}
	return assetName, assetURL, checksumURL
}

// normalizeVersion strips a leading "v" and returns the bare semver string.
func normalizeVersion(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

// semver is a parsed "major.minor.patch[-label.N]" version. Pre is -1 when
// there is no prerelease suffix (e.g. stable "1.5.0"), otherwise the trailing
// numeric segment of the suffix (e.g. 117 for "2.0.0-alpha.117").
type semver struct {
	core [3]int
	pre  int
}

// isNewerVersion returns true if latest is a higher version than current.
// Cores are compared numerically first; if they tie, a version without a
// prerelease suffix outranks one with one, and higher prerelease numbers
// outrank lower ones (so "alpha.117" > "alpha.5" for equal cores).
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
		if lat.core[i] > cur.core[i] {
			return true
		}
		if lat.core[i] < cur.core[i] {
			return false
		}
	}
	if cur.pre == -1 {
		return false // current is a full release; latest can't outrank an equal core
	}
	if lat.pre == -1 {
		return true // latest is a full release of the same core the current prerelease was for
	}
	return lat.pre > cur.pre
}

// parseSemver parses "1.2.3" or "1.2.3-label.4" into a semver. Returns nil on
// any format it doesn't recognize (missing/non-numeric core segments).
func parseSemver(v string) *semver {
	core := v
	pre := -1

	if idx := strings.Index(v, "-"); idx >= 0 {
		core = v[:idx]
		suffix := v[idx+1:]
		if dot := strings.LastIndex(suffix, "."); dot >= 0 {
			if n, err := strconv.Atoi(suffix[dot+1:]); err == nil {
				pre = n
			}
		}
	}

	parts := strings.SplitN(core, ".", 3)
	if len(parts) != 3 {
		return nil
	}
	var nums [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		nums[i] = n
	}
	return &semver{core: nums, pre: pre}
}

// VersionTitle returns the window title including the version.
func VersionTitle() string {
	if Version == "dev" {
		return "Multiterminal UI dev"
	}
	return fmt.Sprintf("Multiterminal UI v%s", normalizeVersion(Version))
}

package backend

import "testing"

func TestParseSemver_Valid(t *testing.T) {
	v := parseSemver("1.5.0")
	if v == nil {
		t.Fatal("parseSemver(1.5.0) = nil, want parsed")
	}
	if v.core != [3]int{1, 5, 0} || v.pre != -1 {
		t.Errorf("parseSemver(1.5.0) = %+v, want core=[1 5 0] pre=-1", v)
	}
}

func TestParseSemver_PrereleaseSuffix(t *testing.T) {
	v := parseSemver("2.0.0-alpha.117")
	if v == nil {
		t.Fatal("parseSemver(2.0.0-alpha.117) = nil, want parsed")
	}
	if v.core != [3]int{2, 0, 0} {
		t.Errorf("core = %v, want [2 0 0]", v.core)
	}
	if v.pre != 117 {
		t.Errorf("pre = %d, want 117", v.pre)
	}
}

func TestParseSemver_Malformed(t *testing.T) {
	for _, in := range []string{"", "1.6", "nightly", "v1.2.3.4", "a.b.c"} {
		if v := parseSemver(in); v != nil {
			t.Errorf("parseSemver(%q) = %+v, want nil", in, v)
		}
	}
}

func TestIsNewerVersion_StableStable(t *testing.T) {
	tests := []struct {
		current, latest string
		want            bool
	}{
		{"1.5.0", "1.5.1", true},
		{"1.5.0", "1.6.0", true},
		{"1.5.0", "2.0.0", true},
		{"1.5.0", "1.5.0", false},
		{"1.5.1", "1.5.0", false},
		{"1.5.0", "1.4.9", false},
	}
	for _, tt := range tests {
		got := isNewerVersion(tt.current, tt.latest)
		if got != tt.want {
			t.Errorf("isNewerVersion(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
		}
	}
}

func TestIsNewerVersion_AlphaSuffix(t *testing.T) {
	tests := []struct {
		current, latest string
		want            bool
	}{
		{"2.0.0-alpha.5", "2.0.0-alpha.117", true},
		{"2.0.0-alpha.117", "2.0.0-alpha.5", false},
		{"2.0.0-alpha.5", "2.0.0-alpha.5", false},
		{"2.0.0-alpha.5", "2.0.0", true},  // full release outranks a prerelease of the same core
		{"2.0.0", "2.0.0-alpha.5", false}, // prerelease never outranks the release it was for
		{"2.0.0-alpha.5", "2.1.0-alpha.1", true},
	}
	for _, tt := range tests {
		got := isNewerVersion(tt.current, tt.latest)
		if got != tt.want {
			t.Errorf("isNewerVersion(%q, %q) = %v, want %v", tt.current, tt.latest, got, tt.want)
		}
	}
}

func TestIsNewerVersion_MalformedDoesNotPanic(t *testing.T) {
	tests := []struct{ current, latest string }{
		{"1.6", "1.7"},
		{"nightly", "1.0.0"},
		{"1.0.0", "nightly"},
		{"", ""},
	}
	for _, tt := range tests {
		if isNewerVersion(tt.current, tt.latest) {
			t.Errorf("isNewerVersion(%q, %q) = true, want false for malformed input", tt.current, tt.latest)
		}
	}
}

func TestResolveAsset_PicksPortableOrInstalled(t *testing.T) {
	release := &updateRelease{
		Assets: []updateAsset{
			{Name: "mtui-portable-1.5.0.exe", BrowserDownloadURL: "https://example.com/portable.exe"},
			{Name: "mtui-portable-1.5.0.exe.sha256", BrowserDownloadURL: "https://example.com/portable.exe.sha256"},
			{Name: "mtui-setup-1.5.0.exe", BrowserDownloadURL: "https://example.com/setup.exe"},
			{Name: "mtui-setup-1.5.0.exe.sha256", BrowserDownloadURL: "https://example.com/setup.exe.sha256"},
		},
	}

	name, url, checksum := resolveAsset(release, true)
	if name != "mtui-portable-1.5.0.exe" || url != "https://example.com/portable.exe" || checksum != "https://example.com/portable.exe.sha256" {
		t.Errorf("portable resolve = (%q, %q, %q), unexpected", name, url, checksum)
	}

	name, url, checksum = resolveAsset(release, false)
	if name != "mtui-setup-1.5.0.exe" || url != "https://example.com/setup.exe" || checksum != "https://example.com/setup.exe.sha256" {
		t.Errorf("installed resolve = (%q, %q, %q), unexpected", name, url, checksum)
	}
}

func TestResolveAsset_NoMatch(t *testing.T) {
	release := &updateRelease{Assets: []updateAsset{{Name: "other-file.zip", BrowserDownloadURL: "https://example.com/x.zip"}}}
	name, url, checksum := resolveAsset(release, true)
	if name != "" || url != "" || checksum != "" {
		t.Errorf("expected empty resolve for no match, got (%q, %q, %q)", name, url, checksum)
	}
}

func TestResolveAsset_MissingChecksum(t *testing.T) {
	release := &updateRelease{Assets: []updateAsset{{Name: "mtui-portable-1.5.0.exe", BrowserDownloadURL: "https://example.com/portable.exe"}}}
	name, url, checksum := resolveAsset(release, true)
	if name != "mtui-portable-1.5.0.exe" || url == "" {
		t.Fatalf("expected asset to resolve, got (%q, %q)", name, url)
	}
	if checksum != "" {
		t.Errorf("checksum = %q, want empty (no sidecar asset present)", checksum)
	}
}

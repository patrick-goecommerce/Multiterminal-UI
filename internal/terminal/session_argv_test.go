package terminal

import (
	"io/fs"
	"reflect"
	"testing"
)

const testComspec = `C:\Windows\System32\cmd.exe`

// TestWindowsArgv covers the wrap decision: real executables start directly,
// shims and failed lookups keep the COMSPEC wrapper.
func TestWindowsArgv(t *testing.T) {
	notFound := func(string) (string, error) { return "", fs.ErrNotExist }
	resolveTo := func(path string) func(string) (string, error) {
		return func(string) (string, error) { return path, nil }
	}

	tests := []struct {
		name    string
		argv    []string
		comspec string
		resolve func(string) (string, error)
		want    []string
	}{
		{
			name:    "exe starts directly",
			argv:    []string{"ripgrep", "--files"},
			comspec: testComspec,
			resolve: resolveTo(`C:\tools\ripgrep.exe`),
			want:    []string{`C:\tools\ripgrep.exe`, "--files"},
		},
		{
			name:    "uppercase EXE starts directly",
			argv:    []string{"tool"},
			comspec: testComspec,
			resolve: resolveTo(`C:\tools\TOOL.EXE`),
			want:    []string{`C:\tools\TOOL.EXE`},
		},
		{
			name:    "com starts directly",
			argv:    []string{"legacy"},
			comspec: testComspec,
			resolve: resolveTo(`C:\tools\legacy.CoM`),
			want:    []string{`C:\tools\legacy.CoM`},
		},
		{
			name:    "cmd shim goes through COMSPEC",
			argv:    []string{"claude", "--dangerously-skip-permissions"},
			comspec: testComspec,
			resolve: resolveTo(`C:\npm\claude.cmd`),
			want:    []string{testComspec, "/c", "claude", "--dangerously-skip-permissions"},
		},
		{
			name:    "bat shim goes through COMSPEC",
			argv:    []string{"gemini"},
			comspec: testComspec,
			resolve: resolveTo(`C:\npm\gemini.BAT`),
			want:    []string{testComspec, "/c", "gemini"},
		},
		{
			name:    "extensionless target goes through COMSPEC",
			argv:    []string{"script"},
			comspec: testComspec,
			resolve: resolveTo(`C:\tools\script`),
			want:    []string{testComspec, "/c", "script"},
		},
		{
			name:    "lookup failure falls back to COMSPEC",
			argv:    []string{"claude"},
			comspec: testComspec,
			resolve: notFound,
			want:    []string{testComspec, "/c", "claude"},
		},
		{
			name:    "nil resolver falls back to COMSPEC",
			argv:    []string{"claude"},
			comspec: testComspec,
			resolve: nil,
			want:    []string{testComspec, "/c", "claude"},
		},
		{
			name:    "empty COMSPEC uses the system default",
			argv:    []string{"claude"},
			comspec: "",
			resolve: notFound,
			want:    []string{defaultComspec, "/c", "claude"},
		},
		{
			name:    "absolute exe path starts directly",
			argv:    []string{`C:\Program Files\node\node.exe`, "server.js"},
			comspec: testComspec,
			resolve: resolveTo(`C:\Program Files\node\node.exe`),
			want:    []string{`C:\Program Files\node\node.exe`, "server.js"},
		},
		{
			name:    "empty argv is returned untouched",
			argv:    nil,
			comspec: testComspec,
			resolve: resolveTo(`C:\tools\x.exe`),
			want:    nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := append([]string(nil), tc.argv...)
			got := windowsArgv(tc.argv, tc.comspec, tc.resolve)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("windowsArgv() = %q, want %q", got, tc.want)
			}
			// The caller's slice must not be mutated in place.
			if !reflect.DeepEqual(tc.argv, in) {
				t.Errorf("input argv mutated: %q, want %q", tc.argv, in)
			}
		})
	}
}

func TestCanStartDirectly(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{`C:\a\b.exe`, true},
		{`C:\a\b.EXE`, true},
		{`C:\a\b.Com`, true},
		{`C:\a\b.cmd`, false},
		{`C:\a\b.bat`, false},
		{`C:\a\b.ps1`, false},
		{`C:\a\b`, false},
		{`C:\dir.exe\b`, false},
		{"", false},
	}
	for _, tc := range tests {
		if got := canStartDirectly(tc.path); got != tc.want {
			t.Errorf("canStartDirectly(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

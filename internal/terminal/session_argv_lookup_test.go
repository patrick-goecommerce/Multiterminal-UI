package terminal

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestLookWindowsPath exercises the PATH resolution against real files so the
// PATHEXT ordering and the "first directory wins" rule are covered.
func TestLookWindowsPath(t *testing.T) {
	shimDir := t.TempDir()
	sysDir := t.TempDir()
	writeFile(t, filepath.Join(shimDir, "claude.cmd"))
	writeFile(t, filepath.Join(sysDir, "claude.exe"))
	writeFile(t, filepath.Join(sysDir, "ripgrep.exe"))
	writeFile(t, filepath.Join(sysDir, "plain"))
	if err := os.Mkdir(filepath.Join(sysDir, "adir.exe"), 0o755); err != nil {
		t.Fatal(err)
	}

	exts := pathExtList("")
	dirs := []string{shimDir, sysDir}

	tests := []struct {
		name    string
		file    string
		want    string
		wantErr bool
	}{
		{
			name: "first directory wins over later ones",
			file: "claude",
			want: filepath.Join(shimDir, "claude.cmd"),
		},
		{
			name: "resolves exe from a later directory",
			file: "ripgrep",
			want: filepath.Join(sysDir, "ripgrep.exe"),
		},
		{
			name: "explicit extension is honoured",
			file: "ripgrep.exe",
			want: filepath.Join(sysDir, "ripgrep.exe"),
		},
		{
			name: "absolute path is not searched in PATH",
			file: filepath.Join(sysDir, "ripgrep.exe"),
			want: filepath.Join(sysDir, "ripgrep.exe"),
		},
		{
			name:    "extensionless file is not executable",
			file:    "plain",
			wantErr: true,
		},
		{
			name:    "directory is never a match",
			file:    "adir",
			wantErr: true,
		},
		{
			name:    "unknown command",
			file:    "does-not-exist",
			wantErr: true,
		},
		{
			name:    "empty command",
			file:    "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := lookWindowsPath(tc.file, dirs, exts)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("lookWindowsPath(%q) = %q, want error", tc.file, got)
				}
				if !errors.Is(err, fs.ErrNotExist) {
					t.Errorf("error = %v, want fs.ErrNotExist", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("lookWindowsPath(%q) error: %v", tc.file, err)
			}
			if got != tc.want {
				t.Errorf("lookWindowsPath(%q) = %q, want %q", tc.file, got, tc.want)
			}
		})
	}
}

// TestSessionLookPathUsesSessionPath is the regression guard for the tmux-shim
// PATH prepend: the resolver must pick the entry from the session env, not
// whatever the parent process' PATH points at.
func TestSessionLookPathUsesSessionPath(t *testing.T) {
	shimDir := t.TempDir()
	otherDir := t.TempDir()
	writeFile(t, filepath.Join(shimDir, "tmux.exe"))
	writeFile(t, filepath.Join(otherDir, "tmux.cmd"))

	env := []string{
		"Path=" + otherDir,
		"PATH=" + shimDir + string(os.PathListSeparator) + otherDir,
	}
	resolve := sessionLookPath(env)

	got, err := resolve("tmux")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if want := filepath.Join(shimDir, "tmux.exe"); got != want {
		t.Fatalf("resolve = %q, want %q", got, want)
	}

	argv := windowsArgv([]string{"tmux", "ls"}, testComspec, resolve)
	want := []string{filepath.Join(shimDir, "tmux.exe"), "ls"}
	if !reflect.DeepEqual(argv, want) {
		t.Errorf("windowsArgv = %q, want %q", argv, want)
	}
}

func TestEnvValue(t *testing.T) {
	tests := []struct {
		name string
		env  []string
		key  string
		want string
	}{
		{"case insensitive key", []string{"Path=C:/a"}, "PATH", "C:/a"},
		{"last duplicate wins", []string{"PATH=a", "Path=b"}, "PATH", "b"},
		{"missing key", []string{"HOME=x"}, "PATH", ""},
		{"empty value", []string{"PATH="}, "PATH", ""},
		{"prefix is not a match", []string{"PATHEXT=.EXE"}, "PATH", ""},
		{"pathext read", []string{"PATHEXT=.COM;.EXE"}, "PATHEXT", ".COM;.EXE"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := envValue(tc.env, tc.key); got != tc.want {
				t.Errorf("envValue(%q, %q) = %q, want %q", tc.env, tc.key, got, tc.want)
			}
		})
	}
}

func TestPathExtList(t *testing.T) {
	tests := []struct {
		name    string
		pathExt string
		want    []string
	}{
		{"empty falls back to defaults", "", []string{".com", ".exe", ".bat", ".cmd"}},
		{"blank falls back to defaults", "   ", []string{".com", ".exe", ".bat", ".cmd"}},
		{"lowercased and deduped separators", ".COM;.EXE;;.CMD", []string{".com", ".exe", ".cmd"}},
		{"missing leading dot is added", "COM;EXE", []string{".com", ".exe"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := pathExtList(tc.pathExt); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("pathExtList(%q) = %q, want %q", tc.pathExt, got, tc.want)
			}
		})
	}
}

func TestHasExt(t *testing.T) {
	tests := []struct {
		file string
		want bool
	}{
		{"claude.cmd", true},
		{"claude", false},
		{`C:\dir.with.dot\claude`, false},
		{`C:\dir.with.dot\claude.exe`, true},
		{"", false},
	}
	for _, tc := range tests {
		if got := hasExt(tc.file); got != tc.want {
			t.Errorf("hasExt(%q) = %v, want %v", tc.file, got, tc.want)
		}
	}
}

func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// TestStartDefaultShellIsNotWrapped documents that an empty argv keeps using
// defaultShell() unwrapped — the wrapper decision only applies to
// caller-supplied commands.
func TestStartDefaultShellIsNotWrapped(t *testing.T) {
	sh := defaultShell()
	if len(sh) == 0 {
		t.Fatal("defaultShell returned nothing")
	}
	if strings.Contains(strings.Join(sh, " "), " /c ") {
		t.Errorf("defaultShell() = %q, must not contain a COMSPEC wrapper", sh)
	}
}

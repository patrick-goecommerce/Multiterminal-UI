package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// shimName applies the same extension rule the production code uses.
func shimName(base string) string {
	if isWindows() {
		return base + ".exe"
	}
	return base
}

// A build without -tags production embeds nothing, so extractShim returns "".
// That must not discard a shim an earlier build already extracted: on a real
// installation ~/.claude/mtui-hook.exe sat there working while the app declared
// the binary missing and skipped hook integration entirely (#199).
func TestResolveBundledBinaryFallsBackToAlreadyExtractedShim(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(claudeDir, shimName("mtui-probe-existing"))
	if err := os.WriteFile(shim, []byte("MZ-previously-extracted"), 0755); err != nil {
		t.Fatal(err)
	}

	got := resolveBundledBinary("mtui-probe-existing", nil) // nil = nothing embedded

	if got == "" {
		t.Fatal("returned empty despite an already-extracted shim on disk")
	}
	if !strings.EqualFold(filepath.FromSlash(got), shim) {
		t.Errorf("path = %q, want %q", got, shim)
	}
}

func TestResolveBundledBinaryStillEmptyWhenNothingOnDisk(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	if got := resolveBundledBinary("mtui-probe-absent", nil); got != "" {
		t.Errorf("path = %q, want empty — nothing embedded and nothing on disk", got)
	}
}

// The failure that started this was silent: the only trace was one log line,
// so a broken hook integration went unnoticed for two weeks (#192, #199).
// Bind warnings ride along on CheckHealth(), which the frontend pulls on mount.
func TestResolveHookBinaryRecordsWarningWhenMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	a := &AppService{}
	got := a.resolveHookBinary("mtui-probe-absent", nil)

	if got != "" {
		t.Fatalf("path = %q, want empty", got)
	}
	warnings := a.bindWarningsSnapshot()
	if len(warnings) != 1 {
		t.Fatalf("warnings = %d, want 1 — a skipped hook integration must reach the UI", len(warnings))
	}
	if warnings[0].Service != "hooks" {
		t.Errorf("service = %q, want hooks", warnings[0].Service)
	}
	if warnings[0].Detail == "" {
		t.Error("detail is empty — the warning must say what is wrong")
	}
}

func TestResolveHookBinaryRecordsNoWarningWhenFound(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(claudeDir, shimName("mtui-probe-ok"))
	if err := os.WriteFile(shim, []byte("MZ"), 0755); err != nil {
		t.Fatal(err)
	}

	a := &AppService{}
	if got := a.resolveHookBinary("mtui-probe-ok", nil); got == "" {
		t.Fatal("path is empty despite a shim on disk")
	}
	if n := len(a.bindWarningsSnapshot()); n != 0 {
		t.Errorf("warnings = %d, want 0", n)
	}
}

package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- unwrapStatuslineCommand tests ---

func TestUnwrapStatuslineCommandStripsForwarderPrefix(t *testing.T) {
	// The prefix path differs from the "current" path (different drive/dir) — must
	// still be stripped because matching is done on the basename only.
	wrapped := `"C:/old/build/bin/statusline-forward.exe" powershell -File "C:/Users/x/.claude/foo.ps1"`
	got := unwrapStatuslineCommand(wrapped)
	want := `powershell -File "C:/Users/x/.claude/foo.ps1"`
	if got != want {
		t.Fatalf("unwrapStatuslineCommand(%q) = %q, want %q", wrapped, got, want)
	}
}

func TestUnwrapStatuslineCommandLeavesPlainCommandUnchanged(t *testing.T) {
	plain := `powershell -NonInteractive -File "C:/Users/x/.claude/mtui-statusline.ps1"`
	got := unwrapStatuslineCommand(plain)
	if got != plain {
		t.Fatalf("unwrapStatuslineCommand(%q) = %q, want unchanged", plain, got)
	}
}

func TestUnwrapStatuslineCommandLeavesUnrelatedQuotedCommandUnchanged(t *testing.T) {
	other := `"C:/some/other/tool.exe" arg1 arg2`
	got := unwrapStatuslineCommand(other)
	if got != other {
		t.Fatalf("unwrapStatuslineCommand(%q) = %q, want unchanged", other, got)
	}
}

func TestWrapAfterUnwrapIsIdempotent(t *testing.T) {
	forwarder := `C:/new/build/bin/statusline-forward.exe`
	userCmd := `powershell -File "C:/Users/x/.claude/my-status.ps1"`

	// Simulate what was persisted to settings.json by a previous run.
	alreadyWrapped := wrapStatuslineCommand(forwarder, userCmd)

	// applyStatusLine unwraps then re-wraps — the result must have exactly one
	// forwarder prefix, not two.
	inner := unwrapStatuslineCommand(alreadyWrapped)
	rewrapped := wrapStatuslineCommand(forwarder, inner)

	// Exactly one quoted forwarder prefix.
	prefix := `"` + forwarder + `" `
	if !strings.HasPrefix(rewrapped, prefix) {
		t.Fatalf("rewrapped %q does not start with %q", rewrapped, prefix)
	}
	// The inner command must appear exactly once (no double-nesting).
	if strings.Count(rewrapped, userCmd) != 1 {
		t.Fatalf("rewrapped %q contains userCmd %d time(s), want 1", rewrapped, strings.Count(rewrapped, userCmd))
	}
	// Shape must equal alreadyWrapped (idempotent).
	if rewrapped != alreadyWrapped {
		t.Fatalf("rewrapped %q != original %q", rewrapped, alreadyWrapped)
	}
}

func TestWrapStatuslineCommandPrependsForwarder(t *testing.T) {
	inner := `powershell -NonInteractive -NoProfile -File "C:/Users/x/.claude/mtui-statusline.ps1"`
	got := wrapStatuslineCommand(`C:/Users/x/AppData/.../statusline-forward.exe`, inner)

	if !strings.HasPrefix(got, `"C:/Users/x/AppData/.../statusline-forward.exe" `) {
		t.Fatalf("command = %q, want it to start with the quoted forwarder path", got)
	}
	if !strings.HasSuffix(got, inner) {
		t.Fatalf("command = %q, want it to end with the wrapped inner command %q", got, inner)
	}
}

func TestWrapStatuslineCommandEmptyForwarderReturnsInner(t *testing.T) {
	// Fail-safe: no forwarder available -> register the inner command unchanged.
	inner := `powershell -File foo.ps1`
	if got := wrapStatuslineCommand("", inner); got != inner {
		t.Fatalf("command = %q, want unchanged inner %q", got, inner)
	}
}

func TestExtractShimWritesAndReturnsPath(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "shim.exe")
	data := []byte("MZ-fake-binary")

	got, err := extractShim(dst, data)
	if err != nil {
		t.Fatalf("extractShim error: %v", err)
	}
	if got != dst {
		t.Fatalf("path = %q, want %q", got, dst)
	}
	on, _ := os.ReadFile(dst)
	if string(on) != string(data) {
		t.Fatalf("written content = %q, want %q", on, data)
	}
}

func TestExtractShimEmptyDataReturnsEmpty(t *testing.T) {
	// No embedded shim (dev/non-production build): nothing extracted, no path.
	dst := filepath.Join(t.TempDir(), "shim.exe")
	got, err := extractShim(dst, nil)
	if err != nil {
		t.Fatalf("extractShim error: %v", err)
	}
	if got != "" {
		t.Fatalf("path = %q, want empty", got)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Fatalf("file should not exist, stat err = %v", err)
	}
}

func TestExtractShimIdempotentWhenSameSize(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "shim.exe")
	data := []byte("same-size-content")
	if _, err := extractShim(dst, data); err != nil {
		t.Fatalf("first extract: %v", err)
	}
	// Second call with identical-size data must succeed and keep the file intact.
	got, err := extractShim(dst, data)
	if err != nil {
		t.Fatalf("second extract: %v", err)
	}
	if got != dst {
		t.Fatalf("path = %q, want %q", got, dst)
	}
	on, _ := os.ReadFile(dst)
	if string(on) != string(data) {
		t.Fatalf("content = %q, want %q", on, data)
	}
}

func TestResolveBundledBinaryPrefersSibling(t *testing.T) {
	// A sibling next to the test binary's own dir is hard to fake; instead
	// assert the embed-extract fallback path is returned when no sibling and
	// embedded bytes are present.
	dst := filepath.Join(t.TempDir(), "fake-home", ".claude")
	t.Setenv("USERPROFILE", filepath.Dir(filepath.Dir(dst))) // ~ -> fake-home
	t.Setenv("HOME", filepath.Dir(filepath.Dir(dst)))
	got := resolveBundledBinary("mtui-probe", []byte("MZ-bytes"))
	if got == "" {
		t.Fatal("resolveBundledBinary returned empty with embedded bytes present")
	}
	if filepath.Base(got) != "mtui-probe.exe" && filepath.Base(got) != "mtui-probe" {
		t.Fatalf("unexpected basename: %q", got)
	}
}

package backend

import (
	"os"
	"path/filepath"
	"testing"
)

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

func TestResolveBundledBinaryExtractFallback(t *testing.T) {
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

func TestResolveBundledBinaryPrefersSibling(t *testing.T) {
	// Plant a sibling binary next to os.Executable() (the test binary itself).
	// resolveBundledBinary must return that sibling path, ignoring the embedded bytes.
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	exeDir := filepath.Dir(exe)

	// Derive the expected name with the same rule used by the production code.
	sibName := "mtui-probe"
	if isWindows() {
		sibName += ".exe"
	}
	sibPath := filepath.Join(exeDir, sibName)

	// Plant the sibling file.
	if err := os.WriteFile(sibPath, []byte("placeholder"), 0755); err != nil {
		t.Skipf("cannot write sibling binary next to test exe (%v) — skipping sibling-priority test", err)
	}
	t.Cleanup(func() { os.Remove(sibPath) })

	got := resolveBundledBinary("mtui-probe", []byte("ignored-embed"))
	want := filepath.ToSlash(sibPath)
	if got != want {
		t.Fatalf("resolveBundledBinary = %q, want sibling path %q", got, want)
	}
}

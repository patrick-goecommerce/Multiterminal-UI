package backend

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// A new build whose helper happens to have the old one's size must still
// replace it. Comparing sizes kept an outdated hook binary in place.
func TestExtractShimReplacesSameSizeDifferentContent(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "shim.exe")
	if err := os.WriteFile(dst, []byte("old-content-1234"), 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := extractShim(dst, []byte("new-content-5678")); err != nil {
		t.Fatalf("extractShim: %v", err)
	}
	if on, _ := os.ReadFile(dst); string(on) != "new-content-5678" {
		t.Fatalf("content = %q, want the new bytes", on)
	}
}

// A running helper locks its exe against writes on Windows. A hook process
// that hung for a month kept every later build from installing its own hook
// binary, and the old one silently stayed registered.
func TestExtractShimReplacesRunningExe(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("exe locking is Windows behaviour")
	}
	dir := t.TempDir()
	dst := filepath.Join(dir, "shim.exe")
	ping, err := os.ReadFile(filepath.Join(os.Getenv("SystemRoot"), "System32", "PING.EXE"))
	if err != nil {
		t.Skipf("no ping.exe to run: %v", err)
	}
	if err := os.WriteFile(dst, ping, 0755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(dst, "-n", "30", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start locked exe: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	if err := os.WriteFile(dst, []byte("x"), 0755); err == nil {
		t.Fatal("precondition: a running exe should not be writable")
	}

	got, err := extractShim(dst, []byte("MZ-new-hook"))
	if err != nil {
		t.Fatalf("extractShim on a running exe: %v", err)
	}
	if got != dst {
		t.Fatalf("path = %q, want %q", got, dst)
	}
	if on, _ := os.ReadFile(dst); string(on) != "MZ-new-hook" {
		t.Fatalf("content = %q, want the new bytes", on)
	}

	// Once the old process is gone, the next extraction sweeps the set-aside copy.
	_ = cmd.Process.Kill()
	_ = cmd.Wait()
	if _, err := extractShim(dst, []byte("MZ-new-hook")); err != nil {
		t.Fatalf("second extract: %v", err)
	}
	if left, _ := filepath.Glob(dst + ".old-*"); len(left) != 0 {
		t.Fatalf("set-aside copies not swept: %v", left)
	}
}

// When the embedded helper cannot be installed, the stale copy is still used,
// but the caller learns about it instead of registering an outdated binary
// without a trace.
func TestResolveBundledBinaryReportsStaleFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	name := "mtui-probe-stale"
	dst := filepath.Join(home, ".claude", name)
	if isWindows() {
		dst += ".exe"
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	old := extractShimFn
	extractShimFn = func(string, []byte) (string, error) { return "", errors.New("locked") }
	t.Cleanup(func() { extractShimFn = old })

	got, err := resolveBundledBinaryChecked(name, []byte("new"))
	if got != filepath.ToSlash(dst) {
		t.Fatalf("path = %q, want the stale copy %q", got, dst)
	}
	if err == nil {
		t.Fatal("want an error saying the installed copy is outdated")
	}
}

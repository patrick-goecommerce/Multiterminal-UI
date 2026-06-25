package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

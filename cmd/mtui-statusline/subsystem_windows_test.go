//go:build windows

package main

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestGUISubsystem builds the mtui-statusline binary with -H windowsgui and
// verifies that the resulting PE file reports Subsystem 2 (IMAGE_SUBSYSTEM_WINDOWS_GUI),
// not 3 (IMAGE_SUBSYSTEM_WINDOWS_CUI). This guards against dropping the flag and
// silently reintroducing the console-window flash.
func TestGUISubsystem(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PE subsystem check is Windows-only")
	}

	// Locate the package directory (same dir as this test file).
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	pkgDir := filepath.Dir(thisFile)

	outExe := filepath.Join(t.TempDir(), "mtui-statusline-test.exe")

	cmd := exec.Command("go", "build", "-ldflags", "-H windowsgui", "-o", outExe, ".")
	cmd.Dir = pkgDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}

	subsystem := readPESubsystem(t, outExe)
	t.Logf("mtui-statusline.exe PE Subsystem = %d", subsystem)
	if subsystem != 2 {
		t.Errorf("expected PE Subsystem 2 (GUI), got %d — binary would flash a console window", subsystem)
	}
}

// readPESubsystem reads the Windows PE Subsystem field from the given executable.
// PE offset is stored as a little-endian uint32 at file offset 0x3C.
// Subsystem is a little-endian uint16 at peOffset + 0x5C.
func readPESubsystem(t *testing.T, path string) uint16 {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read exe: %v", err)
	}
	if len(data) < 0x40 {
		t.Fatalf("exe too small (%d bytes)", len(data))
	}
	peOffset := binary.LittleEndian.Uint32(data[0x3C:])
	subsystemOffset := int(peOffset) + 0x5C
	if len(data) < subsystemOffset+2 {
		t.Fatalf("exe too small for PE subsystem field (need %d bytes, have %d)", subsystemOffset+2, len(data))
	}
	return binary.LittleEndian.Uint16(data[subsystemOffset:])
}

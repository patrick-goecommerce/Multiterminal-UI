package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyChecksum_Match(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "file.exe")
	checksumPath := filepath.Join(dir, "file.exe.sha256")

	if err := os.WriteFile(filePath, []byte("hello world"), 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	// sha256("hello world")
	const expected = "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if err := os.WriteFile(checksumPath, []byte(expected+"  file.exe\n"), 0600); err != nil {
		t.Fatalf("write checksum: %v", err)
	}

	if err := verifyChecksum(filePath, checksumPath); err != nil {
		t.Errorf("verifyChecksum() = %v, want nil", err)
	}
}

func TestVerifyChecksum_Mismatch(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "file.exe")
	checksumPath := filepath.Join(dir, "file.exe.sha256")

	os.WriteFile(filePath, []byte("hello world"), 0600)
	os.WriteFile(checksumPath, []byte("deadbeef  file.exe\n"), 0600)

	if err := verifyChecksum(filePath, checksumPath); err == nil {
		t.Error("verifyChecksum() = nil, want mismatch error")
	}
}

func TestVerifyChecksum_EmptyChecksumFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "file.exe")
	checksumPath := filepath.Join(dir, "file.exe.sha256")

	os.WriteFile(filePath, []byte("hello world"), 0600)
	os.WriteFile(checksumPath, []byte(""), 0600)

	if err := verifyChecksum(filePath, checksumPath); err == nil {
		t.Error("verifyChecksum() with empty checksum file = nil, want error")
	}
}

func TestPSQuote_EscapesSingleQuotes(t *testing.T) {
	got := psQuote(`C:\Users\O'Brien\mtui.exe`)
	want := `'C:\Users\O''Brien\mtui.exe'`
	if got != want {
		t.Errorf("psQuote() = %q, want %q", got, want)
	}
}

func TestBuildUpdateScript_PortableContainsRollback(t *testing.T) {
	script := buildUpdateScript(1234, `C:\temp\new.exe`, `C:\temp\new.exe.sha256`, `C:\app\mtui-portable.exe`, `C:\temp\apply.log`, true)

	for _, want := range []string{
		"Wait-Process -Id 1234",
		"Get-FileHash",
		"Copy-Item -Path 'C:\\app\\mtui-portable.exe' -Destination 'C:\\app\\mtui-portable.exe.bak'",
		"rolling back",
		"Remove-Item -Path $MyInvocation.MyCommand.Path",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("portable script missing expected fragment: %q", want)
		}
	}
}

func TestBuildUpdateScript_InstalledUsesSilentFlags(t *testing.T) {
	script := buildUpdateScript(1234, `C:\temp\setup.exe`, `C:\temp\setup.exe.sha256`, `C:\Program Files\Multiterminal UI\mtui.exe`, `C:\temp\apply.log`, false)

	for _, want := range []string{
		"/VERYSILENT", "/NORESTART", "/SUPPRESSMSGBOXES",
		"Wait-Process -Id 1234",
	} {
		if !strings.Contains(script, want) {
			t.Errorf("installed script missing expected fragment: %q", want)
		}
	}
	if strings.Contains(script, ".bak") {
		t.Error("installed script should not attempt an exe backup/rollback (no cached previous installer)")
	}
}

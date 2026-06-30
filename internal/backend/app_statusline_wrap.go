package backend

import (
	"os"
	"path/filepath"
)

// siblingBinaryPath returns the path to name(.exe) next to the running exe, or "".
func siblingBinaryPath(name string) string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if filepath.Ext(exe) == ".exe" {
		name += ".exe"
	}
	p := filepath.Join(filepath.Dir(exe), name)
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return filepath.ToSlash(p)
}

// resolveBundledBinary resolves a bundled helper binary: a sibling of the running
// exe (dev / E2E), else the embedded bytes extracted to ~/.claude (production).
// Returns "" if neither is available (caller must fail safe).
func resolveBundledBinary(name string, embedded []byte) string {
	if p := siblingBinaryPath(name); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	ext := ""
	if isWindows() {
		ext = ".exe"
	}
	dst := filepath.Join(home, ".claude", name+ext)
	p, err := extractShim(dst, embedded)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(p)
}

// extractShim writes the embedded shim bytes to dst, returning the path to the
// extracted file. It is a no-op (returns "") when data is empty — the non-
// production build embeds nothing, so the sibling path is used instead. If a file
// of the same size already exists it is left untouched (avoids rewriting / a
// sharing violation on a shim that may be mid-invocation on Windows).
func extractShim(dst string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	if fi, err := os.Stat(dst); err == nil && fi.Size() == int64(len(data)) {
		return dst, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, data, 0755); err != nil {
		return "", err
	}
	return dst, nil
}

package backend

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
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

// extractShimFn is extractShim, replaceable in tests.
var extractShimFn = extractShim

// resolveBundledBinary is resolveBundledBinaryChecked for callers that only
// need the path.
func resolveBundledBinary(name string, embedded []byte) string {
	p, _ := resolveBundledBinaryChecked(name, embedded)
	return p
}

// resolveBundledBinaryChecked resolves a bundled helper binary, in order: a
// sibling of the running exe (dev / E2E), the embedded bytes extracted to
// ~/.claude (production), or a shim an earlier build already extracted there.
//
// That last step matters because extractShim yields nothing when the build
// embeds nothing. Without it an installed build that was produced without
// -tags production declared the helper missing even though a working
// ~/.claude/mtui-hook.exe sat right there, and skipped hook integration for
// good (#199). Returns "" if none of the three is available.
//
// The error is set when this build embeds a helper but could not install it,
// so the returned path is an earlier build's copy. It still works, but it
// lacks whatever the new build added: a hook binary from August that no build
// could replace was how "Stop" lost its message and every question read as
// "done". Callers must surface it.
func resolveBundledBinaryChecked(name string, embedded []byte) (string, error) {
	if p := siblingBinaryPath(name); p != "" {
		return p, nil
	}
	home, _ := os.UserHomeDir()
	ext := ""
	if isWindows() {
		ext = ".exe"
	}
	dst := filepath.Join(home, ".claude", name+ext)
	p, extractErr := extractShimFn(dst, embedded)
	if extractErr == nil && p != "" {
		return filepath.ToSlash(p), nil
	}
	if fi, err := os.Stat(dst); err == nil && !fi.IsDir() && fi.Size() > 0 {
		if extractErr != nil {
			return filepath.ToSlash(dst), fmt.Errorf("update failed, using the old copy: %w", extractErr)
		}
		return filepath.ToSlash(dst), nil
	}
	return "", extractErr
}

// extractShim writes the embedded shim bytes to dst, returning the path to the
// extracted file. It is a no-op (returns "") when data is empty — the non-
// production build embeds nothing, so the sibling path is used instead.
//
// A file with the same content is left untouched, so an unchanged helper is
// never rewritten while it may be mid-invocation. The comparison is by
// content: by size alone, a new build whose helper happened to keep the old
// size would never be installed.
//
// Windows refuses to overwrite an exe that is running, and a hook process
// that hangs keeps it running for good. It does allow renaming it, so a
// locked file is moved aside and the new one takes its name. The set-aside
// copies are swept on the next call, once their process is gone.
func extractShim(dst string, data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	sweepSetAside(dst)
	if on, err := os.ReadFile(dst); err == nil && bytes.Equal(on, data) {
		return dst, nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return "", err
	}
	tmp := dst + ".new"
	if err := os.WriteFile(tmp, data, 0755); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dst); err == nil {
		return dst, nil
	}
	aside := dst + ".old-" + strconv.FormatInt(time.Now().UnixNano(), 36)
	if err := os.Rename(dst, aside); err != nil {
		os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Rename(aside, dst)
		os.Remove(tmp)
		return "", err
	}
	return dst, nil
}

// sweepSetAside removes copies extractShim moved aside earlier. One whose
// process still runs stays locked and is tried again next time.
func sweepSetAside(dst string) {
	old, _ := filepath.Glob(dst + ".old-*")
	for _, p := range old {
		_ = os.Remove(p)
	}
}

package backend

import (
	"os"
	"path/filepath"
	"strings"
)

// statuslineForwardSiblingPath returns the path to a statusline-forward shim
// sitting next to the running binary (the dev / E2E build layout where both land
// in build/bin/). Returns "" if no sibling shim exists.
func statuslineForwardSiblingPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	name := "statusline-forward"
	if filepath.Ext(exe) == ".exe" {
		name += ".exe"
	}
	p := filepath.Join(filepath.Dir(exe), name)
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return filepath.ToSlash(p)
}

// statuslineForwardExtractPath returns the writable location the embedded shim is
// extracted to for production (single-portable-exe) builds: alongside the
// statusline script in the user's ~/.claude dir, which MTUI already writes to.
func statuslineForwardExtractPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "mtui-statusline-forward.exe")
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

// ensureStatuslineForward resolves the statusline-forward shim path, preferring a
// sibling binary (dev/E2E) and falling back to extracting the embedded shim
// (production). Returns "" when no shim is available, in which case the
// statusline is registered unwrapped (fail-safe).
func ensureStatuslineForward() string {
	if p := statuslineForwardSiblingPath(); p != "" {
		return p
	}
	p, err := extractShim(statuslineForwardExtractPath(), shimBin)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(p)
}

// wrapStatuslineCommand builds the statusLine.command string: the quoted
// forwarder followed by the inner (real) statusline command it wraps. If
// forwarder is "", the inner command is returned unchanged (fail-safe).
func wrapStatuslineCommand(forwarder, inner string) string {
	if forwarder == "" {
		return inner
	}
	return `"` + forwarder + `" ` + inner
}

// unwrapStatuslineCommand strips a forwarder prefix from cmd if the first
// quoted token's basename is "statusline-forward" or "statusline-forward.exe".
// This makes re-wrapping idempotent: applyStatusLine can call it on an
// existing user statusline command without accumulating nested shim prefixes
// across restarts or settings saves. The match is done on the basename alone
// so it is robust to path differences between dev (sibling) and production
// (extracted temp dir) runs.
func unwrapStatuslineCommand(cmd string) string {
	if len(cmd) == 0 || cmd[0] != '"' {
		return cmd
	}
	// Find the closing quote of the first token.
	end := strings.Index(cmd[1:], `"`)
	if end < 0 {
		return cmd
	}
	firstToken := cmd[1 : end+1] // path inside the quotes
	base := strings.ToLower(filepath.Base(firstToken))
	if base != "statusline-forward.exe" && base != "statusline-forward" {
		return cmd
	}
	// Strip the quoted token and the single space that follows it.
	rest := cmd[end+2:] // skip closing quote
	if strings.HasPrefix(rest, " ") {
		rest = rest[1:]
	}
	return rest
}

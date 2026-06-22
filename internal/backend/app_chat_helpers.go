package backend

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"runtime"
)

// wrapClaudeCmd wraps the claude path with COMSPEC on Windows (.cmd shim),
// otherwise runs it directly.
func wrapClaudeCmd(path string, args []string) *exec.Cmd {
	return wrapClaudeCmdContext(context.Background(), path, args)
}

// wrapClaudeCmdContext is wrapClaudeCmd bound to a context, so the caller can
// enforce a timeout / cancellation on the spawned process.
func wrapClaudeCmdContext(ctx context.Context, path string, args []string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		comspec := os.Getenv("COMSPEC")
		if comspec == "" {
			comspec = `C:\Windows\System32\cmd.exe`
		}
		full := append([]string{"/c", path}, args...)
		return exec.CommandContext(ctx, comspec, full...)
	}
	return exec.CommandContext(ctx, path, args...)
}

// jsonQuote returns a JSON-quoted string literal (with surrounding quotes).
func jsonQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

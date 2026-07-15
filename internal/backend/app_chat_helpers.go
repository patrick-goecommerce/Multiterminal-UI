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
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		comspec := os.Getenv("COMSPEC")
		if comspec == "" {
			comspec = `C:\Windows\System32\cmd.exe`
		}
		full := append([]string{"/c", path}, args...)
		cmd = exec.CommandContext(ctx, comspec, full...)
	} else {
		cmd = exec.CommandContext(ctx, path, args...)
	}
	// Suppress the console window Windows would otherwise allocate for the
	// cmd.exe/claude child of this GUI app — chat sessions and pane-name
	// generation spawn claude outside the PTY. No-op on non-Windows; same
	// pattern every git/gh spawn in this package uses.
	hideConsole(cmd)
	return cmd
}

// jsonQuote returns a JSON-quoted string literal (with surrounding quotes).
func jsonQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

package backend

import (
	"encoding/json"
	"os"
	"os/exec"
	"runtime"
)

// wrapClaudeCmd wraps the claude path with COMSPEC on Windows (.cmd shim),
// otherwise runs it directly.
func wrapClaudeCmd(path string, args []string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		comspec := os.Getenv("COMSPEC")
		if comspec == "" {
			comspec = `C:\Windows\System32\cmd.exe`
		}
		full := append([]string{"/c", path}, args...)
		return exec.Command(comspec, full...)
	}
	return exec.Command(path, args...)
}

// jsonQuote returns a JSON-quoted string literal (with surrounding quotes).
func jsonQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

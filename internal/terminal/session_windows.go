//go:build windows

package terminal

import gopty "github.com/aymanbagabas/go-pty"

// hidePTYConsole is intentionally a no-op on Windows.
// ConPTY already creates a pseudo-console for the child process; setting
// CREATE_NO_WINDOW would prevent the process from attaching to it and
// break terminal I/O entirely.
func hidePTYConsole(_ *gopty.Cmd) {}

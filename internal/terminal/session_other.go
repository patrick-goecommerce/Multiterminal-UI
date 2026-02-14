//go:build !windows

package terminal

import gopty "github.com/aymanbagabas/go-pty"

// hidePTYConsole is a no-op on non-Windows platforms.
func hidePTYConsole(_ *gopty.Cmd) {}

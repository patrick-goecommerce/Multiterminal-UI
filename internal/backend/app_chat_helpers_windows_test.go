//go:build windows

package backend

import "testing"

// TestWrapClaudeCmdHidesConsoleWindow guards that claude subprocesses spawned
// outside the PTY (chat sessions, pane-name generation) do not flash a console
// window on Windows. The builder must apply hideConsole (CREATE_NO_WINDOW).
func TestWrapClaudeCmdHidesConsoleWindow(t *testing.T) {
	cmd := wrapClaudeCmd("claude", []string{"-p"})
	if cmd.SysProcAttr == nil {
		t.Fatal("SysProcAttr is nil; expected CREATE_NO_WINDOW to be set")
	}
	const createNoWindow = 0x08000000
	if cmd.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatalf("CreationFlags=0x%x; CREATE_NO_WINDOW (0x%x) not set",
			cmd.SysProcAttr.CreationFlags, createNoWindow)
	}
}

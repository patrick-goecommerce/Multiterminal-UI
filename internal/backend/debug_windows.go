//go:build windows

package backend

import (
	"os"
	"syscall"
)

// negHandle converts a negative Windows handle constant (e.g. -11) to uintptr.
func negHandle(h int) uintptr { return uintptr(uint32(int32(h))) }

// allocDebugConsole opens a new console window on Windows so that
// log output written to stderr is visible for GUI applications.
func allocDebugConsole() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("AllocConsole")
	proc.Call() //nolint:errcheck
}

// redirectStdStreams reconnects os.Stdout and os.Stderr to the
// newly allocated console so that fmt/log output is visible.
func redirectStdStreams() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getStdHandle := kernel32.NewProc("GetStdHandle")

	hOut, _, _ := getStdHandle.Call(negHandle(syscall.STD_OUTPUT_HANDLE))
	hErr, _, _ := getStdHandle.Call(negHandle(syscall.STD_ERROR_HANDLE))

	os.Stdout = os.NewFile(hOut, "CONOUT$")
	os.Stderr = os.NewFile(hErr, "CONOUT$")
}

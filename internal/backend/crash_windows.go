//go:build windows

// Redirect the process's stderr (fd 2 / STD_ERROR_HANDLE) to the log file so
// Go runtime panic traces — which are written straight to fd 2 and bypass the
// log package — are captured. Without this a panic in a GUI-subsystem binary
// (no attached console) vanishes silently and the app just disappears.
package backend

import (
	"os"

	"golang.org/x/sys/windows"
)

func redirectStderrToFile(f *os.File) {
	_ = windows.SetStdHandle(windows.STD_ERROR_HANDLE, windows.Handle(f.Fd()))
	os.Stderr = f
}

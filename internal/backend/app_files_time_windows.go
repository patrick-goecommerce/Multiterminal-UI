//go:build windows

package backend

import (
	"os"
	"syscall"
	"time"
)

// fileCreationTime returns the file creation time on Windows.
func fileCreationTime(info os.FileInfo) time.Time {
	if sys, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return time.Unix(0, sys.CreationTime.Nanoseconds())
	}
	return info.ModTime()
}

func isWindows() bool { return true }

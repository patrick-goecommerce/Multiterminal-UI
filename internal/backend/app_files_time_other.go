//go:build !windows

package backend

import (
	"os"
	"time"
)

// fileCreationTime falls back to ModTime on non-Windows platforms.
func fileCreationTime(info os.FileInfo) time.Time {
	return info.ModTime()
}

func isWindows() bool { return false }

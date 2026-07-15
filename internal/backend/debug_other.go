//go:build !windows

package backend

// allocDebugConsole is a no-op on non-Windows platforms.
func allocDebugConsole() {}

// redirectStdStreams is a no-op on non-Windows platforms.
func redirectStdStreams() {}

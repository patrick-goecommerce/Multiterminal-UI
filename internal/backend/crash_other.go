//go:build !windows

package backend

import "os"

// redirectStderrToFile points os.Stderr at the log file. On non-Windows the
// runtime already writes panic output to fd 2, so reassigning os.Stderr is
// enough for the log package; a full fd-level dup is unnecessary here.
func redirectStderrToFile(f *os.File) {
	os.Stderr = f
}

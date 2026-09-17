package main

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/discovery"
)

// logFile is where the daemon writes. It has no console to write to: on
// Windows it is linked as a GUI-subsystem binary so that starting it never
// flashes a window, which also means stdout goes nowhere.
const logFile = "mtuid.log"

// logRotateBytes is the size at which the log is started over. The daemon is
// meant to run for days, and an unbounded log in the user's cache directory
// would be a slow leak.
const logRotateBytes = 4 << 20 // 4 MiB

// setupLogging points the standard logger at the daemon's log file and returns
// a function that closes it. Failing to open the file is not fatal: a daemon
// that runs without a log is better than one that refuses to start.
func setupLogging() func() {
	dir, err := discovery.Dir()
	if err != nil {
		return func() {}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return func() {}
	}
	path := filepath.Join(dir, logFile)

	if info, err := os.Stat(path); err == nil && info.Size() > logRotateBytes {
		_ = os.Remove(path)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return func() {}
	}
	log.SetOutput(io.Writer(f))
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	return func() {
		log.SetOutput(os.Stderr)
		_ = f.Close()
	}
}

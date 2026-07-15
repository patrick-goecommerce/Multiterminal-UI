package backend

import (
	"log"
	"os"
)

// InitDebugMode activates debug logging from the very start of the application.
// It opens a console window (Windows only) and sets up file logging immediately,
// so that all log output — including early startup messages — is captured.
func InitDebugMode() {
	// Open a visible console window so stderr output is readable.
	allocDebugConsole()

	// Redirect stdout/stderr to the new console (Windows needs this after AllocConsole).
	redirectStdStreams()

	// Set up file logging immediately.
	path := logFilePath()
	if err := setupFileLogging(path); err != nil {
		log.Printf("[DEBUG] failed to open log file: %v", err)
		return
	}

	log.Printf("[DEBUG] Debug mode active — console + file logging -> %s", path)
	log.Printf("[DEBUG] PID: %d, Args: %v", os.Getpid(), os.Args)
}

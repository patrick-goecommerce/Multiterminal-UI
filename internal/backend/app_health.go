package backend

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

// HealthInfo is returned to the frontend on startup to indicate
// whether crash-based logging should be suggested.
type HealthInfo struct {
	CrashDetected  bool `json:"crash_detected"`
	LoggingEnabled bool `json:"logging_enabled"`
	LoggingAuto    bool `json:"logging_auto"`
}

// CheckHealth returns the current health/logging state for the frontend.
func (a *App) CheckHealth() HealthInfo {
	return HealthInfo{
		CrashDetected:  config.HasRepeatedCrashes(&a.health),
		LoggingEnabled: a.cfg.LoggingEnabled,
		LoggingAuto:    a.health.LoggingAuto,
	}
}

// EnableLogging activates file logging. If auto is true, it was triggered
// by crash detection and will auto-disable after 3 clean shutdowns.
func (a *App) EnableLogging(auto bool) string {
	logPath := logFilePath()
	if err := setupFileLogging(logPath); err != nil {
		log.Printf("[EnableLogging] failed: %v", err)
		return ""
	}

	a.cfg.LoggingEnabled = true
	_ = config.Save(a.cfg)

	if auto {
		config.EnableAutoLogging(&a.health)
		_ = config.SaveHealth(a.health)
	}

	log.Printf("[EnableLogging] Logging enabled (auto=%v) -> %s", auto, logPath)
	return logPath
}

// DisableLogging deactivates file logging and resets to stderr.
func (a *App) DisableLogging() {
	a.cfg.LoggingEnabled = false
	_ = config.Save(a.cfg)

	config.DisableAutoLogging(&a.health)
	_ = config.SaveHealth(a.health)

	log.SetOutput(os.Stderr)
	closeLogFile()
	log.Println("[DisableLogging] Logging disabled, output reset to stderr")
}

// GetLogPath returns the current log file path.
func (a *App) GetLogPath() string {
	return logFilePath()
}

// logFilePath returns the path for the log file in the executable's directory.
func logFilePath() string {
	exe, err := os.Executable()
	if err != nil {
		return "multiterminal.log"
	}
	dir := filepath.Dir(exe)
	ts := time.Now().Format("2006-01-02")
	return filepath.Join(dir, fmt.Sprintf("multiterminal-%s.log", ts))
}

// logFile holds the currently open log file so it can be closed when switching or disabling.
var logFile *os.File

// setupFileLogging redirects log output to the given file path.
func setupFileLogging(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	// Close previous log file if open
	if logFile != nil {
		logFile.Close()
	}
	logFile = f
	// Write to both file and stderr for visibility
	w := io.MultiWriter(os.Stderr, f)
	log.SetOutput(w)
	return nil
}

// closeLogFile closes the current log file handle if open.
func closeLogFile() {
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
}

// InitLoggingFromConfig sets up file logging if the config says it's enabled.
// Called from main.go after config is loaded.
func InitLoggingFromConfig(cfg config.Config) {
	if !cfg.LoggingEnabled {
		return
	}
	path := logFilePath()
	if err := setupFileLogging(path); err != nil {
		log.Printf("[InitLogging] failed to open log file: %v", err)
		return
	}
	log.Printf("[InitLogging] File logging active -> %s", path)
}

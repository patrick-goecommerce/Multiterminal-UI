package backend

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

// HealthInfo is returned to the frontend on startup to indicate
// whether crash-based logging should be suggested and whether any local
// listener failed to come up.
type HealthInfo struct {
	CrashDetected  bool `json:"crash_detected" yaml:"crash_detected"`
	LoggingEnabled bool `json:"logging_enabled" yaml:"logging_enabled"`
	LoggingAuto    bool `json:"logging_auto" yaml:"logging_auto"`
	// BindWarnings lists listeners that failed to start (see BindWarning).
	// Empty in the normal case.
	BindWarnings []BindWarning `json:"bind_warnings" yaml:"bind_warnings"`
	// Runtime carries cheap counters (goroutines, OS threads, hook files) so
	// the frontend can surface a warning before a runaway becomes a five-day
	// CPU burn nobody noticed (see RuntimeStats).
	Runtime RuntimeStats `json:"runtime" yaml:"runtime"`
	// HookFilesHigh is true when the hooks directory holds enough files that
	// the poller is heading for trouble. Pre-computed here so the UI does not
	// have to carry the threshold.
	HookFilesHigh bool `json:"hook_files_high" yaml:"hook_files_high"`
}

// CheckHealth returns the current health/logging state for the frontend.
func (a *AppService) CheckHealth() HealthInfo {
	stats := a.RuntimeStats()
	return HealthInfo{
		CrashDetected:  config.HasRepeatedCrashes(&a.health),
		LoggingEnabled: a.cfg.LoggingEnabled,
		LoggingAuto:    a.health.LoggingAuto,
		BindWarnings:   a.bindWarningsSnapshot(),
		Runtime:        stats,
		HookFilesHigh:  stats.HookFiles > hookFileWarnThreshold,
	}
}

// EnableLogging activates file logging. If auto is true, it was triggered
// by crash detection and will auto-disable after 3 clean shutdowns.
func (a *AppService) EnableLogging(auto bool) string {
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
func (a *AppService) DisableLogging() {
	a.cfg.LoggingEnabled = false
	_ = config.Save(a.cfg)

	config.DisableAutoLogging(&a.health)
	_ = config.SaveHealth(a.health)

	log.SetOutput(os.Stderr)
	closeLogFile()
	log.Println("[DisableLogging] Logging disabled, output reset to stderr")
}

// GetLogPath returns the current log file path.
func (a *AppService) GetLogPath() string {
	return logFilePath()
}

// isPortableLocation returns true if the directory is the user's Downloads or Desktop folder.
func isPortableLocation(dir string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	norm := strings.ToLower(filepath.Clean(dir))
	for _, folder := range []string{"Downloads", "Desktop"} {
		target := strings.ToLower(filepath.Clean(filepath.Join(home, folder)))
		if norm == target {
			return true
		}
	}
	return false
}

// logDir returns the directory where log files are stored.
// Portable locations (Downloads/Desktop) use the exe directory directly.
// Installed locations use a logs/ subdirectory.
func logDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	dir := filepath.Dir(exe)
	if isPortableLocation(dir) {
		return dir
	}
	logsDir := filepath.Join(dir, "logs")
	_ = os.MkdirAll(logsDir, 0755)
	return logsDir
}

// logFilePath returns the path for today's log file.
func logFilePath() string {
	ts := time.Now().Format("2006-01-02")
	return filepath.Join(logDir(), fmt.Sprintf("multiterminal-%s.log", ts))
}

// OpenLogDir opens the log directory in the system file explorer.
func (a *AppService) OpenLogDir() {
	dir := logDir()
	exec.Command("cmd", "/c", "start", "", dir).Start() //nolint:errcheck
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
	// Redirect the process stderr (fd 2) at the log file so Go runtime panic
	// traces — which bypass the log package — land in the log instead of being
	// discarded by the GUI subsystem. After this os.Stderr IS the log file, so
	// logging straight to f (no MultiWriter) avoids double-writing every line.
	redirectStderrToFile(f)
	log.SetOutput(f)
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

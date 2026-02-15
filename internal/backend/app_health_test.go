package backend

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/patrick-goecommerce/multiterminal/internal/config"
)

// ---------------------------------------------------------------------------
// CheckHealth — returns correct HealthInfo based on app state
// ---------------------------------------------------------------------------

func TestCheckHealth_NoCrash(t *testing.T) {
	app := &App{
		cfg:    config.Config{LoggingEnabled: false},
		health: config.HealthState{Shutdowns: []bool{true, true, false}},
	}
	info := app.CheckHealth()

	if info.CrashDetected {
		t.Error("CrashDetected should be false when prior shutdowns were clean")
	}
	if info.LoggingEnabled {
		t.Error("LoggingEnabled should be false")
	}
	if info.LoggingAuto {
		t.Error("LoggingAuto should be false")
	}
}

func TestCheckHealth_CrashDetected(t *testing.T) {
	app := &App{
		cfg: config.Config{LoggingEnabled: false},
		health: config.HealthState{
			Shutdowns: []bool{false, false, false}, // 2 prior dirty + current
		},
	}
	info := app.CheckHealth()

	if !info.CrashDetected {
		t.Error("CrashDetected should be true with 2 prior dirty shutdowns")
	}
}

func TestCheckHealth_LoggingState(t *testing.T) {
	app := &App{
		cfg:    config.Config{LoggingEnabled: true},
		health: config.HealthState{LoggingAuto: true, Shutdowns: []bool{true, false}},
	}
	info := app.CheckHealth()

	if !info.LoggingEnabled {
		t.Error("LoggingEnabled should reflect config")
	}
	if !info.LoggingAuto {
		t.Error("LoggingAuto should reflect health state")
	}
}

// ---------------------------------------------------------------------------
// EnableLogging — activates file logging and sets config
// ---------------------------------------------------------------------------

func TestEnableLogging_Manual(t *testing.T) {
	dir := t.TempDir()
	app := &App{
		cfg:    config.Config{},
		health: config.HealthState{},
	}

	// Override log output to a temp file so test doesn't pollute
	logPath := filepath.Join(dir, "test.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	defer log.SetOutput(os.Stderr) // restore after test

	// Enable logging manually (auto=false)
	result := app.EnableLogging(false)

	if result == "" {
		// May fail if executable dir isn't writable in test env; skip
		t.Skip("EnableLogging returned empty (log dir not writable)")
	}

	if !app.cfg.LoggingEnabled {
		t.Error("Config.LoggingEnabled should be true after EnableLogging")
	}
	if app.health.LoggingAuto {
		t.Error("LoggingAuto should NOT be set for manual enable")
	}
}

func TestEnableLogging_Auto(t *testing.T) {
	app := &App{
		cfg:    config.Config{},
		health: config.HealthState{CleanSinceAuto: 5},
	}
	defer log.SetOutput(os.Stderr)

	result := app.EnableLogging(true)

	if result == "" {
		t.Skip("EnableLogging returned empty (log dir not writable)")
	}

	if !app.cfg.LoggingEnabled {
		t.Error("Config.LoggingEnabled should be true")
	}
	if !app.health.LoggingAuto {
		t.Error("LoggingAuto should be true for auto enable")
	}
	if app.health.CleanSinceAuto != 0 {
		t.Errorf("CleanSinceAuto = %d, want 0 (reset)", app.health.CleanSinceAuto)
	}
}

// ---------------------------------------------------------------------------
// DisableLogging — deactivates logging and resets state
// ---------------------------------------------------------------------------

func TestDisableLogging(t *testing.T) {
	app := &App{
		cfg:    config.Config{LoggingEnabled: true},
		health: config.HealthState{LoggingAuto: true, CleanSinceAuto: 2},
	}
	defer log.SetOutput(os.Stderr)

	app.DisableLogging()

	if app.cfg.LoggingEnabled {
		t.Error("Config.LoggingEnabled should be false after DisableLogging")
	}
	if app.health.LoggingAuto {
		t.Error("LoggingAuto should be false after DisableLogging")
	}
	if app.health.CleanSinceAuto != 0 {
		t.Errorf("CleanSinceAuto = %d, want 0", app.health.CleanSinceAuto)
	}
}

// ---------------------------------------------------------------------------
// GetLogPath — returns a valid path with date stamp
// ---------------------------------------------------------------------------

func TestGetLogPath_ContainsDate(t *testing.T) {
	app := &App{}
	path := app.GetLogPath()

	today := time.Now().Format("2006-01-02")
	if !strings.Contains(path, today) {
		t.Errorf("GetLogPath() = %q, want path containing date %q", path, today)
	}
}

func TestGetLogPath_ContainsPrefix(t *testing.T) {
	app := &App{}
	path := app.GetLogPath()

	base := filepath.Base(path)
	expected := fmt.Sprintf("multiterminal-%s.log", time.Now().Format("2006-01-02"))
	if base != expected {
		t.Errorf("GetLogPath base = %q, want %q", base, expected)
	}
}

// ---------------------------------------------------------------------------
// logFilePath — standalone function tests
// ---------------------------------------------------------------------------

func TestLogFilePath_Format(t *testing.T) {
	path := logFilePath()
	if path == "" {
		t.Fatal("logFilePath() returned empty string")
	}
	if !strings.HasSuffix(path, ".log") {
		t.Errorf("logFilePath() = %q, should end with .log", path)
	}
	if !strings.Contains(path, "multiterminal-") {
		t.Errorf("logFilePath() = %q, should contain 'multiterminal-'", path)
	}
}

// ---------------------------------------------------------------------------
// setupFileLogging — writes to file and stderr
// ---------------------------------------------------------------------------

func TestSetupFileLogging_WritesToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-setup.log")

	defer log.SetOutput(os.Stderr)

	err := setupFileLogging(path)
	if err != nil {
		t.Fatalf("setupFileLogging failed: %v", err)
	}

	log.Println("test log entry")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if !strings.Contains(string(data), "test log entry") {
		t.Errorf("Log file should contain 'test log entry', got: %q", string(data))
	}
}

func TestSetupFileLogging_InvalidPath(t *testing.T) {
	err := setupFileLogging("/nonexistent/dir/test.log")
	if err == nil {
		t.Error("setupFileLogging should fail with invalid path")
	}
}

// ---------------------------------------------------------------------------
// InitLoggingFromConfig — conditional setup
// ---------------------------------------------------------------------------

func TestInitLoggingFromConfig_Disabled(t *testing.T) {
	defer log.SetOutput(os.Stderr)

	// Should be a no-op when disabled
	cfg := config.Config{LoggingEnabled: false}
	InitLoggingFromConfig(cfg)
	// No panic or error = pass
}

func TestInitLoggingFromConfig_Enabled(t *testing.T) {
	defer log.SetOutput(os.Stderr)

	cfg := config.Config{LoggingEnabled: true}
	InitLoggingFromConfig(cfg)
	// If the log file dir is writable, logging is set up.
	// We just verify no panic occurs.
}

// ---------------------------------------------------------------------------
// Integration: Enable → Disable cycle
// ---------------------------------------------------------------------------

func TestLoggingEnableDisableCycle(t *testing.T) {
	app := &App{
		cfg:    config.Config{},
		health: config.HealthState{},
	}
	defer log.SetOutput(os.Stderr)

	// Enable auto-logging
	result := app.EnableLogging(true)
	if result == "" {
		t.Skip("EnableLogging returned empty (log dir not writable)")
	}

	if !app.cfg.LoggingEnabled {
		t.Error("Should be enabled after EnableLogging")
	}
	if !app.health.LoggingAuto {
		t.Error("LoggingAuto should be true")
	}

	// Disable
	app.DisableLogging()

	if app.cfg.LoggingEnabled {
		t.Error("Should be disabled after DisableLogging")
	}
	if app.health.LoggingAuto {
		t.Error("LoggingAuto should be false")
	}

	// Re-enable manually
	result = app.EnableLogging(false)
	if result == "" {
		t.Skip("EnableLogging returned empty on re-enable")
	}

	if !app.cfg.LoggingEnabled {
		t.Error("Should be enabled again")
	}
	if app.health.LoggingAuto {
		t.Error("LoggingAuto should stay false for manual enable")
	}
}

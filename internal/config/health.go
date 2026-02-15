// Package config – health tracking for crash detection.
//
// Tracks the last N shutdown states to detect repeated crashes.
// When 2 consecutive dirty shutdowns are detected, the app suggests
// enabling verbose logging. Auto-enabled logging disables itself
// after 3 consecutive clean shutdowns.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// HealthState tracks shutdown history and auto-logging state.
type HealthState struct {
	// Shutdowns records the last few shutdown states (true=clean, false=dirty).
	Shutdowns []bool `json:"shutdowns"`
	// LoggingAuto is true when logging was auto-enabled due to crashes.
	LoggingAuto bool `json:"logging_auto"`
	// CleanSinceAuto counts clean shutdowns since auto-logging was enabled.
	CleanSinceAuto int `json:"clean_since_auto"`
}

const maxShutdownHistory = 5

// healthPath returns the path to ~/.multiterminal-health.json.
func healthPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".multiterminal-health.json")
}

// LoadHealth reads the health state from disk.
// Returns a zero-value HealthState if no file exists.
func LoadHealth() HealthState {
	p := healthPath()
	if p == "" {
		return HealthState{}
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return HealthState{}
	}
	var h HealthState
	if err := json.Unmarshal(data, &h); err != nil {
		return HealthState{}
	}
	return h
}

// SaveHealth writes the health state to disk.
func SaveHealth(h HealthState) error {
	p := healthPath()
	if p == "" {
		return nil
	}
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// MarkStarting adds a dirty (false) entry to the shutdown history.
// Call this at startup before any real work begins.
func MarkStarting(h *HealthState) {
	h.Shutdowns = append(h.Shutdowns, false)
	if len(h.Shutdowns) > maxShutdownHistory {
		h.Shutdowns = h.Shutdowns[len(h.Shutdowns)-maxShutdownHistory:]
	}
}

// MarkCleanShutdown marks the most recent entry as clean (true).
// Call this during orderly shutdown.
func MarkCleanShutdown(h *HealthState) {
	if len(h.Shutdowns) > 0 {
		h.Shutdowns[len(h.Shutdowns)-1] = true
	}
	if h.LoggingAuto {
		h.CleanSinceAuto++
	}
}

// HasRepeatedCrashes returns true if the last 2 shutdowns were dirty.
func HasRepeatedCrashes(h *HealthState) bool {
	n := len(h.Shutdowns)
	if n < 2 {
		return false
	}
	// Check the last 2 completed sessions (not the current one).
	// The current session was just added as dirty by MarkStarting,
	// so we look at indices n-2 and n-3.
	if n < 3 {
		return !h.Shutdowns[0] && !h.Shutdowns[1]
	}
	return !h.Shutdowns[n-3] && !h.Shutdowns[n-2]
}

// ShouldAutoDisableLogging returns true if auto-logging should be turned off
// (3 consecutive clean shutdowns since it was enabled).
func ShouldAutoDisableLogging(h *HealthState) bool {
	return h.LoggingAuto && h.CleanSinceAuto >= 3
}

// EnableAutoLogging marks logging as auto-enabled and resets the clean counter.
func EnableAutoLogging(h *HealthState) {
	h.LoggingAuto = true
	h.CleanSinceAuto = 0
}

// DisableAutoLogging clears the auto-logging state.
func DisableAutoLogging(h *HealthState) {
	h.LoggingAuto = false
	h.CleanSinceAuto = 0
}

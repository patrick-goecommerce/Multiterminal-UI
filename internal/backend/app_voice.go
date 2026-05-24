// Package backend – Auto-enables Claude Code voice dictation in ~/.claude/settings.json.
package backend

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// voiceInstaller enables Claude Code's voice dictation by default by writing a
// "voice" block into ~/.claude/settings.json. Voice is controlled solely via
// settings.json — there are no CLI flags or env vars — so this is the only way
// to make it active by default for Claude sessions launched inside Multiterminal.
//
// It only writes when no "voice" key exists yet, so a user who later runs
// `/voice off` (which persists voice.enabled=false) is never overridden.
type voiceInstaller struct {
	settingsPath string
	mode         string // "hold" or "tap"
}

func newVoiceInstaller(settingsPath, mode string) *voiceInstaller {
	if mode != "tap" {
		mode = "hold"
	}
	return &voiceInstaller{settingsPath: settingsPath, mode: mode}
}

// Install adds a default voice config to settings.json if none is present.
// Idempotent: an existing "voice" key (ours or the user's) is left untouched.
// Creates a timestamped .bak backup before the first modification.
func (v *voiceInstaller) Install() error {
	var settings map[string]any
	data, err := os.ReadFile(v.settingsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read settings: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(v.settingsPath), 0755); err != nil {
			return err
		}
		settings = make(map[string]any)
	} else {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse settings.json: %w", err)
		}
	}

	// Respect any pre-existing voice configuration (user controls it from here).
	if _, exists := settings["voice"]; exists {
		return nil
	}

	// Backup before modification
	if len(data) > 0 {
		ts := time.Now().Format("20060102-150405")
		backupPath := v.settingsPath + ".bak." + ts
		if err := os.WriteFile(backupPath, data, 0644); err != nil {
			log.Printf("[voice] warning: could not create backup: %v", err)
		}
	}

	settings["voice"] = map[string]any{
		"enabled": true,
		"mode":    v.mode,
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	return os.WriteFile(v.settingsPath, out, 0644)
}

// setupVoice enables Claude voice dictation by default in ~/.claude/settings.json.
// Called once on startup. Non-fatal: on error voice simply stays off.
func (a *AppService) setupVoice() {
	installer := newVoiceInstaller(claudeSettingsPath(), "hold")
	if err := installer.Install(); err != nil {
		log.Printf("[voice] could not enable voice dictation: %v", err)
		return
	}
	log.Println("[voice] voice dictation enabled by default in ~/.claude/settings.json")
}

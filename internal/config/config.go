// Package config loads and provides application configuration.
//
// On first run, a default YAML config is written to ~/.multiterminal.yaml.
// Subsequent runs read and merge that file with built-in defaults.
package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all user-configurable settings.
type Config struct {
	// DefaultShell is the shell spawned for new terminal panes.
	DefaultShell string `yaml:"default_shell"`

	// DefaultDir is the working directory for new tabs.
	// Empty means the current working directory at launch time.
	DefaultDir string `yaml:"default_dir"`

	// Theme can be "dark" or "light".
	Theme string `yaml:"theme"`

	// MaxPanesPerTab limits panes in a single tab (1-12).
	MaxPanesPerTab int `yaml:"max_panes_per_tab"`

	// SidebarWidth is the character width of the file browser sidebar.
	SidebarWidth int `yaml:"sidebar_width"`

	// ClaudeCommand is the base command for launching Claude Code.
	ClaudeCommand string `yaml:"claude_command"`

	// ClaudeModels lists available models for the launch dialog.
	ClaudeModels []ModelEntry `yaml:"claude_models"`

	// CommitReminderMinutes is how often (in minutes) to show a commit reminder.
	// Set to 0 to disable. Default: 30.
	CommitReminderMinutes int `yaml:"commit_reminder_minutes"`
}

// ModelEntry represents a selectable Claude model in the launch dialog.
type ModelEntry struct {
	Label string `yaml:"label"` // Display name (e.g. "Opus 4.6")
	ID    string `yaml:"id"`    // Model identifier (e.g. "claude-opus-4-6")
}

// DefaultConfig returns the built-in defaults.
func DefaultConfig() Config {
	return Config{
		DefaultShell:          "",
		DefaultDir:            "",
		Theme:                 "dark",
		MaxPanesPerTab:        12,
		SidebarWidth:          30,
		ClaudeCommand:         "claude",
		CommitReminderMinutes: 30,
		ClaudeModels: []ModelEntry{
			{Label: "Default", ID: ""},
			{Label: "Opus 4.6", ID: "claude-opus-4-6"},
			{Label: "Sonnet 4.5", ID: "claude-sonnet-4-5-20250929"},
			{Label: "Haiku 4.5", ID: "claude-haiku-4-5-20251001"},
		},
	}
}

// configPath returns the path to ~/.multiterminal.yaml.
func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".multiterminal.yaml")
}

// Load reads the config file, falling back to defaults for missing fields.
func Load() Config {
	cfg := DefaultConfig()

	p := configPath()
	if p == "" {
		return cfg
	}

	data, err := os.ReadFile(p)
	if err != nil {
		// No config file yet – write defaults for future editing
		writeDefaults(p, cfg)
		return cfg
	}

	_ = yaml.Unmarshal(data, &cfg)

	// Apply sensible bounds
	if cfg.MaxPanesPerTab < 1 {
		cfg.MaxPanesPerTab = 1
	}
	if cfg.MaxPanesPerTab > 12 {
		cfg.MaxPanesPerTab = 12
	}
	if cfg.SidebarWidth < 15 {
		cfg.SidebarWidth = 15
	}
	if cfg.SidebarWidth > 60 {
		cfg.SidebarWidth = 60
	}

	// Validate theme name
	validThemes := map[string]bool{"dark": true, "light": true, "dracula": true, "nord": true, "solarized": true}
	if !validThemes[cfg.Theme] {
		cfg.Theme = "dark"
	}

	if cfg.CommitReminderMinutes < 0 {
		cfg.CommitReminderMinutes = 0
	}

	return cfg
}

// writeDefaults persists the default configuration to disk.
func writeDefaults(path string, cfg Config) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return
	}
	header := []byte("# Multiterminal configuration\n# Edit this file to customise defaults.\n\n")
	_ = os.WriteFile(path, append(header, data...), 0644)
}

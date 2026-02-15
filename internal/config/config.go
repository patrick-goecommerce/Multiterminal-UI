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
	DefaultShell          string       `yaml:"default_shell" json:"default_shell"`
	DefaultDir            string       `yaml:"default_dir" json:"default_dir"`
	Theme                 string       `yaml:"theme" json:"theme"`
	TerminalColor         string       `yaml:"terminal_color" json:"terminal_color"`
	MaxPanesPerTab        int          `yaml:"max_panes_per_tab" json:"max_panes_per_tab"`
	SidebarWidth          int          `yaml:"sidebar_width" json:"sidebar_width"`
	ClaudeCommand         string       `yaml:"claude_command" json:"claude_command"`
	ClaudeModels          []ModelEntry `yaml:"claude_models" json:"claude_models"`
	CommitReminderMinutes int            `yaml:"commit_reminder_minutes" json:"commit_reminder_minutes"`
	RestoreSession        *bool          `yaml:"restore_session" json:"restore_session"`
	LoggingEnabled        bool           `yaml:"logging_enabled" json:"logging_enabled"`
	Commands              []CommandEntry `yaml:"commands" json:"commands"`
}

// ModelEntry represents a selectable Claude model in the launch dialog.
type ModelEntry struct {
	Label string `yaml:"label" json:"label"`
	ID    string `yaml:"id" json:"id"`
}

// CommandEntry represents a user-defined command in the command palette.
type CommandEntry struct {
	Name string `yaml:"name" json:"name"`
	Text string `yaml:"text" json:"text"`
}

// DefaultConfig returns the built-in defaults.
// boolPtr returns a pointer to a bool value.
func boolPtr(b bool) *bool { return &b }

func DefaultConfig() Config {
	return Config{
		DefaultShell:          "",
		DefaultDir:            "",
		Theme:                 "dark",
		TerminalColor:         "#39ff14",
		MaxPanesPerTab:        12,
		SidebarWidth:          30,
		ClaudeCommand:         "claude",
		CommitReminderMinutes: 30,
		RestoreSession:        boolPtr(true),
		ClaudeModels: []ModelEntry{
			{Label: "Default", ID: ""},
			{Label: "Opus 4.6", ID: "claude-opus-4-6"},
			{Label: "Sonnet 4.5", ID: "claude-sonnet-4-5-20250929"},
			{Label: "Haiku 4.5", ID: "claude-haiku-4-5-20251001"},
		},
		Commands: []CommandEntry{
			{Name: "Commit & Push", Text: "git add -A && git commit -m 'update' && git push"},
		},
	}
}

// ShouldRestoreSession returns whether the session should be restored.
func (c Config) ShouldRestoreSession() bool {
	if c.RestoreSession == nil {
		return true
	}
	return *c.RestoreSession
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
		_ = writeDefaults(p, cfg)
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

	if cfg.RestoreSession == nil {
		cfg.RestoreSession = boolPtr(true)
	}

	return cfg
}

// Save writes the given config to the YAML file.
func Save(cfg Config) error {
	p := configPath()
	if p == "" {
		return nil
	}
	return writeDefaults(p, cfg)
}

// writeDefaults persists the configuration to disk.
func writeDefaults(path string, cfg Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	header := []byte("# Multiterminal UI configuration\n# Edit this file to customise defaults.\n\n")
	return os.WriteFile(path, append(header, data...), 0644)
}

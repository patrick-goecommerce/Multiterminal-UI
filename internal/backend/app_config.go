package backend

import (
	"fmt"
	"log"
	"os"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

// GetConfig returns the current application configuration.
func (a *AppService) GetConfig() config.Config {
	return a.cfg
}

// SaveConfig saves the given config to disk and updates the in-memory copy.
func (a *AppService) SaveConfig(cfg config.Config) error {
	log.Printf("[SaveConfig] theme=%q terminal_color=%q", cfg.Theme, cfg.TerminalColor)
	wasEnabled := a.cfg.StatusLine.Enabled
	a.cfg = cfg
	if err := config.Save(cfg); err != nil {
		log.Printf("[SaveConfig] error: %v", err)
		return fmt.Errorf("config save failed: %w", err)
	}
	// Re-detect CLI paths in case commands changed
	a.resolveClaudeOnStartup()
	a.resolveCodexOnStartup()
	// Apply or remove statusline in ~/.claude/settings.json
	if cfg.StatusLine.Enabled {
		a.applyStatusLine(cfg.StatusLine)
	} else if wasEnabled {
		a.removeStatusLine()
	}
	return nil
}

// GetWorkingDir returns the effective working directory (from config or cwd).
func (a *AppService) GetWorkingDir() string {
	if a.cfg.DefaultDir != "" {
		return a.cfg.DefaultDir
	}
	dir, _ := os.Getwd()
	return dir
}

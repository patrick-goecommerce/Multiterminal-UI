package backend

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const hookMarker = "# multiterminal-hook"

// hookInstaller manages registration of Multiterminal hooks in ~/.claude/settings.json.
type hookInstaller struct {
	settingsPath string
	command      string
}

func newHookInstaller(settingsPath, command string) *hookInstaller {
	return &hookInstaller{settingsPath: settingsPath, command: command}
}

// Install adds Multiterminal hook entries to settings.json if not already present.
// Idempotent: calling it multiple times produces the same result.
// Creates a timestamped .bak backup before the first modification.
func (h *hookInstaller) Install() error {
	var settings map[string]any
	data, err := os.ReadFile(h.settingsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read settings: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(h.settingsPath), 0755); err != nil {
			return err
		}
		settings = make(map[string]any)
	} else {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("parse settings.json: %w", err)
		}
	}

	if h.isInstalled(settings) {
		return nil
	}

	// Backup before modification
	if len(data) > 0 {
		ts := time.Now().Format("20060102-150405")
		backupPath := h.settingsPath + ".bak." + ts
		if err := os.WriteFile(backupPath, data, 0644); err != nil {
			log.Printf("[hooks] warning: could not create backup: %v", err)
		}
	}

	h.mergeHooks(settings)

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	return os.WriteFile(h.settingsPath, out, 0644)
}

// isInstalled checks if the Multiterminal hook marker is present in PreToolUse.
func (h *hookInstaller) isInstalled(settings map[string]any) bool {
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return false
	}
	preToolUse, ok := hooks["PreToolUse"].([]any)
	if !ok || len(preToolUse) == 0 {
		return false
	}
	for _, entry := range preToolUse {
		e, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		innerHooks, ok := e["hooks"].([]any)
		if !ok {
			continue
		}
		for _, ih := range innerHooks {
			inner, ok := ih.(map[string]any)
			if !ok {
				continue
			}
			cmd, _ := inner["command"].(string)
			if strings.Contains(cmd, hookMarker) {
				return true
			}
		}
	}
	return false
}

// mergeHooks prepends Multiterminal hook entries for all relevant events.
func (h *hookInstaller) mergeHooks(settings map[string]any) {
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		hooks = make(map[string]any)
		settings["hooks"] = hooks
	}

	events := []string{
		"PreToolUse", "PostToolUse", "PostToolUseFailure",
		"PermissionRequest", "Notification", "Stop", "SessionEnd",
	}

	for _, event := range events {
		cmd := fmt.Sprintf("%s %s %s", h.command, event, hookMarker)
		entry := map[string]any{
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": cmd,
				},
			},
		}
		existing, _ := hooks[event].([]any)
		hooks[event] = append([]any{entry}, existing...)
	}
}

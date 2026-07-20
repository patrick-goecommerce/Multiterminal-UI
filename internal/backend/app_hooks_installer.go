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

// hookEvents lists every Claude Code event Multiterminal registers a handler for.
var hookEvents = []string{
	"PreToolUse", "PostToolUse", "PostToolUseFailure",
	"PermissionRequest", "Notification", "Stop", "SessionEnd",
	"UserPromptSubmit",
}

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

	// Migration: drop stale registrations (obsolete .ps1 handler, or our own
	// marker pointing at a moved binary). Runs before the early return — an
	// install whose markers are all present would otherwise keep them forever.
	removed := h.removeStaleHooks(settings)

	if h.isInstalled(settings) && !removed {
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

// isInstalled reports whether every Multiterminal hook event already carries
// our marker. Returns false if any event is missing, so newly-added events are
// backfilled on the next Install (older installs predate UserPromptSubmit).
func (h *hookInstaller) isInstalled(settings map[string]any) bool {
	for _, event := range hookEvents {
		if !markerInEvent(settings, event) {
			return false
		}
	}
	return true
}

// markerInEvent reports whether the given event already has a Multiterminal
// marker entry in settings.
func markerInEvent(settings map[string]any, event string) bool {
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return false
	}
	entries, ok := hooks[event].([]any)
	if !ok {
		return false
	}
	for _, entry := range entries {
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

// isLegacyPS1Command reports whether a hook command invokes Multiterminal's
// obsolete PowerShell handler. Both markers must match so a foreign tool that
// happens to ship its own hook_handler.ps1 is left untouched.
func isLegacyPS1Command(cmd string) bool {
	lower := strings.ToLower(cmd)
	return strings.Contains(lower, "hook_handler.ps1") &&
		strings.Contains(lower, "multiterminal")
}

// isStaleCommand reports whether a hook command is one of ours that no longer
// works: the obsolete PowerShell handler, or a marker entry whose command does
// not match the binary we are installing (MTUI moved — common for the portable
// build). Foreign commands never match and are always preserved.
func (h *hookInstaller) isStaleCommand(cmd string) bool {
	if isLegacyPS1Command(cmd) {
		return true
	}
	return strings.Contains(cmd, hookMarker) && !strings.HasPrefix(cmd, h.command+" ")
}

// removeStaleHooks strips every hook command matching isStaleCommand from
// settings, pruning entries and event keys that become empty. It reports
// whether anything was removed. All events are scanned, not just hookEvents —
// older builds registered events we no longer use.
func (h *hookInstaller) removeStaleHooks(settings map[string]any) bool {
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		return false
	}

	removed := false
	for event, raw := range hooks {
		entries, ok := raw.([]any)
		if !ok {
			continue
		}
		kept := make([]any, 0, len(entries))
		for _, entry := range entries {
			e, ok := entry.(map[string]any)
			if !ok {
				kept = append(kept, entry)
				continue
			}
			innerHooks, ok := e["hooks"].([]any)
			if !ok {
				kept = append(kept, entry)
				continue
			}
			keptInner := make([]any, 0, len(innerHooks))
			for _, ih := range innerHooks {
				inner, ok := ih.(map[string]any)
				if ok {
					if cmd, _ := inner["command"].(string); h.isStaleCommand(cmd) {
						removed = true
						continue
					}
				}
				keptInner = append(keptInner, ih)
			}
			if len(keptInner) == 0 {
				continue // whole entry was legacy — drop it
			}
			e["hooks"] = keptInner
			kept = append(kept, e)
		}
		if len(kept) == 0 {
			delete(hooks, event)
			continue
		}
		hooks[event] = kept
	}
	return removed
}

// mergeHooks prepends a Multiterminal hook entry for every event that does not
// already have one. Per-event idempotency means re-running install only adds
// missing events without duplicating existing ones.
func (h *hookInstaller) mergeHooks(settings map[string]any) {
	hooks, ok := settings["hooks"].(map[string]any)
	if !ok {
		hooks = make(map[string]any)
		settings["hooks"] = hooks
	}

	for _, event := range hookEvents {
		if markerInEvent(settings, event) {
			continue // already present — don't duplicate
		}
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

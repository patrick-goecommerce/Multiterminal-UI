package backend

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

// StatusLineStatus describes the current state of the statusLine key in ~/.claude/settings.json.
type StatusLineStatus struct {
	HasExisting     bool   `json:"has_existing"`
	IsOurs          bool   `json:"is_ours"` // true when the command references mtui-statusline (binary or legacy .ps1)
	ExistingCommand string `json:"existing_command"`
}

// setupStatusLine is called once on startup.
// If statusLine is already enabled in config → reapply (ensures script exists).
// If statusLine is not configured AND no external statusLine exists → auto-enable with defaults.
func (a *AppService) setupStatusLine() {
	if a.cfg.StatusLine.Enabled {
		a.applyStatusLine(a.cfg.StatusLine)
		return
	}
	if a.GetStatusLineStatus().HasExisting {
		return // don't touch external config
	}
	// No statusLine anywhere — set up automatically with defaults
	defaults := config.StatusLineSettings{
		Enabled:     true,
		Template:    "standard",
		ShowModel:   true,
		ShowContext: true,
		ShowCost:    true,
	}
	a.applyStatusLine(defaults)
	a.cfg.StatusLine = defaults
	if err := config.Save(a.cfg); err != nil {
		log.Printf("[statusline] auto-setup save: %v", err)
	}
}

// GetStatusLineStatus reads ~/.claude/settings.json and reports whether a statusLine
// configuration exists and whether it was written by MTUI.
func (a *AppService) GetStatusLineStatus() StatusLineStatus {
	data, err := os.ReadFile(claudeSettingsPath())
	if err != nil {
		return StatusLineStatus{}
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return StatusLineStatus{}
	}
	sl, ok := settings["statusLine"].(map[string]any)
	if !ok {
		return StatusLineStatus{}
	}
	cmd, _ := sl["command"].(string)
	return StatusLineStatus{
		HasExisting:     true,
		IsOurs:          strings.Contains(cmd, "mtui-statusline"),
		ExistingCommand: cmd,
	}
}

// statuslineRenderFlags builds the CLI flag string for the mtui-statusline renderer.
// It always includes --template <name>; then appends --model, --context, --cost,
// --git, --duration for each enabled flag in that order.
func statuslineRenderFlags(cfg config.StatusLineSettings) string {
	var b strings.Builder
	b.WriteString("--template ")
	b.WriteString(cfg.Template)
	if cfg.ShowModel {
		b.WriteString(" --model")
	}
	if cfg.ShowContext {
		b.WriteString(" --context")
	}
	if cfg.ShowCost {
		b.WriteString(" --cost")
	}
	if cfg.ShowGitBranch {
		b.WriteString(" --git")
	}
	if cfg.ShowDuration {
		b.WriteString(" --duration")
	}
	return b.String()
}

// applyStatusLine registers the statusline renderer in ~/.claude/settings.json.
// It resolves the bundled mtui-statusline binary and registers it directly
// (no PowerShell, no console-window flash). If the binary cannot be resolved the
// statusline is left untouched (there is no PowerShell fallback anymore).
func (a *AppService) applyStatusLine(cfg config.StatusLineSettings) {
	settingsPath := claudeSettingsPath()

	// Best-effort: delete a stale PS1 script left over from an older build.
	_ = os.Remove(statusLineScriptPath())

	rendererExe := resolveBundledBinary("mtui-statusline", statuslineBin)
	if rendererExe == "" {
		log.Printf("[statusline] mtui-statusline binary not found — statusline not registered")
		return
	}

	command := fmt.Sprintf(`"%s" %s`, rendererExe, statuslineRenderFlags(cfg))
	log.Printf("[statusline] applyStatusLine: renderer=%q command=%q", rendererExe, command)

	data, _ := os.ReadFile(settingsPath)
	var settings map[string]any
	if len(data) > 0 {
		_ = json.Unmarshal(data, &settings)
	}
	if settings == nil {
		settings = make(map[string]any)
	}
	settings["statusLine"] = map[string]any{
		"type":    "command",
		"command": command,
	}
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		log.Printf("[statusline] marshal: %v", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		log.Printf("[statusline] mkdir settings: %v", err)
		return
	}
	if err := os.WriteFile(settingsPath, out, 0644); err != nil {
		log.Printf("[statusline] write settings: %v", err)
	}

	// Best-effort: delete the stale PS1 again post-write (covers the rare race
	// where the renderer just extracted it next to the script path).
	_ = os.Remove(statusLineScriptPath())
}

// removeStatusLine deletes the statusLine key from ~/.claude/settings.json
// and best-effort removes any stale legacy .ps1 script (migration cleanup —
// the new registration uses a binary, no script is written).
func (a *AppService) removeStatusLine() {
	settingsPath := claudeSettingsPath()
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("[statusline] read settings: %v", err)
		}
		return
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		log.Printf("[statusline] parse settings: %v", err)
		return
	}
	delete(settings, "statusLine")
	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		log.Printf("[statusline] marshal: %v", err)
		return
	}
	if err := os.WriteFile(settingsPath, out, 0644); err != nil {
		log.Printf("[statusline] write settings: %v", err)
	}
	_ = os.Remove(statusLineScriptPath())
}

func claudeSettingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

func statusLineScriptPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "mtui-statusline.ps1")
}

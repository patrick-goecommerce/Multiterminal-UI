package backend

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
)

// StatusLineStatus describes the current state of the statusLine key in ~/.claude/settings.json.
type StatusLineStatus struct {
	HasExisting     bool   `json:"has_existing"`
	IsOurs          bool   `json:"is_ours"` // true when the command references mtui-statusline.ps1
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
		IsOurs:          strings.Contains(cmd, "mtui-statusline.ps1"),
		ExistingCommand: cmd,
	}
}

// applyStatusLine writes the PS1 script and registers it in ~/.claude/settings.json.
func (a *AppService) applyStatusLine(cfg config.StatusLineSettings) {
	scriptPath := statusLineScriptPath()
	if err := os.MkdirAll(filepath.Dir(scriptPath), 0755); err != nil {
		log.Printf("[statusline] mkdir: %v", err)
		return
	}
	if err := os.WriteFile(scriptPath, []byte(buildStatusLineScript(cfg)), 0644); err != nil {
		log.Printf("[statusline] write script: %v", err)
		return
	}

	// Use forward slashes so PowerShell resolves the path correctly on Windows.
	fwdPath := strings.ReplaceAll(scriptPath, `\`, `/`)
	command := `powershell -NonInteractive -NoProfile -File "` + fwdPath + `"`

	settingsPath := claudeSettingsPath()
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
}

// removeStatusLine deletes the statusLine key from ~/.claude/settings.json
// and removes the PS1 script file.
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

// buildStatusLineScript generates a PowerShell script for the given statusline configuration.
func buildStatusLineScript(cfg config.StatusLineSettings) string {
	switch cfg.Template {
	case "minimal":
		return buildMinimalScript(cfg)
	case "extended":
		return buildExtendedScript(cfg)
	default: // "standard"
		return buildStandardScript(cfg)
	}
}

func buildMinimalScript(cfg config.StatusLineSettings) string {
	var b strings.Builder
	b.WriteString("$d = $input | Out-String | ConvertFrom-Json\n")
	b.WriteString("$parts = @()\n")
	if cfg.ShowModel {
		b.WriteString("$model = if ($d.model.display_name) { $d.model.display_name } else { '?' }\n")
		b.WriteString("$parts += \"[$model]\"\n")
	}
	if cfg.ShowContext {
		b.WriteString("$pct = if ($null -ne $d.context_window.used_percentage) { [int]$d.context_window.used_percentage } else { 0 }\n")
		b.WriteString("$parts += \"$pct%\"\n")
	}
	if cfg.ShowCost {
		b.WriteString("if ($null -ne $d.cost.total_cost_usd) { $parts += ('$' + [Math]::Round($d.cost.total_cost_usd, 3).ToString('0.000')) }\n")
	}
	if cfg.ShowGitBranch {
		b.WriteString("try { $br = (git branch --show-current 2>$null); if ($br) { $parts += \"git:$br\" } } catch {}\n")
	}
	if cfg.ShowDuration {
		b.WriteString("if ($null -ne $d.cost.total_duration_ms) { $ms = [int]$d.cost.total_duration_ms; $parts += ('' + [Math]::Floor($ms/60000) + 'm ' + [Math]::Floor(($ms%60000)/1000) + 's') }\n")
	}
	b.WriteString("Write-Host ($parts -join ' | ')\n")
	return b.String()
}

func buildStandardScript(cfg config.StatusLineSettings) string {
	var b strings.Builder
	b.WriteString("$d = $input | Out-String | ConvertFrom-Json\n")
	b.WriteString("$ESC = [char]27\n")
	b.WriteString("$GREEN = \"$ESC[32m\"; $YELLOW = \"$ESC[33m\"; $RED = \"$ESC[31m\"; $RESET = \"$ESC[0m\"\n")
	b.WriteString("$parts = @()\n")
	if cfg.ShowModel {
		b.WriteString("$model = if ($d.model.display_name) { $d.model.display_name } else { '?' }\n")
		b.WriteString("$parts += \"[$model]\"\n")
	}
	if cfg.ShowContext {
		b.WriteString("$pct = if ($null -ne $d.context_window.used_percentage) { [int]$d.context_window.used_percentage } else { 0 }\n")
		b.WriteString("$barColor = if ($pct -ge 90) { $RED } elseif ($pct -ge 70) { $YELLOW } else { $GREEN }\n")
		b.WriteString("$filled = [Math]::Floor($pct / 10)\n")
		b.WriteString("$filledBar = ([string][char]0x2588) * $filled\n")
		b.WriteString("$emptyBar = ([string][char]0x2591) * (10 - $filled)\n")
		b.WriteString("$bar = $barColor + $filledBar + $emptyBar + $RESET\n")
		b.WriteString("$parts += \"$bar $pct%\"\n")
	}
	if cfg.ShowCost {
		b.WriteString("if ($null -ne $d.cost.total_cost_usd) { $parts += ('$' + [Math]::Round($d.cost.total_cost_usd, 3).ToString('0.000')) }\n")
	}
	if cfg.ShowGitBranch {
		b.WriteString("try { $br = (git branch --show-current 2>$null); if ($br) { $parts += \"git:$br\" } } catch {}\n")
	}
	if cfg.ShowDuration {
		b.WriteString("if ($null -ne $d.cost.total_duration_ms) { $ms = [int]$d.cost.total_duration_ms; $parts += ('' + [Math]::Floor($ms/60000) + 'm ' + [Math]::Floor(($ms%60000)/1000) + 's') }\n")
	}
	b.WriteString("Write-Host ($parts -join ' | ')\n")
	return b.String()
}

func buildExtendedScript(cfg config.StatusLineSettings) string {
	var b strings.Builder
	b.WriteString("$d = $input | Out-String | ConvertFrom-Json\n")
	b.WriteString("$ESC = [char]27\n")
	b.WriteString("$CYAN = \"$ESC[36m\"; $GREEN = \"$ESC[32m\"; $YELLOW = \"$ESC[33m\"; $RED = \"$ESC[31m\"; $RESET = \"$ESC[0m\"\n")

	// Line 1: model + dir + git branch
	b.WriteString("$line1 = @()\n")
	if cfg.ShowModel {
		b.WriteString("$model = if ($d.model.display_name) { $d.model.display_name } else { '?' }\n")
		b.WriteString("$line1 += \"$CYAN[$model]$RESET\"\n")
	}
	b.WriteString("if ($d.workspace.current_dir) { $line1 += [System.IO.Path]::GetFileName($d.workspace.current_dir) }\n")
	if cfg.ShowGitBranch {
		b.WriteString("try { $br = (git branch --show-current 2>$null); if ($br) { $line1 += \"git:$br\" } } catch {}\n")
	}
	b.WriteString("Write-Host ($line1 -join ' | ')\n")

	// Line 2: context bar + cost + duration
	b.WriteString("$line2 = @()\n")
	if cfg.ShowContext {
		b.WriteString("$pct = if ($null -ne $d.context_window.used_percentage) { [int]$d.context_window.used_percentage } else { 0 }\n")
		b.WriteString("$barColor = if ($pct -ge 90) { $RED } elseif ($pct -ge 70) { $YELLOW } else { $GREEN }\n")
		b.WriteString("$filled = [Math]::Floor($pct / 10)\n")
		b.WriteString("$filledBar = ([string][char]0x2588) * $filled\n")
		b.WriteString("$emptyBar = ([string][char]0x2591) * (10 - $filled)\n")
		b.WriteString("$bar = $barColor + $filledBar + $emptyBar + $RESET\n")
		b.WriteString("$line2 += \"$bar $pct%\"\n")
	}
	if cfg.ShowCost {
		b.WriteString("if ($null -ne $d.cost.total_cost_usd) { $line2 += ('$' + [Math]::Round($d.cost.total_cost_usd, 3).ToString('0.000')) }\n")
	}
	if cfg.ShowDuration {
		b.WriteString("if ($null -ne $d.cost.total_duration_ms) { $ms = [int]$d.cost.total_duration_ms; $line2 += ('' + [Math]::Floor($ms/60000) + 'm ' + [Math]::Floor(($ms%60000)/1000) + 's') }\n")
	}
	b.WriteString("if ($line2.Count -gt 0) { Write-Host ($line2 -join ' | ') }\n")
	return b.String()
}

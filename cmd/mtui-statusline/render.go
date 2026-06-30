// cmd/mtui-statusline/render.go
// Pure rendering function — byte-parity with buildStatusLineScript (PowerShell).
package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

// RenderConfig controls which segments appear and which template is used.
type RenderConfig struct {
	Template      string // "minimal" | "standard" | "extended"
	ShowModel     bool
	ShowContext    bool
	ShowCost      bool
	ShowGitBranch bool
	ShowDuration  bool
}

// Status carries live data from Claude. Pointer fields are nil when absent
// (mirrors PowerShell's $null checks).
type Status struct {
	Model      string
	ContextPct *int     // nil → treat as absent
	CostUSD    *float64 // nil → omit cost segment
	DurationMs *int     // nil → omit duration segment
	CurrentDir string
}

// ANSI constants — match PowerShell exactly.
const (
	colorReset  = "\x1b[0m"
	colorRed    = "\x1b[31m"
	colorYellow = "\x1b[33m"
	colorGreen  = "\x1b[32m"
	colorCyan   = "\x1b[36m"

	barFull  = "█"
	barEmpty = "░"
)

// Render returns the fully rendered status line, including the trailing newline
// that PowerShell's Write-Host emits. For "extended" it may return two
// newline-terminated lines joined.
func Render(cfg RenderConfig, s Status, gitBranch string) string {
	switch cfg.Template {
	case "extended":
		return renderExtended(cfg, s, gitBranch)
	case "minimal":
		return renderMinimal(cfg, s, gitBranch)
	default: // "standard"
		return renderStandard(cfg, s, gitBranch)
	}
}

// renderMinimal builds a single line with no bar/color for context.
func renderMinimal(cfg RenderConfig, s Status, gitBranch string) string {
	var segs []string

	if cfg.ShowModel {
		segs = append(segs, modelSegment(s.Model))
	}
	if cfg.ShowContext && s.ContextPct != nil {
		segs = append(segs, fmt.Sprintf("%d%%", *s.ContextPct))
	}
	if cfg.ShowCost && s.CostUSD != nil {
		segs = append(segs, costSegment(*s.CostUSD))
	}
	if cfg.ShowGitBranch && gitBranch != "" {
		segs = append(segs, "git:"+gitBranch)
	}
	if cfg.ShowDuration && s.DurationMs != nil {
		segs = append(segs, durationSegment(*s.DurationMs))
	}

	return strings.Join(segs, " | ") + "\n"
}

// renderStandard builds a single line with colored bar for context.
func renderStandard(cfg RenderConfig, s Status, gitBranch string) string {
	var segs []string

	if cfg.ShowModel {
		segs = append(segs, modelSegment(s.Model))
	}
	if cfg.ShowContext && s.ContextPct != nil {
		segs = append(segs, barContextSegment(*s.ContextPct))
	}
	if cfg.ShowCost && s.CostUSD != nil {
		segs = append(segs, costSegment(*s.CostUSD))
	}
	if cfg.ShowGitBranch && gitBranch != "" {
		segs = append(segs, "git:"+gitBranch)
	}
	if cfg.ShowDuration && s.DurationMs != nil {
		segs = append(segs, durationSegment(*s.DurationMs))
	}

	return strings.Join(segs, " | ") + "\n"
}

// renderExtended builds two lines:
//
//	line1: [cyan model?] | dir-basename? | git?
//	line2: bar+pct? | cost? | duration?  (omitted entirely if empty)
func renderExtended(cfg RenderConfig, s Status, gitBranch string) string {
	var line1Segs []string

	if cfg.ShowModel {
		m := s.Model
		if m == "" {
			m = "?"
		}
		line1Segs = append(line1Segs, colorCyan+"["+m+"]"+colorReset)
	}
	if s.CurrentDir != "" {
		line1Segs = append(line1Segs, filepath.Base(s.CurrentDir))
	}
	if cfg.ShowGitBranch && gitBranch != "" {
		line1Segs = append(line1Segs, "git:"+gitBranch)
	}

	line1 := strings.Join(line1Segs, " | ") + "\n"

	var line2Segs []string
	if cfg.ShowContext && s.ContextPct != nil {
		line2Segs = append(line2Segs, barContextSegment(*s.ContextPct))
	}
	if cfg.ShowCost && s.CostUSD != nil {
		line2Segs = append(line2Segs, costSegment(*s.CostUSD))
	}
	if cfg.ShowDuration && s.DurationMs != nil {
		line2Segs = append(line2Segs, durationSegment(*s.DurationMs))
	}

	if len(line2Segs) == 0 {
		return line1
	}
	line2 := strings.Join(line2Segs, " | ") + "\n"
	return line1 + line2
}

// modelSegment returns "[<model>]" or "[?]" when model is empty.
func modelSegment(model string) string {
	if model == "" {
		model = "?"
	}
	return "[" + model + "]"
}

// barContextSegment returns the colored 10-char bar + pct string.
// bar = █×floor(pct/10) + ░×(10−floor(pct/10))
// color: red if pct≥90, yellow if pct≥70, else green.
func barContextSegment(pct int) string {
	filled := pct / 10
	if filled > 10 {
		filled = 10
	}
	empty := 10 - filled
	bar := strings.Repeat(barFull, filled) + strings.Repeat(barEmpty, empty)

	var color string
	switch {
	case pct >= 90:
		color = colorRed
	case pct >= 70:
		color = colorYellow
	default:
		color = colorGreen
	}

	return color + bar + colorReset + fmt.Sprintf(" %d%%", pct)
}

// costSegment formats cost as "$0.420".
func costSegment(cost float64) string {
	return fmt.Sprintf("$%.3f", cost)
}

// durationSegment formats ms as "<m>m <s>s".
func durationSegment(ms int) string {
	m := ms / 60000
	s := (ms % 60000) / 1000
	return fmt.Sprintf("%dm %ds", m, s)
}

// Package backend – Auto-detection of the Codex CLI binary (OpenAI).
package backend

import (
	"log"
	"os/exec"
	"path/filepath"
	"runtime"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

// CodexDetectResult describes the outcome of a Codex CLI detection attempt.
type CodexDetectResult struct {
	Path   string `json:"path"`
	Source string `json:"source"` // "config", "path", "search"
	Valid  bool   `json:"valid"`
}

// DetectCodexPath tries to locate the Codex CLI binary.
// Priority: absolute config path → PATH lookup → known install locations.
func (a *App) DetectCodexPath() CodexDetectResult {
	cmd := a.cfg.CodexCommand
	if cmd == "" {
		cmd = "codex"
	}

	// 1. If the configured command is an absolute path, validate directly
	if filepath.IsAbs(cmd) {
		if fileIsExecutable(cmd) {
			return CodexDetectResult{Path: cmd, Source: "config", Valid: true}
		}
		return CodexDetectResult{Path: cmd, Source: "config", Valid: false}
	}

	// 2. Try PATH lookup
	if p, err := exec.LookPath(cmd); err == nil {
		abs, _ := filepath.Abs(p)
		if abs != "" {
			p = abs
		}
		return CodexDetectResult{Path: p, Source: "path", Valid: true}
	}

	// 3. Search known installation locations
	for _, candidate := range codexSearchPaths() {
		if fileIsExecutable(candidate) {
			return CodexDetectResult{Path: candidate, Source: "search", Valid: true}
		}
	}

	return CodexDetectResult{Source: "search", Valid: false}
}

// GetResolvedCodexPath returns the cached resolved path (set at startup).
func (a *App) GetResolvedCodexPath() string {
	return a.resolvedCodexPath
}

// IsCodexDetected returns whether Codex CLI was found during detection.
func (a *App) IsCodexDetected() bool {
	return a.codexDetected
}

// BrowseForCodex opens a native file picker filtered for executables.
func (a *App) BrowseForCodex() string {
	filters := []wailsrt.FileFilter{
		{DisplayName: "Executables", Pattern: "*"},
	}
	if runtime.GOOS == "windows" {
		filters[0].Pattern = "*.exe;*.cmd;*.bat"
	}
	path, err := wailsrt.OpenFileDialog(a.ctx, wailsrt.OpenDialogOptions{
		Title:   "Codex CLI auswählen",
		Filters: filters,
	})
	if err != nil || path == "" {
		return ""
	}
	return path
}

// resolveCodexOnStartup runs detection and caches the result.
func (a *App) resolveCodexOnStartup() {
	result := a.DetectCodexPath()
	if result.Valid {
		a.resolvedCodexPath = result.Path
		a.codexDetected = true
		log.Printf("[CodexDetect] found via %s: %s", result.Source, result.Path)
	} else {
		a.resolvedCodexPath = a.cfg.CodexCommand
		if a.resolvedCodexPath == "" {
			a.resolvedCodexPath = "codex"
		}
		a.codexDetected = false
		log.Printf("[CodexDetect] not found, falling back to %q", a.resolvedCodexPath)
	}
}

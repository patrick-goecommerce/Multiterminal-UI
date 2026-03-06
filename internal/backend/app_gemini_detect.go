// Package backend – Auto-detection of the Gemini CLI binary (Google).
package backend

import (
	"log"
	"os/exec"
	"path/filepath"
	"runtime"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

// GeminiDetectResult describes the outcome of a Gemini CLI detection attempt.
type GeminiDetectResult struct {
	Path   string `json:"path"`
	Source string `json:"source"` // "config", "path", "search"
	Valid  bool   `json:"valid"`
}

// DetectGeminiPath tries to locate the Gemini CLI binary.
// Priority: absolute config path → PATH lookup → known install locations.
func (a *App) DetectGeminiPath() GeminiDetectResult {
	cmd := a.cfg.GeminiCommand
	if cmd == "" {
		cmd = "gemini"
	}

	// 1. If the configured command is an absolute path, validate directly
	if filepath.IsAbs(cmd) {
		if fileIsExecutable(cmd) {
			return GeminiDetectResult{Path: cmd, Source: "config", Valid: true}
		}
		return GeminiDetectResult{Path: cmd, Source: "config", Valid: false}
	}

	// 2. Try PATH lookup
	if p, err := exec.LookPath(cmd); err == nil {
		abs, _ := filepath.Abs(p)
		if abs != "" {
			p = abs
		}
		return GeminiDetectResult{Path: p, Source: "path", Valid: true}
	}

	// 3. Search known installation locations
	for _, candidate := range geminiSearchPaths() {
		if fileIsExecutable(candidate) {
			return GeminiDetectResult{Path: candidate, Source: "search", Valid: true}
		}
	}

	return GeminiDetectResult{Source: "search", Valid: false}
}

// GetResolvedGeminiPath returns the cached resolved path (set at startup).
func (a *App) GetResolvedGeminiPath() string {
	return a.resolvedGeminiPath
}

// IsGeminiDetected returns whether Gemini CLI was found during detection.
func (a *App) IsGeminiDetected() bool {
	return a.geminiDetected
}

// BrowseForGemini opens a native file picker filtered for executables.
func (a *App) BrowseForGemini() string {
	filters := []wailsrt.FileFilter{
		{DisplayName: "Executables", Pattern: "*"},
	}
	if runtime.GOOS == "windows" {
		filters[0].Pattern = "*.exe;*.cmd;*.bat"
	}
	path, err := wailsrt.OpenFileDialog(a.ctx, wailsrt.OpenDialogOptions{
		Title:   "Gemini CLI auswählen",
		Filters: filters,
	})
	if err != nil || path == "" {
		return ""
	}
	return path
}

// ValidateGeminiPath checks if the given path points to a valid executable.
func (a *App) ValidateGeminiPath(path string) bool {
	return fileIsExecutable(path)
}

// resolveGeminiOnStartup runs detection and caches the result.
func (a *App) resolveGeminiOnStartup() {
	result := a.DetectGeminiPath()
	if result.Valid {
		a.resolvedGeminiPath = result.Path
		a.geminiDetected = true
		log.Printf("[GeminiDetect] found via %s: %s", result.Source, result.Path)
	} else {
		a.resolvedGeminiPath = a.cfg.GeminiCommand
		if a.resolvedGeminiPath == "" {
			a.resolvedGeminiPath = "gemini"
		}
		a.geminiDetected = false
		log.Printf("[GeminiDetect] not found, falling back to %q", a.resolvedGeminiPath)
	}
}

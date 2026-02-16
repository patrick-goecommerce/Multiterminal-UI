// Package backend – Auto-detection of the Claude CLI binary.
package backend

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ClaudeDetectResult describes the outcome of a Claude CLI detection attempt.
type ClaudeDetectResult struct {
	Path   string `json:"path"`
	Source string `json:"source"` // "config", "path", "search", "manual"
	Valid  bool   `json:"valid"`
}

// DetectClaudePath tries to locate the Claude CLI binary.
// Priority: absolute config path → PATH lookup → known install locations.
func (a *App) DetectClaudePath() ClaudeDetectResult {
	cmd := a.cfg.ClaudeCommand
	if cmd == "" {
		cmd = "claude"
	}

	// 1. If the configured command is an absolute path, validate directly
	if filepath.IsAbs(cmd) {
		if fileIsExecutable(cmd) {
			return ClaudeDetectResult{Path: cmd, Source: "config", Valid: true}
		}
		return ClaudeDetectResult{Path: cmd, Source: "config", Valid: false}
	}

	// 2. Try PATH lookup
	if p, err := exec.LookPath(cmd); err == nil {
		abs, _ := filepath.Abs(p)
		if abs != "" {
			p = abs
		}
		return ClaudeDetectResult{Path: p, Source: "path", Valid: true}
	}

	// 3. Search known installation locations
	for _, candidate := range claudeSearchPaths() {
		if fileIsExecutable(candidate) {
			return ClaudeDetectResult{Path: candidate, Source: "search", Valid: true}
		}
	}

	return ClaudeDetectResult{Source: "search", Valid: false}
}

// GetResolvedClaudePath returns the cached resolved path (set at startup).
func (a *App) GetResolvedClaudePath() string {
	return a.resolvedClaudePath
}

// ValidateClaudePath checks whether the given path points to an executable.
func (a *App) ValidateClaudePath(path string) bool {
	if path == "" {
		return false
	}
	if filepath.IsAbs(path) {
		return fileIsExecutable(path)
	}
	_, err := exec.LookPath(path)
	return err == nil
}

// BrowseForClaude opens a native file picker filtered for executables.
func (a *App) BrowseForClaude() string {
	filters := []wailsrt.FileFilter{
		{DisplayName: "Executables", Pattern: "*"},
	}
	if runtime.GOOS == "windows" {
		filters[0].Pattern = "*.exe;*.cmd;*.bat"
	}
	path, err := wailsrt.OpenFileDialog(a.ctx, wailsrt.OpenDialogOptions{
		Title:   "Claude CLI auswählen",
		Filters: filters,
	})
	if err != nil || path == "" {
		return ""
	}
	return path
}

// IsClaudeDetected returns whether Claude CLI was found during detection.
func (a *App) IsClaudeDetected() bool {
	return a.claudeDetected
}

// resolveClaudeOnStartup runs detection and caches the result.
func (a *App) resolveClaudeOnStartup() {
	result := a.DetectClaudePath()
	if result.Valid {
		a.resolvedClaudePath = result.Path
		a.claudeDetected = true
		log.Printf("[ClaudeDetect] found via %s: %s", result.Source, result.Path)
	} else {
		a.resolvedClaudePath = a.cfg.ClaudeCommand
		if a.resolvedClaudePath == "" {
			a.resolvedClaudePath = "claude"
		}
		a.claudeDetected = false
		log.Printf("[ClaudeDetect] not found, falling back to %q", a.resolvedClaudePath)
	}
}

// fileIsExecutable checks if a file exists and is not a directory.
func fileIsExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// expandEnv resolves environment variables in a path, returning "" if
// the primary variable is unset.
func expandEnv(envVar, suffix string) string {
	val := os.Getenv(envVar)
	if val == "" {
		return ""
	}
	return filepath.Join(val, suffix)
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, path[1:])
}

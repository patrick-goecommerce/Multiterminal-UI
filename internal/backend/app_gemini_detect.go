// Package backend – Auto-detection of the Gemini CLI binary (Google).
package backend

import (
	"log"
	"os/exec"
	"path/filepath"
)

// GeminiDetectResult describes the outcome of a Gemini CLI detection attempt.
type GeminiDetectResult struct {
	Path   string `json:"path"`
	Source string `json:"source"` // "config", "path", "search"
	Valid  bool   `json:"valid"`
}

// DetectGeminiPath tries to locate the Gemini CLI binary.
// Priority: absolute config path → PATH lookup → known install locations.
func (a *AppService) DetectGeminiPath() GeminiDetectResult {
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
func (a *AppService) GetResolvedGeminiPath() string {
	return a.resolvedGeminiPath
}

// IsGeminiDetected returns whether Gemini CLI was found during detection.
func (a *AppService) IsGeminiDetected() bool {
	return a.geminiDetected
}

// BrowseForGemini opens a native file picker filtered for executables.
func (a *AppService) BrowseForGemini() string {
	// TODO(wails-v3): migrate to v3 dialog API
	return ""
}

// ValidateGeminiPath checks if the given path points to a valid executable.
func (a *AppService) ValidateGeminiPath(path string) bool {
	return fileIsExecutable(path)
}

// resolveGeminiOnStartup runs detection and caches the result.
func (a *AppService) resolveGeminiOnStartup() {
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

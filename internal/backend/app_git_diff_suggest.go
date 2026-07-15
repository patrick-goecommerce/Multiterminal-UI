package backend

import (
	"fmt"
	"path/filepath"
	"strings"
)

// inferCommitType determines the conventional commit type from file stats.
func inferCommitType(stats []DiffFileStat) string {
	hasNew, hasTest, hasDocs, hasConfig := false, false, false, false
	for _, s := range stats {
		base := filepath.Base(s.Path)
		ext := filepath.Ext(s.Path)
		if s.Status == "?" || s.Status == "A" {
			hasNew = true
		}
		if strings.HasSuffix(base, "_test.go") || strings.HasSuffix(base, ".test.ts") ||
			strings.HasSuffix(base, ".spec.ts") {
			hasTest = true
		}
		if ext == ".md" || ext == ".txt" || strings.Contains(s.Path, "docs/") {
			hasDocs = true
		}
		if ext == ".yaml" || ext == ".yml" || ext == ".json" || ext == ".toml" ||
			base == ".gitignore" || strings.Contains(s.Path, "config") {
			hasConfig = true
		}
	}
	if hasTest {
		return "test"
	}
	if hasDocs {
		return "docs"
	}
	if hasConfig {
		return "chore"
	}
	if hasNew {
		return "feat"
	}
	return "fix"
}

// inferScope extracts a scope from the file paths.
func inferScope(stats []DiffFileStat) string {
	scopeMap := map[string]string{
		"backend":    "backend",
		"terminal":   "terminal",
		"config":     "config",
		"components": "ui",
		"stores":     "ui",
		"lib":        "ui",
	}

	scopes := make(map[string]int)
	for _, s := range stats {
		parts := strings.Split(filepath.ToSlash(s.Path), "/")
		for _, p := range parts {
			if mapped, ok := scopeMap[p]; ok {
				scopes[mapped]++
				break
			}
		}
	}

	// Find the most common scope
	best, bestCount := "", 0
	for scope, count := range scopes {
		if count > bestCount {
			best = scope
			bestCount = count
		}
	}
	return best
}

// inferDescription generates a short commit description.
func inferDescription(stats []DiffFileStat) string {
	if len(stats) == 1 {
		s := stats[0]
		base := filepath.Base(s.Path)
		switch s.Status {
		case "?", "A":
			return "add " + base
		case "D":
			return "remove " + base
		default:
			return "update " + base
		}
	}

	added, updated, removed := 0, 0, 0
	for _, s := range stats {
		switch s.Status {
		case "?", "A":
			added++
		case "D":
			removed++
		default:
			updated++
		}
	}

	var parts []string
	if added > 0 {
		parts = append(parts, fmt.Sprintf("add %d file(s)", added))
	}
	if updated > 0 {
		parts = append(parts, fmt.Sprintf("update %d file(s)", updated))
	}
	if removed > 0 {
		parts = append(parts, fmt.Sprintf("remove %d file(s)", removed))
	}
	if len(parts) == 0 {
		return "update files"
	}
	return strings.Join(parts, ", ")
}

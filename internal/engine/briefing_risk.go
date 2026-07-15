package engine

import (
	"fmt"
	"path/filepath"
	"strings"
)

// findCriticalFiles checks which changed files match critical patterns.
func findCriticalFiles(files []string, patterns []string) []string {
	var result []string
	for _, f := range files {
		for _, pat := range patterns {
			if matchCriticalPattern(f, pat) {
				result = append(result, f)
				break
			}
		}
	}
	return result
}

// matchCriticalPattern matches a file path against a critical pattern.
// Supports ** prefix (match any directory depth) and filepath.Match for the rest.
func matchCriticalPattern(file, pattern string) bool {
	// Handle ** patterns: strip the prefix and match the tail against
	// the filename and every directory suffix of the file path.
	if strings.HasPrefix(pattern, "**/") {
		tail := pattern[3:]
		// Check the full path and every suffix after a separator.
		if matchSimple(file, tail) {
			return true
		}
		for i := 0; i < len(file); i++ {
			if file[i] == '/' || file[i] == '\\' {
				if matchSimple(file[i+1:], tail) {
					return true
				}
			}
		}
		return false
	}
	return matchSimple(file, pattern)
}

// matchSimple wraps filepath.Match, also checking just the base name for
// simple (non-path) patterns.
func matchSimple(file, pattern string) bool {
	// If pattern contains no path separator, match against base name too.
	if !strings.Contains(pattern, "/") && !strings.Contains(pattern, "\\") {
		base := filepath.Base(file)
		if ok, _ := filepath.Match(pattern, base); ok {
			return true
		}
	}
	ok, _ := filepath.Match(pattern, file)
	return ok
}

// assessConflictRisk compares changed files with other active cards.
// high: same file touched by another card.
// medium: same directory as another active card.
// low: no overlap.
func assessConflictRisk(changedFiles []string, activeCardFiles map[string][]string) (risk string, sharedSurfaces []string) {
	if len(activeCardFiles) == 0 {
		return "low", nil
	}

	// Build sets for quick lookup.
	changedSet := make(map[string]bool, len(changedFiles))
	changedDirs := make(map[string]bool, len(changedFiles))
	for _, f := range changedFiles {
		changedSet[f] = true
		changedDirs[filepath.Dir(f)] = true
	}

	fileOverlap := false
	dirOverlap := false

	seen := make(map[string]bool)
	for cardID, files := range activeCardFiles {
		for _, f := range files {
			if changedSet[f] {
				fileOverlap = true
				key := fmt.Sprintf("%s (card %s)", f, cardID)
				if !seen[key] {
					sharedSurfaces = append(sharedSurfaces, key)
					seen[key] = true
				}
			} else if changedDirs[filepath.Dir(f)] {
				dirOverlap = true
				dir := filepath.Dir(f)
				key := fmt.Sprintf("%s/ (card %s)", dir, cardID)
				if !seen[key] {
					sharedSurfaces = append(sharedSurfaces, key)
					seen[key] = true
				}
			}
		}
	}

	switch {
	case fileOverlap:
		return "high", sharedSurfaces
	case dirOverlap:
		return "medium", sharedSurfaces
	default:
		return "low", nil
	}
}

// computeRecommendation determines the final recommendation.
func computeRecommendation(b DecisionBriefing) string {
	if len(b.SecretsFound) > 0 {
		return "needs_human_review"
	}
	if b.DependencyRisk == DependencyRiskHigh {
		return "needs_human_review"
	}
	if b.ConflictRisk == "high" {
		return "needs_human_review"
	}
	if b.ScopeStatus == "exceeded" {
		return "needs_human_review"
	}
	if b.MassDeletes {
		return "needs_human_review"
	}
	return "proceed_to_qa"
}

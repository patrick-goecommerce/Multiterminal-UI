package engine

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/orchestrator"
)

// ScopeCheckResult describes whether a step stayed within limits.
type ScopeCheckResult struct {
	WithinLimits bool     `json:"within_limits"`
	FilesChanged int      `json:"files_changed"`
	LinesChanged int      `json:"lines_changed"`
	Deletes      int      `json:"deletes"`
	Violations   []string `json:"violations"`
}

// ScopeLimitDef defines the maximum allowed changes for a card type.
type ScopeLimitDef struct {
	MaxFiles   int
	MaxLines   int
	MaxDeletes int
}

// DefaultScopeLimits per card type (from spec).
var DefaultScopeLimits = map[string]ScopeLimitDef{
	"bugfix":   {MaxFiles: 10, MaxLines: 200, MaxDeletes: 50},
	"feature":  {MaxFiles: 25, MaxLines: 500, MaxDeletes: 100},
	"refactor": {MaxFiles: 40, MaxLines: 800, MaxDeletes: 400},
}

// deleteExcludePatterns lists patterns for files excluded from delete counting.
var deleteExcludePatterns = []string{
	"*_generated.go", "vendor/**", "dist/**", "build/**",
	"*.min.js", "*.min.css", "node_modules/**",
}

// CheckScope verifies a step's changes against scope limits.
// workDir is the worktree where changes were made.
// cardType is "bugfix", "feature", or "refactor" — determines limits.
// Returns the check result (never errors — if git fails, assumes within limits).
func CheckScope(ctx context.Context, workDir, cardType string) ScopeCheckResult {
	limits, ok := DefaultScopeLimits[cardType]
	if !ok {
		limits = DefaultScopeLimits["feature"]
	}

	files, added, deleted := parseDiffStat(ctx, workDir)

	// Filter out renames from delete count.
	renames := countRenames(ctx, workDir)
	effectiveDeletes := deleted - renames
	if effectiveDeletes < 0 {
		effectiveDeletes = 0
	}

	// Filter out excluded patterns from delete count.
	excludedDeletes := countExcludedDeletes(ctx, workDir, deleteExcludePatterns)
	effectiveDeletes -= excludedDeletes
	if effectiveDeletes < 0 {
		effectiveDeletes = 0
	}

	result := ScopeCheckResult{
		FilesChanged: files,
		LinesChanged: added + deleted,
		Deletes:      effectiveDeletes,
		WithinLimits: true,
	}

	if files > limits.MaxFiles {
		result.WithinLimits = false
		result.Violations = append(result.Violations,
			fmt.Sprintf("files: %d exceeds limit %d", files, limits.MaxFiles))
	}
	if added+deleted > limits.MaxLines {
		result.WithinLimits = false
		result.Violations = append(result.Violations,
			fmt.Sprintf("lines: %d exceeds limit %d", added+deleted, limits.MaxLines))
	}
	if effectiveDeletes > limits.MaxDeletes {
		result.WithinLimits = false
		result.Violations = append(result.Violations,
			fmt.Sprintf("deletes: %d exceeds limit %d", effectiveDeletes, limits.MaxDeletes))
	}

	return result
}

// parseDiffStat runs git diff --numstat and extracts file count, additions, deletions.
func parseDiffStat(ctx context.Context, workDir string) (files, added, deleted int) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--cached", "--numstat")
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, 0
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		// Binary files show "-" for added/deleted.
		a, errA := strconv.Atoi(parts[0])
		d, errD := strconv.Atoi(parts[1])
		if errA != nil || errD != nil {
			files++ // binary file still counts
			continue
		}
		files++
		added += a
		deleted += d
	}
	return files, added, deleted
}

// countRenames runs git diff --cached --diff-filter=R to count renamed files.
func countRenames(ctx context.Context, workDir string) int {
	cmd := exec.CommandContext(ctx,
		"git", "diff", "--cached", "--diff-filter=R", "--name-only")
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			count++
		}
	}
	return count
}

// countExcludedDeletes counts deleted lines in files matching exclude patterns.
func countExcludedDeletes(ctx context.Context, workDir string, patterns []string) int {
	cmd := exec.CommandContext(ctx, "git", "diff", "--cached", "--numstat")
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return 0
	}

	total := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		file := parts[2]
		d, err := strconv.Atoi(parts[1])
		if err != nil || d == 0 {
			continue
		}
		if matchesAnyPattern(file, patterns) {
			total += d
		}
	}
	return total
}

// matchesAnyPattern checks if a file path matches any of the glob patterns.
func matchesAnyPattern(file string, patterns []string) bool {
	for _, pat := range patterns {
		if matched, _ := filepath.Match(pat, file); matched {
			return true
		}
		// Also match against just the filename for patterns like "*_generated.go".
		if matched, _ := filepath.Match(pat, filepath.Base(file)); matched {
			return true
		}
	}
	return false
}

// FileConflict describes an overlap where two steps modify the same file.
type FileConflict struct {
	StepA string
	StepB string
	File  string
}

// DetectFileConflicts checks if steps within a wave have files_modify overlaps.
func DetectFileConflicts(steps []orchestrator.PlanStep) []FileConflict {
	var conflicts []FileConflict
	for i := 0; i < len(steps); i++ {
		for j := i + 1; j < len(steps); j++ {
			for _, fA := range steps[i].FilesModify {
				for _, fB := range steps[j].FilesModify {
					if fA == fB {
						conflicts = append(conflicts, FileConflict{
							StepA: steps[i].ID,
							StepB: steps[j].ID,
							File:  fA,
						})
					}
				}
			}
		}
	}
	return conflicts
}

// SplitConflictingSteps separates conflicting steps from a wave.
// Returns safe steps (can run in parallel) and deferred steps (must wait).
// The step with the later ID (alphabetically) gets deferred.
func SplitConflictingSteps(steps []orchestrator.PlanStep) (safe, deferred []orchestrator.PlanStep) {
	conflicts := DetectFileConflicts(steps)
	if len(conflicts) == 0 {
		return steps, nil
	}

	deferredIDs := map[string]bool{}
	for _, c := range conflicts {
		if c.StepA > c.StepB {
			deferredIDs[c.StepA] = true
		} else {
			deferredIDs[c.StepB] = true
		}
	}

	for _, s := range steps {
		if deferredIDs[s.ID] {
			deferred = append(deferred, s)
		} else {
			safe = append(safe, s)
		}
	}
	return safe, deferred
}

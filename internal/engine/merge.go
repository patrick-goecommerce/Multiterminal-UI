package engine

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// MergeResult describes the outcome of merging wave worktrees.
type MergeResult struct {
	Success          bool             `json:"success"`
	MergedBranches   []string         `json:"merged_branches"`
	Conflicts        []MergeConflict  `json:"conflicts"`
	Reconciliation   *ReconcileResult `json:"reconciliation,omitempty"`
	NeedsHumanReview bool             `json:"needs_human_review"`
	Reason           string           `json:"reason,omitempty"`
}

// MergeConflict describes a single merge conflict.
type MergeConflict struct {
	File       string `json:"file"`
	IsCritical bool   `json:"is_critical"`
	HunkCount  int    `json:"hunk_count"`
	LinesTotal int    `json:"lines_total"`
}

// ReconcileResult describes the outcome of dependency reconciliation.
type ReconcileResult struct {
	Command string           `json:"command"`
	Success bool             `json:"success"`
	Risk    DependencyRisk   `json:"risk"`
	Changes []ManifestChange `json:"changes"`
}

// AI merge limits (from spec: "Kleiner Konflikt" definition).
const (
	MaxAIMergeFiles        = 3  // max files with conflicts
	MaxAIMergeHunksPerFile = 2  // max conflict hunks per file
	MaxAIMergeLinesPerHunk = 20 // max lines per conflict hunk
)

// MergeWave merges worktree branches into the target branch sequentially.
// targetBranch is the card branch (NOT main).
// worktreeBranches are the step branches to merge in order.
func MergeWave(ctx context.Context, repoDir, targetBranch string, worktreeBranches []string) MergeResult {
	result := MergeResult{}

	// Checkout card branch first.
	if err := gitCheckout(ctx, repoDir, targetBranch); err != nil {
		result.NeedsHumanReview = true
		result.Reason = fmt.Sprintf("checkout %s failed: %v", targetBranch, err)
		return result
	}

	for _, branch := range worktreeBranches {
		if err := attemptMerge(ctx, repoDir, branch); err != nil {
			conflicts := analyzeConflicts(ctx, repoDir)
			abortMerge(ctx, repoDir)
			result.Conflicts = append(result.Conflicts, conflicts...)
			result.NeedsHumanReview = true

			if canAIMerge(conflicts) {
				result.Reason = fmt.Sprintf(
					"merge conflict in branch %s (AI merge not yet implemented)", branch)
			} else {
				result.Reason = fmt.Sprintf(
					"structural conflict in branch %s exceeds AI merge limits", branch)
			}
			return result
		}
		result.MergedBranches = append(result.MergedBranches, branch)
	}

	// All merged successfully — run dependency reconciliation.
	result.Reconciliation = reconcileDependencies(ctx, repoDir)
	if result.Reconciliation != nil && result.Reconciliation.Risk == DependencyRiskHigh {
		result.NeedsHumanReview = true
		result.Reason = "dependency reconciliation caused high-risk changes"
	}

	result.Success = !result.NeedsHumanReview
	return result
}

// gitCheckout switches the repo to the given branch.
func gitCheckout(ctx context.Context, repoDir, branch string) error {
	cmd := exec.CommandContext(ctx, "git", "checkout", branch)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", branch, strings.TrimSpace(string(out)))
	}
	return nil
}

// attemptMerge tries to merge sourceBranch into the current branch.
func attemptMerge(ctx context.Context, repoDir, sourceBranch string) error {
	cmd := exec.CommandContext(ctx, "git", "merge", sourceBranch, "--no-edit")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", sourceBranch, strings.TrimSpace(string(out)))
	}
	return nil
}

// analyzeConflicts inspects unmerged files and counts conflict markers.
func analyzeConflicts(ctx context.Context, repoDir string) []MergeConflict {
	// List unmerged files via git diff --name-only --diff-filter=U.
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "--diff-filter=U")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	var conflicts []MergeConflict
	for _, file := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if file == "" {
			continue
		}
		hunks, lines := countConflictMarkers(filepath.Join(repoDir, file))
		conflicts = append(conflicts, MergeConflict{
			File:       file,
			IsCritical: isCriticalFile(file),
			HunkCount:  hunks,
			LinesTotal: lines,
		})
	}
	return conflicts
}

// countConflictMarkers scans a file for <<<<<<< / >>>>>>> pairs.
func countConflictMarkers(path string) (hunks, lines int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	inConflict := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "<<<<<<<") {
			hunks++
			inConflict = true
			continue
		}
		if strings.HasPrefix(line, ">>>>>>>") {
			inConflict = false
			continue
		}
		if inConflict {
			lines++
		}
	}
	return hunks, lines
}

// isCriticalFile checks if a file matches any critical pattern.
func isCriticalFile(file string) bool {
	return len(findCriticalFiles([]string{file}, DefaultCriticalFilePatterns)) > 0
}

// canAIMerge checks if conflicts are within AI merge limits.
func canAIMerge(conflicts []MergeConflict) bool {
	if len(conflicts) > MaxAIMergeFiles {
		return false
	}
	for _, c := range conflicts {
		if c.IsCritical {
			return false
		}
		if c.HunkCount > MaxAIMergeHunksPerFile {
			return false
		}
		if c.LinesTotal > MaxAIMergeLinesPerHunk*c.HunkCount {
			return false
		}
	}
	return true
}

// abortMerge aborts a failed merge to restore clean repo state.
func abortMerge(ctx context.Context, repoDir string) {
	cmd := exec.CommandContext(ctx, "git", "merge", "--abort")
	cmd.Dir = repoDir
	_ = cmd.Run()
}

// reconcileDependencies runs dependency tools after a successful wave merge.
// Currently supports go mod tidy only.
func reconcileDependencies(ctx context.Context, repoDir string) *ReconcileResult {
	gomod := filepath.Join(repoDir, "go.mod")
	if _, err := os.Stat(gomod); err != nil {
		return nil // no manifest files found
	}

	result := &ReconcileResult{Command: "go mod tidy"}
	cmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	cmd.Dir = repoDir
	if _, err := cmd.CombinedOutput(); err != nil {
		result.Success = false
		result.Risk = DependencyRiskHigh
		return result
	}
	result.Success = true

	// Check what changed.
	changedFiles := getChangedFiles(ctx, repoDir)
	if len(changedFiles) > 0 {
		result.Changes, result.Risk = ParseManifestChanges(ctx, repoDir, changedFiles)
		commitReconciliation(ctx, repoDir)
	} else {
		result.Risk = DependencyRiskNone
	}
	return result
}

// getChangedFiles returns unstaged changed file names.
func getChangedFiles(ctx context.Context, repoDir string) []string {
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

// commitReconciliation stages and commits dependency reconciliation changes.
func commitReconciliation(ctx context.Context, repoDir string) {
	add := exec.CommandContext(ctx, "git", "add", "-A")
	add.Dir = repoDir
	_ = add.Run()

	commit := exec.CommandContext(ctx, "git", "commit", "-m",
		"chore: dependency reconciliation after wave merge")
	commit.Dir = repoDir
	_ = commit.Run()
}

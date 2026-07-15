package engine

import (
	"context"
	"os/exec"
	"strconv"
	"strings"
)

// DecisionBriefing is the Pre-QA gate assessment.
type DecisionBriefing struct {
	CardID       string `json:"card_id"`
	FilesChanged int    `json:"files_changed"`
	LinesAdded   int    `json:"lines_added"`
	LinesDeleted int    `json:"lines_deleted"`
	ScopeStatus  string `json:"scope_status"` // "within_limits" | "exceeded" | "warning"

	SecretsFound []SecretFinding `json:"secrets_found"`
	MassDeletes  bool            `json:"mass_deletes"`

	LoopHistory []string `json:"loop_history"` // summary of detected loops

	ConflictRisk   string   `json:"conflict_risk"`   // "low" | "medium" | "high"
	CriticalFiles  []string `json:"critical_files"`
	SharedSurfaces []string `json:"shared_surfaces"`

	DependencyRisk  DependencyRisk   `json:"dependency_risk"`
	ManifestChanges []ManifestChange `json:"manifest_changes"`

	Recommendation string   `json:"recommendation"` // "proceed_to_qa" | "needs_human_review" | "revert_recommended"
	Reasons        []string `json:"reasons"`
}

// ScopeLimit defines max changes per card type.
type ScopeLimit struct {
	MaxFiles   int `json:"max_files"`
	MaxLines   int `json:"max_lines"`
	MaxDeletes int `json:"max_deletes"`
}

// DefaultCriticalFilePatterns lists files that require extra scrutiny.
var DefaultCriticalFilePatterns = []string{
	"go.mod", "go.sum", "package.json", "package-lock.json",
	"**/router.go", "**/routes.ts", "**/routes.go",
	"**/schema.sql", "**/migrations/**",
	"**/auth.go", "**/auth.ts", "**/middleware/**",
	".env*", "**/config.go", "**/config.ts",
	"Dockerfile", "docker-compose.yml", ".github/workflows/**",
}

// BuildBriefing creates a full decision briefing for a card.
// workDir: the git repo or worktree where changes were made.
// changedFiles: list from git diff --name-only.
// activeCardFiles: files being modified by other active cards (for conflict risk).
// scopeLimit: limits for the card type (nil = no limit check).
func BuildBriefing(ctx context.Context, workDir, cardID string, changedFiles []string, activeCardFiles map[string][]string, scopeLimit *ScopeLimit) DecisionBriefing {
	b := DecisionBriefing{CardID: cardID}

	// 1. Diff stats
	b.FilesChanged, b.LinesAdded, b.LinesDeleted = getDiffStats(ctx, workDir)

	// 2. Scope check
	if scopeLimit != nil {
		b.ScopeStatus = checkScope(b.FilesChanged, b.LinesAdded+b.LinesDeleted, b.LinesDeleted, *scopeLimit)
		if b.ScopeStatus == "exceeded" {
			b.Reasons = append(b.Reasons, "scope_exceeded")
		}
	} else {
		b.ScopeStatus = "within_limits"
	}

	// 3. Secrets scan
	b.SecretsFound = scanSecrets(workDir, changedFiles)
	if len(b.SecretsFound) > 0 {
		b.Reasons = append(b.Reasons, "secret_detected")
	}

	// 4. Mass deletes
	if b.LinesDeleted > b.LinesAdded*2 && b.LinesDeleted > 100 {
		b.MassDeletes = true
		b.Reasons = append(b.Reasons, "mass_deletes")
	}

	// 5. Critical files
	b.CriticalFiles = findCriticalFiles(changedFiles, DefaultCriticalFilePatterns)

	// 6. Conflict risk
	b.ConflictRisk, b.SharedSurfaces = assessConflictRisk(changedFiles, activeCardFiles)
	if b.ConflictRisk == "high" {
		b.Reasons = append(b.Reasons, "critical_file_overlap")
	}

	// 7. Dependency risk
	b.ManifestChanges, b.DependencyRisk = ParseManifestChanges(ctx, workDir, changedFiles)
	if b.DependencyRisk == DependencyRiskHigh {
		b.Reasons = append(b.Reasons, "dependency_version_changed")
	}

	// 8. Recommendation
	b.Recommendation = computeRecommendation(b)

	return b
}

// getDiffStats runs git diff --stat and parses the summary line.
func getDiffStats(ctx context.Context, workDir string) (files, added, deleted int) {
	cmd := exec.CommandContext(ctx, "git", "diff", "HEAD", "--stat")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return 0, 0, 0
	}
	return parseDiffStatSummary(string(output))
}

// parseDiffStatSummary extracts files/insertions/deletions from the last line
// of git diff --stat output, e.g.:
// " 3 files changed, 45 insertions(+), 12 deletions(-)"
func parseDiffStatSummary(stat string) (files, added, deleted int) {
	lines := strings.Split(strings.TrimSpace(stat), "\n")
	if len(lines) == 0 {
		return 0, 0, 0
	}
	summary := lines[len(lines)-1]
	for _, part := range strings.Split(summary, ",") {
		part = strings.TrimSpace(part)
		fields := strings.Fields(part)
		if len(fields) < 2 {
			continue
		}
		n, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		word := fields[1]
		switch {
		case strings.HasPrefix(word, "file"):
			files = n
		case strings.HasPrefix(word, "insertion"):
			added = n
		case strings.HasPrefix(word, "deletion"):
			deleted = n
		}
	}
	return
}

// checkScope evaluates whether changes are within scope limits.
func checkScope(files, totalLines, deletes int, limit ScopeLimit) string {
	exceeded := false
	warning := false

	if limit.MaxFiles > 0 && files > limit.MaxFiles {
		exceeded = true
	} else if limit.MaxFiles > 0 && files > limit.MaxFiles*80/100 {
		warning = true
	}

	if limit.MaxLines > 0 && totalLines > limit.MaxLines {
		exceeded = true
	} else if limit.MaxLines > 0 && totalLines > limit.MaxLines*80/100 {
		warning = true
	}

	if limit.MaxDeletes > 0 && deletes > limit.MaxDeletes {
		exceeded = true
	}

	if exceeded {
		return "exceeded"
	}
	if warning {
		return "warning"
	}
	return "within_limits"
}

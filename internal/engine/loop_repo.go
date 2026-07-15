package engine

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"unicode"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/orchestrator"
)

// RepoLoopDetector analyzes git commit history for pathological patterns.
type RepoLoopDetector struct{}

// NewRepoLoopDetector creates a new RepoLoopDetector.
func NewRepoLoopDetector() *RepoLoopDetector {
	return &RepoLoopDetector{}
}

// Detect analyzes the last 15 commits in the given directory for loop signals.
func (d *RepoLoopDetector) Detect(ctx context.Context, workDir string) []orchestrator.LoopSignal {
	var signals []orchestrator.LoopSignal

	messages, err := getRecentCommitMessages(ctx, workDir, 15)
	if err != nil || len(messages) == 0 {
		return nil
	}

	fileChanges, _ := getRecentFileChanges(ctx, workDir, 15)

	// Check fix_chain: 3+ consecutive commits with "fix" in message.
	if chain := detectFixChain(messages); chain > 0 {
		signals = append(signals, orchestrator.LoopSignal{
			Type:   "fix_chain",
			Detail: fmt.Sprintf("%d consecutive fix commits", chain),
			Source: "repo",
		})
	}

	// Check revert: any commit starting with "Revert" or "revert".
	if msg := detectRevert(messages); msg != "" {
		signals = append(signals, orchestrator.LoopSignal{
			Type:   "revert",
			Detail: msg,
			Source: "repo",
		})
	}

	// Check file_churn: same file in 5+ of last 15 commits.
	if file, count := detectFileChurn(fileChanges); file != "" {
		signals = append(signals, orchestrator.LoopSignal{
			Type:   "file_churn",
			Detail: fmt.Sprintf("%s changed in %d of last 15 commits", file, count),
			Source: "repo",
		})
	}

	// Check pendulum: A-B-A alternating pattern.
	if detail := detectPendulum(messages); detail != "" {
		signals = append(signals, orchestrator.LoopSignal{
			Type:   "pendulum",
			Detail: detail,
			Source: "repo",
		})
	}

	return signals
}

// getRecentCommitMessages returns the last n commit messages (subject line only).
func getRecentCommitMessages(ctx context.Context, workDir string, n int) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "log", "--oneline", fmt.Sprintf("-%d", n), "--format=%s")
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}
	return strings.Split(raw, "\n"), nil
}

// getRecentFileChanges returns files changed per commit for the last n commits.
// Returns a slice of slices: each inner slice contains the files changed in that commit.
func getRecentFileChanges(ctx context.Context, workDir string, n int) ([][]string, error) {
	// Use --format to produce a clear separator between commits.
	cmd := exec.CommandContext(ctx, "git", "log", "--name-only",
		"--pretty=format:---COMMIT---", fmt.Sprintf("-%d", n))
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}

	var result [][]string
	var current []string

	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "---COMMIT---" {
			if current != nil {
				result = append(result, current)
			}
			current = []string{}
			continue
		}
		if line == "" {
			continue
		}
		if current != nil {
			current = append(current, line)
		}
	}
	if current != nil {
		result = append(result, current)
	}
	return result, nil
}

// detectFixChain returns the length of the longest consecutive run of commits
// containing "fix" (case-insensitive). Returns 0 if fewer than 3 consecutive.
func detectFixChain(messages []string) int {
	maxRun := 0
	run := 0
	for _, msg := range messages {
		if strings.Contains(strings.ToLower(msg), "fix") {
			run++
			if run > maxRun {
				maxRun = run
			}
		} else {
			run = 0
		}
	}
	if maxRun < 3 {
		return 0
	}
	return maxRun
}

// detectRevert returns the first commit message that starts with "Revert" or "revert".
func detectRevert(messages []string) string {
	for _, msg := range messages {
		trimmed := strings.TrimSpace(msg)
		if strings.HasPrefix(trimmed, "Revert") || strings.HasPrefix(trimmed, "revert") {
			return trimmed
		}
	}
	return ""
}

// detectFileChurn returns the filename and count if any file appears in 5+ commits.
func detectFileChurn(fileChanges [][]string) (string, int) {
	counts := make(map[string]int)
	for _, files := range fileChanges {
		seen := make(map[string]bool)
		for _, f := range files {
			if !seen[f] {
				counts[f]++
				seen[f] = true
			}
		}
	}
	var maxFile string
	var maxCount int
	for file, count := range counts {
		if count > maxCount {
			maxCount = count
			maxFile = file
		}
	}
	if maxCount >= 5 {
		return maxFile, maxCount
	}
	return "", 0
}

// detectPendulum checks for A-B-A alternating commit message patterns.
func detectPendulum(messages []string) string {
	if len(messages) < 3 {
		return ""
	}
	for i := 2; i < len(messages); i++ {
		if normalizedFirst3Match(messages[i], messages[i-2]) &&
			!normalizedFirst3Match(messages[i], messages[i-1]) {
			return fmt.Sprintf("alternating: %q ↔ %q", messages[i-2], messages[i-1])
		}
	}
	return ""
}

// normalizedFirst3Match returns true if the first 3 words of a and b match
// after lowercasing and stripping punctuation.
func normalizedFirst3Match(a, b string) bool {
	wa := firstNWords(normalize(a), 3)
	wb := firstNWords(normalize(b), 3)
	if len(wa) == 0 || len(wb) == 0 {
		return false
	}
	if len(wa) != len(wb) {
		return false
	}
	for i := range wa {
		if wa[i] != wb[i] {
			return false
		}
	}
	return true
}

// normalize lowercases and strips punctuation from a string.
func normalize(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// firstNWords returns the first n whitespace-separated words from s.
func firstNWords(s string, n int) []string {
	words := strings.Fields(s)
	if len(words) > n {
		words = words[:n]
	}
	return words
}

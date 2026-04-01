package engine

import (
	"crypto/sha256"
	"fmt"
	"time"
)

// ProgressSnapshot captures the state at a checkpoint.
type ProgressSnapshot struct {
	DiffHash     string    `json:"diff_hash" yaml:"diff_hash"`         // hash of git diff output
	FailingTests int       `json:"failing_tests" yaml:"failing_tests"` // number of failing tests
	ErrorClass   string    `json:"error_class" yaml:"error_class"`     // current error class (build/test/etc)
	FilesExist   int       `json:"files_exist" yaml:"files_exist"`     // count of new/modified files
	HasNewCommit bool      `json:"has_new_commit" yaml:"has_new_commit"` // new commit since last check
	Timestamp    time.Time `json:"timestamp" yaml:"timestamp"`
}

// CheckpointGuard monitors agent progress based on multiple signals.
// It counts consecutive no-progress checks and signals timeout when the
// threshold is reached. The guard does not enforce timing — the caller
// decides when to call Record and check ShouldTimeout.
type CheckpointGuard struct {
	history       []ProgressSnapshot
	maxNoProgress int // consecutive checks without progress before timeout
}

// NewCheckpointGuard creates a guard.
// maxNoProgress=3 means timeout after 3 consecutive checks with no progress.
func NewCheckpointGuard(maxNoProgress int) *CheckpointGuard {
	return &CheckpointGuard{
		maxNoProgress: maxNoProgress,
	}
}

// Record adds a new progress snapshot to the history.
func (g *CheckpointGuard) Record(snap ProgressSnapshot) {
	if snap.Timestamp.IsZero() {
		snap.Timestamp = time.Now()
	}
	g.history = append(g.history, snap)
}

// ShouldTimeout returns true if there has been no progress for
// maxNoProgress consecutive checks. Also returns the number of
// consecutive no-progress checks from the tail of the history.
func (g *CheckpointGuard) ShouldTimeout() (timeout bool, consecutiveNoProgress int) {
	consecutiveNoProgress = g.consecutiveNoProgress()
	timeout = consecutiveNoProgress >= g.maxNoProgress
	return
}

// IsWarning returns true if we're at maxNoProgress-1
// (one more no-progress check will trigger timeout).
func (g *CheckpointGuard) IsWarning() bool {
	return g.consecutiveNoProgress() == g.maxNoProgress-1
}

// Reset clears the history (for reuse across steps).
func (g *CheckpointGuard) Reset() {
	g.history = nil
}

// consecutiveNoProgress counts how many consecutive no-progress checks
// exist at the tail of the history.
func (g *CheckpointGuard) consecutiveNoProgress() int {
	if len(g.history) < 2 {
		return 0
	}
	count := 0
	for i := len(g.history) - 1; i >= 1; i-- {
		if !HasProgress(g.history[i-1], g.history[i]) {
			count++
		} else {
			break
		}
	}
	return count
}

// HasProgress compares two snapshots and returns true if any progress
// signal changed positively between prev and curr.
func HasProgress(prev, curr ProgressSnapshot) bool {
	// DiffHash changed → code was modified
	if prev.DiffHash != curr.DiffHash {
		return true
	}
	// FailingTests decreased → tests improving
	if curr.FailingTests < prev.FailingTests {
		return true
	}
	// ErrorClass changed → different kind of error (shifting problem)
	if prev.ErrorClass != curr.ErrorClass && curr.ErrorClass != "" {
		return true
	}
	// FilesExist increased → new files created
	if curr.FilesExist > prev.FilesExist {
		return true
	}
	// HasNewCommit → agent committed work
	if curr.HasNewCommit {
		return true
	}
	return false
}

// DiffHash computes a short SHA-256 hash of a diff string for comparison.
func DiffHash(diff string) string {
	h := sha256.Sum256([]byte(diff))
	return fmt.Sprintf("%x", h[:8])
}

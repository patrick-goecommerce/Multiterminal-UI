package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/orchestrator"
)

// initGitRepo creates a temp dir with an initial commit.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitRun(t, dir, "init")
	gitRun(t, dir, "config", "user.email", "t@t.com")
	gitRun(t, dir, "config", "user.name", "test")
	os.WriteFile(filepath.Join(dir, "seed.txt"), []byte("seed\n"), 0644)
	gitRun(t, dir, "add", "seed.txt")
	gitRun(t, dir, "commit", "-m", "init")
	return dir
}

// stageFile writes content and stages it.
func stageFile(t *testing.T, dir, name, content string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0755)
	os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
	gitRun(t, dir, "add", name)
}

// nLines returns a string with n lines.
func nLines(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	return b.String()
}

// hasViolationPrefix checks if any violation starts with prefix.
func hasViolationPrefix(res ScopeCheckResult, prefix string) bool {
	for _, v := range res.Violations {
		if strings.HasPrefix(v, prefix) {
			return true
		}
	}
	return false
}

// --- Scope Check Tests ---

func TestCheckScope_WithinLimits(t *testing.T) {
	dir := initGitRepo(t)
	stageFile(t, dir, "a.go", "line1\nline2\n")
	res := CheckScope(context.Background(), dir, "feature")
	if !res.WithinLimits {
		t.Fatalf("expected within limits, got violations: %v", res.Violations)
	}
}

func TestCheckScope_FilesExceeded(t *testing.T) {
	dir := initGitRepo(t)
	for i := 0; i < 30; i++ {
		stageFile(t, dir, fmt.Sprintf("f%d.go", i), "x\n")
	}
	res := CheckScope(context.Background(), dir, "bugfix")
	if res.WithinLimits {
		t.Fatal("expected files violation")
	}
	if !hasViolationPrefix(res, "files:") {
		t.Fatalf("missing files violation in %v", res.Violations)
	}
}

func TestCheckScope_LinesExceeded(t *testing.T) {
	dir := initGitRepo(t)
	stageFile(t, dir, "big.go", nLines(250))
	res := CheckScope(context.Background(), dir, "bugfix")
	if res.WithinLimits {
		t.Fatal("expected lines violation")
	}
	if !hasViolationPrefix(res, "lines:") {
		t.Fatalf("missing lines violation in %v", res.Violations)
	}
}

func TestCheckScope_DeletesExceeded(t *testing.T) {
	dir := initGitRepo(t)
	os.WriteFile(filepath.Join(dir, "big.go"), []byte(nLines(120)), 0644)
	gitRun(t, dir, "add", "big.go")
	gitRun(t, dir, "commit", "-m", "add big")
	os.WriteFile(filepath.Join(dir, "big.go"), []byte("small\n"), 0644)
	gitRun(t, dir, "add", "big.go")
	res := CheckScope(context.Background(), dir, "bugfix")
	if res.Deletes > 50 && res.WithinLimits {
		t.Fatal("expected deletes violation")
	}
}

func TestCheckScope_RenamesNotCountedAsDeletes(t *testing.T) {
	dir := initGitRepo(t)
	os.WriteFile(filepath.Join(dir, "old.go"), []byte(nLines(60)), 0644)
	gitRun(t, dir, "add", "old.go")
	gitRun(t, dir, "commit", "-m", "add old")
	gitRun(t, dir, "mv", "old.go", "new.go")
	res := CheckScope(context.Background(), dir, "bugfix")
	if res.Deletes > 10 {
		t.Fatalf("expected renames to reduce deletes, got %d", res.Deletes)
	}
}

func TestCheckScope_GeneratedFilesExcluded(t *testing.T) {
	dir := initGitRepo(t)
	os.WriteFile(filepath.Join(dir, "models_generated.go"), []byte(nLines(80)), 0644)
	gitRun(t, dir, "add", "models_generated.go")
	gitRun(t, dir, "commit", "-m", "add gen")
	os.WriteFile(filepath.Join(dir, "models_generated.go"), []byte("small\n"), 0644)
	gitRun(t, dir, "add", "models_generated.go")
	res := CheckScope(context.Background(), dir, "bugfix")
	if res.Deletes > 10 {
		t.Fatalf("expected generated file deletes excluded, got %d", res.Deletes)
	}
}

func TestCheckScope_UnknownCardType(t *testing.T) {
	dir := initGitRepo(t)
	stageFile(t, dir, "a.go", "line\n")
	res := CheckScope(context.Background(), dir, "unknown_type")
	if !res.WithinLimits {
		t.Fatalf("expected fallback to feature limits, got: %v", res.Violations)
	}
}

func TestCheckScope_MultipleViolations(t *testing.T) {
	dir := initGitRepo(t)
	for i := 0; i < 15; i++ {
		stageFile(t, dir, fmt.Sprintf("f%d.go", i), nLines(20))
	}
	res := CheckScope(context.Background(), dir, "bugfix")
	if res.WithinLimits {
		t.Fatal("expected multiple violations")
	}
	if len(res.Violations) < 2 {
		t.Fatalf("expected >=2 violations, got %v", res.Violations)
	}
}

// --- Conflict Avoidance Tests ---

func TestDetectFileConflicts_NoConflicts(t *testing.T) {
	steps := []orchestrator.PlanStep{
		{ID: "s1", FilesModify: []string{"a.go", "b.go"}},
		{ID: "s2", FilesModify: []string{"c.go", "d.go"}},
	}
	if c := DetectFileConflicts(steps); len(c) != 0 {
		t.Fatalf("expected no conflicts, got %v", c)
	}
}

func TestDetectFileConflicts_Detected(t *testing.T) {
	steps := []orchestrator.PlanStep{
		{ID: "s1", FilesModify: []string{"a.go", "shared.go"}},
		{ID: "s2", FilesModify: []string{"b.go", "shared.go"}},
	}
	c := DetectFileConflicts(steps)
	if len(c) != 1 || c[0].File != "shared.go" {
		t.Fatalf("expected 1 conflict on shared.go, got %v", c)
	}
}

func TestSplitConflictingSteps_Resolves(t *testing.T) {
	steps := []orchestrator.PlanStep{
		{ID: "s1", FilesModify: []string{"shared.go"}},
		{ID: "s2", FilesModify: []string{"shared.go"}},
	}
	safe, deferred := SplitConflictingSteps(steps)
	if len(safe) != 1 || len(deferred) != 1 {
		t.Fatalf("expected 1+1, got %d+%d", len(safe), len(deferred))
	}
	if deferred[0].ID != "s2" {
		t.Fatalf("expected s2 deferred, got %s", deferred[0].ID)
	}
}

func TestSplitConflictingSteps_Deterministic(t *testing.T) {
	steps := []orchestrator.PlanStep{
		{ID: "b", FilesModify: []string{"x.go"}},
		{ID: "a", FilesModify: []string{"x.go"}},
	}
	_, deferred := SplitConflictingSteps(steps)
	if len(deferred) != 1 || deferred[0].ID != "b" {
		t.Fatalf("expected b deferred, got %v", deferred)
	}
}

func TestSplitConflictingSteps_ThreeSteps(t *testing.T) {
	steps := []orchestrator.PlanStep{
		{ID: "a", FilesModify: []string{"shared.go"}},
		{ID: "b", FilesModify: []string{"other.go"}},
		{ID: "c", FilesModify: []string{"shared.go"}},
	}
	safe, deferred := SplitConflictingSteps(steps)
	if len(deferred) != 1 || deferred[0].ID != "c" {
		t.Fatalf("expected c deferred, got %v", deferred)
	}
	if len(safe) != 2 {
		t.Fatalf("expected 2 safe, got %d", len(safe))
	}
}

func TestSplitConflictingSteps_NoConflicts(t *testing.T) {
	steps := []orchestrator.PlanStep{
		{ID: "s1", FilesModify: []string{"a.go"}},
		{ID: "s2", FilesModify: []string{"b.go"}},
	}
	safe, deferred := SplitConflictingSteps(steps)
	if len(safe) != 2 || deferred != nil {
		t.Fatalf("expected all safe, got %d+%d", len(safe), len(deferred))
	}
}

# Diff-View & One-Click Commit/Push Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add per-pane diff view and one-click commit & push to Multiterminal, inspired by claude-squad.

**Architecture:** New backend APIs (`app_git_diff.go`) provide git diff/stage/commit/push operations with selective file staging and auto-generated conventional commit messages. A new `DiffView.svelte` component renders diffs in an overlay panel per pane with file checkboxes for selective staging. A `CommitPushDialog.svelte` provides conventional commit type selector, auto-suggested scope and description, and push/PR options. The pane titlebar gets a "Changes" button that opens the diff view. Small, focused conventional commits are the goal — not one big "commit everything".

**Tech Stack:** Go (git CLI wrappers), Svelte 4, CSS (diff coloring), Wails v3 bindings

---

### Task 1: Backend — Git Diff APIs (`app_git_diff.go`)

**Files:**
- Create: `internal/backend/app_git_diff.go`
- Test: `internal/backend/app_git_diff_test.go`

**Step 1: Write the test file**

```go
// internal/backend/app_git_diff_test.go
package backend

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initTestRepo creates a temp git repo with one committed file.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s – %v", args, out, err)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "test")
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello\n"), 0644)
	run("add", "hello.txt")
	run("commit", "-m", "init")
	return dir
}

func TestGetDiffStats_NoChanges(t *testing.T) {
	dir := initTestRepo(t)
	app := &AppService{}
	stats := app.GetDiffStats(dir)
	if len(stats) != 0 {
		t.Errorf("expected 0 stats, got %d", len(stats))
	}
}

func TestGetDiffStats_ModifiedFile(t *testing.T) {
	dir := initTestRepo(t)
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello\nworld\n"), 0644)
	app := &AppService{}
	stats := app.GetDiffStats(dir)
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat, got %d", len(stats))
	}
	if stats[0].Path != "hello.txt" {
		t.Errorf("expected path hello.txt, got %s", stats[0].Path)
	}
	if stats[0].Status != "M" {
		t.Errorf("expected status M, got %s", stats[0].Status)
	}
}

func TestGetDiffStats_NewFile(t *testing.T) {
	dir := initTestRepo(t)
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0644)
	app := &AppService{}
	stats := app.GetDiffStats(dir)
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat, got %d", len(stats))
	}
	if stats[0].Status != "?" {
		t.Errorf("expected status ?, got %s", stats[0].Status)
	}
}

func TestGetFileDiff_Modified(t *testing.T) {
	dir := initTestRepo(t)
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello\nworld\n"), 0644)
	app := &AppService{}
	diff := app.GetFileDiff(dir, "hello.txt")
	if diff == "" {
		t.Error("expected non-empty diff")
	}
}

func TestGetWorkingDiff(t *testing.T) {
	dir := initTestRepo(t)
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("changed\n"), 0644)
	app := &AppService{}
	diff := app.GetWorkingDiff(dir)
	if diff == "" {
		t.Error("expected non-empty diff")
	}
}

func TestStageAndCommit(t *testing.T) {
	dir := initTestRepo(t)
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("committed\n"), 0644)
	app := &AppService{}
	err := app.StageAndCommit(dir, "test commit")
	if err != nil {
		t.Fatalf("StageAndCommit failed: %v", err)
	}
	// Verify no pending changes
	stats := app.GetDiffStats(dir)
	if len(stats) != 0 {
		t.Errorf("expected 0 stats after commit, got %d", len(stats))
	}
}

func TestStageAndCommit_NoChanges(t *testing.T) {
	dir := initTestRepo(t)
	app := &AppService{}
	err := app.StageAndCommit(dir, "empty")
	if err == nil {
		t.Error("expected error for empty commit")
	}
}

func TestPushBranch_NoRemote(t *testing.T) {
	dir := initTestRepo(t)
	app := &AppService{}
	err := app.PushBranch(dir)
	if err == nil {
		t.Error("expected error when no remote exists")
	}
}

func TestStageFiles_Selective(t *testing.T) {
	dir := initTestRepo(t)
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b\n"), 0644)
	app := &AppService{}

	// Stage only a.txt
	err := app.StageFiles(dir, []string{"a.txt"})
	if err != nil {
		t.Fatalf("StageFiles failed: %v", err)
	}

	// Commit only staged
	err = app.CommitStaged(dir, "test: add a.txt only")
	if err != nil {
		t.Fatalf("CommitStaged failed: %v", err)
	}

	// b.txt should still be untracked
	stats := app.GetDiffStats(dir)
	if len(stats) != 1 {
		t.Fatalf("expected 1 remaining file, got %d", len(stats))
	}
	if stats[0].Path != "b.txt" {
		t.Errorf("expected b.txt remaining, got %s", stats[0].Path)
	}
}

func TestGenerateCommitSuggestion_NewFile(t *testing.T) {
	dir := initTestRepo(t)
	os.WriteFile(filepath.Join(dir, "feature.go"), []byte("package main\n"), 0644)
	app := &AppService{}
	suggestion := app.GenerateCommitSuggestion(dir, nil)
	if suggestion.Type != "feat" {
		t.Errorf("expected type 'feat', got '%s'", suggestion.Type)
	}
	if suggestion.Full == "" {
		t.Error("expected non-empty Full message")
	}
}

func TestGenerateCommitSuggestion_TestFile(t *testing.T) {
	dir := initTestRepo(t)
	os.WriteFile(filepath.Join(dir, "hello_test.go"), []byte("package main\n"), 0644)
	// Also modify existing to prevent "feat" from new-file heuristic
	app := &AppService{}
	suggestion := app.GenerateCommitSuggestion(dir, []string{"hello_test.go"})
	if suggestion.Type != "test" {
		t.Errorf("expected type 'test', got '%s'", suggestion.Type)
	}
}

func TestGenerateCommitSuggestion_FilteredPaths(t *testing.T) {
	dir := initTestRepo(t)
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("a\n"), 0644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte("b\n"), 0644)
	app := &AppService{}
	suggestion := app.GenerateCommitSuggestion(dir, []string{"a.go"})
	if suggestion.Description == "" {
		t.Error("expected non-empty description")
	}
	// Should describe only 1 file, not 2
	if strings.Contains(suggestion.Description, "2") {
		t.Errorf("should only describe 1 file, got: %s", suggestion.Description)
	}
}

func TestInferScope(t *testing.T) {
	stats := []DiffFileStat{
		{Path: "internal/backend/app_git.go", Status: "M"},
		{Path: "internal/backend/app_diff.go", Status: "A"},
	}
	scope := inferScope(stats)
	if scope != "backend" {
		t.Errorf("expected scope 'backend', got '%s'", scope)
	}
}

func TestInferScope_Frontend(t *testing.T) {
	stats := []DiffFileStat{
		{Path: "frontend/src/components/DiffView.svelte", Status: "A"},
	}
	scope := inferScope(stats)
	if scope != "ui" {
		t.Errorf("expected scope 'ui', got '%s'", scope)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /d/repos/Multiterminal && go test ./internal/backend/ -run "TestGet(DiffStats|FileDiff|WorkingDiff)|TestStageAndCommit|TestPushBranch" -v`
Expected: FAIL — functions not defined

**Step 3: Implement the backend**

```go
// internal/backend/app_git_diff.go
package backend

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// DiffFileStat describes one changed file with insertions/deletions.
type DiffFileStat struct {
	Path       string `json:"path" yaml:"path"`
	Status     string `json:"status" yaml:"status"`         // M, A, D, R, ?
	Insertions int    `json:"insertions" yaml:"insertions"`
	Deletions  int    `json:"deletions" yaml:"deletions"`
}

// CommitSuggestion is an auto-generated conventional commit message.
type CommitSuggestion struct {
	Type        string `json:"type" yaml:"type"`               // feat, fix, refactor, chore, docs, test, style, perf, ci, build
	Scope       string `json:"scope" yaml:"scope"`             // e.g. "backend", "ui", "config"
	Description string `json:"description" yaml:"description"` // short summary of changes
	Full        string `json:"full" yaml:"full"`               // assembled "type(scope): description"
}

// GetDiffStats returns a summary of all changed files (tracked + untracked).
func (a *AppService) GetDiffStats(dir string) []DiffFileStat {
	if dir == "" {
		return nil
	}
	var result []DiffFileStat

	// Tracked changes: git diff --numstat HEAD
	cmd := exec.Command("git", "diff", "--numstat", "HEAD")
	cmd.Dir = dir
	hideConsole(cmd)
	if out, err := cmd.Output(); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "\t", 3)
			if len(parts) < 3 {
				continue
			}
			ins, _ := strconv.Atoi(parts[0])
			del, _ := strconv.Atoi(parts[1])
			// Determine status from porcelain
			status := "M"
			result = append(result, DiffFileStat{
				Path: parts[2], Status: status,
				Insertions: ins, Deletions: del,
			})
		}
	}

	// Enrich with porcelain status (A, D, R, M)
	statusMap := a.GetGitFileStatuses(dir)
	for i := range result {
		if s, ok := statusMap[result[i].Path]; ok {
			result[i].Status = s
		}
	}

	// Untracked files
	for path, status := range statusMap {
		if status == "?" {
			result = append(result, DiffFileStat{
				Path: path, Status: "?",
			})
		}
	}

	return result
}

// GetFileDiff returns the unified diff for a single file.
// For untracked files, returns the file content as a pseudo-diff.
func (a *AppService) GetFileDiff(dir, path string) string {
	if dir == "" || path == "" {
		return ""
	}

	// Try tracked diff first
	cmd := exec.Command("git", "diff", "HEAD", "--", path)
	cmd.Dir = dir
	hideConsole(cmd)
	if out, err := cmd.Output(); err == nil && len(out) > 0 {
		return string(out)
	}

	// Untracked file: show as new file diff
	cmd = exec.Command("git", "diff", "--no-index", "/dev/null", path)
	cmd.Dir = dir
	hideConsole(cmd)
	out, _ := cmd.CombinedOutput() // exit code 1 is expected for new files
	return string(out)
}

// GetWorkingDiff returns the full unified diff of all working changes vs HEAD.
func (a *AppService) GetWorkingDiff(dir string) string {
	if dir == "" {
		return ""
	}
	cmd := exec.Command("git", "diff", "HEAD")
	cmd.Dir = dir
	hideConsole(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// StageAndCommit stages all changes and creates a commit.
func (a *AppService) StageAndCommit(dir, message string) error {
	if dir == "" {
		return fmt.Errorf("no directory specified")
	}
	if message == "" {
		return fmt.Errorf("commit message required")
	}

	// git add -A
	addCmd := exec.Command("git", "add", "-A")
	addCmd.Dir = dir
	hideConsole(addCmd)
	if out, err := addCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %s – %w", strings.TrimSpace(string(out)), err)
	}

	// git commit -m
	commitCmd := exec.Command("git", "commit", "-m", message)
	commitCmd.Dir = dir
	hideConsole(commitCmd)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %s – %w", strings.TrimSpace(string(out)), err)
	}

	return nil
}

// PushBranch pushes the current branch to origin.
func (a *AppService) PushBranch(dir string) error {
	if dir == "" {
		return fmt.Errorf("no directory specified")
	}

	// Get current branch
	branch := a.GetGitBranch(dir)
	if branch == "" {
		return fmt.Errorf("could not determine current branch")
	}

	cmd := exec.Command("git", "push", "-u", "origin", branch)
	cmd.Dir = dir
	hideConsole(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git push failed: %s – %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// StageFiles stages specific files (not all). paths are relative to dir.
func (a *AppService) StageFiles(dir string, paths []string) error {
	if dir == "" || len(paths) == 0 {
		return fmt.Errorf("no files to stage")
	}
	args := append([]string{"add", "--"}, paths...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	hideConsole(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git add failed: %s – %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// CommitStaged commits only the currently staged changes.
func (a *AppService) CommitStaged(dir, message string) error {
	if dir == "" {
		return fmt.Errorf("no directory specified")
	}
	if message == "" {
		return fmt.Errorf("commit message required")
	}
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = dir
	hideConsole(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git commit failed: %s – %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// GenerateCommitSuggestion analyzes changed files and suggests a conventional commit message.
// If paths is non-empty, only those files are considered; otherwise all changes are used.
func (a *AppService) GenerateCommitSuggestion(dir string, paths []string) CommitSuggestion {
	stats := a.GetDiffStats(dir)
	if len(paths) > 0 {
		pathSet := make(map[string]bool)
		for _, p := range paths {
			pathSet[p] = true
		}
		var filtered []DiffFileStat
		for _, s := range stats {
			if pathSet[s.Path] {
				filtered = append(filtered, s)
			}
		}
		stats = filtered
	}
	if len(stats) == 0 {
		return CommitSuggestion{}
	}

	commitType := inferCommitType(stats)
	scope := inferScope(stats)
	desc := inferDescription(stats)

	full := commitType
	if scope != "" {
		full += "(" + scope + ")"
	}
	full += ": " + desc

	return CommitSuggestion{
		Type:        commitType,
		Scope:       scope,
		Description: desc,
		Full:        full,
	}
}

// inferCommitType guesses conventional commit type from file patterns.
func inferCommitType(stats []DiffFileStat) string {
	hasNew, hasTest, hasDocs, hasConfig := false, false, false, false
	for _, s := range stats {
		low := strings.ToLower(s.Path)
		if s.Status == "?" || s.Status == "A" {
			hasNew = true
		}
		if strings.Contains(low, "_test.") || strings.Contains(low, ".test.") ||
			strings.Contains(low, ".spec.") || strings.HasPrefix(low, "test") {
			hasTest = true
		}
		if strings.HasSuffix(low, ".md") || strings.HasPrefix(low, "docs/") {
			hasDocs = true
		}
		if strings.HasSuffix(low, ".yaml") || strings.HasSuffix(low, ".yml") ||
			strings.HasSuffix(low, ".json") || strings.HasSuffix(low, ".toml") ||
			strings.Contains(low, "config") {
			hasConfig = true
		}
	}
	// Pure test changes
	if hasTest && !hasNew && !hasDocs {
		return "test"
	}
	// Pure docs
	if hasDocs && len(stats) == 1 {
		return "docs"
	}
	// Pure config
	if hasConfig && len(stats) <= 2 {
		return "chore"
	}
	// New files → feat
	if hasNew {
		return "feat"
	}
	// Default: small change = fix
	return "fix"
}

// inferScope extracts a scope from the common directory of changed files.
func inferScope(stats []DiffFileStat) string {
	if len(stats) == 0 {
		return ""
	}
	// Collect top-level directory or known component names
	dirs := make(map[string]int)
	for _, s := range stats {
		parts := strings.Split(strings.ReplaceAll(s.Path, "\\", "/"), "/")
		if len(parts) > 1 {
			dir := parts[0]
			// Use second level for known prefixes
			if (dir == "internal" || dir == "frontend" || dir == "src") && len(parts) > 2 {
				dir = parts[1]
			}
			dirs[dir]++
		}
	}
	// Return most common directory as scope
	best, bestCount := "", 0
	for d, c := range dirs {
		if c > bestCount {
			best = d
			bestCount = c
		}
	}
	// Map known directories to shorter scopes
	scopeMap := map[string]string{
		"backend":    "backend",
		"terminal":   "terminal",
		"config":     "config",
		"components": "ui",
		"stores":     "ui",
		"lib":        "ui",
	}
	if mapped, ok := scopeMap[best]; ok {
		return mapped
	}
	return best
}

// inferDescription builds a short description from the changed files.
func inferDescription(stats []DiffFileStat) string {
	if len(stats) == 1 {
		s := stats[0]
		name := filepath.Base(s.Path)
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		switch s.Status {
		case "A", "?":
			return "add " + base
		case "D":
			return "remove " + base
		default:
			return "update " + base
		}
	}

	// Group by action
	added, modified, deleted := 0, 0, 0
	for _, s := range stats {
		switch s.Status {
		case "A", "?":
			added++
		case "D":
			deleted++
		default:
			modified++
		}
	}

	var parts []string
	if added > 0 {
		parts = append(parts, fmt.Sprintf("add %d file(s)", added))
	}
	if modified > 0 {
		parts = append(parts, fmt.Sprintf("update %d file(s)", modified))
	}
	if deleted > 0 {
		parts = append(parts, fmt.Sprintf("remove %d file(s)", deleted))
	}
	return strings.Join(parts, ", ")
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /d/repos/Multiterminal && go test ./internal/backend/ -run "TestGet(DiffStats|FileDiff|WorkingDiff)|TestStageAndCommit|TestPushBranch" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/backend/app_git_diff.go internal/backend/app_git_diff_test.go
git commit -m "feat(git): add diff, commit, and push backend APIs"
```

---

### Task 2: Frontend Types — models.ts Update

**Files:**
- Modify: `frontend/wailsjs/go/models.ts`

**Step 1: Add DiffFileStat class to models.ts**

Find the end of the existing classes in `models.ts` and add:

```typescript
export class DiffFileStat {
    path: string;
    status: string;
    insertions: number;
    deletions: number;

    constructor(source: any = {}) {
        this.path = source["path"];
        this.status = source["status"];
        this.insertions = source["insertions"];
        this.deletions = source["deletions"];
    }

    static createFrom(source: any = {}) {
        return new DiffFileStat(source);
    }
}

export class CommitSuggestion {
    type: string;
    scope: string;
    description: string;
    full: string;

    constructor(source: any = {}) {
        this.type = source["type"];
        this.scope = source["scope"];
        this.description = source["description"];
        this.full = source["full"];
    }

    static createFrom(source: any = {}) {
        return new CommitSuggestion(source);
    }
}
```

**Step 2: Verify TypeScript compiles**

Run: `cd /d/repos/Multiterminal/frontend && npx tsc --noEmit 2>&1 | head -20`
Expected: No new errors related to DiffFileStat

**Step 3: Commit**

```bash
git add frontend/wailsjs/go/models.ts
git commit -m "feat(models): add DiffFileStat type for frontend bindings"
```

---

### Task 3: Frontend — DiffView Component

**Files:**
- Create: `frontend/src/components/DiffView.svelte`

**Step 1: Create the DiffView component**

This is a panel/overlay that shows:
- Left sidebar: list of changed files with +/- stats
- Right area: unified diff of the selected file with color coding
- Bottom bar: "Commit & Push" button

```svelte
<!-- frontend/src/components/DiffView.svelte -->
<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { App } from '../../wailsjs/go/backend/AppService';

  export let dir: string = '';
  export let visible: boolean = false;

  interface FileStat {
    path: string;
    status: string;
    insertions: number;
    deletions: number;
  }

  const dispatch = createEventDispatcher();

  let files: FileStat[] = [];
  let selectedFile: string = '';
  let diffContent: string = '';
  let loading: boolean = false;
  let checkedFiles: Set<string> = new Set();

  // Reload when becoming visible or dir changes
  $: if (visible && dir) loadStats();

  async function loadStats() {
    loading = true;
    try {
      files = await App.GetDiffStats(dir) || [];
      // Default: all files checked
      checkedFiles = new Set(files.map(f => f.path));
      if (files.length > 0 && !selectedFile) {
        selectFile(files[0].path);
      } else if (files.length === 0) {
        selectedFile = '';
        diffContent = '';
      }
    } catch (err) {
      console.error('[DiffView] loadStats failed:', err);
      files = [];
    }
    loading = false;
  }

  async function selectFile(path: string) {
    selectedFile = path;
    try {
      diffContent = await App.GetFileDiff(dir, path) || '';
    } catch (err) {
      console.error('[DiffView] GetFileDiff failed:', err);
      diffContent = '';
    }
  }

  function statusColor(status: string): string {
    switch (status) {
      case 'M': return '#e2b714';
      case 'A': return '#50fa7b';
      case 'D': return '#ff5555';
      case 'R': return '#8be9fd';
      case '?': return '#50fa7b';
      default: return '#999';
    }
  }

  function statusLabel(status: string): string {
    switch (status) {
      case 'M': return 'M';
      case 'A': return 'A';
      case 'D': return 'D';
      case 'R': return 'R';
      case '?': return 'U';
      default: return status;
    }
  }

  function parseDiffLines(raw: string): Array<{ text: string; type: string }> {
    return raw.split('\n').map(line => {
      let type = 'context';
      if (line.startsWith('+') && !line.startsWith('+++')) type = 'add';
      else if (line.startsWith('-') && !line.startsWith('---')) type = 'del';
      else if (line.startsWith('@@')) type = 'hunk';
      else if (line.startsWith('diff ') || line.startsWith('index ') ||
               line.startsWith('---') || line.startsWith('+++')) type = 'header';
      return { text: line, type };
    });
  }

  function toggleFile(path: string) {
    if (checkedFiles.has(path)) {
      checkedFiles.delete(path);
    } else {
      checkedFiles.add(path);
    }
    checkedFiles = checkedFiles; // trigger reactivity
  }

  function toggleAll() {
    if (checkedFiles.size === files.length) {
      checkedFiles = new Set();
    } else {
      checkedFiles = new Set(files.map(f => f.path));
    }
  }

  function openCommitDialog() {
    const selected = [...checkedFiles];
    dispatch('commitRequest', { dir, files: selected, fileCount: selected.length });
  }

  function close() {
    dispatch('close');
  }

  async function refresh() {
    await loadStats();
    if (selectedFile) {
      await selectFile(selectedFile);
    }
  }
</script>

{#if visible}
<div class="diff-overlay">
  <div class="diff-panel">
    <!-- Header -->
    <div class="diff-header">
      <h3>Änderungen</h3>
      <div class="diff-header-actions">
        <span class="file-count">{files.length} Datei{files.length !== 1 ? 'en' : ''} geändert</span>
        <button class="icon-btn" on:click={refresh} title="Aktualisieren">&#x21bb;</button>
        <button class="icon-btn" on:click={close} title="Schließen">&times;</button>
      </div>
    </div>

    <div class="diff-body">
      <!-- File List Sidebar -->
      <div class="file-list">
        {#if files.length > 0}
          <button class="select-all" on:click={toggleAll}>
            <input type="checkbox" checked={checkedFiles.size === files.length} readOnly />
            <span>{checkedFiles.size}/{files.length} ausgewählt</span>
          </button>
        {/if}
        {#each files as file}
          <div
            class="file-entry"
            class:selected={file.path === selectedFile}
          >
            <input
              type="checkbox"
              checked={checkedFiles.has(file.path)}
              on:click|stopPropagation={() => toggleFile(file.path)}
            />
            <button class="file-btn" on:click={() => selectFile(file.path)}>
              <span class="file-status" style="color: {statusColor(file.status)}">{statusLabel(file.status)}</span>
              <span class="file-path" title={file.path}>{file.path}</span>
              {#if file.insertions > 0 || file.deletions > 0}
                <span class="file-stats">
                  {#if file.insertions > 0}<span class="stat-add">+{file.insertions}</span>{/if}
                  {#if file.deletions > 0}<span class="stat-del">-{file.deletions}</span>{/if}
                </span>
              {/if}
            </button>
          </div>
        {:else}
          <div class="empty-state">
            {#if loading}Lade...{:else}Keine Änderungen{/if}
          </div>
        {/each}
      </div>

      <!-- Diff Content -->
      <div class="diff-content">
        {#if diffContent}
          <pre class="diff-pre">{#each parseDiffLines(diffContent) as line}<span class="diff-line diff-{line.type}">{line.text}</span>
{/each}</pre>
        {:else if selectedFile}
          <div class="empty-state">Kein Diff verfügbar</div>
        {:else}
          <div class="empty-state">Datei auswählen</div>
        {/if}
      </div>
    </div>

    <!-- Action Bar -->
    {#if files.length > 0}
      <div class="diff-actions">
        <span class="selected-count">{checkedFiles.size} Datei{checkedFiles.size !== 1 ? 'en' : ''} für Commit ausgewählt</span>
        <button class="action-btn primary" on:click={openCommitDialog} disabled={checkedFiles.size === 0}>
          Commit {checkedFiles.size} Datei{checkedFiles.size !== 1 ? 'en' : ''}
        </button>
      </div>
    {/if}
  </div>
</div>
{/if}

<style>
  .diff-overlay {
    position: absolute;
    inset: 0;
    z-index: 50;
    background: var(--bg-primary, #1e1e2e);
    display: flex;
    flex-direction: column;
  }
  .diff-panel {
    display: flex;
    flex-direction: column;
    height: 100%;
  }
  .diff-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border-color, #313244);
    flex-shrink: 0;
  }
  .diff-header h3 {
    margin: 0;
    font-size: 13px;
    font-weight: 600;
    color: var(--text-primary, #cdd6f4);
  }
  .diff-header-actions {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .file-count {
    font-size: 11px;
    color: var(--text-secondary, #a6adc8);
  }
  .icon-btn {
    background: none;
    border: none;
    color: var(--text-secondary, #a6adc8);
    cursor: pointer;
    font-size: 16px;
    padding: 2px 6px;
    border-radius: 4px;
  }
  .icon-btn:hover { background: var(--bg-hover, #313244); }
  .diff-body {
    display: flex;
    flex: 1;
    overflow: hidden;
  }
  .file-list {
    width: 260px;
    min-width: 200px;
    border-right: 1px solid var(--border-color, #313244);
    overflow-y: auto;
    flex-shrink: 0;
  }
  .file-entry {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
    padding: 5px 10px;
    background: none;
    border: none;
    color: var(--text-primary, #cdd6f4);
    cursor: pointer;
    font-size: 12px;
    text-align: left;
  }
  .file-entry:hover { background: var(--bg-hover, #313244); }
  .file-entry.selected { background: var(--bg-selected, #45475a); }
  .file-entry input[type="checkbox"] { flex-shrink: 0; cursor: pointer; accent-color: #7c3aed; }
  .file-btn {
    display: flex; align-items: center; gap: 6px; flex: 1;
    background: none; border: none; color: inherit; cursor: pointer;
    font-size: 12px; text-align: left; padding: 0; min-width: 0;
  }
  .select-all {
    display: flex; align-items: center; gap: 6px; width: 100%;
    padding: 4px 10px; background: none; border: none; border-bottom: 1px solid var(--border-color, #313244);
    color: var(--text-secondary, #a6adc8); cursor: pointer; font-size: 11px;
  }
  .select-all:hover { background: var(--bg-hover, #313244); }
  .file-status { font-family: monospace; font-weight: 700; width: 14px; text-align: center; flex-shrink: 0; }
  .file-path { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }
  .file-stats { flex-shrink: 0; font-size: 11px; font-family: monospace; }
  .stat-add { color: #50fa7b; margin-right: 4px; }
  .stat-del { color: #ff5555; }
  .diff-content {
    flex: 1;
    overflow: auto;
    padding: 0;
  }
  .diff-pre {
    margin: 0;
    padding: 8px 12px;
    font-family: 'Cascadia Code', 'Fira Code', monospace;
    font-size: 12px;
    line-height: 1.5;
    white-space: pre;
    tab-size: 4;
  }
  .diff-line { display: block; }
  .diff-add { color: #50fa7b; background: #50fa7b11; }
  .diff-del { color: #ff5555; background: #ff555511; }
  .diff-hunk { color: #bd93f9; }
  .diff-header { color: #6272a4; }
  .diff-context { color: var(--text-secondary, #a6adc8); }
  .diff-actions {
    display: flex;
    justify-content: flex-end;
    padding: 8px 12px;
    border-top: 1px solid var(--border-color, #313244);
    flex-shrink: 0;
  }
  .action-btn {
    padding: 6px 16px;
    border: none;
    border-radius: 6px;
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
    background: var(--bg-hover, #313244);
    color: var(--text-primary, #cdd6f4);
  }
  .action-btn.primary {
    background: #7c3aed;
    color: #fff;
  }
  .action-btn.primary:hover:not(:disabled) { background: #6d28d9; }
  .action-btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .selected-count { font-size: 11px; color: var(--text-secondary, #a6adc8); }
  .empty-state {
    padding: 24px;
    text-align: center;
    color: var(--text-secondary, #a6adc8);
    font-size: 13px;
  }
</style>
```

**Step 2: Commit**

```bash
git add frontend/src/components/DiffView.svelte
git commit -m "feat(ui): add DiffView component for per-pane diff display"
```

---

### Task 4: Frontend — CommitPushDialog Component

**Files:**
- Create: `frontend/src/components/CommitPushDialog.svelte`

**Step 1: Create the CommitPushDialog component**

Dialog with: conventional commit type selector, auto-generated scope + description, push toggle, PR option, file list.

The dialog receives the list of selected files from DiffView and uses `GenerateCommitSuggestion` to pre-fill the message fields.

```svelte
<!-- frontend/src/components/CommitPushDialog.svelte -->
<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { App } from '../../wailsjs/go/backend/AppService';

  export let visible: boolean = false;
  export let dir: string = '';
  export let files: string[] = [];   // selected file paths from DiffView
  export let issueNumber: number = 0;
  export let branch: string = '';

  const dispatch = createEventDispatcher();

  const COMMIT_TYPES = [
    { value: 'feat',     label: 'feat',     desc: 'Neues Feature' },
    { value: 'fix',      label: 'fix',      desc: 'Bugfix' },
    { value: 'refactor', label: 'refactor', desc: 'Code-Umbau' },
    { value: 'chore',    label: 'chore',    desc: 'Wartung / Build' },
    { value: 'docs',     label: 'docs',     desc: 'Dokumentation' },
    { value: 'test',     label: 'test',     desc: 'Tests' },
    { value: 'style',    label: 'style',    desc: 'Formatierung' },
    { value: 'perf',     label: 'perf',     desc: 'Performance' },
    { value: 'ci',       label: 'ci',       desc: 'CI/CD' },
    { value: 'build',    label: 'build',    desc: 'Build-System' },
  ];

  let commitType: string = 'feat';
  let scope: string = '';
  let description: string = '';
  let body: string = '';
  let doPush: boolean = true;
  let createPR: boolean = false;
  let committing: boolean = false;
  let error: string = '';
  let loading: boolean = false;

  function buildMessage(): string {
    let msg = commitType;
    if (scope.trim()) msg += `(${scope.trim()})`;
    msg += ': ' + description.trim();
    if (body.trim()) msg += '\n\n' + body.trim();
    if (issueNumber > 0) {
      msg += `\n\nCloses #${issueNumber}`;
    }
    return msg;
  }

  async function initDialog() {
    error = '';
    committing = false;
    loading = true;
    doPush = true;
    createPR = issueNumber > 0;
    body = '';

    // Auto-generate suggestion from selected files
    try {
      const suggestion = await App.GenerateCommitSuggestion(dir, files);
      commitType = suggestion.type || 'feat';
      scope = suggestion.scope || '';
      description = suggestion.description || '';
    } catch {
      commitType = 'feat';
      scope = '';
      description = '';
    }
    loading = false;
  }

  $: if (visible) initDialog();

  async function doCommit() {
    if (!description.trim()) {
      error = 'Beschreibung erforderlich';
      return;
    }
    committing = true;
    error = '';

    try {
      // Stage only selected files
      await App.StageFiles(dir, files);

      // Commit staged changes
      const message = buildMessage();
      await App.CommitStaged(dir, message);

      if (doPush) {
        await App.PushBranch(dir);
      }

      if (createPR && doPush && issueNumber > 0) {
        dispatch('createPR', { issueNumber, branch });
      }

      dispatch('committed', { pushed: doPush });
      close();
    } catch (err: any) {
      error = err?.message || String(err);
    } finally {
      committing = false;
    }
  }

  function close() {
    dispatch('close');
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) doCommit();
  }
</script>

{#if visible}
<!-- svelte-ignore a11y-click-events-have-key-events -->
<div class="backdrop" on:click={close}>
  <!-- svelte-ignore a11y-click-events-have-key-events -->
  <div class="dialog" on:click|stopPropagation on:keydown={handleKeydown}>
    <h3>Commit{doPush ? ' & Push' : ''}</h3>

    <div class="summary">
      {files.length} Datei{files.length !== 1 ? 'en' : ''} ausgewählt
      {#if branch}&middot; <code>{branch}</code>{/if}
    </div>

    <!-- Conventional Commit Type -->
    <label class="field-label">Typ</label>
    <div class="type-grid">
      {#each COMMIT_TYPES as ct}
        <button
          class="type-btn"
          class:active={commitType === ct.value}
          on:click={() => commitType = ct.value}
          disabled={committing}
          title={ct.desc}
        >
          {ct.label}
        </button>
      {/each}
    </div>

    <!-- Scope (optional) -->
    <label class="field-label">Scope <span class="optional">(optional)</span></label>
    <input
      class="text-input"
      type="text"
      bind:value={scope}
      placeholder="z.B. ui, backend, config"
      disabled={committing}
    />

    <!-- Description (required) -->
    <label class="field-label">Beschreibung</label>
    <input
      class="text-input"
      type="text"
      bind:value={description}
      placeholder="Kurze Zusammenfassung der Änderung"
      disabled={committing}
    />

    <!-- Optional body for longer explanation -->
    <label class="field-label">Details <span class="optional">(optional)</span></label>
    <textarea
      class="message-input"
      bind:value={body}
      placeholder="Ausführlichere Erklärung..."
      rows="2"
      disabled={committing}
    ></textarea>

    <!-- Preview -->
    <div class="preview">
      <span class="preview-label">Vorschau:</span>
      <code class="preview-msg">{buildMessage().split('\n')[0]}</code>
    </div>

    <div class="options">
      <label class="toggle">
        <input type="checkbox" bind:checked={doPush} disabled={committing} />
        Push zu Origin
      </label>
      {#if issueNumber > 0 && doPush}
        <label class="toggle">
          <input type="checkbox" bind:checked={createPR} disabled={committing} />
          Pull Request erstellen
        </label>
      {/if}
    </div>

    {#if error}
      <div class="error">{error}</div>
    {/if}

    <div class="actions">
      <button class="btn cancel" on:click={close} disabled={committing}>Abbrechen</button>
      <button class="btn primary" on:click={doCommit} disabled={committing || !description.trim()}>
        {#if committing}Läuft...{:else}Commit{doPush ? ' & Push' : ''}{/if}
      </button>
    </div>

    <div class="hint">Ctrl+Enter zum Bestätigen</div>
  </div>
</div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 100;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
  }
  .dialog {
    background: var(--bg-primary, #1e1e2e);
    border: 1px solid var(--border-color, #313244);
    border-radius: 12px;
    padding: 20px;
    width: 440px;
    max-width: 90vw;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
  }
  h3 {
    margin: 0 0 12px;
    font-size: 15px;
    color: var(--text-primary, #cdd6f4);
  }
  .summary {
    font-size: 12px;
    color: var(--text-secondary, #a6adc8);
    margin-bottom: 12px;
  }
  .summary code {
    background: var(--bg-hover, #313244);
    padding: 1px 5px;
    border-radius: 3px;
    font-size: 11px;
  }
  .field-label {
    display: block;
    font-size: 11px;
    color: var(--text-secondary, #a6adc8);
    margin-bottom: 4px;
    margin-top: 10px;
    font-weight: 600;
  }
  .field-label .optional { font-weight: 400; opacity: 0.6; }
  .type-grid {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    margin-bottom: 4px;
  }
  .type-btn {
    padding: 3px 10px;
    border: 1px solid var(--border-color, #313244);
    border-radius: 4px;
    background: var(--bg-secondary, #181825);
    color: var(--text-secondary, #a6adc8);
    font-size: 11px;
    font-weight: 600;
    cursor: pointer;
  }
  .type-btn:hover { border-color: #7c3aed; color: var(--text-primary, #cdd6f4); }
  .type-btn.active { background: #7c3aed; color: #fff; border-color: #7c3aed; }
  .text-input {
    width: 100%;
    background: var(--bg-secondary, #181825);
    border: 1px solid var(--border-color, #313244);
    border-radius: 6px;
    color: var(--text-primary, #cdd6f4);
    padding: 6px 8px;
    font-size: 13px;
    font-family: inherit;
    box-sizing: border-box;
  }
  .text-input:focus { outline: none; border-color: #7c3aed; }
  .preview {
    margin: 10px 0 4px;
    padding: 6px 8px;
    background: var(--bg-secondary, #181825);
    border-radius: 4px;
    font-size: 11px;
  }
  .preview-label { color: var(--text-secondary, #a6adc8); margin-right: 6px; }
  .preview-msg { color: #50fa7b; font-family: monospace; }
  .message-input {
    width: 100%;
    background: var(--bg-secondary, #181825);
    border: 1px solid var(--border-color, #313244);
    border-radius: 6px;
    color: var(--text-primary, #cdd6f4);
    padding: 8px;
    font-size: 13px;
    font-family: inherit;
    resize: vertical;
    box-sizing: border-box;
  }
  .message-input:focus { outline: none; border-color: #7c3aed; }
  .options {
    display: flex;
    flex-direction: column;
    gap: 6px;
    margin: 12px 0;
  }
  .toggle {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    color: var(--text-primary, #cdd6f4);
    cursor: pointer;
  }
  .error {
    background: #ff555522;
    border: 1px solid #ff5555;
    border-radius: 6px;
    padding: 6px 10px;
    font-size: 12px;
    color: #ff5555;
    margin-bottom: 8px;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    margin-top: 4px;
  }
  .btn {
    padding: 6px 16px;
    border: none;
    border-radius: 6px;
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
  }
  .btn.cancel {
    background: var(--bg-hover, #313244);
    color: var(--text-primary, #cdd6f4);
  }
  .btn.primary {
    background: #7c3aed;
    color: #fff;
  }
  .btn.primary:hover:not(:disabled) { background: #6d28d9; }
  .btn:disabled { opacity: 0.5; cursor: not-allowed; }
  .hint {
    text-align: center;
    font-size: 10px;
    color: var(--text-secondary, #a6adc8);
    margin-top: 8px;
    opacity: 0.6;
  }
</style>
```

**Step 2: Commit**

```bash
git add frontend/src/components/CommitPushDialog.svelte
git commit -m "feat(ui): add CommitPushDialog for commit message and push options"
```

---

### Task 5: Integration — TerminalPane + PaneTitlebar

**Files:**
- Modify: `frontend/src/components/PaneTitlebar.svelte`
- Modify: `frontend/src/components/TerminalPane.svelte`

**Step 1: Add "Changes" button to PaneTitlebar**

In `PaneTitlebar.svelte`, add a new button next to the worktree button that toggles the diff view. The button should show a badge with the number of changed files.

Add to the script section:
```typescript
export let changeCount: number = 0;
// ... existing exports ...
```

Add a new event:
```typescript
function toggleDiffView() {
  dispatch('toggleDiff');
}
```

Add a button in the template (after the worktree-wrap div, before the model label):
```svelte
{#if pane.mode !== 'shell'}
  <button
    class="changes-btn"
    class:has-changes={changeCount > 0}
    on:click|stopPropagation={toggleDiffView}
    title="Änderungen anzeigen ({changeCount} Dateien)"
  >
    <span class="changes-icon">&#916;</span>
    {#if changeCount > 0}
      <span class="changes-badge">{changeCount}</span>
    {/if}
  </button>
{/if}
```

Add CSS:
```css
.changes-btn {
  display: flex;
  align-items: center;
  gap: 3px;
  background: none;
  border: 1px solid transparent;
  border-radius: 4px;
  color: var(--text-secondary, #a6adc8);
  cursor: pointer;
  padding: 1px 6px;
  font-size: 12px;
}
.changes-btn:hover { background: var(--bg-hover, #313244); }
.changes-btn.has-changes { color: #e2b714; border-color: #e2b71444; }
.changes-icon { font-weight: 700; }
.changes-badge {
  background: #e2b714;
  color: #000;
  border-radius: 8px;
  padding: 0 5px;
  font-size: 10px;
  font-weight: 700;
  line-height: 16px;
}
```

**Step 2: Add DiffView to TerminalPane**

In `TerminalPane.svelte`, add the DiffView overlay and CommitPushDialog.

Add to script:
```typescript
import DiffView from './DiffView.svelte';
import CommitPushDialog from './CommitPushDialog.svelte';

let showDiff = false;
let showCommitDialog = false;
let changeCount = 0;
let commitFiles: string[] = [];

// Poll for change count when pane uses claude/yolo mode
let diffPollInterval: ReturnType<typeof setInterval> | null = null;

function getPaneDir(): string {
  return pane.worktreePath || tabDir;
}

async function pollChangeCount() {
  if (pane.mode === 'shell') return;
  try {
    const stats = await App.GetDiffStats(getPaneDir());
    changeCount = stats?.length || 0;
  } catch {}
}

function startDiffPolling() {
  stopDiffPolling();
  if (pane.mode !== 'shell') {
    pollChangeCount();
    diffPollInterval = setInterval(pollChangeCount, 10_000);
  }
}

function stopDiffPolling() {
  if (diffPollInterval) {
    clearInterval(diffPollInterval);
    diffPollInterval = null;
  }
}

function handleToggleDiff() {
  showDiff = !showDiff;
  if (showDiff) pollChangeCount();
}

function handleCommitRequest(e: CustomEvent) {
  commitFiles = e.detail.files;
  showCommitDialog = true;
}

function handleCommitted() {
  showCommitDialog = false;
  // Keep DiffView open — it will refresh and show remaining uncommitted files
  // This enables the "small commits" workflow: commit a group, then commit another
  pollChangeCount();
}

function handleCreatePR(e: CustomEvent) {
  const { issueNumber } = e.detail;
  // Send gh pr create command to terminal
  const cmd = `gh pr create --title "Closes #${issueNumber}" --body "Resolves #${issueNumber}" --fill\n`;
  App.WriteToSession(pane.sessionId, btoa(cmd));
}
```

Lifecycle: start polling on mount (if not shell), stop on destroy. Add to existing `onMount`/`onDestroy`.

Add to template (inside the pane container, after the terminal div, before the closing wrapper):
```svelte
<DiffView
  dir={getPaneDir()}
  visible={showDiff}
  on:close={() => showDiff = false}
  on:commitRequest={handleCommitRequest}
/>

<CommitPushDialog
  visible={showCommitDialog}
  dir={getPaneDir()}
  files={commitFiles}
  issueNumber={pane.issueNumber || 0}
  branch={pane.branch || pane.issueBranch || ''}
  on:close={() => showCommitDialog = false}
  on:committed={handleCommitted}
  on:createPR={handleCreatePR}
/>
```

Pass `changeCount` to PaneTitlebar:
```svelte
<PaneTitlebar
  ...existing props...
  {changeCount}
  on:toggleDiff={handleToggleDiff}
/>
```

**Step 3: Forward toggleDiff event through PaneGrid**

In `PaneGrid.svelte`, add `on:toggleDiff` to the event forwarding list for TerminalPane.

**Step 4: Verify it compiles**

Run: `cd /d/repos/Multiterminal/frontend && npm run build 2>&1 | tail -10`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add frontend/src/components/PaneTitlebar.svelte frontend/src/components/TerminalPane.svelte frontend/src/components/PaneGrid.svelte
git commit -m "feat(ui): integrate DiffView and CommitPushDialog into pane UI"
```

---

### Task 6: Upgrade Existing Issue Actions

**Files:**
- Modify: `frontend/src/App.svelte`

**Step 1: Replace shell-command issue actions with new dialog**

In `App.svelte`, the existing `handleIssueAction` function (around line 562) sends raw shell commands to the terminal. Upgrade the `commit` action to open the new CommitPushDialog instead.

Find the `handleIssueAction` function and replace the `commit` branch:

```typescript
// OLD: App.WriteToSession(sessionId, encodeForPty(`git add -A && git commit -m '${msg}' && git push\n`));
// NEW: Let the TerminalPane handle it via its built-in CommitPushDialog
// The issueAction event with action='commit' should trigger the diff view in the pane
```

The cleanest approach: when `action === 'commit'`, find the pane and dispatch a custom event to it, OR simply let the existing PaneTitlebar issue menu route through the same `toggleDiff` mechanism.

Change the issue action menu in `PaneTitlebar.svelte`: Replace the `issueAction('commit')` handler to call `toggleDiffView()` instead so clicking "Commit & Push" in the issue menu opens the diff view with commit dialog.

**Step 2: Verify build**

Run: `cd /d/repos/Multiterminal/frontend && npm run build 2>&1 | tail -10`
Expected: Build succeeds

**Step 3: Test manually**

1. Start `wails dev`
2. Open a Claude pane in a repo with changes
3. Click the Delta (&#916;) button in the titlebar → DiffView should appear
4. File list should show changed files with +/- stats
5. Click a file → diff should appear with color coding
6. Click "Commit & Push" → CommitPushDialog should open
7. Enter message → commit succeeds → diff view closes

**Step 4: Commit**

```bash
git add frontend/src/App.svelte frontend/src/components/PaneTitlebar.svelte
git commit -m "feat(ui): upgrade issue commit action to use DiffView + CommitPushDialog"
```

---

### Task 7: Run Full Test Suite & Final Verification

**Step 1: Run Go tests**

Run: `cd /d/repos/Multiterminal && go test ./... -v 2>&1 | tail -30`
Expected: All tests PASS

**Step 2: Run go vet**

Run: `cd /d/repos/Multiterminal && go vet ./...`
Expected: No issues

**Step 3: Run frontend build**

Run: `cd /d/repos/Multiterminal/frontend && npm run build`
Expected: Build succeeds

**Step 4: Final commit (if any fixes needed)**

```bash
git add -A
git commit -m "fix: address test/build issues from diff-view integration"
```

---

## Summary of New Files

| File | Purpose |
|------|---------|
| `internal/backend/app_git_diff.go` | Git diff, staging, commit, push + auto-commit-suggestion APIs |
| `internal/backend/app_git_diff_test.go` | Tests for above |
| `frontend/src/components/DiffView.svelte` | Per-pane diff overlay with file checkboxes + diff rendering |
| `frontend/src/components/CommitPushDialog.svelte` | Conventional commit dialog: type selector, scope, auto-description, push/PR |

## Modified Files

| File | Change |
|------|--------|
| `frontend/wailsjs/go/models.ts` | Add `DiffFileStat` + `CommitSuggestion` classes |
| `frontend/src/components/PaneTitlebar.svelte` | Add "Changes" button + badge |
| `frontend/src/components/TerminalPane.svelte` | Integrate DiffView + CommitPushDialog + polling |
| `frontend/src/components/PaneGrid.svelte` | Forward `toggleDiff` event |
| `frontend/src/App.svelte` | Upgrade issue commit action |

## Key Design Decisions

1. **Selective staging** — Users check files in DiffView, only checked files get staged + committed
2. **Conventional commits** — Type grid (feat/fix/refactor/...) + scope + description, assembled automatically
3. **Auto-suggestion** — `GenerateCommitSuggestion` analyzes file patterns to pre-fill type, scope, and description
4. **Small commits** — After committing, DiffView stays open showing remaining uncommitted files for the next commit
5. **No `git add -A`** — `StageFiles` stages only selected paths, `CommitStaged` commits only staged changes

# Worktree-pro-Pane mit Finish-Flow — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Jedes Terminal-Pane kann opt-in in einem isolierten git-Worktree (Sibling-Verzeichnis) starten; ein ✓-Button führt einen verifizierten ff-only-Merge in den Ziel-Branch mit vollständigem Cleanup aus.

**Architecture:** Go-Backend hält den Finish-Zustand pro Session (Zustandsmaschine + pfad-gekeyter Idempotenz-Marker), orchestriert Prep über die bestehende Prompt-Queue und merged im Haupt-Worktree. Svelte-Frontend liefert Opt-in im LaunchDialog, ✓-Button/Badge in der Titlebar und ein Bestätigungs-Overlay; Kommunikation über sessionId-getaggte Wails-Events.

**Tech Stack:** Go 1.21+, Wails v3 alpha, Svelte 4 (legacy mode), xterm.js, git CLI (via `gitCmd`).

**Spec:** `docs/superpowers/specs/2026-07-02-worktree-pro-pane-design.md` (Rev. 2, red-team-geprüft — bei Abweichung gilt die Spec).

## Global Constraints

- **Max 300 Zeilen pro Go-Datei** — bei Überschreitung logisch splitten.
- **Alle git-/Prozess-Spawns über `gitCmd` bzw. mit `hideConsole`** (`internal/backend/app_git_cmd.go`, `hide_windows.go`) — sonst Console-Flash.
- **Frontend-exponierte Go-Structs brauchen `json`+`yaml`-Tags UND manuellen Sync in `frontend/wailsjs/go/models.ts`** (Klasse + Feld + Konstruktor-Zeile), sonst werden Felder still verworfen.
- **UI-Texte Deutsch, Code/Kommentare Englisch.**
- **SettingsDialog/Svelte: niemals Zuweisungen direkt in `$:`-Blöcke** (Recurring Bug) — Init-Logik in Funktionen.
- **Branch:** Arbeit auf `alpha-main` (bzw. Feature-Branch davon). Commits: Conventional Commits.
- **Ein lokaler Ziel-Ref für alle Checks UND den Merge** — kein `fetch`, kein `origin/…` (Spec §8).
- **Kein `git branch -D`, kein blindes `git add -A`** — Datenverlust-Leitplanken der Spec.
- Tests: `go test ./internal/backend/... ./internal/terminal/... ./internal/config/...`; git-Tests bauen echte Temp-Repos (git CLI ist auf allen Dev-/CI-Maschinen vorhanden).

## Datei-Landkarte

| Datei | Verantwortung |
|---|---|
| `internal/backend/app_worktree_pane.go` (neu) | `mainRepoRoot`, Sibling-Pfad, freie Namensfindung, `CreatePaneWorktree`, `GetPaneWorktreeDefaults` |
| `internal/backend/app_worktree_pane_files.go` (neu) | Steuerdateien: `CLAUDE.local.md`, `.claude/settings.local.json`, `info/exclude` |
| `internal/backend/app_worktree_finish_status.go` (neu) | `GetWorktreeFinishStatus` + git-Check-Helfer (`revCount`, `isAncestor`, `currentBranch`) |
| `internal/backend/app_worktree_marker.go` (neu) | Pfad-gekeyter Idempotenz-Marker (`~/.multiterminal-worktree-finish.json`) |
| `internal/backend/app_worktree_finish.go` (neu) | `finishState`-Maschine, `StartWorktreeFinish`, `CancelWorktreeFinish`, `onQueueItemDone`, `FinishWorktree`, Reconcile |
| `internal/backend/app_worktree_shell.go` (neu) | Shell-Finish-Primitive: `GetWorktreeChangedFiles`, `CommitWorktreeFiles`, `RebaseWorktreeOntoTarget` |
| `internal/backend/kill_windows.go` / `kill_other.go` (neu) | `killProcessTree` (taskkill /T /F bzw. Unix-Kill) |
| `internal/backend/app_events.go` (mod) | Event-Payload-Structs `WorktreeFinish*Event` |
| `internal/backend/app_queue.go` (mod) | Item-Done-Hook, Prep-Item-Guards in Remove/Clear, Queue-Sperre |
| `internal/backend/app_scan.go` (mod) | waiting*-Weiterleitung bei aktivem Finish |
| `internal/backend/app.go` (mod) | `finishStates`-Map-Init, Cleanup in `CloseSession` |
| `internal/terminal/session.go` (mod) | `Pid()`-Getter |
| `internal/config/session.go` (mod) | `SavedPane`: `worktree_path`, `worktree_branch`, `target_branch` |
| `frontend/wailsjs/go/models.ts` (mod) | Sync: SavedPane-Felder, `PaneWorktreeInfo`, `WorktreeFinishStatus`, `WorktreeFileChange` |
| `frontend/src/stores/tabs.ts` (mod) | `Pane.targetBranch`, `addPane`-Param, `setFinishPhase` |
| `frontend/src/lib/session.ts` (mod) | save/restore inkl. **CWD = worktree_path** |
| `frontend/src/components/LaunchDialog.svelte` (mod) | Worktree-Checkbox + Felder, Chat-Ausblendung |
| `frontend/src/App.svelte` (mod) | Launch-Verkettung, Event-Wiring, Finish-Dialog-Einbindung |
| `frontend/src/components/PaneTitlebar.svelte` (mod) | ⎇-Badge, ✓-Button, preparing-Spinner |
| `frontend/src/components/WorktreeFinishDialog.svelte` (neu) | Overlay: ready/blocked/cleanup/Staging (Shell) |

---

### Task 1: `mainRepoRoot` — Haupt-Worktree-Bestimmung

**Files:**
- Create: `internal/backend/app_worktree_pane.go`
- Test: `internal/backend/app_worktree_pane_test.go`

**Interfaces:**
- Consumes: `parseWorktreePorcelain(output string) []WorktreeInfo` (existiert, `app_worktree.go:121`), `gitCmd(dir, args...)` (`app_git_cmd.go:15`)
- Produces: `mainRepoRoot(dir string) (string, error)` — absoluter Pfad des Haupt-Worktrees; von Task 2, 4, 5, 11 benutzt. Test-Helfer `initPaneTestRepo(t *testing.T) string`.

**Hintergrund (Spec 3.2):** `--show-toplevel` liefert im Worktree den Worktree-Pfad, `--git-common-dir` liefert im Haupt-Repo das relative `.git`. Einzig robust: erster Eintrag von `git worktree list --porcelain` ist git-garantiert der Haupt-Worktree.

- [ ] **Step 1: Failing Test schreiben**

```go
// internal/backend/app_worktree_pane_test.go
package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// initPaneTestRepo creates a real git repo on branch alpha-main with one commit.
// Returned path is EvalSymlinks-resolved (t.TempDir may be a symlink on Windows/macOS).
func initPaneTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		dir = r
	}
	gitRun(t, dir, "init", "-b", "alpha-main")
	gitRun(t, dir, "config", "user.email", "test@test.local")
	gitRun(t, dir, "config", "user.name", "Test")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("init\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "init")
	return dir
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := gitCmd(dir, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestMainRepoRoot_FromMainRepo(t *testing.T) {
	repo := initPaneTestRepo(t)
	got, err := mainRepoRoot(repo)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(got, repo) {
		t.Errorf("mainRepoRoot = %q, want %q", got, repo)
	}
}

func TestMainRepoRoot_FromInsideWorktree(t *testing.T) {
	repo := initPaneTestRepo(t)
	wt := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+"-wt")
	gitRun(t, repo, "worktree", "add", "-b", "terminal/x", wt, "alpha-main")
	got, err := mainRepoRoot(wt)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(got, repo) {
		t.Errorf("mainRepoRoot from worktree = %q, want main repo %q", got, repo)
	}
}

func TestMainRepoRoot_NotARepo(t *testing.T) {
	if _, err := mainRepoRoot(t.TempDir()); err == nil {
		t.Error("expected error for non-repo dir")
	}
}
```

- [ ] **Step 2: Test laufen lassen — muss fehlschlagen**

Run: `go test ./internal/backend/ -run TestMainRepoRoot -v`
Expected: FAIL / compile error `undefined: mainRepoRoot`

- [ ] **Step 3: Minimale Implementierung**

```go
// internal/backend/app_worktree_pane.go
// Package backend – per-pane git worktrees (sibling directory) with a
// deterministic finish flow (ff-only merge into the target branch + cleanup).
package backend

import (
	"fmt"
	"path/filepath"
)

// mainRepoRoot returns the absolute path of the MAIN worktree for any dir
// inside the repo (main checkout or linked worktree).
// It relies on the git guarantee that `git worktree list --porcelain` always
// lists the main worktree first. --show-toplevel (worktree path) and
// --git-common-dir (relative ".git" in the main repo) are both unsuitable.
func mainRepoRoot(dir string) (string, error) {
	out, err := gitCmd(dir, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return "", fmt.Errorf("not a git repo: %w", err)
	}
	entries := parseWorktreePorcelain(string(out))
	if len(entries) == 0 || entries[0].Path == "" {
		return "", fmt.Errorf("no worktrees found in %s", dir)
	}
	return filepath.FromSlash(entries[0].Path), nil
}
```

- [ ] **Step 4: Test laufen lassen — muss grün sein**

Run: `go test ./internal/backend/ -run TestMainRepoRoot -v`
Expected: PASS (3 Tests)

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_worktree_pane.go internal/backend/app_worktree_pane_test.go
git commit -m "feat(worktree): mainRepoRoot via first worktree-list entry"
```

---

### Task 2: Sibling-Pfad + freie Namensfindung

**Files:**
- Modify: `internal/backend/app_worktree_pane.go`
- Test: `internal/backend/app_worktree_pane_test.go`

**Interfaces:**
- Consumes: `sanitizeWorktreeName(name string) string` (existiert, `app_worktree.go:224`), `branchExists(root, branch string) bool` (existiert, `app_git_branch.go`), `mainRepoRoot` (Task 1)
- Produces: `paneWorktreeBase(mainRoot string) string` und `findFreePaneName(mainRoot, base string) string` — von Task 4 benutzt.

- [ ] **Step 1: Failing Tests schreiben** (an `app_worktree_pane_test.go` anhängen)

```go
func TestPaneWorktreeBase(t *testing.T) {
	got := paneWorktreeBase(filepath.Join("D:", "repos", "Foo"))
	want := filepath.Join("D:", "repos", "Foo.mt-worktrees")
	if got != want {
		t.Errorf("paneWorktreeBase = %q, want %q", got, want)
	}
}

func TestFindFreePaneName_NoCollision(t *testing.T) {
	repo := initPaneTestRepo(t)
	if got := findFreePaneName(repo, "My Feature!"); got != "my-feature" {
		t.Errorf("got %q, want my-feature", got)
	}
}

func TestFindFreePaneName_BranchCollisionIncrements(t *testing.T) {
	repo := initPaneTestRepo(t)
	gitRun(t, repo, "branch", "terminal/fix")
	if got := findFreePaneName(repo, "fix"); got != "fix-2" {
		t.Errorf("got %q, want fix-2", got)
	}
}

func TestFindFreePaneName_DirCollisionIncrements(t *testing.T) {
	repo := initPaneTestRepo(t)
	dir := filepath.Join(paneWorktreeBase(repo), "fix")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if got := findFreePaneName(repo, "fix"); got != "fix-2" {
		t.Errorf("got %q, want fix-2", got)
	}
}
```

- [ ] **Step 2: Testlauf — FAIL** — Run: `go test ./internal/backend/ -run 'TestPaneWorktreeBase|TestFindFreePaneName' -v` → compile error `undefined`

- [ ] **Step 3: Implementierung** (an `app_worktree_pane.go` anhängen; `os` importieren)

```go
// paneWorktreeBase returns the sibling directory that holds all pane
// worktrees for a repo: <parent>/<repo-name>.mt-worktrees
// Sibling (not in-repo) so builds, watchers and CLAUDE.md discovery in the
// main repo never see worktree contents (spec 3.1).
func paneWorktreeBase(mainRoot string) string {
	return filepath.Join(filepath.Dir(mainRoot), filepath.Base(mainRoot)+".mt-worktrees")
}

// findFreePaneName sanitizes base and appends -2, -3, … until neither the
// branch terminal/<name> nor the sibling directory exists. Default names like
// pane-3 would otherwise collide with leftover branches every launch (spec 3.3/2).
func findFreePaneName(mainRoot, base string) string {
	name := sanitizeWorktreeName(base)
	for i := 1; ; i++ {
		candidate := name
		if i > 1 {
			candidate = fmt.Sprintf("%s-%d", name, i)
		}
		if branchExists(mainRoot, "terminal/"+candidate) {
			continue
		}
		if _, err := os.Stat(filepath.Join(paneWorktreeBase(mainRoot), candidate)); err == nil {
			continue
		}
		return candidate
	}
}
```

- [ ] **Step 4: Testlauf — PASS** — Run wie Step 2, Expected: PASS (4 Tests)

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_worktree_pane.go internal/backend/app_worktree_pane_test.go
git commit -m "feat(worktree): sibling base dir + collision-free pane names"
```

---

### Task 3: Steuerdateien (CLAUDE.local.md, settings.local.json, info/exclude)

**Files:**
- Create: `internal/backend/app_worktree_pane_files.go`
- Test: `internal/backend/app_worktree_pane_files_test.go`

**Interfaces:**
- Consumes: nichts Projektspezifisches (nur stdlib)
- Produces: `writeWorktreeControlFiles(wtPath, mainRoot, branch, target string) error` und `ensureInfoExclude(mainRoot string, patterns []string) error` — von Task 4 benutzt.

- [ ] **Step 1: Failing Tests schreiben**

```go
// internal/backend/app_worktree_pane_files_test.go
package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureInfoExclude_AddsOnceIdempotent(t *testing.T) {
	repo := initPaneTestRepo(t)
	for i := 0; i < 2; i++ {
		if err := ensureInfoExclude(repo, []string{"CLAUDE.local.md", ".claude/settings.local.json"}); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "CLAUDE.local.md"); got != 1 {
		t.Errorf("CLAUDE.local.md appears %d times, want 1", got)
	}
	if !strings.Contains(string(data), ".claude/settings.local.json") {
		t.Error("settings.local.json pattern missing")
	}
}

func TestWriteWorktreeControlFiles(t *testing.T) {
	repo := initPaneTestRepo(t)
	wt := filepath.Join(filepath.Dir(repo), filepath.Base(repo)+".mt-worktrees", "feat")
	gitRun(t, repo, "worktree", "add", "-b", "terminal/feat", wt, "alpha-main")
	if err := writeWorktreeControlFiles(wt, repo, "terminal/feat", "alpha-main"); err != nil {
		t.Fatal(err)
	}
	md, err := os.ReadFile(filepath.Join(wt, "CLAUDE.local.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"terminal/feat", "alpha-main", "NIEMALS"} {
		if !strings.Contains(string(md), want) {
			t.Errorf("CLAUDE.local.md missing %q", want)
		}
	}
	settings, err := os.ReadFile(filepath.Join(wt, ".claude", "settings.local.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"deny"`, "git merge ", "git worktree remove ", "git branch -D ", "git push "} {
		if !strings.Contains(string(settings), want) {
			t.Errorf("settings.local.json missing %q", want)
		}
	}
	// Control files must be invisible to git status (tracked-only AND untracked):
	if out := gitRun(t, wt, "status", "--porcelain"); strings.Contains(out, "CLAUDE.local.md") || strings.Contains(out, "settings.local.json") {
		t.Errorf("control files leak into git status: %q", out)
	}
}
```

- [ ] **Step 2: Testlauf — FAIL** — Run: `go test ./internal/backend/ -run 'TestEnsureInfoExclude|TestWriteWorktreeControlFiles' -v` → compile error

- [ ] **Step 3: Implementierung**

```go
// internal/backend/app_worktree_pane_files.go
// Control files written into each pane worktree: CLAUDE.local.md (context for
// Claude Code) and .claude/settings.local.json (best-effort deny backstop).
// Both are excluded via the SHARED .git/info/exclude so they never appear in
// git status or get committed by "git add -A" (spec 3.3/5 — the real safety
// net is the merge verification, not these files).
package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ensureInfoExclude appends patterns to <mainRoot>/.git/info/exclude unless an
// exact trimmed line already exists. info/exclude is shared across all
// worktrees of the repo.
func ensureInfoExclude(mainRoot string, patterns []string) error {
	p := filepath.Join(mainRoot, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	existing, _ := os.ReadFile(p) // missing file is fine
	have := map[string]bool{}
	for _, line := range strings.Split(string(existing), "\n") {
		have[strings.TrimSpace(line)] = true
	}
	var add []string
	for _, pat := range patterns {
		if !have[strings.TrimSpace(pat)] {
			add = append(add, pat)
		}
	}
	if len(add) == 0 {
		return nil
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	prefix := ""
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		prefix = "\n"
	}
	_, err = f.WriteString(prefix + strings.Join(add, "\n") + "\n")
	return err
}

func claudeLocalMD(branch, target string) string {
	return fmt.Sprintf(`# MTUI-Worktree

Du arbeitest in einem isolierten MTUI-Worktree.
- Branch: `+"`%s`"+`
- Ziel-Branch: `+"`%s`"+` (lokal — kein fetch nötig, kein Push)

Diese Worktree-Regeln haben Vorrang vor der Git-Workflow-Sektion der
Projekt-CLAUDE.md (kein PR, kein Push, kein Branch-Wechsel aus diesem Worktree).

Regeln:
- Committe abgeschlossene Arbeit in nachvollziehbaren Commits.
  Committe KEINE Secrets, .env-Dateien oder Build-Artefakte — ergänze für
  solche Dateien .gitignore-Einträge oder lass sie untracked stehen.
- Merge NIEMALS selbst in `+"`%s`"+`. Lösche NIEMALS diesen Worktree
  oder Branch. Beides macht MTUI über den ✓-Button in der Pane-Titelleiste.
- Bei Rebase-Konflikten: löse sie NICHT eigenmächtig — führe
  `+"`git rebase --abort`"+` aus und nenne die Konfliktdateien. Der User entscheidet.
- Wenn der User die Arbeit abschließen will: committe alle offenen Änderungen,
  rebase auf den lokalen `+"`%s`"+` und weise den User auf den ✓-Button hin.
`, branch, target, target, target)
}

func settingsLocalJSON() string {
	return `{
  "permissions": {
    "deny": [
      "Bash(git merge *)",
      "Bash(git worktree remove *)",
      "Bash(git branch -D *)",
      "Bash(git push *)",
      "Write(CLAUDE.local.md)",
      "Edit(CLAUDE.local.md)",
      "Write(.claude/settings.local.json)",
      "Edit(.claude/settings.local.json)"
    ]
  }
}
`
}

// writeWorktreeControlFiles writes both control files into wtPath and makes
// them invisible to git via the shared info/exclude.
func writeWorktreeControlFiles(wtPath, mainRoot, branch, target string) error {
	if err := ensureInfoExclude(mainRoot, []string{"CLAUDE.local.md", ".claude/settings.local.json"}); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(wtPath, "CLAUDE.local.md"), []byte(claudeLocalMD(branch, target)), 0644); err != nil {
		return err
	}
	claudeDir := filepath.Join(wtPath, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(claudeDir, "settings.local.json"), []byte(settingsLocalJSON()), 0644)
}
```

- [ ] **Step 4: Testlauf — PASS** — Run wie Step 2, Expected: PASS (2 Tests)

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_worktree_pane_files.go internal/backend/app_worktree_pane_files_test.go
git commit -m "feat(worktree): control files + shared info/exclude entries"
```

---

### Task 4: `CreatePaneWorktree` + `GetPaneWorktreeDefaults` (Bindings)

**Files:**
- Modify: `internal/backend/app_worktree_pane.go`
- Test: `internal/backend/app_worktree_pane_test.go`

**Interfaces:**
- Consumes: Tasks 1–3, `branchExists`, `gitCmd`
- Produces (frontend-exponiert, models.ts-Sync in Task 13):

```go
type PaneWorktreeInfo struct {
    Path         string `json:"path" yaml:"path"`
    Branch       string `json:"branch" yaml:"branch"`
    TargetBranch string `json:"target_branch" yaml:"target_branch"`
}
type PaneWorktreeDefaults struct {
    Name         string `json:"name" yaml:"name"`
    TargetBranch string `json:"target_branch" yaml:"target_branch"` // "" bei detached HEAD
}
func (a *AppService) CreatePaneWorktree(dir, name, targetBranch string) (*PaneWorktreeInfo, error)
func (a *AppService) GetPaneWorktreeDefaults(dir, base string) PaneWorktreeDefaults
```

- [ ] **Step 1: Failing Tests schreiben** (anhängen)

```go
func TestCreatePaneWorktree_HappyPath(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	wt, err := a.CreatePaneWorktree(repo, "login fix", "alpha-main")
	if err != nil {
		t.Fatal(err)
	}
	if wt.Branch != "terminal/login-fix" || wt.TargetBranch != "alpha-main" {
		t.Errorf("unexpected info: %+v", wt)
	}
	if !strings.HasPrefix(strings.ToLower(wt.Path), strings.ToLower(paneWorktreeBase(repo)+string(filepath.Separator))) {
		t.Errorf("worktree not in sibling base: %q", wt.Path)
	}
	if _, err := os.Stat(filepath.Join(wt.Path, "CLAUDE.local.md")); err != nil {
		t.Error("CLAUDE.local.md missing")
	}
	if _, err := os.Stat(filepath.Join(wt.Path, ".claude", "settings.local.json")); err != nil {
		t.Error("settings.local.json missing")
	}
}

func TestCreatePaneWorktree_MissingTargetBranch(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	if _, err := a.CreatePaneWorktree(repo, "x", "nope"); err == nil {
		t.Error("expected error for missing target branch")
	}
	if _, err := a.CreatePaneWorktree(repo, "x", ""); err == nil {
		t.Error("expected error for empty target branch")
	}
}

func TestCreatePaneWorktree_ExistingBranchIsHardError(t *testing.T) {
	repo := initPaneTestRepo(t)
	gitRun(t, repo, "branch", "terminal/x")
	a := &AppService{}
	if _, err := a.CreatePaneWorktree(repo, "x", "alpha-main"); err == nil {
		t.Error("expected error for manually chosen colliding name")
	}
}

func TestGetPaneWorktreeDefaults(t *testing.T) {
	repo := initPaneTestRepo(t)
	gitRun(t, repo, "branch", "terminal/fix")
	a := &AppService{}
	d := a.GetPaneWorktreeDefaults(repo, "fix")
	if d.Name != "fix-2" || d.TargetBranch != "alpha-main" {
		t.Errorf("defaults = %+v, want fix-2/alpha-main", d)
	}
}
```

- [ ] **Step 2: Testlauf — FAIL** — Run: `go test ./internal/backend/ -run 'TestCreatePaneWorktree|TestGetPaneWorktreeDefaults' -v` → compile error

- [ ] **Step 3: Implementierung** (anhängen; Imports `log`, `os`, `strings` ergänzen)

```go
// PaneWorktreeInfo describes a pane worktree returned to the frontend.
type PaneWorktreeInfo struct {
	Path         string `json:"path" yaml:"path"`
	Branch       string `json:"branch" yaml:"branch"`
	TargetBranch string `json:"target_branch" yaml:"target_branch"`
}

// PaneWorktreeDefaults prefills the launch dialog fields.
type PaneWorktreeDefaults struct {
	Name         string `json:"name" yaml:"name"`
	TargetBranch string `json:"target_branch" yaml:"target_branch"`
}

// GetPaneWorktreeDefaults returns a collision-free name and the branch
// currently checked out in the MAIN worktree ("" on detached HEAD — the
// dialog then forces an explicit choice, spec 3.3/3).
func (a *AppService) GetPaneWorktreeDefaults(dir, base string) PaneWorktreeDefaults {
	root, err := mainRepoRoot(dir)
	if err != nil {
		return PaneWorktreeDefaults{Name: sanitizeWorktreeName(base)}
	}
	target := ""
	if out, err := gitCmd(root, "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		if b := strings.TrimSpace(string(out)); b != "HEAD" {
			target = b
		}
	}
	return PaneWorktreeDefaults{Name: findFreePaneName(root, base), TargetBranch: target}
}

// CreatePaneWorktree creates the sibling worktree with branch terminal/<name>
// forked from targetBranch and writes the control files.
func (a *AppService) CreatePaneWorktree(dir, name, targetBranch string) (*PaneWorktreeInfo, error) {
	root, err := mainRepoRoot(dir)
	if err != nil {
		return nil, err
	}
	if targetBranch == "" {
		return nil, fmt.Errorf("Ziel-Branch fehlt")
	}
	if !branchExists(root, targetBranch) {
		return nil, fmt.Errorf("Ziel-Branch %q existiert lokal nicht", targetBranch)
	}
	safe := sanitizeWorktreeName(name)
	branch := "terminal/" + safe
	wtPath := filepath.Join(paneWorktreeBase(root), safe)
	if branchExists(root, branch) {
		return nil, fmt.Errorf("Branch %q existiert bereits – bitte anderen Namen wählen", branch)
	}
	if _, err := os.Stat(wtPath); err == nil {
		return nil, fmt.Errorf("Verzeichnis %q existiert bereits – bitte anderen Namen wählen", wtPath)
	}
	if err := os.MkdirAll(filepath.Dir(wtPath), 0755); err != nil {
		return nil, fmt.Errorf("Worktree-Basisverzeichnis nicht anlegbar (Repo-Parent schreibgeschützt?): %w", err)
	}
	out, err := gitCmd(root, "worktree", "add", "-b", branch, wtPath, targetBranch).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("worktree add failed: %s – %w", strings.TrimSpace(string(out)), err)
	}
	if err := writeWorktreeControlFiles(wtPath, root, branch, targetBranch); err != nil {
		log.Printf("[CreatePaneWorktree] control files: %v", err) // non-fatal
	}
	log.Printf("[CreatePaneWorktree] %s on %s (target %s)", wtPath, branch, targetBranch)
	return &PaneWorktreeInfo{Path: wtPath, Branch: branch, TargetBranch: targetBranch}, nil
}
```

- [ ] **Step 4: Testlauf — PASS** — Run wie Step 2, Expected: PASS (4 Tests). Falls `app_worktree_pane.go` > 300 Zeilen: Defaults+Info-Structs nach `app_worktree_pane_files.go` verschieben.

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_worktree_pane.go internal/backend/app_worktree_pane_test.go
git commit -m "feat(worktree): CreatePaneWorktree + launch defaults binding"
```

---

### Task 5: Status-Checks `GetWorktreeFinishStatus`

**Files:**
- Create: `internal/backend/app_worktree_finish_status.go`
- Test: `internal/backend/app_worktree_finish_status_test.go`

**Interfaces:**
- Consumes: `mainRepoRoot` (Task 1), `gitCmd`
- Produces (frontend-exponiert, Sync Task 13):

```go
type WorktreeFinishStatus struct {
    State     string   `json:"state" yaml:"state"` // "ready" | "cleanup_only" | "blocked"
    Reason    string   `json:"reason,omitempty" yaml:"reason,omitempty"`
    Commits   []string `json:"commits,omitempty" yaml:"commits,omitempty"`
    Stat      string   `json:"stat,omitempty" yaml:"stat,omitempty"`
    Untracked []string `json:"untracked,omitempty" yaml:"untracked,omitempty"`
}
func (a *AppService) GetWorktreeFinishStatus(worktreePath, branch, target string) WorktreeFinishStatus
```
- Intern: `revCount(root, from, to string) (int, error)`, `isAncestor(root, anc, desc string) bool`, `checkedOutBranch(root string) string` — von Task 11 wiederverwendet.

- [ ] **Step 1: Failing Tests schreiben**

```go
// internal/backend/app_worktree_finish_status_test.go
package backend

import (
	"os"
	"path/filepath"
	"testing"
)

// finishFixture: repo + pane worktree with one committed change on the branch.
func finishFixture(t *testing.T) (repo, wt string) {
	t.Helper()
	repo = initPaneTestRepo(t)
	a := &AppService{}
	info, err := a.CreatePaneWorktree(repo, "feat", "alpha-main")
	if err != nil {
		t.Fatal(err)
	}
	wt = info.Path
	if err := os.WriteFile(filepath.Join(wt, "work.txt"), []byte("x\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, wt, "add", "work.txt")
	gitRun(t, wt, "commit", "-m", "feat: work")
	return repo, wt
}

func TestFinishStatus_Ready(t *testing.T) {
	_, wt := finishFixture(t)
	a := &AppService{}
	s := a.GetWorktreeFinishStatus(wt, "terminal/feat", "alpha-main")
	if s.State != "ready" {
		t.Fatalf("state = %s (%s), want ready", s.State, s.Reason)
	}
	if len(s.Commits) != 1 || s.Stat == "" {
		t.Errorf("commits/stat not populated: %+v", s)
	}
}

func TestFinishStatus_CleanupOnly_NoCommits(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	info, _ := a.CreatePaneWorktree(repo, "empty", "alpha-main")
	s := a.GetWorktreeFinishStatus(info.Path, "terminal/empty", "alpha-main")
	if s.State != "cleanup_only" {
		t.Errorf("state = %s, want cleanup_only (0 commits ⇒ nichts zu mergen, deckt auch Crash-nach-Merge ab)", s.State)
	}
}

func TestFinishStatus_Blocked_NotRebased(t *testing.T) {
	repo, wt := finishFixture(t)
	// Move target forward so branch is no longer rebased onto it.
	if err := os.WriteFile(filepath.Join(repo, "main.txt"), []byte("y\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", "main.txt")
	gitRun(t, repo, "commit", "-m", "main moves")
	a := &AppService{}
	s := a.GetWorktreeFinishStatus(wt, "terminal/feat", "alpha-main")
	if s.State != "blocked" {
		t.Errorf("state = %s, want blocked (not rebased)", s.State)
	}
}

func TestFinishStatus_Blocked_DirtyTracked_ButUntrackedOK(t *testing.T) {
	_, wt := finishFixture(t)
	a := &AppService{}
	// Untracked file must NOT block (spec 5.3/3), only list:
	if err := os.WriteFile(filepath.Join(wt, "scratch.log"), []byte("tmp"), 0644); err != nil {
		t.Fatal(err)
	}
	s := a.GetWorktreeFinishStatus(wt, "terminal/feat", "alpha-main")
	if s.State != "ready" || len(s.Untracked) != 1 {
		t.Fatalf("untracked handling wrong: %+v", s)
	}
	// Modified tracked file MUST block:
	if err := os.WriteFile(filepath.Join(wt, "work.txt"), []byte("dirty\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if s := a.GetWorktreeFinishStatus(wt, "terminal/feat", "alpha-main"); s.State != "blocked" {
		t.Errorf("dirty tracked file not blocking: %+v", s)
	}
}

func TestFinishStatus_Blocked_MainDirtyOrWrongBranch(t *testing.T) {
	repo, wt := finishFixture(t)
	a := &AppService{}
	// Main worktree dirty (tracked change) blocks:
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("dirty\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if s := a.GetWorktreeFinishStatus(wt, "terminal/feat", "alpha-main"); s.State != "blocked" {
		t.Error("dirty main worktree not blocking")
	}
	gitRun(t, repo, "checkout", "--", "README.md")
	// Wrong branch checked out in main blocks:
	gitRun(t, repo, "checkout", "-b", "other")
	if s := a.GetWorktreeFinishStatus(wt, "terminal/feat", "alpha-main"); s.State != "blocked" {
		t.Error("wrong main branch not blocking")
	}
}
```

- [ ] **Step 2: Testlauf — FAIL** — Run: `go test ./internal/backend/ -run TestFinishStatus -v` → compile error

- [ ] **Step 3: Implementierung**

```go
// internal/backend/app_worktree_finish_status.go
// Hard verification gate before a worktree finish: all checks run against the
// LOCAL target ref — the same ref the merge later uses (spec 5.3).
package backend

import (
	"fmt"
	"strconv"
	"strings"
)

// WorktreeFinishStatus is the result of the pre-merge verification.
type WorktreeFinishStatus struct {
	State     string   `json:"state" yaml:"state"` // "ready" | "cleanup_only" | "blocked"
	Reason    string   `json:"reason,omitempty" yaml:"reason,omitempty"`
	Commits   []string `json:"commits,omitempty" yaml:"commits,omitempty"`
	Stat      string   `json:"stat,omitempty" yaml:"stat,omitempty"`
	Untracked []string `json:"untracked,omitempty" yaml:"untracked,omitempty"`
}

func revCount(root, from, to string) (int, error) {
	out, err := gitCmd(root, "rev-list", "--count", from+".."+to).Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

func isAncestor(root, anc, desc string) bool {
	return gitCmd(root, "merge-base", "--is-ancestor", anc, desc).Run() == nil
}

func checkedOutBranch(root string) string {
	out, err := gitCmd(root, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// trackedDirty reports whether tracked files have modifications (untracked
// files are deliberately ignored — spec 5.3/3, untracked-artifact deadlock).
func trackedDirty(dir string) bool {
	out, err := gitCmd(dir, "status", "--porcelain", "--untracked-files=no").Output()
	return err != nil || len(strings.TrimSpace(string(out))) > 0
}

func untrackedFiles(dir string) []string {
	out, err := gitCmd(dir, "status", "--porcelain").Output()
	if err != nil {
		return nil
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "?? ") {
			files = append(files, strings.TrimPrefix(line, "?? "))
		}
	}
	return files
}

func blocked(reason string) WorktreeFinishStatus {
	return WorktreeFinishStatus{State: "blocked", Reason: reason}
}

// GetWorktreeFinishStatus verifies a pane worktree is mergeable into target.
func (a *AppService) GetWorktreeFinishStatus(worktreePath, branch, target string) WorktreeFinishStatus {
	root, err := mainRepoRoot(worktreePath)
	if err != nil {
		return blocked(err.Error())
	}
	count, err := revCount(root, target, branch)
	if err != nil {
		return blocked(fmt.Sprintf("git rev-list fehlgeschlagen: %v", err))
	}
	if count == 0 {
		// Branch fully contained in target: either never worked, or a crash
		// happened after merge but before the marker — both end in a safe
		// cleanup instead of a deadlock (spec 5.3/1, red-team G2-K2).
		return WorktreeFinishStatus{State: "cleanup_only", Untracked: untrackedFiles(worktreePath)}
	}
	if !isAncestor(root, target, branch) {
		return blocked(fmt.Sprintf("Branch ist nicht auf %s rebased — erneut vorbereiten", target))
	}
	if trackedDirty(worktreePath) {
		return blocked("Uncommittete Änderungen im Worktree")
	}
	if got := checkedOutBranch(root); got != target {
		return blocked(fmt.Sprintf("Im Haupt-Repo ist %q ausgecheckt, nicht der Ziel-Branch %q — paralleles Finishen geht nur auf denselben Ziel-Branch", got, target))
	}
	if trackedDirty(root) {
		return blocked("Das Haupt-Repo hat uncommittete Änderungen — der Merge würde Dateien dort bewegen")
	}
	logOut, _ := gitCmd(root, "log", "--oneline", target+".."+branch).Output()
	var commits []string
	for _, l := range strings.Split(strings.TrimSpace(string(logOut)), "\n") {
		if l != "" {
			commits = append(commits, l)
		}
	}
	statOut, _ := gitCmd(root, "diff", "--stat", target+"..."+branch).Output()
	return WorktreeFinishStatus{
		State:     "ready",
		Commits:   commits,
		Stat:      strings.TrimSpace(string(statOut)),
		Untracked: untrackedFiles(worktreePath),
	}
}
```

- [ ] **Step 4: Testlauf — PASS** — Run wie Step 2, Expected: PASS (5 Tests)

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_worktree_finish_status.go internal/backend/app_worktree_finish_status_test.go
git commit -m "feat(worktree): pre-merge verification gate (GetWorktreeFinishStatus)"
```

---

### Task 6: Idempotenz-Marker (pfad-gekeyt)

**Files:**
- Create: `internal/backend/app_worktree_marker.go`
- Test: `internal/backend/app_worktree_marker_test.go`

**Interfaces:**
- Consumes: stdlib
- Produces: `loadFinishMarkers(path string) map[string]finishMarker`, `saveFinishMarker(path, wtPath string, m finishMarker) error`, `deleteFinishMarker(path, wtPath string) error`, `finishMarkerPath() string` — von Task 11 benutzt. Gekeyt nach **absolutem Worktree-Pfad** (Session-IDs überleben Neustarts nicht; Session-JSON wird vom Frontend überschrieben — Spec 4.4).

- [ ] **Step 1: Failing Tests schreiben**

```go
// internal/backend/app_worktree_marker_test.go
package backend

import (
	"path/filepath"
	"testing"
)

func TestFinishMarker_Roundtrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "markers.json")
	m := finishMarker{Phase: "merged", Branch: "terminal/x", TargetBranch: "alpha-main"}
	if err := saveFinishMarker(p, `D:\repos\Foo.mt-worktrees\x`, m); err != nil {
		t.Fatal(err)
	}
	got := loadFinishMarkers(p)
	if got[`D:\repos\Foo.mt-worktrees\x`].Phase != "merged" {
		t.Fatalf("roundtrip failed: %+v", got)
	}
	if err := deleteFinishMarker(p, `D:\repos\Foo.mt-worktrees\x`); err != nil {
		t.Fatal(err)
	}
	if len(loadFinishMarkers(p)) != 0 {
		t.Error("marker not deleted")
	}
}

func TestFinishMarker_MissingFileIsEmpty(t *testing.T) {
	if got := loadFinishMarkers(filepath.Join(t.TempDir(), "nope.json")); len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}
```

- [ ] **Step 2: Testlauf — FAIL** — Run: `go test ./internal/backend/ -run TestFinishMarker -v` → compile error

- [ ] **Step 3: Implementierung**

```go
// internal/backend/app_worktree_marker.go
// Crash-safe finish phase persistence. Keyed by ABSOLUTE worktree path and
// written ONLY by the backend: session ids do not survive restarts and the
// session JSON is rewritten wholesale by the frontend (spec 4.4).
package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type finishMarker struct {
	Phase        string `json:"phase"` // "merged" | "cleanup"
	Branch       string `json:"branch"`
	TargetBranch string `json:"target_branch"`
}

func finishMarkerPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".multiterminal-worktree-finish.json")
}

func loadFinishMarkers(path string) map[string]finishMarker {
	markers := map[string]finishMarker{}
	data, err := os.ReadFile(path)
	if err != nil {
		return markers
	}
	_ = json.Unmarshal(data, &markers)
	return markers
}

func writeFinishMarkers(path string, markers map[string]finishMarker) error {
	if len(markers) == 0 {
		err := os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	data, err := json.MarshalIndent(markers, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func saveFinishMarker(path, wtPath string, m finishMarker) error {
	markers := loadFinishMarkers(path)
	markers[wtPath] = m
	return writeFinishMarkers(path, markers)
}

func deleteFinishMarker(path, wtPath string) error {
	markers := loadFinishMarkers(path)
	delete(markers, wtPath)
	return writeFinishMarkers(path, markers)
}
```

- [ ] **Step 4: Testlauf — PASS** — Run wie Step 2, Expected: PASS (2 Tests)

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_worktree_marker.go internal/backend/app_worktree_marker_test.go
git commit -m "feat(worktree): path-keyed idempotency markers for finish flow"
```

---

### Task 7: finishState-Maschine + Events + `StartWorktreeFinish`/`CancelWorktreeFinish`

**Files:**
- Create: `internal/backend/app_worktree_finish.go`
- Modify: `internal/backend/app_events.go` (Structs anhängen)
- Modify: `internal/backend/app.go` (Map-Init im Konstruktor — die Stelle, an der `queues:` bzw. `a.queues` initialisiert wird; identisches Muster)
- Test: `internal/backend/app_worktree_finish_test.go`

**Interfaces:**
- Consumes: `AddToQueue`/`GetQueue` (Task 8 ergänzt Guards), `GetWorktreeFinishStatus` (Task 5), `a.app.Event.Emit` (nil-guarded wie `emitQueueUpdate`)
- Produces:

```go
// Phasen: "" | "preparing" | "ready" | "blocked" | "merging" | "merged" | "cleanup"
type finishState struct {
    Phase, TargetBranch, WorktreePath, Branch, BlockReason, Mode string
    PrepItemID int
}
func (a *AppService) StartWorktreeFinish(sessionId int, worktreePath, branch, target, mode string)
func (a *AppService) CancelWorktreeFinish(sessionId int)
func (a *AppService) getFinishState(sessionId int) *finishState        // Kopie, für Tests/Scan
func (a *AppService) setFinishBlocked(sessionId int, reason string)    // intern
func (a *AppService) onQueueItemDone(sessionId, itemID int)            // von Task 8 aufgerufen
```

Events (in `app_events.go`, models.ts-Sync nicht nötig — Events sind untypisierte Payloads im Frontend):

```go
// WorktreeFinishReadyEvent: checks green, show confirm overlay.
type WorktreeFinishReadyEvent struct {
	SessionID    int      `json:"sessionId"`
	TargetBranch string   `json:"targetBranch"`
	Commits      []string `json:"commits"`
	Stat         string   `json:"stat"`
	Untracked    []string `json:"untracked"`
	CleanupOnly  bool     `json:"cleanupOnly"`
}

// WorktreeFinishBlockedEvent: a check failed or the flow was reset/informed.
type WorktreeFinishBlockedEvent struct {
	SessionID int    `json:"sessionId"`
	Phase     string `json:"phase"`
	Reason    string `json:"reason"`
}

// WorktreeFinishDoneEvent: merge+cleanup finished; frontend relaunches the pane.
type WorktreeFinishDoneEvent struct {
	SessionID    int    `json:"sessionId"`
	MainRoot     string `json:"mainRoot"`
	TargetBranch string `json:"targetBranch"`
	Mode         string `json:"mode"`
}
```

**Event-Namen:** `worktree:finish-ready`, `worktree:finish-blocked`, `worktree:finish-done`. Emission ist Broadcast an alle Fenster — das Frontend filtert per `sessionId` (Spec §6).

- [ ] **Step 1: Failing Tests schreiben**

```go
// internal/backend/app_worktree_finish_test.go
package backend

import (
	"testing"

	"github.com/patrick-goecommerce/multiterminal/internal/terminal"
)

// newTestApp builds an AppService with initialized maps but no Wails app
// (Event.Emit is nil-guarded everywhere, same as emitQueueUpdate).
func newTestApp() *AppService {
	return &AppService{
		sessions:     map[int]*terminal.Session{},
		queues:       map[int]*sessionQueue{},
		finishStates: map[int]*finishState{},
	}
}

func TestStartFinish_QueueNotEmptyBlocks(t *testing.T) {
	a := newTestApp()
	a.AddToQueue(1, "vorhandener prompt")
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	st := a.getFinishState(1)
	if st == nil || st.Phase != "blocked" {
		t.Fatalf("phase = %+v, want blocked (pending queue)", st)
	}
}

func TestStartFinish_SetsPreparingAndEnqueuesPrep(t *testing.T) {
	a := newTestApp()
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	st := a.getFinishState(1)
	if st == nil || st.Phase != "preparing" || st.PrepItemID == 0 {
		t.Fatalf("state = %+v, want preparing with PrepItemID", st)
	}
	q := a.GetQueue(1)
	if len(q) != 1 || q[0].ID != st.PrepItemID {
		t.Fatalf("prep item not enqueued: %+v", q)
	}
}

func TestStartFinish_DoubleClickIsNoop(t *testing.T) {
	a := newTestApp()
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	first := a.getFinishState(1).PrepItemID
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	if got := a.getFinishState(1).PrepItemID; got != first {
		t.Errorf("second start changed PrepItemID %d → %d", first, got)
	}
	if got := len(a.GetQueue(1)); got != 1 {
		t.Errorf("queue has %d items, want 1", got)
	}
}

func TestCancelFinish_ResetsStateAndRemovesPrepItem(t *testing.T) {
	a := newTestApp()
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	a.CancelWorktreeFinish(1)
	if st := a.getFinishState(1); st != nil {
		t.Errorf("state not cleared: %+v", st)
	}
	if got := len(a.GetQueue(1)); got != 0 {
		t.Errorf("prep item not removed, queue: %d", got)
	}
}

func TestBlockedRetry_StartsNewPrepCycle(t *testing.T) {
	a := newTestApp()
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	a.setFinishBlocked(1, "test reason")
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	if st := a.getFinishState(1); st == nil || st.Phase != "preparing" {
		t.Fatalf("retry from blocked did not re-enter preparing: %+v", st)
	}
}
```

- [ ] **Step 2: Testlauf — FAIL** — Run: `go test ./internal/backend/ -run 'TestStartFinish|TestCancelFinish|TestBlockedRetry' -v` → compile error (`finishStates` undefined etc.). **Hinweis:** Modulpfad im Import an `go.mod` anpassen (`head -1 go.mod`).

- [ ] **Step 3: Implementierung**

3a. In `app.go`: Feld + Init ergänzen (Muster `queues`):

```go
// im AppService-Struct:
	finishStates map[int]*finishState
// im Konstruktor, neben queues-Init:
	finishStates: map[int]*finishState{},
// in CloseSession, neben delete(a.queues, id) (app.go:283):
	delete(a.finishStates, id)
```

3b. Event-Structs (oben) an `app_events.go` anhängen.

3c. `app_worktree_finish.go`:

```go
// internal/backend/app_worktree_finish.go
// Backend finish state machine (spec 4.3). Lives in the backend so it
// survives tab detach, webview reloads and multi-window moves; the frontend
// only renders worktree:finish-* events.
package backend

import (
	"fmt"
	"log"
	"time"
)

const prepPromptTemplate = "Committe alle offenen Änderungen in nachvollziehbaren Commits. " +
	"Committe keine Secrets, .env-Dateien oder Build-Artefakte — ergänze für solche Dateien " +
	".gitignore-Einträge oder lass sie untracked und erwähne sie. " +
	"Rebase dann %s auf den lokalen %s. Bei Rebase-Konflikten: nicht selbst lösen, " +
	"`git rebase --abort` ausführen und die Konfliktdateien nennen. " +
	"Merge nicht selbst, pushe nicht, erstelle keinen PR."

const finishPrepTimeout = 10 * time.Minute

// finishState tracks one session's finish flow. Guarded by a.mu.
type finishState struct {
	Phase        string // "" | "preparing" | "ready" | "blocked" | "merging" | "merged" | "cleanup"
	TargetBranch string
	WorktreePath string
	Branch       string
	BlockReason  string
	Mode         string // "claude" | "shell"
	PrepItemID   int
	startedAt    time.Time
}

func (a *AppService) getFinishState(sessionId int) *finishState {
	a.mu.Lock()
	defer a.mu.Unlock()
	st := a.finishStates[sessionId]
	if st == nil {
		return nil
	}
	cp := *st
	return &cp
}

func (a *AppService) emitFinishBlocked(sessionId int, phase, reason string) {
	if a.app == nil {
		return
	}
	a.app.Event.Emit("worktree:finish-blocked", WorktreeFinishBlockedEvent{
		SessionID: sessionId, Phase: phase, Reason: reason,
	})
}

func (a *AppService) setFinishBlocked(sessionId int, reason string) {
	a.mu.Lock()
	st := a.finishStates[sessionId]
	if st != nil {
		st.Phase = "blocked"
		st.BlockReason = reason
	}
	a.mu.Unlock()
	if st != nil {
		a.emitFinishBlocked(sessionId, "blocked", reason)
	}
}

// StartWorktreeFinish begins (or retries) the finish flow for a session.
// No-op while a phase other than "blocked" is active (double-click guard).
func (a *AppService) StartWorktreeFinish(sessionId int, worktreePath, branch, target, mode string) {
	a.mu.Lock()
	if st := a.finishStates[sessionId]; st != nil && st.Phase != "blocked" {
		a.mu.Unlock()
		return
	}
	q := a.queues[sessionId]
	pending := 0
	if q != nil {
		for _, it := range q.items {
			if it.Status == "pending" || it.Status == "sent" {
				pending++
			}
		}
	}
	if pending > 0 {
		a.finishStates[sessionId] = &finishState{
			Phase: "blocked", TargetBranch: target, WorktreePath: worktreePath,
			Branch: branch, Mode: mode,
			BlockReason: fmt.Sprintf("Queue nicht leer (%d Prompts) — erst abarbeiten oder verwerfen", pending),
		}
		a.mu.Unlock()
		a.emitFinishBlocked(sessionId, "blocked", a.getFinishState(sessionId).BlockReason)
		return
	}
	a.mu.Unlock()

	if mode == "shell" {
		// Shell panes skip the prompt: frontend runs the staging dialog first
		// (task 17), then calls CheckWorktreeFinish directly.
		a.mu.Lock()
		a.finishStates[sessionId] = &finishState{
			Phase: "preparing", TargetBranch: target, WorktreePath: worktreePath,
			Branch: branch, Mode: mode, startedAt: time.Now(),
		}
		a.mu.Unlock()
		return
	}

	prompt := fmt.Sprintf(prepPromptTemplate, branch, target)
	item := a.AddToQueue(sessionId, prompt) // enqueue BEFORE state exists (queue lock, task 8)
	a.mu.Lock()
	a.finishStates[sessionId] = &finishState{
		Phase: "preparing", TargetBranch: target, WorktreePath: worktreePath,
		Branch: branch, Mode: mode, PrepItemID: item.ID, startedAt: time.Now(),
	}
	a.mu.Unlock()
	log.Printf("[finish] session %d: preparing (prep item %d)", sessionId, item.ID)

	time.AfterFunc(finishPrepTimeout, func() {
		if st := a.getFinishState(sessionId); st != nil && st.Phase == "preparing" && st.PrepItemID == item.ID {
			a.emitFinishBlocked(sessionId, "preparing",
				"Vorbereitung läuft seit 10 Minuten — prüfen oder abbrechen")
		}
	})
}

// CancelWorktreeFinish aborts the flow (allowed in preparing/ready/blocked).
func (a *AppService) CancelWorktreeFinish(sessionId int) {
	a.mu.Lock()
	st := a.finishStates[sessionId]
	if st == nil || st.Phase == "merging" || st.Phase == "merged" || st.Phase == "cleanup" {
		a.mu.Unlock()
		return
	}
	prepID := st.PrepItemID
	delete(a.finishStates, sessionId)
	a.mu.Unlock()
	if prepID != 0 {
		a.RemoveFromQueue(sessionId, prepID)
	}
	a.emitFinishBlocked(sessionId, "", "Fertigstellen abgebrochen")
}

// CheckWorktreeFinish runs the verification gate and moves preparing→ready/blocked.
// Called by onQueueItemDone (claude) and by the frontend after the shell
// staging dialog committed+rebased.
func (a *AppService) CheckWorktreeFinish(sessionId int) {
	st := a.getFinishState(sessionId)
	if st == nil {
		return
	}
	status := a.GetWorktreeFinishStatus(st.WorktreePath, st.Branch, st.TargetBranch)
	if status.State == "blocked" {
		a.setFinishBlocked(sessionId, status.Reason)
		return
	}
	a.mu.Lock()
	if cur := a.finishStates[sessionId]; cur != nil {
		cur.Phase = "ready"
		cur.BlockReason = ""
	}
	a.mu.Unlock()
	if a.app != nil {
		a.app.Event.Emit("worktree:finish-ready", WorktreeFinishReadyEvent{
			SessionID: sessionId, TargetBranch: st.TargetBranch,
			Commits: status.Commits, Stat: status.Stat, Untracked: status.Untracked,
			CleanupOnly: status.State == "cleanup_only",
		})
	}
}

// onQueueItemDone is invoked by processQueue whenever an item transitions to
// "done". Only the exact prep item of an active preparing flow triggers the
// check — generic done transitions (earlier items, other turns) do nothing
// (spec 5.1/2, red-team L-K4/U-H1).
func (a *AppService) onQueueItemDone(sessionId, itemID int) {
	st := a.getFinishState(sessionId)
	if st == nil || st.Phase != "preparing" || st.PrepItemID != itemID {
		return
	}
	go a.CheckWorktreeFinish(sessionId)
}
```

**Hinweis für Step 3:** `onQueueItemDone` wird erst in Task 8 aufgerufen; die Tests hier rufen `StartWorktreeFinish`/`CancelWorktreeFinish` direkt. Falls `AddToQueue` in Task 8 schon die Queue-Sperre hat, gilt die Reihenfolge „Item einreihen, DANN State setzen" — genau so implementiert.

- [ ] **Step 4: Testlauf — PASS** — Run wie Step 2, Expected: PASS (5 Tests). Zusätzlich `go build ./...` (app.go-Änderung).

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_worktree_finish.go internal/backend/app_worktree_finish_test.go internal/backend/app_events.go internal/backend/app.go
git commit -m "feat(worktree): backend finish state machine with start/cancel/check"
```

---

### Task 8: Queue-Hook + Guards + Sperre

**Files:**
- Modify: `internal/backend/app_queue.go`
- Test: `internal/backend/app_queue_finish_test.go` (neu)

**Interfaces:**
- Consumes: `finishStates`/`onQueueItemDone` (Task 7)
- Produces: `processQueue` meldet jede Item-done-Transition an `onQueueItemDone`; `RemoveFromQueue`/`ClearQueue` setzen einen aktiven Finish zurück; `AddToQueue` lehnt neue Items während aktiver Finish-Phase ab (Rückgabe `QueueItem{}` mit ID 0).

- [ ] **Step 1: Failing Tests schreiben**

```go
// internal/backend/app_queue_finish_test.go
package backend

import "testing"

func TestProcessQueue_ReportsItemDone(t *testing.T) {
	a := newTestApp()
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	prepID := a.getFinishState(1).PrepItemID
	// Simulate the scan loop: first done sends the item, second done completes it.
	a.processQueue(1) // pending → sent (no session ⇒ write skipped, status still advances)
	a.processQueue(1) // sent → done ⇒ onQueueItemDone(1, prepID) fires
	q := a.GetQueue(1)
	if len(q) != 1 || q[0].ID != prepID || q[0].Status != "done" {
		t.Fatalf("prep item not completed: %+v", q)
	}
	// CheckWorktreeFinish runs async against a nonexistent path ⇒ blocked.
	// Wait briefly for the goroutine, then assert the transition happened.
	deadline := timeoutAfter(t, 2)
	for {
		st := a.getFinishState(1)
		if st != nil && st.Phase == "blocked" {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("onQueueItemDone never advanced state: %+v", a.getFinishState(1))
		default:
		}
	}
}

func TestRemovePrepItem_ResetsFinish(t *testing.T) {
	a := newTestApp()
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	prepID := a.getFinishState(1).PrepItemID
	a.RemoveFromQueue(1, prepID)
	if st := a.getFinishState(1); st != nil {
		t.Errorf("finish state survived prep item removal: %+v", st)
	}
}

func TestClearQueue_ResetsFinish(t *testing.T) {
	a := newTestApp()
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	a.ClearQueue(1)
	if st := a.getFinishState(1); st != nil {
		t.Errorf("finish state survived ClearQueue: %+v", st)
	}
}

func TestAddToQueue_LockedDuringFinish(t *testing.T) {
	a := newTestApp()
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	item := a.AddToQueue(1, "sollte abgelehnt werden")
	if item.ID != 0 {
		t.Errorf("queue accepted item during active finish: %+v", item)
	}
	if got := len(a.GetQueue(1)); got != 1 {
		t.Errorf("queue length %d, want 1 (only prep item)", got)
	}
}
```

Helfer in derselben Datei:

```go
func timeoutAfter(t *testing.T, seconds int) <-chan struct{} {
	t.Helper()
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		<-timeAfter(seconds)
	}()
	return ch
}
```

**Vereinfachung erlaubt:** statt `timeAfter` direkt `time.After(2 * time.Second)` inline im Test verwenden und den Helfer weglassen — Hauptsache, der Test wartet begrenzt auf die Goroutine.

- [ ] **Step 2: Testlauf — FAIL** — Run: `go test ./internal/backend/ -run 'TestProcessQueue_Reports|TestRemovePrepItem|TestClearQueue_Resets|TestAddToQueue_Locked' -v` → Failures (kein Hook, keine Guards, keine Sperre)

- [ ] **Step 3: Implementierung** (Änderungen an `app_queue.go`)

3a. `AddToQueue` — Sperre am Anfang (nach `a.mu.Lock()`):

```go
	// Queue is locked for new items while a finish flow is active. The prep
	// item itself is enqueued BEFORE the state is created (task 7 ordering).
	if st := a.finishStates[sessionId]; st != nil {
		a.mu.Unlock()
		log.Printf("[queue] session %d: rejected item during finish phase %q", sessionId, st.Phase)
		return QueueItem{}
	}
```

3b. `processQueue` — done-Transition melden. Im Block „Mark current sent item as done" die ID merken und nach `a.mu.Unlock()` melden:

```go
	// Mark current "sent" item as "done"
	doneItemID := 0
	for i := range q.items {
		if q.items[i].Status == "sent" {
			q.items[i].Status = "done"
			doneItemID = q.items[i].ID
			break
		}
	}
```

und direkt nach `a.mu.Unlock()` (vor dem `if hasNext …`-Block):

```go
	if doneItemID != 0 {
		a.onQueueItemDone(sessionId, doneItemID)
	}
```

3c. `RemoveFromQueue` — Guard nach erfolgreichem Entfernen (die Schleife setzt ein Flag `removed := item.ID == itemId`); nach `a.mu.Unlock()`:

```go
	if removed {
		if st := a.getFinishState(sessionId); st != nil && st.PrepItemID == itemId {
			a.mu.Lock()
			delete(a.finishStates, sessionId)
			a.mu.Unlock()
			a.emitFinishBlocked(sessionId, "", "Fertigstellen abgebrochen (Prep-Prompt entfernt)")
		}
	}
```

3d. `ClearQueue` — Guard nach `a.mu.Unlock()`:

```go
	if st := a.getFinishState(sessionId); st != nil && st.Phase == "preparing" {
		a.mu.Lock()
		delete(a.finishStates, sessionId)
		a.mu.Unlock()
		a.emitFinishBlocked(sessionId, "", "Fertigstellen abgebrochen (Queue geleert)")
	}
```

**Achtung Lock-Disziplin:** `RemoveFromQueue` wird auch von `CancelWorktreeFinish` (Task 7) aufgerufen, nachdem der State gelöscht ist — der Guard greift dann nicht (kein State mehr), keine Doppel-Events.

- [ ] **Step 4: Testlauf — PASS** — Run wie Step 2 + `go test ./internal/backend/ -v` (alle bisherigen Tests, keine Regression in Queue-Tests)

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_queue.go internal/backend/app_queue_finish_test.go
git commit -m "feat(queue): item-done hook, prep-item guards, finish lock"
```

---

### Task 9: Scan-Verdrahtung (Rückfragen während preparing)

**Files:**
- Modify: `internal/backend/app_scan.go` (im Block `if activityChanged …`, um Zeile 163)
- Test: `internal/backend/app_worktree_finish_test.go` (anhängen)

**Interfaces:**
- Consumes: `getFinishState` (Task 7), bestehende Zustands-Strings `waitingPermission`/`waitingAnswer` (NICHT „input" — existiert nicht, Red-Team L2-H2)
- Produces: `notifyFinishOnActivity(sessionId int, actStr string)` — informatives blocked-Event, Phase bleibt `preparing` (Korrelation läuft weiter, Übergangstabelle Spec 4.3).

- [ ] **Step 1: Failing Test schreiben** (anhängen an `app_worktree_finish_test.go`)

```go
func TestNotifyFinishOnActivity_WaitingKeepsPreparing(t *testing.T) {
	a := newTestApp()
	a.StartWorktreeFinish(1, `C:\wt`, "terminal/x", "alpha-main", "claude")
	a.notifyFinishOnActivity(1, "waitingAnswer")
	if st := a.getFinishState(1); st == nil || st.Phase != "preparing" {
		t.Fatalf("waitingAnswer must NOT change phase: %+v", st)
	}
	// Non-finish sessions and other states are ignored:
	a.notifyFinishOnActivity(2, "waitingAnswer")
	a.notifyFinishOnActivity(1, "active")
}
```

- [ ] **Step 2: Testlauf — FAIL** — Run: `go test ./internal/backend/ -run TestNotifyFinishOnActivity -v` → compile error

- [ ] **Step 3: Implementierung**

In `app_worktree_finish.go` anhängen:

```go
// notifyFinishOnActivity surfaces "Claude has a question" while preparing.
// The phase stays preparing — the prep item correlation keeps running.
func (a *AppService) notifyFinishOnActivity(sessionId int, actStr string) {
	if actStr != "waitingPermission" && actStr != "waitingAnswer" {
		return
	}
	if st := a.getFinishState(sessionId); st != nil && st.Phase == "preparing" {
		a.emitFinishBlocked(sessionId, "preparing", "Claude hat eine Rückfrage — bitte im Pane antworten")
	}
}
```

In `app_scan.go` im bestehenden Block ergänzen (nach `a.processQueue(id)`-Trigger, gleiche Bedingungsebene):

```go
		// Surface waiting states to an active finish flow (spec 5.1/2)
		if activityChanged && a.app != nil {
			a.notifyFinishOnActivity(id, actStr)
		}
```

- [ ] **Step 4: Testlauf — PASS** — Run wie Step 2 + `go build ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_worktree_finish.go internal/backend/app_scan.go internal/backend/app_worktree_finish_test.go
git commit -m "feat(worktree): surface claude questions during finish preparing"
```

---

### Task 10: `Session.Pid()` + `killProcessTree`

**Files:**
- Modify: `internal/terminal/session.go` (Getter anhängen)
- Create: `internal/backend/kill_windows.go`, `internal/backend/kill_other.go`
- Test: `internal/terminal/session_pid_test.go` (neu), `internal/backend/kill_windows_test.go` (neu)

**Interfaces:**
- Consumes: `hideConsole` (`hide_windows.go`; auf Unix existiert das no-op Pendant `hide_other.go` — prüfen mit `ls internal/backend/hide_*`, sonst Aufruf nur in der Windows-Datei)
- Produces: `(*terminal.Session).Pid() int` (0 wenn nicht gestartet) und `killProcessTree(pid int)` (toleriert tote Prozesse) — von Task 11 benutzt. **Reihenfolge-Kontrakt (Spec 5.2):** `killProcessTree` MUSS vor `Session.Close()` laufen, solange der Wrapper lebt — sonst sind die Enkel verwaist und `taskkill /T` findet sie nicht.

- [ ] **Step 1: Failing Tests schreiben**

```go
// internal/terminal/session_pid_test.go
package terminal

import "testing"

func TestPid_ZeroBeforeStart(t *testing.T) {
	s := NewSession(1, 24, 80)
	if got := s.Pid(); got != 0 {
		t.Errorf("Pid before Start = %d, want 0", got)
	}
}
```

```go
// internal/backend/kill_windows_test.go
//go:build windows

package backend

import (
	"os/exec"
	"testing"
	"time"
)

func TestKillProcessTree_KillsChildAndToleratesDead(t *testing.T) {
	cmd := exec.Command("cmd.exe", "/c", "ping -n 30 127.0.0.1 >NUL")
	hideConsole(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	killProcessTree(pid)
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case <-done: // killed → Wait returns promptly
	case <-time.After(5 * time.Second):
		t.Fatal("process tree not killed within 5s")
	}
	killProcessTree(pid) // second call on dead tree must not panic/hang
}
```

- [ ] **Step 2: Testlauf — FAIL** — Run: `go test ./internal/terminal/ -run TestPid -v; go test ./internal/backend/ -run TestKillProcessTree -v` → compile errors

- [ ] **Step 3: Implementierung**

An `internal/terminal/session.go` anhängen:

```go
// Pid returns the wrapper process id (cmd.exe on Windows), or 0 before Start.
// The finish flow needs it to kill the whole process tree BEFORE Close():
// after Process.Kill() the grandchildren are orphaned and taskkill /T cannot
// find them anymore.
func (s *Session) Pid() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Pid
	}
	return 0
}
```

```go
// internal/backend/kill_windows.go
//go:build windows

package backend

import (
	"log"
	"os/exec"
	"strconv"
)

// killProcessTree force-kills pid and all descendants. Session.Close() only
// kills the cmd.exe wrapper — node/MCP/watcher grandchildren survive and hold
// handles inside the worktree, which makes `git worktree remove` fail on
// Windows (spec 5.2). Non-zero exit (tree already gone, taskkill exit 128)
// is tolerated.
func killProcessTree(pid int) {
	if pid <= 0 {
		return
	}
	cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
	hideConsole(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[killProcessTree] pid %d: %v – %s (tolerated)", pid, err, out)
	}
}
```

```go
// internal/backend/kill_other.go
//go:build !windows

package backend

import "syscall"

// killProcessTree best-effort kill on Unix. ConPTY-style orphan handles are a
// Windows-only failure mode; a plain SIGKILL suffices here.
func killProcessTree(pid int) {
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}
```

- [ ] **Step 4: Testlauf — PASS** — Run wie Step 2, Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/terminal/session.go internal/terminal/session_pid_test.go internal/backend/kill_windows.go internal/backend/kill_other.go internal/backend/kill_windows_test.go
git commit -m "feat(session): Pid getter + cross-platform killProcessTree"
```

---

### Task 11: `FinishWorktree` (Merge + Cleanup) + Startup-Reconcile

**Files:**
- Modify: `internal/backend/app_worktree_finish.go` (Orchestrierung, ~60 Zeilen)
- Create: `internal/backend/app_worktree_cleanup.go` (git-Primitive + Reconcile — hält beide Dateien < 300 Zeilen)
- Test: `internal/backend/app_worktree_cleanup_test.go`

**Interfaces:**
- Consumes: Tasks 1, 5, 6, 7, 10; `a.sessions[sessionId]` (für Pid/Close)
- Produces:

```go
func (a *AppService) FinishWorktree(sessionId int)                       // Binding; validiert ready, Phase→merging, Goroutine
func mergeWorktreeBranch(mainRoot, branch, target string) error         // ff-Recheck + merge --ff-only IM Haupt-Worktree
func cleanupWorktree(mainRoot, wtPath, branch string) error             // remove (Retry/Backoff, dann --force) + prune + branch -d
func (a *AppService) ReconcileFinishMarkers(dir string)                 // Binding; beim App-Start vom Frontend aufgerufen
```

- **Finish-Mutex:** neues Feld `finishMu sync.Mutex` am `AppService` (app.go) — serialisiert Merge+Cleanup global (index.lock, TOCTOU).

- [ ] **Step 1: Failing Tests schreiben** (git-Primitive; die Goroutine-Orchestrierung wird über die Primitive + bestehende State-Tests abgedeckt)

```go
// internal/backend/app_worktree_cleanup_test.go
package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMergeWorktreeBranch_FFOnly(t *testing.T) {
	repo, _ := finishFixture(t) // branch terminal/feat mit 1 Commit, rebased
	if err := mergeWorktreeBranch(repo, "terminal/feat", "alpha-main"); err != nil {
		t.Fatal(err)
	}
	// Ziel-Branch muss den Commit jetzt enthalten:
	if out := gitRun(t, repo, "log", "--oneline", "-1"); !contains(out, "feat: work") {
		t.Errorf("target branch head = %q, want feat commit", out)
	}
	// Arbeitskopie im Haupt-Worktree wurde mitbewegt (ff):
	if _, err := os.Stat(filepath.Join(repo, "work.txt")); err != nil {
		t.Error("ff merge did not update main working tree")
	}
}

func TestMergeWorktreeBranch_RefusesNonFF(t *testing.T) {
	repo, _ := finishFixture(t)
	// Ziel bewegt sich → nicht mehr ff:
	if err := os.WriteFile(filepath.Join(repo, "clash.txt"), []byte("z\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", "clash.txt")
	gitRun(t, repo, "commit", "-m", "target moves")
	if err := mergeWorktreeBranch(repo, "terminal/feat", "alpha-main"); err == nil {
		t.Fatal("expected non-ff merge to be refused")
	}
}

func TestCleanupWorktree_RemovesWorktreeAndBranch(t *testing.T) {
	repo, wt := finishFixture(t)
	if err := mergeWorktreeBranch(repo, "terminal/feat", "alpha-main"); err != nil {
		t.Fatal(err)
	}
	if err := cleanupWorktree(repo, wt, "terminal/feat"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Error("worktree dir still exists")
	}
	if branchExists(repo, "terminal/feat") {
		t.Error("branch still exists")
	}
}

func TestCleanupWorktree_UnmergedBranchSurvives(t *testing.T) {
	repo, wt := finishFixture(t)
	// KEIN Merge — branch -d muss verweigern, kein -D-Fallback (Spec 5.4/5):
	err := cleanupWorktree(repo, wt, "terminal/feat")
	if err == nil {
		t.Fatal("expected error: unmerged branch must not be deleted")
	}
	if !branchExists(repo, "terminal/feat") {
		t.Fatal("DATA LOSS: unmerged branch was deleted")
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && strings.Contains(s, sub) }
```

(`strings` importieren; alternativ `strings.Contains` direkt verwenden und den Helfer weglassen.)

- [ ] **Step 2: Testlauf — FAIL** — Run: `go test ./internal/backend/ -run 'TestMergeWorktreeBranch|TestCleanupWorktree' -v` → compile error

- [ ] **Step 3: Implementierung**

3a. `app.go`: Feld `finishMu sync.Mutex` am `AppService` ergänzen (`sync` ist dort bereits importiert).

3b. `internal/backend/app_worktree_cleanup.go`:

```go
// internal/backend/app_worktree_cleanup.go
// Merge + cleanup primitives. The merge MUST run in the MAIN worktree: git
// merge only ever moves the HEAD of the worktree it runs in — anywhere else
// would leave the target branch untouched and then delete the work (spec 5.4).
package backend

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// mergeWorktreeBranch re-verifies ff and merges branch into target inside the
// main worktree. Never uses --force anything; a non-ff state aborts.
func mergeWorktreeBranch(mainRoot, branch, target string) error {
	if got := checkedOutBranch(mainRoot); got != target {
		return fmt.Errorf("Haupt-Worktree steht auf %q, nicht auf %q", got, target)
	}
	if !isAncestor(mainRoot, target, branch) {
		return fmt.Errorf("Ziel-Branch hat sich bewegt — erneut vorbereiten (kein ff-Merge möglich)")
	}
	out, err := gitCmd(mainRoot, "merge", "--ff-only", branch).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ff-merge fehlgeschlagen: %s – %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// cleanupWorktree removes the worktree dir (retry with backoff — Windows
// releases handles lazily; --force only as last resort AFTER the merge is
// verified through) and deletes the branch with -d (never -D: -d is the last
// safety net against data loss).
func cleanupWorktree(mainRoot, wtPath, branch string) error {
	var lastErr error
	delays := []time.Duration{0, 200 * time.Millisecond, 500 * time.Millisecond, time.Second, 2 * time.Second}
	for i, d := range delays {
		time.Sleep(d)
		args := []string{"worktree", "remove", wtPath}
		if i == len(delays)-1 {
			args = []string{"worktree", "remove", "--force", wtPath}
		}
		out, err := gitCmd(mainRoot, args...).CombinedOutput()
		if err == nil {
			lastErr = nil
			break
		}
		lastErr = fmt.Errorf("worktree remove: %s – %w", strings.TrimSpace(string(out)), err)
	}
	if lastErr != nil {
		return lastErr
	}
	_ = gitCmd(mainRoot, "worktree", "prune").Run()
	if out, err := gitCmd(mainRoot, "branch", "-d", branch).CombinedOutput(); err != nil {
		// Deliberately NO -D fallback (spec 5.4/5): report and keep the branch.
		return fmt.Errorf("branch -d verweigert (Branch bleibt stehen, manuell prüfen): %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ReconcileFinishMarkers resumes interrupted cleanups after a restart:
// for every marker whose worktree still exists, cleanup is re-run — a merge
// is NEVER repeated (spec 4.4). Called once from the frontend on startup.
func (a *AppService) ReconcileFinishMarkers(dir string) {
	root, err := mainRepoRoot(dir)
	if err != nil {
		return
	}
	markers := loadFinishMarkers(finishMarkerPath())
	for wtPath, m := range markers {
		sub, err := mainRepoRoot(wtPath)
		if err != nil || !strings.EqualFold(sub, root) {
			continue // marker belongs to another repo or the worktree is gone
		}
		log.Printf("[finish] reconcile: resuming cleanup for %s (phase %s)", wtPath, m.Phase)
		a.finishMu.Lock()
		err = cleanupWorktree(root, wtPath, m.Branch)
		a.finishMu.Unlock()
		if err == nil {
			_ = deleteFinishMarker(finishMarkerPath(), wtPath)
		} else {
			log.Printf("[finish] reconcile failed for %s: %v", wtPath, err)
		}
	}
	// Markers whose worktree dir vanished entirely: prune + drop marker.
	for wtPath := range markers {
		if _, err := mainRepoRoot(wtPath); err != nil {
			_ = gitCmd(root, "worktree", "prune").Run()
			_ = deleteFinishMarker(finishMarkerPath(), wtPath)
		}
	}
}
```

3c. `FinishWorktree`-Orchestrierung an `app_worktree_finish.go` anhängen:

```go
// FinishWorktree executes merge + cleanup after the user confirmed the
// overlay. Runs in a goroutine (the remove retry may take seconds — a Wails
// binding must not block, spec 5.4) serialized by finishMu.
func (a *AppService) FinishWorktree(sessionId int) {
	a.mu.Lock()
	st := a.finishStates[sessionId]
	if st == nil || st.Phase != "ready" {
		a.mu.Unlock()
		return
	}
	st.Phase = "merging"
	cp := *st
	sess := a.sessions[sessionId]
	a.mu.Unlock()

	go func() {
		a.finishMu.Lock()
		defer a.finishMu.Unlock()

		// Re-check whether the branch is already contained (cleanup_only path
		// or crash recovery); otherwise merge.
		count, err := revCount_must(cp, a)
		if err != nil {
			a.setFinishBlocked(sessionId, err.Error())
			return
		}
		root, _ := mainRepoRoot(cp.WorktreePath)
		if count > 0 {
			if err := mergeWorktreeBranch(root, cp.Branch, cp.TargetBranch); err != nil {
				a.setFinishBlocked(sessionId, err.Error())
				return
			}
		}
		_ = saveFinishMarker(finishMarkerPath(), cp.WorktreePath, finishMarker{
			Phase: "merged", Branch: cp.Branch, TargetBranch: cp.TargetBranch,
		})
		a.mu.Lock()
		if cur := a.finishStates[sessionId]; cur != nil {
			cur.Phase = "cleanup"
		}
		a.mu.Unlock()

		// Kill the whole tree BEFORE Close (spec 5.2), then close synchronously.
		if sess != nil {
			killProcessTree(sess.Pid())
			sess.Close()
		}
		if err := cleanupWorktree(root, cp.WorktreePath, cp.Branch); err != nil {
			a.setFinishBlocked(sessionId, "Merge ist durch, Cleanup fehlgeschlagen: "+err.Error()+" — erneut versuchen")
			return
		}
		_ = deleteFinishMarker(finishMarkerPath(), cp.WorktreePath)
		a.mu.Lock()
		delete(a.finishStates, sessionId)
		delete(a.sessions, sessionId)
		delete(a.queues, sessionId)
		a.mu.Unlock()
		if a.app != nil {
			a.app.Event.Emit("worktree:finish-done", WorktreeFinishDoneEvent{
				SessionID: sessionId, MainRoot: root,
				TargetBranch: cp.TargetBranch, Mode: cp.Mode,
			})
		}
		log.Printf("[finish] session %d: merged %s into %s and cleaned up", sessionId, cp.Branch, cp.TargetBranch)
	}()
}

// revCount_must wraps revCount with root resolution for the goroutine above.
func revCount_must(cp finishState, a *AppService) (int, error) {
	root, err := mainRepoRoot(cp.WorktreePath)
	if err != nil {
		return 0, err
	}
	return revCount(root, cp.TargetBranch, cp.Branch)
}
```

**Hinweis:** Bei Cleanup-Retry über das Overlay ruft das Frontend erneut `FinishWorktree` — dafür muss `Phase == "blocked"` mit vorhandenem Marker ebenfalls akzeptiert werden. Ergänze in der Eingangsvalidierung: `if st == nil || (st.Phase != "ready" && !(st.Phase == "blocked" && hasFinishMarker(cp(st))))`. Einfachste Umsetzung: Marker-Lookup `loadFinishMarkers(finishMarkerPath())[st.WorktreePath]` — existiert er, direkt in den Cleanup-Teil springen (count==0-Pfad greift ohnehin, da der Merge durch ist).

- [ ] **Step 4: Testlauf — PASS** — Run: `go test ./internal/backend/ -v` (alle), `go vet ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_worktree_cleanup.go internal/backend/app_worktree_cleanup_test.go internal/backend/app_worktree_finish.go internal/backend/app.go
git commit -m "feat(worktree): ff-only merge in main worktree + idempotent cleanup"
```

---

### Task 12: `SavedPane`-Persistenzfelder

**Files:**
- Modify: `internal/config/session.go` (SavedPane, nach `IssueBranch`)
- Test: `internal/config/session_worktree_test.go` (neu)

**Interfaces:**
- Produces: `SavedPane.WorktreePath/WorktreeBranch/TargetBranch` (json: `worktree_path`, `worktree_branch`, `target_branch`) — von Task 13/14 benutzt.

- [ ] **Step 1: Failing Test schreiben**

```go
// internal/config/session_worktree_test.go
package config

import (
	"encoding/json"
	"testing"
)

func TestSavedPane_WorktreeFieldsRoundtrip(t *testing.T) {
	p := SavedPane{Name: "x", WorktreePath: `D:\repos\Foo.mt-worktrees\a`, WorktreeBranch: "terminal/a", TargetBranch: "alpha-main"}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var got SavedPane
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.WorktreePath != p.WorktreePath || got.WorktreeBranch != p.WorktreeBranch || got.TargetBranch != p.TargetBranch {
		t.Errorf("roundtrip lost fields: %+v", got)
	}
}
```

- [ ] **Step 2: Testlauf — FAIL** — Run: `go test ./internal/config/ -run TestSavedPane_Worktree -v` → compile error

- [ ] **Step 3: Implementierung** — in `SavedPane` nach `IssueBranch` einfügen:

```go
	WorktreePath   string `json:"worktree_path,omitempty"`   // pane worktree CWD (restore MUST use this as session dir)
	WorktreeBranch string `json:"worktree_branch,omitempty"` // terminal/<name>
	TargetBranch   string `json:"target_branch,omitempty"`   // merge-back target
```

- [ ] **Step 4: Testlauf — PASS** — Run wie Step 2

- [ ] **Step 5: Commit**

```bash
git add internal/config/session.go internal/config/session_worktree_test.go
git commit -m "feat(config): persist pane worktree path/branch/target"
```

---

### Task 13: `models.ts`-Sync (Pflicht, sonst Silent-Strip)

**Files:**
- Modify: `frontend/wailsjs/go/models.ts`

**Interfaces:**
- Consumes: Structs aus Tasks 4, 5, 12
- Produces: TS-Klassen `PaneWorktreeInfo`, `PaneWorktreeDefaults`, `WorktreeFinishStatus`; `SavedPane`-Felder. **Muster exakt wie bestehende Klassen** (z. B. `WorktreeInfo`): Klasse mit Feldern + `constructor(source)` mit `source["feld"]`-Zuweisungen.

- [ ] **Step 1: Klassen anlegen** (im namespace/Modul neben `WorktreeInfo`)

```typescript
export class PaneWorktreeInfo {
    path: string;
    branch: string;
    target_branch: string;
    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.path = source["path"];
        this.branch = source["branch"];
        this.target_branch = source["target_branch"];
    }
}

export class PaneWorktreeDefaults {
    name: string;
    target_branch: string;
    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.name = source["name"];
        this.target_branch = source["target_branch"];
    }
}

export class WorktreeFinishStatus {
    state: string;
    reason: string;
    commits: string[];
    stat: string;
    untracked: string[];
    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.state = source["state"];
        this.reason = source["reason"];
        this.commits = source["commits"];
        this.stat = source["stat"];
        this.untracked = source["untracked"];
    }
}
```

- [ ] **Step 2: `SavedPane`-Klasse ergänzen** — Felddeklarationen `worktree_path?: string; worktree_branch?: string; target_branch?: string;` UND im Konstruktor `this.worktree_path = source["worktree_path"];` (analog die anderen beiden).

- [ ] **Step 3: Bindings-Wrapper prüfen** — `frontend/wailsjs/go/backend/App.*` (bzw. `AppService`): neue Methoden `CreatePaneWorktree`, `GetPaneWorktreeDefaults`, `GetWorktreeFinishStatus`, `StartWorktreeFinish`, `CancelWorktreeFinish`, `FinishWorktree`, `CheckWorktreeFinish`, `ReconcileFinishMarkers` nach dem Muster der bestehenden Einträge (z. B. `CreateNamedWorktree`) ergänzen — Wails v3 generiert hier nicht nach.

- [ ] **Step 4: Typcheck** — Run: `cd frontend && npx svelte-check --threshold error` (bzw. `npm run check`, falls Script existiert; sonst `npm run build`) → keine neuen Fehler.

- [ ] **Step 5: Commit**

```bash
git add frontend/wailsjs/go/models.ts frontend/wailsjs/go/backend/
git commit -m "feat(bindings): sync worktree finish types + methods into models.ts"
```

---

### Task 14: `tabs.ts` + `session.ts` (targetBranch, Restore-CWD)

**Files:**
- Modify: `frontend/src/stores/tabs.ts` (Pane-Interface, `addPane`, neue Methode `setWorktree`)
- Modify: `frontend/src/lib/session.ts` (save + restore)
- Test: `frontend/src/lib/session.test.ts` (neu, vitest — Muster wie `dashboard.test.ts`)

**Interfaces:**
- Consumes: `SavedPane`-Felder (Task 13), `tabStore.addPane(...)` (Signatur siehe unten)
- Produces: `Pane.targetBranch: string`; `addPane` erhält Parameter `targetBranch` (nach `branch`, vor `background` — **alle bestehenden Aufrufer anpassen**: `App.svelte` Zeilen ~397/465/500/617/648, `session.ts`); Helfer `paneToSaved(pane)` (exportiert, pur, testbar).

- [ ] **Step 1: Failing Test schreiben**

```typescript
// frontend/src/lib/session.test.ts
import { describe, it, expect } from 'vitest';
import { paneToSaved } from './session';

describe('paneToSaved', () => {
  it('serialisiert Worktree-Felder', () => {
    const saved = paneToSaved({
      name: 'x', mode: 'claude', model: '', issueNumber: null, issueBranch: '',
      zoomDelta: 0, display: 'terminal', conversationId: '', claudeSessionId: '',
      userRenamed: false,
      worktreePath: 'D:/repos/Foo.mt-worktrees/a', branch: 'terminal/a', targetBranch: 'alpha-main',
    } as any);
    expect(saved.worktree_path).toBe('D:/repos/Foo.mt-worktrees/a');
    expect(saved.worktree_branch).toBe('terminal/a');
    expect(saved.target_branch).toBe('alpha-main');
  });
});
```

- [ ] **Step 2: Testlauf — FAIL** — Run: `cd frontend && npx vitest run src/lib/session.test.ts` → `paneToSaved` not exported

- [ ] **Step 3: Implementierung**

3a. `tabs.ts` — im `Pane`-Interface nach `branch: string;` ergänzen: `targetBranch: string;`. In `addPane` Signatur nach `branch?: string` den Parameter `targetBranch?: string` einfügen und im Push-Objekt `targetBranch: targetBranch ?? '',` (nach `branch:`-Zeile). **Alle Aufrufer** um das zusätzliche Argument erweitern (`''` wo kein Worktree; TypeScript-Compiler findet die Stellen).

3b. `session.ts` — Mapping-Funktion extrahieren und in `saveSession()` verwenden:

```typescript
/** Pure mapping Pane → SavedPane shape (testbar, eine Quelle der Wahrheit). */
export function paneToSaved(pane: any) {
  return {
    name: pane.name,
    mode: MODE_TO_INDEX[pane.mode] ?? 0,
    model: pane.model || '',
    issue_number: pane.issueNumber || 0,
    issue_branch: pane.issueBranch || '',
    zoom_delta: pane.zoomDelta || 0,
    display: pane.display || 'terminal',
    conversation_id: pane.conversationId || '',
    claude_session_id: pane.claudeSessionId || '',
    user_renamed: pane.userRenamed || false,
    worktree_path: pane.worktreePath || '',
    worktree_branch: pane.branch || '',
    target_branch: pane.targetBranch || '',
  };
}
// in saveSession(): panes: tab.panes.map(paneToSaved),
```

3c. `session.ts` `restoreSession()` — **CWD-Fix (Spec 4.2, kritisch)**. Vor dem `App.CreateSession`-Aufruf:

```typescript
        // Worktree panes MUST restore into their worktree, not the tab dir
        // (spec 4.2 — otherwise badge/finish point at the worktree while the
        // session runs in the main repo).
        let sessionDir = savedTab.dir || '';
        const wtPath = (savedPane as any).worktree_path || '';
        let wtBranch = (savedPane as any).worktree_branch || '';
        let wtTarget = (savedPane as any).target_branch || '';
        if (wtPath) {
          const exists = await App.WorktreeDirExists(wtPath).catch(() => false);
          if (exists) {
            sessionDir = wtPath;
          } else {
            console.warn('[restoreSession] worktree missing, falling back to main repo:', wtPath);
            wtBranch = ''; wtTarget = '';
          }
        }
        const sessionId = await App.CreateSession(argv, sessionDir, 24, 80, mode);
```

und im `addPane`-Aufruf `wtPath && sessionDir === wtPath ? wtPath : ''`, `wtBranch`, `wtTarget` an den Positionen worktreePath/branch/targetBranch übergeben.

3d. Backend-Mini-Binding für 3c (in `app_worktree_pane.go` anhängen + Bindings-Wrapper wie Task 13):

```go
// WorktreeDirExists reports whether a saved pane worktree still exists on
// disk (restore fallback, spec 4.2). Prunes stale git metadata when gone.
func (a *AppService) WorktreeDirExists(path string) bool {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return true
	}
	if root, err := mainRepoRoot(filepath.Dir(path)); err == nil {
		_ = gitCmd(root, "worktree", "prune").Run()
	}
	return false
}
```

- [ ] **Step 4: Tests + Typcheck — PASS** — Run: `cd frontend && npx vitest run src/lib/session.test.ts && npm run build`; Backend: `go build ./...`

- [ ] **Step 5: Commit**

```bash
git add frontend/src/stores/tabs.ts frontend/src/lib/session.ts frontend/src/lib/session.test.ts frontend/src/App.svelte internal/backend/app_worktree_pane.go frontend/wailsjs/go/
git commit -m "feat(frontend): persist+restore pane worktrees with correct CWD"
```

---

### Task 15: LaunchDialog — Opt-in-Checkbox + Felder

**Files:**
- Modify: `frontend/src/components/LaunchDialog.svelte`
- Modify: `frontend/src/stores/config.ts` (Merk-Flag)

**Interfaces:**
- Consumes: `App.GetPaneWorktreeDefaults(dir, base)` (Task 4/13), `selectedDisplay` (existiert, Zeile 16), `dispatch('launch', {...})` (Zeile 62)
- Produces: `launch`-Event-Detail um `worktree: { name: string; targetBranch: string } | null` erweitert — von Task 16 konsumiert. Persistenter Checkbox-Zustand `worktreeLaunchDefault` (localStorage über den bestehenden Config-Store-Mechanismus; falls der Store nur Backend-YAML spiegelt, stattdessen direkt `localStorage.getItem/setItem('mtui.worktreeLaunchDefault')` — kein neues YAML-Feld in v1, Spec §7).

- [ ] **Step 1: Props/State ergänzen** (Script-Block)

```typescript
  export let dir: string = ''; // Projektverzeichnis des aktiven Tabs (von App.svelte durchgereicht)

  let useWorktree = localStorage.getItem('mtui.worktreeLaunchDefault') === '1';
  let wtName = '';
  let wtTarget = '';
  let wtDefaultsLoaded = false;

  async function loadWorktreeDefaults() {
    if (wtDefaultsLoaded || !dir) return;
    wtDefaultsLoaded = true;
    try {
      const d = await App.GetPaneWorktreeDefaults(dir, 'pane');
      wtName = d.name || 'pane';
      wtTarget = d.target_branch || '';
    } catch { /* kein git-Repo: Felder bleiben leer, Launch ohne Worktree */ }
  }

  function toggleWorktree() {
    useWorktree = !useWorktree;
    localStorage.setItem('mtui.worktreeLaunchDefault', useWorktree ? '1' : '0');
    if (useWorktree) loadWorktreeDefaults();
  }

  // Gemerkt-aktiv ⇒ Felder beim Öffnen direkt laden (präventiv, Spec §2).
  // KEINE Zuweisungen im $:-Block (Recurring Bug) — nur Funktionsaufruf:
  $: if (visible && useWorktree) loadWorktreeDefaults();
```

In `launch(type)` (Zeile 58) das Detail erweitern — Worktree nur für Terminal-Display (Chat hat keinen Finish-Flow, Spec §2):

```typescript
    const worktree = useWorktree && display !== 'chat' && wtTarget
      ? { name: wtName, targetBranch: wtTarget } : null;
    dispatch('launch', { type, model: selectedModel, issue: issueContext, display, permissionMode, worktree });
```

- [ ] **Step 2: Markup ergänzen** — direkt vor dem `display-picker`-Block (Zeile ~197); bei `selectedDisplay === 'chat'` ausblenden:

```svelte
      {#if selectedDisplay !== 'chat'}
        <div class="worktree-opt">
          <label class="wt-check">
            <input type="checkbox" checked={useWorktree} on:change={toggleWorktree} />
            <span>⎇ Isolierter Worktree</span>
          </label>
          {#if useWorktree}
            <input class="wt-field" type="text" bind:value={wtName} placeholder="Name" />
            <input class="wt-field" type="text" bind:value={wtTarget} placeholder="Ziel-Branch (z.B. alpha-main)" />
            <div class="wt-hint">Eigener Branch <code>terminal/{wtName}</code>, Merge zurück nach <code>{wtTarget || '?'}</code> per ✓</div>
          {/if}
        </div>
      {/if}
```

Styles (an bestehende Dialog-Styles anlehnen): `.worktree-opt { margin: 10px 0; } .wt-check { display:flex; gap:6px; align-items:center; font-size:12px; cursor:pointer; } .wt-field { width:100%; margin-top:6px; padding:6px 8px; background:var(--bg-secondary); border:1px solid var(--border); border-radius:6px; color:var(--fg); font-size:12px; } .wt-hint { font-size:10px; color:var(--fg-muted); margin-top:4px; }`

- [ ] **Step 3: `dir`-Prop durchreichen** — in `App.svelte` Zeile ~931: `<LaunchDialog … dir={$activeTab?.dir ?? ''} …>`

- [ ] **Step 4: Verifikation** — `cd frontend && npm run build` fehlerfrei; `grep -n '\$:' frontend/src/components/LaunchDialog.svelte` → neue `$:`-Zeile enthält nur den Funktionsaufruf.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/LaunchDialog.svelte frontend/src/App.svelte
git commit -m "feat(launch): opt-in worktree checkbox with name/target fields"
```

---

### Task 16: App.svelte — Launch-Verkettung

**Files:**
- Modify: `frontend/src/App.svelte` (`handleLaunch`, um Zeile 459)

**Interfaces:**
- Consumes: `e.detail.worktree` (Task 15), `App.CreatePaneWorktree` (Task 4/13), `tabStore.addPane(..., worktreePath, branch, targetBranch, ...)` (Task 14)
- Produces: Panes mit gesetztem `worktreePath`/`branch`/`targetBranch`; Session-CWD = Worktree-Pfad.

- [ ] **Step 1: `handleLaunch` erweitern** — vor dem `App.CreateSession`-Aufruf (Zeile ~459):

```typescript
      let paneWt: { path: string; branch: string; target_branch: string } | null = null;
      if (e.detail.worktree) {
        try {
          paneWt = await App.CreatePaneWorktree(tab.dir || '', e.detail.worktree.name, e.detail.worktree.targetBranch);
          if (paneWt) sessionDir = paneWt.path;
        } catch (err: any) {
          alert(`Worktree-Erstellung fehlgeschlagen:\n${err?.message || err}`);
          return; // kein stiller Fallback in den Haupt-Branch
        }
      }
```

und im `tabStore.addPane(...)`-Aufruf die Argumente worktreePath/branch/targetBranch mit `paneWt?.path ?? ''`, `paneWt?.branch ?? paneBranch`, `paneWt?.target_branch ?? ''` belegen.

- [ ] **Step 2: Startup-Reconcile einhängen** — im bestehenden `onMount` von `App.svelte`, nach dem Session-Restore: `App.ReconcileFinishMarkers($activeTab?.dir || '').catch(() => {});`

- [ ] **Step 3: Verifikation** — `npm run build`; manuell (`wails dev` bzw. Build starten): Launch mit Checkbox erzeugt Sibling-Worktree, Pane-CWD liegt darin (im Terminal `git branch --show-current` → `terminal/<name>`), `CLAUDE.local.md` + `.claude/settings.local.json` existieren, `git status` im Worktree ist leer.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/App.svelte
git commit -m "feat(launch): chain CreatePaneWorktree before session start"
```

---

### Task 17: PaneTitlebar — ⎇-Badge, ✓-Button, preparing-Spinner

**Files:**
- Modify: `frontend/src/components/PaneTitlebar.svelte`
- Modify: `frontend/src/stores/tabs.ts` (Finish-Phase am Pane), `frontend/src/components/PaneGrid.svelte` + `TerminalPane.svelte` (Event-Durchreichung, Muster `commitPush`)

**Interfaces:**
- Consumes: `pane.worktreePath/branch/targetBranch` (Task 14), `App.StartWorktreeFinish/CancelWorktreeFinish` (Task 13)
- Produces: `Pane.finishPhase: string` (`''|'preparing'|'ready'|'blocked'|'merging'|'cleanup'`), Store-Methode `setFinishPhase(sessionId: number, phase: string)`; Titlebar-Event `finishWorktree` → Handler in App.svelte (Task 18).

- [ ] **Step 1: Store erweitern** — `tabs.ts`: `finishPhase: string;` im Pane-Interface (Default `''` in `addPane`), Methode nach Muster von `updateActivity`:

```typescript
    setFinishPhase(sessionId: number, phase: string) {
      update((state) => {
        for (const tab of state.tabs) {
          const pane = tab.panes.find((p) => p.sessionId === sessionId);
          if (pane) pane.finishPhase = phase;
        }
        return state;
      });
    },
```

- [ ] **Step 2: Titlebar-Markup** — neben dem ☁-Button (Zeile ~198); nur bei `pane.worktreePath`:

```svelte
    {#if pane.worktreePath}
      <span class="wt-badge" title={`Worktree: ${pane.worktreePath}\nZiel: ${pane.targetBranch || '?'}`}>⎇ {pane.branch}</span>
      {#if pane.finishPhase === 'preparing' || pane.finishPhase === 'merging' || pane.finishPhase === 'cleanup'}
        <button class="pane-btn finish-btn spinning" title="Fertigstellen läuft – klicken zum Abbrechen"
          on:click|stopPropagation={() => dispatch('cancelFinish', { sessionId: pane.sessionId })}>◌</button>
      {:else}
        <button class="pane-btn finish-btn" title="Worktree fertigstellen: mergen & aufräumen"
          on:click|stopPropagation={() => dispatch('finishWorktree', { paneId: pane.id, sessionId: pane.sessionId })}>✓</button>
      {/if}
    {/if}
```

Styles: `.wt-badge { font-size:10px; color:var(--accent); background:var(--bg-tertiary); border:1px solid var(--border); border-radius:4px; padding:1px 6px; max-width:140px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; } .finish-btn { color:#4ade80; font-weight:700; } .finish-btn.spinning { animation: wt-spin 1s linear infinite; } @keyframes wt-spin { to { transform: rotate(360deg); } }`

- [ ] **Step 3: Events durchreichen** — `finishWorktree`/`cancelFinish` in `TerminalPane.svelte` und `PaneGrid.svelte` exakt nach dem `commitPush`-Muster (PaneGrid.svelte:40/85, TerminalPane.svelte:555) bis `App.svelte` hochreichen.

- [ ] **Step 4: Verifikation** — `npm run build`; manuell: Badge + ✓ erscheinen nur bei Worktree-Panes, Chat-Panes zeigen kein ✓.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/PaneTitlebar.svelte frontend/src/components/PaneGrid.svelte frontend/src/components/TerminalPane.svelte frontend/src/stores/tabs.ts
git commit -m "feat(titlebar): worktree badge + finish button with progress state"
```

---

### Task 18: WorktreeFinishDialog + Event-Wiring

**Files:**
- Create: `frontend/src/components/WorktreeFinishDialog.svelte`
- Modify: `frontend/src/App.svelte` (Event-Listener, Handler, Dialog-Einbindung)

**Interfaces:**
- Consumes: Events `worktree:finish-ready/-blocked/-done` (Task 7; Payloads siehe dort — **Filterung per `sessionId` über Pane-Ownership**, Muster `terminal:activity` App.svelte:262), `App.FinishWorktree/CancelWorktreeFinish/StartWorktreeFinish` (Task 13), `tabStore.setFinishPhase` (Task 17)
- Produces: Overlay mit Zuständen ready/cleanup-only/blocked; Relaunch-Logik nach finish-done.

- [ ] **Step 1: Dialog-Komponente** (Struktur/Styles nach Vorbild `WorktreeCreateDialog.svelte`)

```svelte
<!-- frontend/src/components/WorktreeFinishDialog.svelte -->
<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  export let visible = false;
  export let state: 'ready' | 'blocked' = 'ready';
  export let targetBranch = '';
  export let commits: string[] = [];
  export let stat = '';
  export let untracked: string[] = [];
  export let cleanupOnly = false;
  export let reason = '';
  const dispatch = createEventDispatcher();
</script>

{#if visible}
  <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
  <div class="overlay" on:click={() => dispatch('close')}>
    <!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
    <div class="dialog" on:click|stopPropagation>
      <div class="dialog-header"><span class="dialog-icon">⎇</span>
        <h3>{state === 'blocked' ? 'Fertigstellen blockiert' : cleanupOnly ? 'Nur aufräumen' : `Mergen nach ${targetBranch}`}</h3>
      </div>
      {#if state === 'blocked'}
        <p class="reason">{reason}</p>
        <div class="dialog-footer">
          <button class="btn-cancel" on:click={() => dispatch('cancel')}>Abbrechen</button>
          <button class="btn-create" on:click={() => dispatch('retry')}>Erneut vorbereiten</button>
        </div>
      {:else}
        {#if cleanupOnly}
          <p class="reason">Der Branch enthält keine neuen Commits gegenüber <code>{targetBranch}</code> — es gibt nichts zu mergen. Worktree und Branch werden entfernt.</p>
        {:else}
          <div class="commits">{#each commits as c}<div class="commit-line">{c}</div>{/each}</div>
          <pre class="stat">{stat}</pre>
        {/if}
        {#if untracked.length > 0}
          <div class="untracked">⚠ Untracked Dateien gehen beim Aufräumen verloren: {untracked.join(', ')}</div>
        {/if}
        <div class="dialog-footer">
          <button class="btn-cancel" on:click={() => dispatch('cancel')}>Abbrechen</button>
          <button class="btn-create" on:click={() => dispatch('confirm')}>{cleanupOnly ? 'Nur aufräumen' : 'Mergen & Aufräumen'}</button>
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  /* Overlay/Dialog-Styles 1:1 aus WorktreeCreateDialog.svelte übernehmen; zusätzlich: */
  .commits { max-height: 180px; overflow-y: auto; font-family: monospace; font-size: 11px; margin: 8px 0; }
  .commit-line { padding: 2px 0; color: var(--fg); }
  .stat { font-size: 10px; color: var(--fg-muted); max-height: 120px; overflow: auto; }
  .untracked { font-size: 11px; color: #fbbf24; margin: 8px 0; }
  .reason { font-size: 12px; color: var(--fg); }
</style>
```

- [ ] **Step 2: App.svelte-Wiring** — Script: State `let finishDialog = { visible: false, sessionId: 0, state: 'ready' as 'ready'|'blocked', targetBranch: '', commits: [] as string[], stat: '', untracked: [] as string[], cleanupOnly: false, reason: '' };` — Event-Listener neben den bestehenden (`terminal:activity`-Block, Zeile ~262), Filterung: Payload nur verarbeiten, wenn ein Pane dieses Fensters die `sessionId` besitzt (`findPaneBySession(payload.sessionId)`-Helper analog zum Activity-Handling):

```typescript
    Events.On('worktree:finish-ready', (ev: any) => {
      const p = ev.data ?? ev; if (!ownsSession(p.sessionId)) return;
      tabStore.setFinishPhase(p.sessionId, 'ready');
      finishDialog = { visible: true, sessionId: p.sessionId, state: 'ready', targetBranch: p.targetBranch,
        commits: p.commits || [], stat: p.stat || '', untracked: p.untracked || [], cleanupOnly: !!p.cleanupOnly, reason: '' };
    });
    Events.On('worktree:finish-blocked', (ev: any) => {
      const p = ev.data ?? ev; if (!ownsSession(p.sessionId)) return;
      tabStore.setFinishPhase(p.sessionId, p.phase || '');
      if (p.phase !== 'preparing') // informative Rückfrage-Events nur als Phase-Update
        finishDialog = { ...finishDialog, visible: true, sessionId: p.sessionId, state: 'blocked', reason: p.reason || '' };
    });
    Events.On('worktree:finish-done', async (ev: any) => {
      const p = ev.data ?? ev; if (!ownsSession(p.sessionId)) return;
      finishDialog = { ...finishDialog, visible: false };
      await relaunchPaneAfterFinish(p.sessionId, p.mainRoot, p.mode);
    });
```

(Exakte Event-API — `Events.On` aus `@wailsio/runtime` bzw. das im Projekt verwendete Muster — an den bestehenden `terminal:activity`-Listener anpassen; `ownsSession(id)` = es existiert ein Pane mit dieser sessionId im Store dieses Fensters.)

Handler:

```typescript
    function handleFinishWorktree(e: CustomEvent) {
      const { sessionId } = e.detail;
      const pane = findPaneBySession(sessionId);
      if (!pane?.worktreePath) return;
      const target = pane.targetBranch; // Alt-Worktrees ohne Target: Picker (v1: Prompt)
      const t = target || window.prompt('Ziel-Branch für den Merge:', branch || 'alpha-main') || '';
      if (!t) return;
      const mode = pane.mode === 'shell' ? 'shell' : 'claude';
      tabStore.setFinishPhase(sessionId, 'preparing');
      App.StartWorktreeFinish(sessionId, pane.worktreePath, pane.branch, t, mode);
      if (mode === 'shell') openShellStaging(sessionId, pane.worktreePath, t); // Task 20
    }

    async function relaunchPaneAfterFinish(sessionId: number, mainRoot: string, mode: string) {
      const loc = findPaneLocation(sessionId); // {tab, pane} — Helper analog bestehender Suche
      if (!loc) return;
      const { tab, pane } = loc;
      tabStore.closePane(tab.id, pane.id);
      const sid = mode !== 'shell' ? genSessionId() : '';
      const argv = mode !== 'shell'
        ? buildClaudeArgv(pane.mode, pane.model, resolvedClaudePath, resolvedCodexPath, resolvedGeminiPath, { sessionId: sid })
        : [];
      const newId = await App.CreateSession(argv, mainRoot, 24, 80, pane.mode);
      if (newId > 0) tabStore.addPane(tab.id, newId, pane.name, pane.mode, pane.model, null, '', '', '', '', '', false, 'terminal', '', sid);
    }
```

Dialog-Einbindung neben den anderen Dialogen (Zeile ~932):

```svelte
  <WorktreeFinishDialog {...finishDialog}
    on:confirm={() => { tabStore.setFinishPhase(finishDialog.sessionId, 'merging'); finishDialog.visible = false; App.FinishWorktree(finishDialog.sessionId); }}
    on:retry={() => { finishDialog.visible = false; handleRetryFinish(finishDialog.sessionId); }}
    on:cancel={() => { finishDialog.visible = false; App.CancelWorktreeFinish(finishDialog.sessionId); tabStore.setFinishPhase(finishDialog.sessionId, ''); }}
    on:close={() => (finishDialog.visible = false)} />
```

`handleRetryFinish(sessionId)` ruft `handleFinishWorktree`-Logik erneut auf (StartWorktreeFinish re-entert aus blocked, Task 7).

- [ ] **Step 3: Pane-Close-Dialog** — im bestehenden Close-Pfad (App.svelte ~586/610): wenn `pane.worktreePath` gesetzt und `finishPhase === ''`, `confirm('Pane hat einen aktiven Worktree. Behalten? (Abbrechen = Worktree bleibt liegen und ist über das ⎇-Dropdown erreichbar)')` — v1 bewusst nur Hinweis, kein Lösch-Automatismus (Spec 5.6: kein stilles Verwaisen; Entfernen erledigt der ✓-Flow).

- [ ] **Step 4: Verifikation** — `npm run build`; manueller E2E-Smoke (echtes Repo, echter Claude): Launch mit Worktree → Arbeit → ✓ → Prep läuft → Overlay zeigt Commits → Bestätigen → `git log` im Ziel-Branch enthält Commits, Worktree-Verzeichnis weg, `git branch` ohne `terminal/<name>`, Pane neu im Haupt-Repo.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/WorktreeFinishDialog.svelte frontend/src/App.svelte
git commit -m "feat(finish): confirm overlay + event wiring + pane relaunch"
```

---

### Task 19: Shell-Finish — Backend-Primitive

**Files:**
- Create: `internal/backend/app_worktree_shell.go`
- Test: `internal/backend/app_worktree_shell_test.go`

**Interfaces:**
- Consumes: `gitCmd`, `mainRepoRoot`
- Produces (frontend-exponiert, models.ts-Sync analog Task 13):

```go
type WorktreeFileChange struct {
    Path   string `json:"path" yaml:"path"`
    Status string `json:"status" yaml:"status"` // "M", "A", "D", "??", …
}
func (a *AppService) GetWorktreeChangedFiles(path string) []WorktreeFileChange
func (a *AppService) CommitWorktreeFiles(path string, files []string, message string) error // git add -- <files>; KEIN add -A (Spec 5.5)
func (a *AppService) RebaseWorktreeOntoTarget(path, target string) error
func (a *AppService) AbortWorktreeRebase(path string) error
```

- [ ] **Step 1: Failing Tests schreiben**

```go
// internal/backend/app_worktree_shell_test.go
package backend

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShellFinishPrimitives(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	info, err := a.CreatePaneWorktree(repo, "sh", "alpha-main")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(info.Path, "keep.txt"), []byte("k\n"), 0644)
	os.WriteFile(filepath.Join(info.Path, "secret.env"), []byte("s\n"), 0644)

	changes := a.GetWorktreeChangedFiles(info.Path)
	if len(changes) != 2 {
		t.Fatalf("changes = %+v, want 2 untracked", changes)
	}
	// Selektiver Commit: nur keep.txt — secret.env darf NICHT committet werden.
	if err := a.CommitWorktreeFiles(info.Path, []string{"keep.txt"}, "feat: keep"); err != nil {
		t.Fatal(err)
	}
	if out := gitRun(t, info.Path, "show", "--stat", "--oneline", "HEAD"); contains(out, "secret.env") {
		t.Fatal("selective commit staged unrelated file")
	}
	// Rebase auf unbewegtes Ziel ist ein No-op-Erfolg:
	if err := a.RebaseWorktreeOntoTarget(info.Path, "alpha-main"); err != nil {
		t.Fatal(err)
	}
}

func TestRebaseConflict_ReportsAndAborts(t *testing.T) {
	repo := initPaneTestRepo(t)
	a := &AppService{}
	info, _ := a.CreatePaneWorktree(repo, "conf", "alpha-main")
	// Konflikt bauen: gleiche Datei in Ziel und Branch ändern.
	os.WriteFile(filepath.Join(repo, "README.md"), []byte("target\n"), 0644)
	gitRun(t, repo, "commit", "-am", "target change")
	os.WriteFile(filepath.Join(info.Path, "README.md"), []byte("branch\n"), 0644)
	gitRun(t, info.Path, "commit", "-am", "branch change")

	if err := a.RebaseWorktreeOntoTarget(info.Path, "alpha-main"); err == nil {
		t.Fatal("expected rebase conflict error")
	}
	if err := a.AbortWorktreeRebase(info.Path); err != nil {
		t.Fatal(err)
	}
	// Nach Abort: kein rebase-in-progress, Branch-Commit intakt.
	if out := gitRun(t, info.Path, "log", "--oneline", "-1"); !contains(out, "branch change") {
		t.Errorf("abort lost the branch commit: %q", out)
	}
}
```

- [ ] **Step 2: Testlauf — FAIL** — Run: `go test ./internal/backend/ -run 'TestShellFinish|TestRebaseConflict' -v` → compile error

- [ ] **Step 3: Implementierung**

```go
// internal/backend/app_worktree_shell.go
// Mechanical finish for shell panes: MTUI itself commits (SELECTED files only
// — never add -A, spec 5.5) and rebases; conflicts are reported, never
// auto-resolved.
package backend

import (
	"fmt"
	"strings"
)

// WorktreeFileChange is one changed/untracked file for the staging dialog.
type WorktreeFileChange struct {
	Path   string `json:"path" yaml:"path"`
	Status string `json:"status" yaml:"status"`
}

// GetWorktreeChangedFiles lists modified + untracked files (porcelain).
func (a *AppService) GetWorktreeChangedFiles(path string) []WorktreeFileChange {
	out, err := gitCmd(path, "status", "--porcelain").Output()
	if err != nil {
		return nil
	}
	var changes []WorktreeFileChange
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 4 {
			continue
		}
		changes = append(changes, WorktreeFileChange{
			Status: strings.TrimSpace(line[:2]),
			Path:   strings.TrimSpace(line[3:]),
		})
	}
	return changes
}

// CommitWorktreeFiles stages exactly the given files and commits.
func (a *AppService) CommitWorktreeFiles(path string, files []string, message string) error {
	if len(files) == 0 {
		return fmt.Errorf("keine Dateien ausgewählt")
	}
	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("Commit-Message fehlt")
	}
	args := append([]string{"add", "--"}, files...)
	if out, err := gitCmd(path, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %s – %w", strings.TrimSpace(string(out)), err)
	}
	if out, err := gitCmd(path, "commit", "-m", message).CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %s – %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// RebaseWorktreeOntoTarget rebases the worktree branch onto the LOCAL target.
// On conflict the rebase stays in progress; the caller offers abort/manual.
func (a *AppService) RebaseWorktreeOntoTarget(path, target string) error {
	if out, err := gitCmd(path, "rebase", target).CombinedOutput(); err != nil {
		return fmt.Errorf("rebase auf %s fehlgeschlagen (Konflikt?): %s", target, strings.TrimSpace(string(out)))
	}
	return nil
}

// AbortWorktreeRebase aborts an in-progress rebase.
func (a *AppService) AbortWorktreeRebase(path string) error {
	if out, err := gitCmd(path, "rebase", "--abort").CombinedOutput(); err != nil {
		return fmt.Errorf("rebase --abort: %s – %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
```

- [ ] **Step 4: Testlauf — PASS** — Run wie Step 2, Expected: PASS (2 Tests)

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_worktree_shell.go internal/backend/app_worktree_shell_test.go
git commit -m "feat(worktree): shell finish primitives (selective commit, rebase)"
```

---

### Task 20: Shell-Finish — Staging-Dialog (Frontend)

**Files:**
- Modify: `frontend/src/components/WorktreeFinishDialog.svelte` (dritter Zustand `staging`)
- Modify: `frontend/src/App.svelte` (`openShellStaging`, Verkettung)
- Modify: `frontend/wailsjs/go/models.ts` (+ Bindings-Wrapper für Task-19-Methoden)

**Interfaces:**
- Consumes: Task-19-Bindings, `App.CheckWorktreeFinish(sessionId)` (Task 7/13)
- Produces: ✓ auf Shell-Panes → Staging-Dialog (Dateiliste mit Checkboxen, `.env`/Artefakte default ABGEWÄHLT via Heuristik `/\.env|node_modules|dist\/|build\//`, Message-Feld) → `CommitWorktreeFiles` → `RebaseWorktreeOntoTarget` → bei Erfolg `CheckWorktreeFinish` (weiter wie Claude-Flow) → bei Rebase-Fehler Dialog-Zustand `blocked` mit Buttons „Rebase abbrechen" (`AbortWorktreeRebase` + `CancelWorktreeFinish`) und „Im Terminal auflösen" (Dialog zu, Phase bleibt; User löst im Pane, klickt ✓ erneut).

- [ ] **Step 1: Dialog um `staging`-Zustand erweitern** — Props `state: 'ready'|'blocked'|'staging'`, `files: {path: string; status: string; selected: boolean}[]`, `commitMessage`; Markup: Checkbox-Liste + Message-Input + „Committen & Rebasen"-Button (`dispatch('stageCommit', { files, message })`). Leere Auswahl + vorhandene Commits ⇒ Button „Nur Rebasen" (`dispatch('rebaseOnly')`).
- [ ] **Step 2: `openShellStaging(sessionId, wtPath, target)`** in App.svelte: `GetWorktreeChangedFiles` laden, Default-Abwahl-Heuristik anwenden, Dialog mit `state:'staging'` öffnen; `on:stageCommit` → `CommitWorktreeFiles` → `RebaseWorktreeOntoTarget` → `CheckWorktreeFinish(sessionId)`; Fehlerpfade wie oben.
- [ ] **Step 3: Verifikation** — `npm run build`; manuell: Shell-Pane mit Worktree, Datei anlegen, ✓ → Staging → Commit+Rebase → Overlay → Merge; `.env`-Datei bleibt uncommittet.
- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/WorktreeFinishDialog.svelte frontend/src/App.svelte frontend/wailsjs/go/
git commit -m "feat(finish): shell staging review dialog (no blind add -A)"
```

---

### Task 21: Abschluss-Verifikation + E2E-Checkliste

**Files:** keine neuen — Verifikation + Doku.

- [ ] **Step 1: Volle Test-Suite** — Run: `go test ./internal/... && go vet ./...` → alles PASS, keine vet-Findings. `cd frontend && npx vitest run && npm run build` → PASS.
- [ ] **Step 2: 300-Zeilen-Check** — Run (PowerShell): `Get-ChildItem internal/backend/app_worktree_*.go, internal/backend/kill_*.go | ForEach-Object { "$($_.Name): $((Get-Content $_ | Measure-Object -Line).Lines)" }` → alle < 300, sonst splitten.
- [ ] **Step 3: E2E-Checkliste dokumentieren** — als Issue-Kommentar bzw. `needs-e2e-testing`-Label am Tracking-Issue; zu testen mit echtem Claude CLI:
  1. Launch mit Worktree (Claude-Pane) → `CLAUDE.local.md` wird von Claude erwähnt/beachtet (nachfragen: „in welchem Branch bist du?").
  2. `permissions.deny` real: Claude auffordern `git merge alpha-main` auszuführen → muss verweigert werden. Pattern-Syntax ggf. anpassen (Spec 3.5, E2E-Auftrag).
  3. ✓-Flow komplett inkl. Rebase-Konflikt (Ziel-Branch parallel bewegen).
  4. App-Kill zwischen Merge und Cleanup (Prozess im Taskmanager beenden, sobald Marker-Datei existiert) → Neustart räumt auf, mergt nicht doppelt.
  5. Zwei Panes, gleicher Ziel-Branch: nacheinander finishen — zweites bekommt „erneut vorbereiten" und läuft nach Re-Prep durch.
  6. Tab mit laufendem preparing in anderes Fenster ziehen → Flow läuft weiter (Backend-State).
  7. Windows-Handles: Claude-Session mit MCP-Servern im Worktree → ✓ → Worktree-Verzeichnis wird trotz Kindprozessen entfernt.
- [ ] **Step 4: Spec-Abgleich** — `docs/superpowers/specs/2026-07-02-worktree-pro-pane-design.md` §5.1/2 Rebase-Polling (`.git/worktrees/<name>/rebase-merge`) ist in v1 NICHT implementiert (Timeout-Event deckt den Hang ab) → Spec-Abschnitt um „(v1: über Timeout abgedeckt)" ergänzen ODER Nachtrags-Issue anlegen — nie still divergieren (CLAUDE.md-Regel).
- [ ] **Step 5: Commit + Abschluss**

```bash
git add docs/
git commit -m "docs(worktree): e2e checklist + v1 deviations

Refs #<tracking-issue>"
```

---

## Self-Review-Ergebnis (beim Schreiben geprüft)

- **Spec-Coverage:** §3.1–3.5 → Tasks 1–4; §4.1–4.4 → Tasks 6, 12–14, 17; §5.1–5.4 → Tasks 5, 7–11; §5.5 → Tasks 19–20; §5.6/§6 → Tasks 15–18; §9-Tests → in den jeweiligen Tasks. Bewusste v1-Lücke: Rebase-Polling (Task 21 Step 4 dokumentiert), Sidebar-Umschaltung auf Worktree-Pfad (Spec §6 „Verifikations-Auftrag" — bei E2E prüfen, ggf. Folge-Issue).
- **Typkonsistenz:** `StartWorktreeFinish(sessionId, worktreePath, branch, target, mode)` und `finishState`-Felder in Tasks 7/8/9/11/18 identisch; `PaneWorktreeInfo.target_branch` (json) überall gleich.
- **Platzhalter:** keine — alle Steps enthalten konkreten Code oder exakte Kommandos.





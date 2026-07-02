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

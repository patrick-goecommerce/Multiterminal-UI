package main

import (
	"encoding/json"
	"testing"
)

// editEvent builds a PreToolUse Edit event for path. Session id is unused by
// the force branch (no sidecar written) but kept realistic.
func editEvent(t *testing.T, path string) claudeEvent {
	t.Helper()
	input, err := json.Marshal(map[string]string{"file_path": path})
	if err != nil {
		t.Fatal(err)
	}
	return claudeEvent{SessionID: "force-sess", ToolName: "Edit", ToolInput: input}
}

// The force branch only engages when no worktree context is active, so these
// tests deliberately use an empty hooksDir (no sidecar) and set no
// MULTITERMINAL_WORKTREE_PATH.

func TestForceWorktree_NoEnvVarStillFailsOpen(t *testing.T) {
	blocked, _, _ := checkPathFirewall(editEvent(t, `D:\repo\internal\app.go`), t.TempDir())
	if blocked {
		t.Fatal("without MULTITERMINAL_FORCE_WORKTREE_ROOT the firewall must fail open")
	}
}

func TestForceWorktree_BlocksCodeInMainRepo(t *testing.T) {
	t.Setenv("MULTITERMINAL_FORCE_WORKTREE_ROOT", `D:\repo`)

	blocked, path, reason := checkPathFirewall(editEvent(t, `D:\repo\internal\app.go`), t.TempDir())
	if !blocked {
		t.Fatal("expected a code write in the main repo to be blocked")
	}
	if path != `D:\repo\internal\app.go` {
		t.Errorf("path = %q", path)
	}
	if reason == "" {
		t.Error("a block must carry a reason — it is what tells the model to call EnterWorktree")
	}
}

func TestForceWorktree_AllowsPathsOutsideTheRepo(t *testing.T) {
	t.Setenv("MULTITERMINAL_FORCE_WORKTREE_ROOT", `D:\repo`)

	blocked, _, _ := checkPathFirewall(editEvent(t, `C:\Temp\scratch\thing.go`), t.TempDir())
	if blocked {
		t.Fatal("paths outside the protected root must never be blocked")
	}
}

func TestForceWorktree_ExemptPaths(t *testing.T) {
	t.Setenv("MULTITERMINAL_FORCE_WORKTREE_ROOT", `D:\repo`)

	exempt := []string{
		`D:\repo\README.md`,
		`D:\repo\README.MD`,                 // extension match is case-insensitive
		`D:\repo\internal\backend\notes.md`, // .md anywhere, not just at the root
		`D:\repo\docs\plan.md`,
		`D:\repo\docs\diagrams\flow.go`, // docs/ is exempt wholesale, not only markdown
		`D:\repo\Docs\plan.md`,          // directory match is case-insensitive
		`D:\repo\.mtui\config.json`,
		`D:\repo\.claude\settings.local.json`,
	}
	for _, p := range exempt {
		if blocked, _, _ := checkPathFirewall(editEvent(t, p), t.TempDir()); blocked {
			t.Errorf("%s should be exempt from the worktree requirement", p)
		}
	}

	blocked := []string{
		`D:\repo\main.go`,
		`D:\repo\internal\backend\app.go`,
		`D:\repo\frontend\src\App.svelte`,
		`D:\repo\documentation\x.go`, // must not match the "docs" prefix loosely
	}
	for _, p := range blocked {
		if b, _, _ := checkPathFirewall(editEvent(t, p), t.TempDir()); !b {
			t.Errorf("%s should require a worktree", p)
		}
	}
}

func TestForceWorktree_ReadToolsUnaffected(t *testing.T) {
	t.Setenv("MULTITERMINAL_FORCE_WORKTREE_ROOT", `D:\repo`)

	ev := editEvent(t, `D:\repo\internal\app.go`)
	ev.ToolName = "Read"
	if blocked, _, _ := checkPathFirewall(ev, t.TempDir()); blocked {
		t.Fatal("only Edit/Write/NotebookEdit are policed; reads must pass")
	}
}

func TestForceWorktree_ActiveWorktreeTakesPrecedence(t *testing.T) {
	// With a worktree active the ORIGINAL branch runs, which has no exemptions:
	// a .md file in the main repo stays blocked there.
	t.Setenv("MULTITERMINAL_FORCE_WORKTREE_ROOT", `D:\repo`)
	t.Setenv("MULTITERMINAL_WORKTREE_PATH", `D:\repo\.claude\worktrees\a`)
	t.Setenv("MULTITERMINAL_MAIN_REPO_ROOT", `D:\repo`)
	hooksDir := t.TempDir()

	if blocked, _, _ := checkPathFirewall(editEvent(t, `D:\repo\.claude\worktrees\a\app.go`), hooksDir); blocked {
		t.Error("writes inside the active worktree must be allowed")
	}
	if blocked, _, _ := checkPathFirewall(editEvent(t, `D:\repo\README.md`), hooksDir); !blocked {
		t.Error("with a worktree active the main repo stays fully off-limits, markdown included")
	}
}

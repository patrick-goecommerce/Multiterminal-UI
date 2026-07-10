package backend

import "testing"

func TestRenderPlaceholders_AllThree(t *testing.T) {
	got := renderPlaceholders("push {{branch}} to {{targetBranch}} from {{worktreePath}}", "feat/x", "alpha-main", `C:\wt`)
	want := `push feat/x to alpha-main from C:\wt`
	if got != want {
		t.Errorf("renderPlaceholders() = %q, want %q", got, want)
	}
}

func TestRenderPlaceholders_NoPlaceholders(t *testing.T) {
	got := renderPlaceholders("just do the thing, no vars here", "feat/x", "alpha-main", `C:\wt`)
	if got != "just do the thing, no vars here" {
		t.Errorf("renderPlaceholders() = %q, want unchanged", got)
	}
}

func TestRenderPlaceholders_RepeatedPlaceholder(t *testing.T) {
	got := renderPlaceholders("{{branch}}...{{branch}}", "feat/x", "alpha-main", `C:\wt`)
	if got != "feat/x...feat/x" {
		t.Errorf("renderPlaceholders() = %q, want repeated substitution", got)
	}
}

func TestRenderFinishPrompt_DefaultWhenEmpty(t *testing.T) {
	a := newTestApp()
	got := a.renderFinishPrompt("feat/x", "alpha-main", `C:\wt`)
	want := "Rebase dann feat/x auf den lokalen alpha-main."
	if !contains(got, want) {
		t.Errorf("renderFinishPrompt() default = %q, missing %q", got, want)
	}
}

func TestRenderFinishPrompt_CustomTemplate(t *testing.T) {
	a := newTestApp()
	a.cfg.FinishPrepPrompt = "Push {{branch}}, open a PR against {{targetBranch}}, then clean up {{worktreePath}}."
	got := a.renderFinishPrompt("feat/y", "main", `C:\wt\feat-y`)
	want := `Push feat/y, open a PR against main, then clean up C:\wt\feat-y.`
	if got != want {
		t.Errorf("renderFinishPrompt() = %q, want %q", got, want)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}

// Shared placeholder rendering for user-configurable prompt templates (the
// worktree-finish prep prompt today; any future pane-context prompt reuses
// the same three placeholders).
package backend

import "strings"

// renderPlaceholders replaces {{branch}}, {{targetBranch}} and
// {{worktreePath}} in tpl with their actual values. Placeholders the
// template doesn't use are simply not substituted; the template may use
// each placeholder zero, one, or multiple times.
func renderPlaceholders(tpl, branch, target, worktreePath string) string {
	r := strings.NewReplacer(
		"{{branch}}", branch,
		"{{targetBranch}}", target,
		"{{worktreePath}}", worktreePath,
	)
	return r.Replace(tpl)
}

// renderFinishPrompt builds the prep prompt sent to a pane before a worktree
// finish: the user's own template (Config.FinishPrepPrompt) when set,
// otherwise the built-in default (prepPromptTemplate).
func (a *AppService) renderFinishPrompt(branch, target, worktreePath string) string {
	tpl := a.cfg.FinishPrepPrompt
	if tpl == "" {
		tpl = prepPromptTemplate
	}
	return renderPlaceholders(tpl, branch, target, worktreePath)
}

/** Substitute {{branch}}, {{targetBranch}} and {{worktreePath}} in a
 *  quick-action prompt template with the values of the pane the button was
 *  clicked on. Panes without a worktree pass '' for targetBranch/worktreePath
 *  — the placeholders simply resolve to an empty string. */
export function renderQuickActionPrompt(
  template: string,
  branch: string,
  targetBranch: string,
  worktreePath: string,
): string {
  return template
    .split('{{branch}}').join(branch)
    .split('{{targetBranch}}').join(targetBranch)
    .split('{{worktreePath}}').join(worktreePath);
}

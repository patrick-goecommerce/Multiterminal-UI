import { describe, it, expect } from 'vitest';
import { renderQuickActionPrompt } from './quickActions';

describe('renderQuickActionPrompt', () => {
  it('substitutes all three placeholders', () => {
    const result = renderQuickActionPrompt(
      'push {{branch}} to {{targetBranch}} from {{worktreePath}}',
      'feat/x', 'alpha-main', '/repo/.worktrees/feat-x',
    );
    expect(result).toBe('push feat/x to alpha-main from /repo/.worktrees/feat-x');
  });

  it('returns the template unchanged when it has no placeholders', () => {
    const result = renderQuickActionPrompt('/code-review', 'feat/x', 'alpha-main', '/repo/.worktrees/feat-x');
    expect(result).toBe('/code-review');
  });

  it('substitutes a repeated placeholder every time it appears', () => {
    const result = renderQuickActionPrompt('{{branch}}...{{branch}}', 'feat/x', '', '');
    expect(result).toBe('feat/x...feat/x');
  });

  it('substitutes empty string for placeholders with no value (non-worktree pane)', () => {
    const result = renderQuickActionPrompt('target={{targetBranch}} path={{worktreePath}}', 'main', '', '');
    expect(result).toBe('target= path=');
  });
});

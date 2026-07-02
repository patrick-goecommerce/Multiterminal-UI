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

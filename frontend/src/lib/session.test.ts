import { describe, it, expect, vi, beforeEach } from 'vitest';

const { loadTabs, ensureProjectWorktreeSetup, createSession, worktreeDirExists, linkSessionIssue } = vi.hoisted(() => ({
  loadTabs: vi.fn(),
  ensureProjectWorktreeSetup: vi.fn(),
  createSession: vi.fn(),
  worktreeDirExists: vi.fn(),
  linkSessionIssue: vi.fn(),
}));

vi.mock('../../wailsjs/go/backend/App', () => ({
  LoadTabs: loadTabs,
  EnsureProjectWorktreeSetup: ensureProjectWorktreeSetup,
  CreateSession: createSession,
  WorktreeDirExists: worktreeDirExists,
  LinkSessionIssue: linkSessionIssue,
}));

import { paneToSaved, restoreSession } from './session';

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

// Bug: EnsureProjectWorktreeSetup only ran on fresh pane launches (App.svelte
// handleLaunch), never on restore. A project restored after an app restart
// stayed stuck with a stale/missing CLAUDE.local.md and settings.local.json
// forever, since a Claude session only reads its memory files once at start.
describe('restoreSession worktree setup', () => {
  beforeEach(() => {
    loadTabs.mockReset();
    ensureProjectWorktreeSetup.mockReset().mockResolvedValue(undefined);
    createSession.mockReset().mockResolvedValue(1);
    worktreeDirExists.mockReset().mockResolvedValue(false);
    linkSessionIssue.mockReset();
  });

  function savedPane(mode: number) {
    return {
      name: 'p', mode, model: '', display: 'terminal', conversation_id: '',
      claude_session_id: '', user_renamed: false, worktree_path: '',
      worktree_branch: '', target_branch: '', issue_number: 0, issue_branch: '',
      zoom_delta: 0,
    };
  }

  it('re-checks project worktree setup for a tab with a non-shell pane', async () => {
    loadTabs.mockResolvedValue({
      active_tab: 0,
      tabs: [{ name: 't', dir: 'D:/repos/foo', focus_idx: 0, panes: [savedPane(1)] }],
    });

    await restoreSession('claude');

    expect(ensureProjectWorktreeSetup).toHaveBeenCalledWith('D:/repos/foo');
  });

  it('skips the setup check for a shell-only tab', async () => {
    loadTabs.mockResolvedValue({
      active_tab: 0,
      tabs: [{ name: 't', dir: 'D:/repos/foo', focus_idx: 0, panes: [savedPane(0)] }],
    });

    await restoreSession('claude');

    expect(ensureProjectWorktreeSetup).not.toHaveBeenCalled();
  });
});

import { describe, it, expect, vi, beforeEach } from 'vitest';

const { loadTabs, ensureProjectWorktreeSetup, createSession, worktreeDirExists, linkSessionIssue, resolveMCPProfile, seedActivitySince } = vi.hoisted(() => ({
  loadTabs: vi.fn(),
  ensureProjectWorktreeSetup: vi.fn(),
  createSession: vi.fn(),
  worktreeDirExists: vi.fn(),
  linkSessionIssue: vi.fn(),
  resolveMCPProfile: vi.fn(),
  seedActivitySince: vi.fn(),
}));

vi.mock('../../wailsjs/go/backend/App', () => ({
  LoadTabs: loadTabs,
  EnsureProjectWorktreeSetup: ensureProjectWorktreeSetup,
  CreateSession: createSession,
  WorktreeDirExists: worktreeDirExists,
  LinkSessionIssue: linkSessionIssue,
  ResolveMCPProfile: resolveMCPProfile,
  SeedActivitySince: seedActivitySince,
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

  it('serialisiert das MCP-Profil (sonst startet der Pane nach Neustart wieder alle MCP-Server)', () => {
    expect(paneToSaved({ name: 'x', mode: 'claude', mcpProfile: 'none' } as any).mcp_profile).toBe('none');
    expect(paneToSaved({ name: 'x', mode: 'claude' } as any).mcp_profile).toBe('');
  });

  it('round-trips activitySince through save and restore', () => {
    const pane = { name: 'x', mode: 'claude', activitySince: 1700000000 } as any;
    const saved = paneToSaved(pane);
    expect(saved.activity_since).toBe(1700000000);
  });

  it('defaults activity_since to 0 for a pane with no activitySince yet', () => {
    expect(paneToSaved({ name: 'x', mode: 'claude' } as any).activity_since).toBe(0);
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
    resolveMCPProfile.mockReset().mockResolvedValue('');
  });

  function savedPane(mode: number, extra: Record<string, unknown> = {}) {
    return {
      name: 'p', mode, model: '', display: 'terminal', conversation_id: '',
      claude_session_id: '', user_renamed: false, worktree_path: '',
      worktree_branch: '', target_branch: '', issue_number: 0, issue_branch: '',
      zoom_delta: 0, ...extra,
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

// A restored pane must keep the MCP profile it was launched with — otherwise
// every app restart silently re-multiplies the MCP child processes (#179).
describe('restoreSession MCP profile', () => {
  beforeEach(() => {
    loadTabs.mockReset();
    ensureProjectWorktreeSetup.mockReset().mockResolvedValue(undefined);
    createSession.mockReset().mockResolvedValue(1);
    worktreeDirExists.mockReset().mockResolvedValue(false);
    linkSessionIssue.mockReset();
    resolveMCPProfile.mockReset().mockResolvedValue('');
  });

  function restoreWith(extra: Record<string, unknown>) {
    loadTabs.mockResolvedValue({
      active_tab: 0,
      tabs: [{
        name: 't', dir: 'D:/repos/foo', focus_idx: 0,
        panes: [{
          name: 'p', mode: 1, model: '', display: 'terminal', conversation_id: '',
          claude_session_id: '', user_renamed: false, worktree_path: '',
          worktree_branch: '', target_branch: '', issue_number: 0, issue_branch: '',
          zoom_delta: 0, ...extra,
        }],
      }],
    });
    return restoreSession('claude');
  }

  it('restores a "none" pane with --strict-mcp-config and no backend lookup', async () => {
    await restoreWith({ mcp_profile: 'none' });

    expect(resolveMCPProfile).not.toHaveBeenCalled();
    const argv = createSession.mock.calls[0][0];
    expect(argv).toContain('--strict-mcp-config');
    expect(argv).not.toContain('--mcp-config');
  });

  it('restores a named profile with the backend-resolved --mcp-config path', async () => {
    resolveMCPProfile.mockResolvedValue('D:/home/.multiterminal/mcp-profiles/p-1.json');

    await restoreWith({ mcp_profile: 'Nur MTUI' });

    expect(resolveMCPProfile).toHaveBeenCalledWith('D:/repos/foo', 'Nur MTUI');
    const argv = createSession.mock.calls[0][0];
    expect(argv.slice(-3)).toEqual(['--strict-mcp-config', '--mcp-config', 'D:/home/.multiterminal/mcp-profiles/p-1.json']);
  });

  it('adds no MCP flags for a pane saved without a profile', async () => {
    await restoreWith({});

    const argv = createSession.mock.calls[0][0];
    expect(argv).not.toContain('--strict-mcp-config');
  });
});

// A pane's state duration must survive an MTUI restart (#189): the saved
// timestamp is seeded back into the backend's debounce bookkeeping right
// after the session is recreated.
describe('restoreSession activitySince seeding', () => {
  beforeEach(() => {
    loadTabs.mockReset();
    ensureProjectWorktreeSetup.mockReset().mockResolvedValue(undefined);
    createSession.mockReset().mockResolvedValue(1);
    worktreeDirExists.mockReset().mockResolvedValue(false);
    linkSessionIssue.mockReset();
    resolveMCPProfile.mockReset().mockResolvedValue('');
    seedActivitySince.mockReset();
  });

  function restoreWith(extra: Record<string, unknown>) {
    loadTabs.mockResolvedValue({
      active_tab: 0,
      tabs: [{
        name: 't', dir: 'D:/repos/foo', focus_idx: 0,
        panes: [{
          name: 'p', mode: 1, model: '', display: 'terminal', conversation_id: '',
          claude_session_id: '', user_renamed: false, worktree_path: '',
          worktree_branch: '', target_branch: '', issue_number: 0, issue_branch: '',
          zoom_delta: 0, ...extra,
        }],
      }],
    });
    return restoreSession('claude');
  }

  it('seeds the backend with the restored timestamp when the saved pane has one', async () => {
    await restoreWith({ activity_since: 1700000000 });

    expect(seedActivitySince).toHaveBeenCalledWith(1, 1700000000);
  });

  // A session file from before this field existed has no activity_since key
  // at all. `?? 0` must turn that into 0, not undefined — otherwise the
  // badge later computes a duration against NaN.
  it('does not call SeedActivitySince for a pane saved without activity_since (predates the field)', async () => {
    await restoreWith({});

    expect(seedActivitySince).not.toHaveBeenCalled();
  });
});

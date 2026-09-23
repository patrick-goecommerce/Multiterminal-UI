import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';
import type { Pane } from '../stores/tabs';

const { getGitBranch, getMainRepoRoot } = vi.hoisted(() => ({
  getGitBranch: vi.fn(),
  getMainRepoRoot: vi.fn(),
}));

vi.mock('../../wailsjs/go/backend/App', () => ({
  GetGitBranch: getGitBranch,
  GetMainRepoRoot: getMainRepoRoot,
}));

import PaneTitlebar from './PaneTitlebar.svelte';
import { resetBranchCache } from '../lib/git-polling';

function pane(overrides: Partial<Pane> = {}): Pane {
  return {
    id: 'pane-1', sessionId: 42, name: 'Claude', mode: 'claude', model: '', focused: true,
    activity: 'idle', cost: '', running: true, maximized: false, issueNumber: null,
    issueTitle: '', issueBranch: '', worktreePath: '', branch: '', targetBranch: '',
    zoomDelta: 0, background: false, display: 'terminal', conversationId: '',
    claudeSessionId: '', mcpProfile: '', autoName: '', oscTitle: '', autoNameSource: '',
    userRenamed: false, finishPhase: '', activitySince: 0,
    ...overrides,
  };
}

beforeEach(() => {
  resetBranchCache();
  getGitBranch.mockReset().mockResolvedValue('main');
  getMainRepoRoot.mockReset().mockResolvedValue('D:/repo');
});
afterEach(() => cleanup());

// The tab store mutates panes in place and re-emits the same object, and in
// legacy mode that counts as a changed prop. Each of these updates used to
// start one git process per pane.
async function updateInPlace(component: any, p: Pane, times: number) {
  for (let i = 0; i < times; i++) {
    p.cost = `$0.${i}`;
    p.activity = i % 2 ? 'active' : 'idle';
    component.$set({ pane: p });
    await tick();
  }
}

describe('PaneTitlebar git lookups', () => {
  it('reads the branch once, not once per store update', async () => {
    const p = pane();
    const { component } = render(PaneTitlebar, { props: { pane: p, tabDir: 'D:/repo' } });
    await tick();
    await updateInPlace(component, p, 10);
    expect(getGitBranch).toHaveBeenCalledTimes(1);
  });

  it('reads the main repo root once per worktree path', async () => {
    const p = pane({ worktreePath: 'D:/repo/.claude/worktrees/a' });
    const { component } = render(PaneTitlebar, { props: { pane: p, tabDir: 'D:/repo' } });
    await tick();
    await updateInPlace(component, p, 10);
    expect(getMainRepoRoot).toHaveBeenCalledTimes(1);
    expect(getGitBranch).not.toHaveBeenCalled();

    p.worktreePath = 'D:/repo/.claude/worktrees/b';
    component.$set({ pane: p });
    await tick();
    expect(getMainRepoRoot).toHaveBeenCalledTimes(2);
  });

  it('shares one lookup between panes of the same directory', async () => {
    render(PaneTitlebar, { props: { pane: pane({ id: 'a' }), tabDir: 'D:/repo' } });
    render(PaneTitlebar, { props: { pane: pane({ id: 'b' }), tabDir: 'D:/repo' } });
    await tick();
    expect(getGitBranch).toHaveBeenCalledTimes(1);
  });
});

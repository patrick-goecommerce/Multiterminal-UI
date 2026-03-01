import { describe, it, expect } from 'vitest';
import { groupPanesByActivity } from './dashboard';
import type { Tab, Pane } from '../stores/tabs';

function makePane(overrides: Partial<Pane>): Pane {
  return {
    id: 'p1', sessionId: 1, name: 'Claude', mode: 'claude', model: '',
    focused: false, activity: 'idle', cost: '', running: true, maximized: false,
    issueNumber: null, issueTitle: '', issueBranch: '', worktreePath: '',
    branch: 'main', zoomDelta: 0,
    ...overrides,
  };
}

function makeTab(id: string, name: string, panes: Pane[]): Tab {
  return { id, name, dir: '/proj', panes, focusedPaneId: '', unreadActivity: null };
}

describe('groupPanesByActivity', () => {
  it('returns empty groups for no tabs', () => {
    const groups = groupPanesByActivity([]);
    expect(groups.needsAttention).toEqual([]);
    expect(groups.active).toEqual([]);
    expect(groups.done).toEqual([]);
    expect(groups.idle).toEqual([]);
  });

  it('groups waitingPermission into needsAttention', () => {
    const tab = makeTab('t1', 'Auth', [makePane({ id: 'p1', activity: 'waitingPermission' })]);
    const groups = groupPanesByActivity([tab]);
    expect(groups.needsAttention).toHaveLength(1);
    expect(groups.needsAttention[0].tabName).toBe('Auth');
  });

  it('groups waitingAnswer into needsAttention', () => {
    const tab = makeTab('t1', 'API', [makePane({ id: 'p1', activity: 'waitingAnswer' })]);
    const groups = groupPanesByActivity([tab]);
    expect(groups.needsAttention).toHaveLength(1);
  });

  it('groups error into needsAttention', () => {
    const tab = makeTab('t1', 'Test', [makePane({ id: 'p1', activity: 'error' })]);
    const groups = groupPanesByActivity([tab]);
    expect(groups.needsAttention).toHaveLength(1);
  });

  it('groups active into active', () => {
    const tab = makeTab('t1', 'Frontend', [makePane({ id: 'p1', activity: 'active' })]);
    const groups = groupPanesByActivity([tab]);
    expect(groups.active).toHaveLength(1);
    expect(groups.active[0].tabId).toBe('t1');
  });

  it('groups done into done', () => {
    const tab = makeTab('t1', 'Backend', [makePane({ id: 'p1', activity: 'done' })]);
    const groups = groupPanesByActivity([tab]);
    expect(groups.done).toHaveLength(1);
  });

  it('groups idle into idle', () => {
    const tab = makeTab('t1', 'Shell', [makePane({ id: 'p1', activity: 'idle', mode: 'shell' })]);
    const groups = groupPanesByActivity([tab]);
    expect(groups.idle).toHaveLength(1);
  });

  it('attaches tabId and tabName to each pane', () => {
    const tab = makeTab('tab-42', 'My Project', [makePane({ id: 'p99', activity: 'done' })]);
    const groups = groupPanesByActivity([tab]);
    expect(groups.done[0].tabId).toBe('tab-42');
    expect(groups.done[0].tabName).toBe('My Project');
    expect(groups.done[0].id).toBe('p99');
  });

  it('handles multiple tabs with mixed panes', () => {
    const tabs = [
      makeTab('t1', 'Auth', [
        makePane({ id: 'p1', activity: 'waitingPermission' }),
        makePane({ id: 'p2', activity: 'active' }),
      ]),
      makeTab('t2', 'API', [
        makePane({ id: 'p3', activity: 'done' }),
        makePane({ id: 'p4', activity: 'idle' }),
      ]),
    ];
    const groups = groupPanesByActivity(tabs);
    expect(groups.needsAttention).toHaveLength(1);
    expect(groups.active).toHaveLength(1);
    expect(groups.done).toHaveLength(1);
    expect(groups.idle).toHaveLength(1);
  });
});

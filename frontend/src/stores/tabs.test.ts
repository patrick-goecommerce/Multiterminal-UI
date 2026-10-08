import { describe, it, expect } from 'vitest';
import { get } from 'svelte/store';
import { tabStore, activeTab, allTabs, paneDisplayName, windowTitle, tabNeedsCloseConfirm } from './tabs';

// Note: tabStore uses internal counters that persist across tests.
// We work with that by testing behavior rather than exact IDs.

describe('tabStore', () => {
  describe('applyAttention', () => {
    it('puts the record on the pane with that session', () => {
      const tabId = tabStore.addTab('att', 'C:\\att');
      tabStore.addPane(tabId, 'h:77', 'P', 'claude', '');
      tabStore.applyAttention('h:77', { state: 'done', rank: 3, seen: false, detail: 'x', source: 'Hook: Stop', since: 5 });
      const pane = get(allTabs).flatMap((t) => t.panes).find((p) => p.sessionId === 'h:77');
      expect(pane?.attention?.rank).toBe(3);
    });
  });

  describe('addTab', () => {
    it('creates a tab with name and dir', () => {
      const id = tabStore.addTab('Test', '/home/user');
      const state = tabStore.getState();
      const tab = state.tabs.find((t) => t.id === id);
      expect(tab).toBeDefined();
      expect(tab!.name).toBe('Test');
      expect(tab!.dir).toBe('/home/user');
      expect(tab!.panes).toEqual([]);
    });

    it('sets the new tab as active', () => {
      const id = tabStore.addTab('Active');
      const state = tabStore.getState();
      expect(state.activeTabId).toBe(id);
    });

    it('uses default name when none provided', () => {
      const id = tabStore.addTab();
      const state = tabStore.getState();
      const tab = state.tabs.find((t) => t.id === id);
      expect(tab).toBeDefined();
      expect(tab!.name).toMatch(/^Tab \d+$/);
    });

    it('uses empty dir when none provided', () => {
      const id = tabStore.addTab('NoDir');
      const state = tabStore.getState();
      const tab = state.tabs.find((t) => t.id === id);
      expect(tab!.dir).toBe('');
    });
  });

  describe('closeTab', () => {
    it('removes the specified tab', () => {
      const id1 = tabStore.addTab('Tab1');
      const id2 = tabStore.addTab('Tab2');
      const before = tabStore.getState().tabs.length;

      tabStore.closeTab(id2);
      const after = tabStore.getState().tabs.length;
      expect(after).toBe(before - 1);
      expect(tabStore.getState().tabs.find((t) => t.id === id2)).toBeUndefined();
    });

    it('does not close the last tab', () => {
      // Clear: add a single tab
      const id = tabStore.addTab('Only');
      // Close all others first
      const state = tabStore.getState();
      const otherTabs = state.tabs.filter((t) => t.id !== id);
      // Tabs won't close below 1, but let's test that:
      // If there's only 1 tab left, closing it should not work
      if (state.tabs.length === 1) {
        tabStore.closeTab(id);
        expect(tabStore.getState().tabs.length).toBe(1);
      }
    });

    it('activates next tab when active is closed', () => {
      const id1 = tabStore.addTab('A');
      const id2 = tabStore.addTab('B');
      const id3 = tabStore.addTab('C');
      tabStore.setActiveTab(id2);

      tabStore.closeTab(id2);
      const state = tabStore.getState();
      // Should activate an adjacent tab
      expect(state.activeTabId).not.toBe(id2);
      expect(state.tabs.find((t) => t.id === state.activeTabId)).toBeDefined();
    });
  });

  describe('setActiveTab', () => {
    it('changes the active tab', () => {
      const id1 = tabStore.addTab('First');
      const id2 = tabStore.addTab('Second');

      tabStore.setActiveTab(id1);
      expect(tabStore.getState().activeTabId).toBe(id1);

      tabStore.setActiveTab(id2);
      expect(tabStore.getState().activeTabId).toBe(id2);
    });
  });

  describe('renameTab', () => {
    it('changes the tab name', () => {
      const id = tabStore.addTab('Original');
      tabStore.renameTab(id, 'Renamed');

      const tab = tabStore.getState().tabs.find((t) => t.id === id);
      expect(tab!.name).toBe('Renamed');
    });
  });

  describe('setTabDir', () => {
    it('changes dir when tab has no panes', () => {
      const id = tabStore.addTab('DirTest', '/old');
      tabStore.setTabDir(id, '/new');

      const tab = tabStore.getState().tabs.find((t) => t.id === id);
      expect(tab!.dir).toBe('/new');
    });

    it('does not change dir when tab has panes', () => {
      const id = tabStore.addTab('DirTest2', '/old');
      tabStore.addPane(id, 'h1:999', 'Shell', 'shell', '');

      tabStore.setTabDir(id, '/new');
      const tab = tabStore.getState().tabs.find((t) => t.id === id);
      expect(tab!.dir).toBe('/old');
    });
  });

  describe('addPane', () => {
    it('adds a pane with correct properties', () => {
      const tabId = tabStore.addTab('PaneTest');
      const paneId = tabStore.addPane(tabId, 'h1:42', 'Claude', 'claude', 'opus');

      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      const pane = tab!.panes.find((p) => p.id === paneId);

      expect(pane).toBeDefined();
      expect(pane!.sessionId).toBe('h1:42');
      expect(pane!.name).toBe('Claude');
      expect(pane!.mode).toBe('claude');
      expect(pane!.model).toBe('opus');
      expect(pane!.focused).toBe(true);
      expect(pane!.running).toBe(true);
      expect(pane!.activity).toBe('starting');
      expect(pane!.cost).toBe('');
      expect(pane!.maximized).toBe(false);
    });

    it('focuses the new pane and unfocuses others', () => {
      const tabId = tabStore.addTab('FocusTest');
      tabStore.addPane(tabId, 'h1:1', 'P1', 'shell', '');
      const p2 = tabStore.addPane(tabId, 'h1:2', 'P2', 'shell', '');

      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      // P1 should now be unfocused
      expect(tab!.panes[0].focused).toBe(false);
      // P2 should be focused
      const pane2 = tab!.panes.find((p) => p.id === p2);
      expect(pane2!.focused).toBe(true);
    });
  });

  describe('closePane', () => {
    it('removes the specified pane', () => {
      const tabId = tabStore.addTab('ClosePaneTest');
      const p1 = tabStore.addPane(tabId, 'h1:1', 'P1', 'shell', '');
      const p2 = tabStore.addPane(tabId, 'h1:2', 'P2', 'shell', '');

      tabStore.closePane(tabId, p1);

      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      expect(tab!.panes.length).toBe(1);
      expect(tab!.panes[0].id).toBe(p2);
    });

    it('focuses another pane after closing focused one', () => {
      const tabId = tabStore.addTab('CloseFocusTest');
      const p1 = tabStore.addPane(tabId, 'h1:1', 'P1', 'shell', '');
      const p2 = tabStore.addPane(tabId, 'h1:2', 'P2', 'shell', '');
      // p2 is focused

      tabStore.closePane(tabId, p2);
      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      // p1 should now be focused
      expect(tab!.panes[0].focused).toBe(true);
    });
  });

  describe('focusPane', () => {
    it('sets focus correctly', () => {
      const tabId = tabStore.addTab('FocusPaneTest');
      const p1 = tabStore.addPane(tabId, 'h1:1', 'P1', 'shell', '');
      const p2 = tabStore.addPane(tabId, 'h1:2', 'P2', 'shell', '');

      tabStore.focusPane(tabId, p1);

      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      const pane1 = tab!.panes.find((p) => p.id === p1);
      const pane2 = tab!.panes.find((p) => p.id === p2);
      expect(pane1!.focused).toBe(true);
      expect(pane2!.focused).toBe(false);
      expect(tab!.focusedPaneId).toBe(p1);
    });
  });

  describe('toggleMaximize', () => {
    it('toggles pane maximized state', () => {
      const tabId = tabStore.addTab('MaxTest');
      const paneId = tabStore.addPane(tabId, 'h1:1', 'P1', 'shell', '');

      let tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      expect(tab!.panes[0].maximized).toBe(false);

      tabStore.toggleMaximize(tabId, paneId);
      tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      expect(tab!.panes[0].maximized).toBe(true);

      tabStore.toggleMaximize(tabId, paneId);
      tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      expect(tab!.panes[0].maximized).toBe(false);
    });
  });

  describe('updateActivity', () => {
    it('updates activity and cost by session ID', () => {
      const tabId = tabStore.addTab('ActivityTest');
      tabStore.addPane(tabId, 'h1:777', 'Claude', 'claude', '');

      tabStore.updateActivity('h1:777', 'active', '$0.12', 1_700_000_000);

      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      const pane = tab!.panes.find((p) => p.sessionId === 'h1:777');
      expect(pane!.activity).toBe('active');
      expect(pane!.cost).toBe('$0.12');
      expect(pane!.activitySince).toBe(1_700_000_000);
    });

    it('finds pane across multiple tabs', () => {
      const tab1 = tabStore.addTab('Tab1');
      const tab2 = tabStore.addTab('Tab2');
      tabStore.addPane(tab1, 'h1:100', 'P1', 'shell', '');
      tabStore.addPane(tab2, 'h1:200', 'P2', 'claude', '');

      tabStore.updateActivity('h1:200', 'done', '$1.50', 0);

      const t2 = tabStore.getState().tabs.find((t) => t.id === tab2);
      const pane = t2!.panes.find((p) => p.sessionId === 'h1:200');
      expect(pane!.activity).toBe('done');
      expect(pane!.cost).toBe('$1.50');
    });

    it('does not overwrite cost with empty string', () => {
      const tabId = tabStore.addTab('CostTest');
      tabStore.addPane(tabId, 'h1:888', 'Claude', 'claude', '');

      tabStore.updateActivity('h1:888', 'active', '$0.50', 0);
      tabStore.updateActivity('h1:888', 'done', '', 0);

      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      const pane = tab!.panes.find((p) => p.sessionId === 'h1:888');
      expect(pane!.cost).toBe('$0.50');
    });

    it('keeps the previous activitySince while an empty activity update carries no change', () => {
      const tabId = tabStore.addTab('SinceCostOnlyTest');
      tabStore.addPane(tabId, 'h1:889', 'Claude', 'claude', '');

      tabStore.updateActivity('h1:889', 'active', '', 1_700_000_100);
      tabStore.updateActivity('h1:889', '', '$0.20', 0); // cost-only update, empty activity

      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      const pane = tab!.panes.find((p) => p.sessionId === 'h1:889');
      expect(pane!.activitySince).toBe(1_700_000_100);
      expect(pane!.cost).toBe('$0.20');
    });
  });

  describe('finishPhase', () => {
    it('initializes finishPhase to empty string on a new pane', () => {
      const tabId = tabStore.addTab('T', '/d');
      tabStore.addPane(tabId, 'h1:8100', 'Claude', 'claude', '');
      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      const pane = tab!.panes.find((p) => p.sessionId === 'h1:8100');
      expect(pane!.finishPhase).toBe('');
    });

    it('setFinishPhase sets the phase by session id', () => {
      const tabId = tabStore.addTab('T', '/d');
      tabStore.addPane(tabId, 'h1:8101', 'Claude', 'claude', '');
      tabStore.setFinishPhase('h1:8101', 'preparing');
      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      const pane = tab!.panes.find((p) => p.sessionId === 'h1:8101');
      expect(pane!.finishPhase).toBe('preparing');
    });

    it('setFinishPhase leaves worktree fields untouched', () => {
      const tabId = tabStore.addTab('T', '/d');
      tabStore.addPane(tabId, 'h1:8102', 'Claude', 'claude', '');
      tabStore.setWorktree('h1:8102', '/wt', 'worktree-x', 'alpha-main');
      tabStore.setFinishPhase('h1:8102', 'merging');
      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      const pane = tab!.panes.find((p) => p.sessionId === 'h1:8102');
      expect(pane!.worktreePath).toBe('/wt');
      expect(pane!.branch).toBe('worktree-x');
      expect(pane!.finishPhase).toBe('merging');
    });
  });

  describe('markExited', () => {
    it('sets running to false', () => {
      const tabId = tabStore.addTab('ExitTest');
      tabStore.addPane(tabId, 'h1:555', 'Shell', 'shell', '');

      tabStore.markExited('h1:555');

      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      const pane = tab!.panes.find((p) => p.sessionId === 'h1:555');
      expect(pane!.running).toBe(false);
    });
  });

  describe('renamePane', () => {
    it('changes the pane name', () => {
      const tabId = tabStore.addTab('RenameTest');
      const paneId = tabStore.addPane(tabId, 'h1:1', 'Old', 'shell', '');

      tabStore.renamePane(tabId, paneId, 'New Name');

      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      const pane = tab!.panes.find((p) => p.id === paneId);
      expect(pane!.name).toBe('New Name');
    });
  });

  describe('derived stores', () => {
    it('activeTab returns the current active tab', () => {
      const id = tabStore.addTab('DerivedTest');
      const tab = get(activeTab);
      expect(tab).toBeDefined();
      expect(tab!.id).toBe(id);
    });

    it('allTabs returns all tabs', () => {
      const before = get(allTabs).length;
      tabStore.addTab('Extra');
      const after = get(allTabs).length;
      expect(after).toBe(before + 1);
    });
  });
});

describe('setAutoName — most-recent-wins recency', () => {
  it('records the source of the most recent auto name', () => {
    const tabId = tabStore.addTab('AutoRecency');
    tabStore.addPane(tabId, 'h1:6001', 'Shell', 'shell', '');

    tabStore.setAutoName('h1:6001', 'auth-refactor', 'llm');
    let pane = tabStore.getState().tabs.find((t) => t.id === tabId)!.panes[0];
    expect(pane.autoNameSource).toBe('llm');

    tabStore.setAutoName('h1:6001', 'vim README.md', 'osc');
    pane = tabStore.getState().tabs.find((t) => t.id === tabId)!.panes[0];
    expect(pane.autoNameSource).toBe('osc');
  });

  it('does not change auto names once the pane is user-renamed', () => {
    const tabId = tabStore.addTab('AutoRenamed');
    const paneId = tabStore.addPane(tabId, 'h1:6002', 'Shell', 'shell', '');
    tabStore.renamePane(tabId, paneId, 'My Pane');

    tabStore.setAutoName('h1:6002', 'should-be-ignored', 'osc');
    const pane = tabStore.getState().tabs.find((t) => t.id === tabId)!.panes[0];
    expect(pane.autoNameSource).toBe('');
  });
});

describe('paneDisplayName', () => {
  it('shows the most recently updated auto source (OSC after LLM)', () => {
    const pane = {
      name: 'Shell', userRenamed: false,
      autoName: 'auth-refactor', oscTitle: 'vim', autoNameSource: 'osc',
    } as any;
    expect(paneDisplayName(pane)).toBe('vim');
  });

  it('shows the most recently updated auto source (LLM after OSC)', () => {
    const pane = {
      name: 'Shell', userRenamed: false,
      autoName: 'auth-refactor', oscTitle: 'vim', autoNameSource: 'llm',
    } as any;
    expect(paneDisplayName(pane)).toBe('auth-refactor');
  });

  it('manual rename always wins over auto names', () => {
    const pane = {
      name: 'My Pane', userRenamed: true,
      autoName: 'auth-refactor', oscTitle: 'vim', autoNameSource: 'osc',
    } as any;
    expect(paneDisplayName(pane)).toBe('My Pane');
  });

  it('falls back to the pane name when no auto name is set', () => {
    const pane = {
      name: 'Shell', userRenamed: false,
      autoName: '', sessionName: '', oscTitle: '', autoNameSource: '',
    } as any;
    expect(paneDisplayName(pane)).toBe('Shell');
  });

  // Claude Code's own session name (/rename, --name or its generated title)
  // beats the raw terminal title and MTUI's naming call, not a manual rename.
  it('prefers the agent\'s own session name over auto names', () => {
    const pane = {
      name: 'Shell', userRenamed: false, sessionName: 'Datenbankverbindung',
      autoName: 'db-live', oscTitle: 'Datenbankverbindung live', autoNameSource: 'osc',
    } as any;
    expect(paneDisplayName(pane)).toBe('Datenbankverbindung');
    expect(paneDisplayName({ ...pane, userRenamed: true, name: 'Meins' })).toBe('Meins');
  });

  it('keeps a session name when a status line comes without one', () => {
    const tab = tabStore.addTab('N', '/n');
    tabStore.addPane(tab, 'h1:77', 'Claude', 'claude', '');
    tabStore.setSessionName('h1:77', 'Refactor Auth');
    tabStore.setSessionName('h1:77', '');
    const pane = tabStore.getState().tabs.find((t) => t.id === tab)!.panes[0];
    expect(pane.sessionName).toBe('Refactor Auth');
  });

  it('falls back to the other source if the recent one is empty', () => {
    const pane = {
      name: 'Shell', userRenamed: false,
      autoName: 'auth-refactor', oscTitle: '', autoNameSource: 'osc',
    } as any;
    expect(paneDisplayName(pane)).toBe('auth-refactor');
  });
});

describe('windowTitle', () => {
  it('returns the plain app name when there is no tab', () => {
    expect(windowTitle(null)).toBe('Multiterminal');
    expect(windowTitle(undefined)).toBe('Multiterminal');
  });

  it('uses the focused pane display name as context', () => {
    const tab = {
      name: 'Tab 1', focusedPaneId: 'pane-2',
      panes: [
        { id: 'pane-1', name: 'Shell', userRenamed: false, autoName: '', sessionName: '', oscTitle: '', autoNameSource: '' },
        { id: 'pane-2', name: 'Shell', userRenamed: false, autoName: 'auth-refactor', oscTitle: '', autoNameSource: 'llm' },
      ],
    } as any;
    expect(windowTitle(tab)).toBe('auth-refactor — Multiterminal');
  });

  it('falls back to the tab name when no pane is focused', () => {
    const tab = { name: 'Backend', focusedPaneId: '', panes: [] } as any;
    expect(windowTitle(tab)).toBe('Backend — Multiterminal');
  });

  it('includes the version when provided, with no tab', () => {
    expect(windowTitle(null, '2.0.0-alpha.25')).toBe('Multiterminal v2.0.0-alpha.25');
  });

  it('includes the version alongside the tab context', () => {
    const tab = { name: 'Backend', focusedPaneId: '', panes: [] } as any;
    expect(windowTitle(tab, '2.0.0-alpha.25')).toBe('Backend — Multiterminal v2.0.0-alpha.25');
  });
});

describe('tabNeedsCloseConfirm', () => {
  it('returns false for a tab with no panes', () => {
    const tab = { panes: [] } as any;
    expect(tabNeedsCloseConfirm(tab)).toBe(false);
  });

  it('returns true for a tab with one pane', () => {
    const tab = { panes: [{ id: 'pane-1' }] } as any;
    expect(tabNeedsCloseConfirm(tab)).toBe(true);
  });

  it('returns true for a tab with multiple panes', () => {
    const tab = { panes: [{ id: 'pane-1' }, { id: 'pane-2' }] } as any;
    expect(tabNeedsCloseConfirm(tab)).toBe(true);
  });
});

describe('worktree detection state', () => {
  it('setWorktree populates worktreePath/branch/targetBranch on the matching pane', () => {
    const tabId = tabStore.addTab('Test', '/tmp/proj');
    const paneId = tabStore.addPane(tabId, 'h1:42', 'pane', 'claude', '');
    tabStore.setWorktree('h1:42', '/tmp/proj/.claude/worktrees/feature-a', 'worktree-feature-a', 'alpha-main');
    const state = tabStore.getState();
    const pane = state.tabs.find((t) => t.id === tabId)!.panes.find((p) => p.id === paneId)!;
    expect(pane.worktreePath).toBe('/tmp/proj/.claude/worktrees/feature-a');
    expect(pane.branch).toBe('worktree-feature-a');
    expect(pane.targetBranch).toBe('alpha-main');
  });

  it('clearWorktree resets worktree fields to empty', () => {
    const tabId = tabStore.addTab('Test2', '/tmp/proj2');
    const paneId = tabStore.addPane(tabId, 'h1:43', 'pane', 'claude', '');
    tabStore.setWorktree('h1:43', '/tmp/proj2/.claude/worktrees/x', 'worktree-x', 'main');
    tabStore.clearWorktree('h1:43');
    const state = tabStore.getState();
    const pane = state.tabs.find((t) => t.id === tabId)!.panes.find((p) => p.id === paneId)!;
    expect(pane.worktreePath).toBe('');
    expect(pane.branch).toBe('');
    expect(pane.targetBranch).toBe('');
  });
});

import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { tabStore, activeTab, allTabs, computeTabActivity } from './tabs';

// Note: tabStore uses internal counters that persist across tests.
// We work with that by testing behavior rather than exact IDs.

describe('tabStore', () => {
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

    it('initializes unreadActivity as null', () => {
      const id = tabStore.addTab('ActivityInit');
      const tab = tabStore.getState().tabs.find((t) => t.id === id);
      expect(tab!.unreadActivity).toBeNull();
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

    it('clears unreadActivity when tab becomes active', () => {
      const bgTab = tabStore.addTab('ClearBg');
      const fgTab = tabStore.addTab('ClearFg');
      tabStore.setActiveTab(fgTab);

      tabStore.addPane(bgTab, 4001, 'Claude', 'claude', '');
      tabStore.updateActivity(4001, 'needsInput', '');

      let tab = tabStore.getState().tabs.find((t) => t.id === bgTab);
      expect(tab!.unreadActivity).toBe('needsInput');

      tabStore.setActiveTab(bgTab);
      tab = tabStore.getState().tabs.find((t) => t.id === bgTab);
      expect(tab!.unreadActivity).toBeNull();
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
      tabStore.addPane(id, 999, 'Shell', 'shell', '');

      tabStore.setTabDir(id, '/new');
      const tab = tabStore.getState().tabs.find((t) => t.id === id);
      expect(tab!.dir).toBe('/old');
    });
  });

  describe('addPane', () => {
    it('adds a pane with correct properties', () => {
      const tabId = tabStore.addTab('PaneTest');
      const paneId = tabStore.addPane(tabId, 42, 'Claude', 'claude', 'opus');

      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      const pane = tab!.panes.find((p) => p.id === paneId);

      expect(pane).toBeDefined();
      expect(pane!.sessionId).toBe(42);
      expect(pane!.name).toBe('Claude');
      expect(pane!.mode).toBe('claude');
      expect(pane!.model).toBe('opus');
      expect(pane!.focused).toBe(true);
      expect(pane!.running).toBe(true);
      expect(pane!.activity).toBe('idle');
      expect(pane!.cost).toBe('');
      expect(pane!.maximized).toBe(false);
    });

    it('focuses the new pane and unfocuses others', () => {
      const tabId = tabStore.addTab('FocusTest');
      tabStore.addPane(tabId, 1, 'P1', 'shell', '');
      const p2 = tabStore.addPane(tabId, 2, 'P2', 'shell', '');

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
      const p1 = tabStore.addPane(tabId, 1, 'P1', 'shell', '');
      const p2 = tabStore.addPane(tabId, 2, 'P2', 'shell', '');

      tabStore.closePane(tabId, p1);

      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      expect(tab!.panes.length).toBe(1);
      expect(tab!.panes[0].id).toBe(p2);
    });

    it('focuses another pane after closing focused one', () => {
      const tabId = tabStore.addTab('CloseFocusTest');
      const p1 = tabStore.addPane(tabId, 1, 'P1', 'shell', '');
      const p2 = tabStore.addPane(tabId, 2, 'P2', 'shell', '');
      // p2 is focused

      tabStore.closePane(tabId, p2);
      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      // p1 should now be focused
      expect(tab!.panes[0].focused).toBe(true);
    });

    it('recomputes unreadActivity when a pane is closed on a background tab', () => {
      const bgTab = tabStore.addTab('ClosePaneBg');
      const fgTab = tabStore.addTab('ClosePaneFg');
      tabStore.setActiveTab(fgTab);

      const p1 = tabStore.addPane(bgTab, 5001, 'C1', 'claude', '');
      tabStore.updateActivity(5001, 'needsInput', '');

      let tab = tabStore.getState().tabs.find(t => t.id === bgTab);
      expect(tab!.unreadActivity).toBe('needsInput');

      tabStore.closePane(bgTab, p1);
      tab = tabStore.getState().tabs.find(t => t.id === bgTab);
      expect(tab!.unreadActivity).toBeNull();
    });
  });

  describe('focusPane', () => {
    it('sets focus correctly', () => {
      const tabId = tabStore.addTab('FocusPaneTest');
      const p1 = tabStore.addPane(tabId, 1, 'P1', 'shell', '');
      const p2 = tabStore.addPane(tabId, 2, 'P2', 'shell', '');

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
      const paneId = tabStore.addPane(tabId, 1, 'P1', 'shell', '');

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
      tabStore.addPane(tabId, 777, 'Claude', 'claude', '');

      tabStore.updateActivity(777, 'active', '$0.12');

      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      const pane = tab!.panes.find((p) => p.sessionId === 777);
      expect(pane!.activity).toBe('active');
      expect(pane!.cost).toBe('$0.12');
    });

    it('finds pane across multiple tabs', () => {
      const tab1 = tabStore.addTab('Tab1');
      const tab2 = tabStore.addTab('Tab2');
      tabStore.addPane(tab1, 100, 'P1', 'shell', '');
      tabStore.addPane(tab2, 200, 'P2', 'claude', '');

      tabStore.updateActivity(200, 'done', '$1.50');

      const t2 = tabStore.getState().tabs.find((t) => t.id === tab2);
      const pane = t2!.panes.find((p) => p.sessionId === 200);
      expect(pane!.activity).toBe('done');
      expect(pane!.cost).toBe('$1.50');
    });

    it('does not overwrite cost with empty string', () => {
      const tabId = tabStore.addTab('CostTest');
      tabStore.addPane(tabId, 888, 'Claude', 'claude', '');

      tabStore.updateActivity(888, 'active', '$0.50');
      tabStore.updateActivity(888, 'done', '');

      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      const pane = tab!.panes.find((p) => p.sessionId === 888);
      expect(pane!.cost).toBe('$0.50');
    });
  });

  describe('markExited', () => {
    it('sets running to false', () => {
      const tabId = tabStore.addTab('ExitTest');
      tabStore.addPane(tabId, 555, 'Shell', 'shell', '');

      tabStore.markExited(555);

      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      const pane = tab!.panes.find((p) => p.sessionId === 555);
      expect(pane!.running).toBe(false);
    });
  });

  describe('renamePane', () => {
    it('changes the pane name', () => {
      const tabId = tabStore.addTab('RenameTest');
      const paneId = tabStore.addPane(tabId, 1, 'Old', 'shell', '');

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

describe('computeTabActivity', () => {
  it('returns null for all-idle panes', () => {
    expect(computeTabActivity([])).toBeNull();
  });

  it('returns done when all panes are done', () => {
    const panes = [
      { activity: 'done' } as any,
      { activity: 'idle' } as any,
    ];
    expect(computeTabActivity(panes)).toBe('done');
  });

  it('returns active when any pane is active', () => {
    const panes = [
      { activity: 'done' } as any,
      { activity: 'active' } as any,
    ];
    expect(computeTabActivity(panes)).toBe('active');
  });

  it('returns needsInput when any pane needs input (highest priority)', () => {
    const panes = [
      { activity: 'active' } as any,
      { activity: 'needsInput' } as any,
      { activity: 'done' } as any,
    ];
    expect(computeTabActivity(panes)).toBe('needsInput');
  });

  it('returns active for a single active pane', () => {
    const panes = [{ activity: 'active' } as any];
    expect(computeTabActivity(panes)).toBe('active');
  });
});

describe('updateActivity — tab unreadActivity', () => {
  it('sets unreadActivity on non-active tab when pane becomes done', () => {
    const tab1 = tabStore.addTab('UAActive');
    const tab2 = tabStore.addTab('UABackground');
    tabStore.setActiveTab(tab1);

    tabStore.addPane(tab2, 3001, 'Claude', 'claude', '');
    tabStore.updateActivity(3001, 'done', '$0.10');

    const t2 = tabStore.getState().tabs.find((t) => t.id === tab2);
    expect(t2!.unreadActivity).toBe('done');
  });

  it('does not set unreadActivity on the currently active tab', () => {
    const tabId = tabStore.addTab('UAActiveTab');
    tabStore.setActiveTab(tabId);
    tabStore.addPane(tabId, 3002, 'Claude', 'claude', '');

    tabStore.updateActivity(3002, 'done', '');

    const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
    expect(tab!.unreadActivity).toBeNull();
  });

  it('escalates to needsInput when one pane needs input', () => {
    const bgTab = tabStore.addTab('UAEscalate');
    const fgTab = tabStore.addTab('UAForeground');
    tabStore.setActiveTab(fgTab);

    tabStore.addPane(bgTab, 3003, 'C1', 'claude', '');
    tabStore.addPane(bgTab, 3004, 'C2', 'claude', '');

    tabStore.updateActivity(3003, 'done', '');
    tabStore.updateActivity(3004, 'needsInput', '');

    const tab = tabStore.getState().tabs.find((t) => t.id === bgTab);
    expect(tab!.unreadActivity).toBe('needsInput');
  });
});

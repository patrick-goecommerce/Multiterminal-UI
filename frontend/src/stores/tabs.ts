import { writable, derived, get } from 'svelte/store';

export type PaneMode = 'shell' | 'claude' | 'claude-yolo';

export interface Pane {
  id: string;
  sessionId: number;
  name: string;
  mode: PaneMode;
  model: string;
  focused: boolean;
  activity: 'idle' | 'active' | 'done' | 'needsInput';
  cost: string;
  running: boolean;
  maximized: boolean;
}

export interface Tab {
  id: string;
  name: string;
  dir: string;
  panes: Pane[];
  focusedPaneId: string;
}

function createTabStore() {
  const { subscribe, update, set } = writable<{
    tabs: Tab[];
    activeTabId: string;
  }>({
    tabs: [],
    activeTabId: '',
  });

  let nextTabNum = 1;
  let nextPaneNum = 1;

  return {
    subscribe,

    addTab(name?: string, dir?: string) {
      const id = `tab-${nextTabNum++}`;
      const tabName = name || `Tab ${nextTabNum - 1}`;
      update((state) => {
        state.tabs.push({
          id,
          name: tabName,
          dir: dir || '',
          panes: [],
          focusedPaneId: '',
        });
        state.activeTabId = id;
        return state;
      });
      return id;
    },

    closeTab(tabId: string) {
      update((state) => {
        if (state.tabs.length <= 1) return state;
        const idx = state.tabs.findIndex((t) => t.id === tabId);
        if (idx === -1) return state;
        state.tabs.splice(idx, 1);
        if (state.activeTabId === tabId) {
          state.activeTabId = state.tabs[Math.min(idx, state.tabs.length - 1)].id;
        }
        return state;
      });
    },

    setActiveTab(tabId: string) {
      update((state) => {
        state.activeTabId = tabId;
        return state;
      });
    },

    renameTab(tabId: string, name: string) {
      update((state) => {
        const tab = state.tabs.find((t) => t.id === tabId);
        if (tab) tab.name = name;
        return state;
      });
    },

    setTabDir(tabId: string, dir: string) {
      update((state) => {
        const tab = state.tabs.find((t) => t.id === tabId);
        if (tab && tab.panes.length === 0) {
          tab.dir = dir;
        }
        return state;
      });
    },

    renamePane(tabId: string, paneId: string, name: string) {
      update((state) => {
        const tab = state.tabs.find((t) => t.id === tabId);
        if (!tab) return state;
        const pane = tab.panes.find((p) => p.id === paneId);
        if (pane) pane.name = name;
        return state;
      });
    },

    addPane(tabId: string, sessionId: number, name: string, mode: PaneMode, model: string): string {
      const paneId = `pane-${nextPaneNum++}`;
      update((state) => {
        const tab = state.tabs.find((t) => t.id === tabId);
        if (!tab) return state;
        // Unfocus all existing panes
        tab.panes.forEach((p) => (p.focused = false));
        tab.panes.push({
          id: paneId,
          sessionId,
          name,
          mode,
          model,
          focused: true,
          activity: 'idle',
          cost: '',
          running: true,
          maximized: false,
        });
        tab.focusedPaneId = paneId;
        return state;
      });
      return paneId;
    },

    closePane(tabId: string, paneId: string) {
      update((state) => {
        const tab = state.tabs.find((t) => t.id === tabId);
        if (!tab) return state;
        const idx = tab.panes.findIndex((p) => p.id === paneId);
        if (idx === -1) return state;
        tab.panes.splice(idx, 1);
        if (tab.focusedPaneId === paneId && tab.panes.length > 0) {
          const newIdx = Math.min(idx, tab.panes.length - 1);
          tab.panes[newIdx].focused = true;
          tab.focusedPaneId = tab.panes[newIdx].id;
        }
        return state;
      });
    },

    focusPane(tabId: string, paneId: string) {
      update((state) => {
        const tab = state.tabs.find((t) => t.id === tabId);
        if (!tab) return state;
        tab.panes.forEach((p) => (p.focused = p.id === paneId));
        tab.focusedPaneId = paneId;
        return state;
      });
    },

    toggleMaximize(tabId: string, paneId: string) {
      update((state) => {
        const tab = state.tabs.find((t) => t.id === tabId);
        if (!tab) return state;
        const pane = tab.panes.find((p) => p.id === paneId);
        if (pane) pane.maximized = !pane.maximized;
        return state;
      });
    },

    updateActivity(sessionId: number, activity: string, cost: string) {
      update((state) => {
        for (const tab of state.tabs) {
          for (const pane of tab.panes) {
            if (pane.sessionId === sessionId) {
              pane.activity = activity as Pane['activity'];
              if (cost) pane.cost = cost;
              return state;
            }
          }
        }
        return state;
      });
    },

    markExited(sessionId: number) {
      update((state) => {
        for (const tab of state.tabs) {
          for (const pane of tab.panes) {
            if (pane.sessionId === sessionId) {
              pane.running = false;
              return state;
            }
          }
        }
        return state;
      });
    },

    getActiveTab(): Tab | undefined {
      const state = get({ subscribe });
      return state.tabs.find((t) => t.id === state.activeTabId);
    },

    getState() {
      return get({ subscribe });
    },
  };
}

export const tabStore = createTabStore();

export const activeTab = derived(tabStore, ($state) =>
  $state.tabs.find((t) => t.id === $state.activeTabId)
);

export const allTabs = derived(tabStore, ($state) => $state.tabs);

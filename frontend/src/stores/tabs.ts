import { writable, derived, get } from 'svelte/store';

export type PaneMode = 'shell' | 'claude' | 'claude-auto' | 'claude-yolo' | 'codex' | 'codex-auto' | 'gemini' | 'gemini-yolo';

export interface Pane {
  id: string;
  sessionId: number;
  name: string;
  mode: PaneMode;
  model: string;
  focused: boolean;
  activity: 'starting' | 'idle' | 'active' | 'done' | 'waitingPermission' | 'waitingAnswer' | 'error';
  cost: string;
  running: boolean;
  maximized: boolean;
  issueNumber: number | null;
  issueTitle: string;
  issueBranch: string;
  worktreePath: string;
  branch: string;
  /** Merge-back target branch for a pane worktree (empty when not a worktree pane). */
  targetBranch: string;
  zoomDelta: number;
  background: boolean;
  display: 'terminal' | 'chat';
  conversationId: string;
  /** Claude session id pinned at launch (--session-id), used to resume on terminal⇄chat toggle. Empty for shell/codex/gemini. */
  claudeSessionId: string;
  /** LLM-generated pane name (from the user's prompt). */
  autoName: string;
  /** OSC-derived window title from the PTY. */
  oscTitle: string;
  /** Which auto source was updated most recently — drives display (most-recent-wins). */
  autoNameSource: '' | 'llm' | 'osc';
  /** True once the user manually renamed the pane — suppresses all auto names. */
  userRenamed: boolean;
}

/** Resolve the name shown in a pane titlebar.
 *  Priority: manual rename > most-recently-updated auto source > pane name.
 *  If the most-recent auto source happens to be empty, fall back to the other. */
export function paneDisplayName(pane: Pane): string {
  if (pane.userRenamed) return pane.name;
  const recent = pane.autoNameSource === 'llm' ? pane.autoName
    : pane.autoNameSource === 'osc' ? pane.oscTitle
    : '';
  return recent || pane.autoName || pane.oscTitle || pane.name;
}

/** Native OS window title for the given active tab. Reflects the focused pane
 *  (so multi-window setups are distinguishable), falling back to the tab name. */
export function windowTitle(tab: Tab | null | undefined): string {
  const base = 'Multiterminal';
  if (!tab) return base;
  const focused = tab.panes.find((p) => p.id === tab.focusedPaneId);
  const ctx = focused ? paneDisplayName(focused) : tab.name;
  return ctx ? `${ctx} — ${base}` : base;
}

export interface Tab {
  id: string;
  name: string;
  dir: string;
  panes: Pane[];
  focusedPaneId: string;
  _highlight?: boolean;
  unreadActivity: 'waitingPermission' | 'waitingAnswer' | 'error' | 'active' | 'done' | null;
}

export function computeTabActivity(panes: Pane[]): Tab['unreadActivity'] {
  let result: Tab['unreadActivity'] = null;
  for (const pane of panes) {
    if (pane.activity === 'waitingPermission') return 'waitingPermission';
    if (pane.activity === 'waitingAnswer') result = 'waitingAnswer';
    else if (pane.activity === 'error' && result !== 'waitingAnswer') result = 'error';
    else if (pane.activity === 'active' && result !== 'error') result = 'active';
    else if (pane.activity === 'done' && result === null) result = 'done';
  }
  return result;
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

  // Activity smoothing: Claude Code's animated TUI (spinner, cursor blink,
  // status bar) emits bursty output, so the backend detector flips between
  // "active" and "done"/"idle" every couple of seconds. We debounce only the
  // transitions INTO the calm states so the badge/border doesn't flicker;
  // attention states (waiting/error) and "active" apply immediately.
  const calmDebounce = new Map<number, ReturnType<typeof setTimeout>>();
  const CALM_DELAY_MS = 900;

  function applyActivity(sessionId: number, activity: string, cost: string) {
    update((state) => {
      for (const tab of state.tabs) {
        for (const pane of tab.panes) {
          if (pane.sessionId === sessionId) {
            if (activity) pane.activity = activity as Pane['activity'];
            if (cost) pane.cost = cost;
            if (tab.id !== state.activeTabId) {
              tab.unreadActivity = computeTabActivity(tab.panes);
            }
            return state;
          }
        }
      }
      return state;
    });
  }

  return {
    subscribe,

    addTab(name?: string, dir?: string, setActive: boolean = true) {
      const id = `tab-${nextTabNum++}`;
      const tabName = name || `Tab ${nextTabNum - 1}`;
      update((state) => {
        state.tabs.push({
          id,
          name: tabName,
          dir: dir || '',
          panes: [],
          focusedPaneId: '',
          unreadActivity: null,
        });
        if (setActive) state.activeTabId = id;
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
        const tab = state.tabs.find((t) => t.id === tabId);
        if (tab) tab.unreadActivity = null;
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
        if (pane) {
          pane.name = name;
          pane.userRenamed = true; // a manual rename wins over any auto name
        }
        return state;
      });
    },

    /** Apply an auto-generated name (by session id). Ignored once the user
     *  manually renamed the pane. The most recently applied source wins on
     *  display (see paneDisplayName). source 'llm' sets autoName, 'osc' sets oscTitle. */
    setAutoName(sessionId: number, value: string, source: 'llm' | 'osc') {
      update((state) => {
        for (const tab of state.tabs) {
          const pane = tab.panes.find((p) => p.sessionId === sessionId);
          if (!pane || pane.userRenamed) continue;
          if (source === 'llm') pane.autoName = value;
          else pane.oscTitle = value;
          pane.autoNameSource = source;
        }
        return state;
      });
    },

    addPane(tabId: string, sessionId: number, name: string, mode: PaneMode, model: string, issueNumber?: number | null, issueTitle?: string, issueBranch?: string, worktreePath?: string, branch?: string, targetBranch?: string, background?: boolean, display: 'terminal' | 'chat' = 'terminal', conversationId = '', claudeSessionId = ''): string {
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
          activity: 'starting',
          cost: '',
          running: true,
          maximized: false,
          issueNumber: issueNumber ?? null,
          issueTitle: issueTitle ?? '',
          issueBranch: issueBranch ?? '',
          worktreePath: worktreePath ?? '',
          branch: branch ?? issueBranch ?? '',
          targetBranch: targetBranch ?? '',
          zoomDelta: 0,
          background: background ?? false,
          display,
          conversationId,
          claudeSessionId,
          autoName: '',
          oscTitle: '',
          autoNameSource: '',
          userRenamed: false,
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
        if (tab.id !== state.activeTabId) {
          tab.unreadActivity = computeTabActivity(tab.panes);
        }
        if (tab.focusedPaneId === paneId && tab.panes.length > 0) {
          const newIdx = Math.min(idx, tab.panes.length - 1);
          tab.panes.forEach((p) => (p.focused = false));
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

    setZoomDelta(tabId: string, paneId: string, delta: number) {
      update((state) => {
        const tab = state.tabs.find((t) => t.id === tabId);
        if (!tab) return state;
        const pane = tab.panes.find((p) => p.id === paneId);
        if (pane) pane.zoomDelta = delta;
        return state;
      });
    },

    updateActivity(sessionId: number, activity: string, cost: string) {
      // Any incoming update cancels a pending calm transition.
      const pending = calmDebounce.get(sessionId);
      if (pending) { clearTimeout(pending); calmDebounce.delete(sessionId); }

      // "active" is real-time truth (output is flowing) — apply at once.
      // The states classified after a quiet pause (done/idle/waiting*) are
      // deferred: a redraw of Claude's TUI quickly flips back to "active" and
      // cancels the pending change, so the badge reads calmly as "läuft" while
      // work continues and only settles once a state truly holds.
      const isCalm = activity === 'done' || activity === 'idle'
        || activity === 'waitingPermission' || activity === 'waitingAnswer';
      if (isCalm) {
        // Cost still updates immediately so the title bar number stays live.
        if (cost) applyActivity(sessionId, '', cost);
        const timer = setTimeout(() => {
          calmDebounce.delete(sessionId);
          applyActivity(sessionId, activity, '');
        }, CALM_DELAY_MS);
        calmDebounce.set(sessionId, timer);
        return;
      }
      applyActivity(sessionId, activity, cost);
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

    forceCloseTab(tabId: string) {
      update((state) => {
        const tabs = state.tabs.filter(t => t.id !== tabId);
        const activeTabId = state.activeTabId === tabId
          ? (tabs.length > 0 ? tabs[tabs.length - 1].id : '')
          : state.activeTabId;
        return { ...state, tabs, activeTabId };
      });
    },

    importTab(tab: Tab) {
      update((s) => ({
        ...s,
        tabs: [...s.tabs, { ...tab, _highlight: true, unreadActivity: null }],
        activeTabId: tab.id,
      }));
      setTimeout(() => {
        update((s) => ({
          ...s,
          tabs: s.tabs.map((t) => (t.id === tab.id ? { ...t, _highlight: false } : t)),
        }));
      }, 2000);
    },
  };
}

export const tabStore = createTabStore();

export const activeTab = derived(tabStore, ($state) =>
  $state.tabs.find((t) => t.id === $state.activeTabId)
);

export const allTabs = derived(tabStore, ($state) => $state.tabs);

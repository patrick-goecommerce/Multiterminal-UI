import { writable, derived } from 'svelte/store';

/** Navigation items for the left nav pane */
export type NavItem =
  | 'terminals'      // Default: Terminal panes
  | 'dashboard'      // Cross-project overview
  | 'kanban'         // Kanban board with planning + automation
  | 'chat'           // Chat view
  | 'queue';         // Queue overview

/** Sidebar views (open as side panel next to terminals) */
export type SidebarView = 'explorer' | 'source-control' | 'issues';

export interface WorkspaceState {
  /** Which main content view is active */
  activeView: NavItem;
  /** Whether the left nav is collapsed (icons only) */
  leftNavCollapsed: boolean;
  /** Which sidebar view is open (null = closed) — only applies when activeView === 'terminals' */
  sidebarView: SidebarView | null;
  /** Whether sidebar is pinned open */
  sidebarPinned: boolean;
}

const initialState: WorkspaceState = {
  activeView: 'terminals',
  leftNavCollapsed: false,
  sidebarView: null,
  sidebarPinned: false,
};

function createWorkspaceStore() {
  const { subscribe, set, update } = writable<WorkspaceState>(initialState);

  return {
    subscribe,
    set,
    update,

    /** Navigate to a main content view */
    setView(view: NavItem) {
      update(s => ({ ...s, activeView: view }));
    },

    /** Toggle or set sidebar view (only relevant when activeView is 'terminals') */
    toggleSidebar(view?: SidebarView) {
      update(s => {
        // If clicking the same sidebar view, close it (unless pinned)
        if (view && s.sidebarView === view) {
          if (s.sidebarPinned) return s;
          return { ...s, sidebarView: null };
        }
        // Open the requested view and switch to terminals
        return {
          ...s,
          activeView: 'terminals',
          sidebarView: view || (s.sidebarView ? null : 'explorer'),
        };
      });
    },

    /** Open a sidebar view and switch to terminals */
    openSidebar(view: SidebarView) {
      update(s => ({
        ...s,
        activeView: 'terminals',
        sidebarView: view,
      }));
    },

    /** Close sidebar */
    closeSidebar() {
      update(s => {
        if (s.sidebarPinned) return s;
        return { ...s, sidebarView: null };
      });
    },

    /** Toggle left nav collapsed state */
    toggleCollapsed() {
      update(s => ({ ...s, leftNavCollapsed: !s.leftNavCollapsed }));
    },

    /** Set sidebar pinned state */
    setSidebarPinned(pinned: boolean) {
      update(s => ({ ...s, sidebarPinned: pinned }));
    },
  };
}

export const workspace = createWorkspaceStore();

/** Derived: is the current view a main content view (replaces pane grid) */
export const isMainContentView = derived(workspace, $w =>
  $w.activeView !== 'terminals'
);

/** Derived: should the sidebar be visible */
export const showSidePanel = derived(workspace, $w =>
  $w.activeView === 'terminals' && $w.sidebarView !== null
);

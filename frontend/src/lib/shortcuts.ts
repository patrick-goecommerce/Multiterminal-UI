export interface ShortcutCallbacks {
  onNewPane: () => void;
  onNewTab: () => void;
  onCloseTab: () => void;
  onToggleSidebar: () => void;
  onToggleMaximize: () => void;
  onFocusPane: (index: number) => void;
  onOpenIssues: () => void;
  canAddPane: () => boolean;
}

/** Create a global keydown handler for the application shortcuts. */
export function createGlobalKeyHandler(cb: ShortcutCallbacks): (e: KeyboardEvent) => void {
  return (e: KeyboardEvent) => {
    if (!e.ctrlKey) return;

    switch (e.key) {
      case 'n':
        e.preventDefault();
        if (cb.canAddPane()) cb.onNewPane();
        return;
      case 't':
        e.preventDefault();
        cb.onNewTab();
        return;
      case 'w':
        e.preventDefault();
        cb.onCloseTab();
        return;
      case 'b':
        e.preventDefault();
        cb.onToggleSidebar();
        return;
      case 'z':
        e.preventDefault();
        cb.onToggleMaximize();
        return;
      case 'i':
        e.preventDefault();
        cb.onOpenIssues();
        return;
      case 'f':
        return; // let terminal pane handle search
    }

    // Ctrl+1-9 → focus pane by index
    if (e.key >= '1' && e.key <= '9') {
      e.preventDefault();
      cb.onFocusPane(parseInt(e.key) - 1);
    }
  };
}

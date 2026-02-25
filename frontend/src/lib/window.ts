// frontend/src/lib/window.ts
// Helpers for multi-window identity. Each Wails window loads the same
// frontend assets but with different URL query params.

/** Returns the windowId for this window instance (e.g. "main", "win-1"). */
export function getWindowId(): string {
  const params = new URLSearchParams(window.location.search);
  return params.get('windowId') ?? 'main';
}

/** Returns true if this is the main application window. */
export function isMainWindow(): boolean {
  return getWindowId() === 'main';
}

/** Returns the initial tab IDs this window should display.
 *  Empty array = main window (loads all tabs from session restore).
 */
export function getInitialTabs(): string[] {
  const params = new URLSearchParams(window.location.search);
  const tabs = params.get('tabs');
  return tabs ? tabs.split(',').filter(Boolean) : [];
}

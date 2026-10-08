import { get, writable } from 'svelte/store';
import { tabStore, allTabs, activeTab } from '../stores/tabs';
import { floatingPane, focusOrders, setFocusOrder } from '../stores/focusLayout';
import { promoteInOrder, reconcileOrder, type LayoutMode } from './focusLayout';
import { attentionList, nextEntry, cursorOf, type AttentionEntry, type AttentionCursor } from './attention';

// Moving through what waits: a click in the panel, Ctrl+Shift+J and
// Ctrl+Shift+K, a toast click. Module state on purpose: there is one walk per
// window, and the panel, the toolbar counter and the shortcuts share it.

let returnTo: { tabId: string; paneId: string } | null = null;
let cursor: AttentionCursor | null = null;

/** The pane the keyboard walk is on, for the panel's highlight; '' = none. */
export const walkPaneId = writable('');

function remember() {
  if (returnTo) return;
  const tab = get(activeTab);
  if (tab) returnTo = { tabId: tab.id, paneId: tab.focusedPaneId };
}

// In the focus layout a focused pane also moves to a big slot.
function focusIn(tabId: string, paneId: string) {
  const tab = get(allTabs).find((t) => t.id === tabId);
  if (tab) setFocusOrder(tabId, promoteInOrder(reconcileOrder(get(focusOrders)[tabId], tab.panes.map((p) => p.id)), paneId, true));
  tabStore.focusPane(tabId, paneId);
}

/** Switch to the pane's tab and give it the keyboard. */
export function jumpTo(tabId: string, paneId: string) {
  remember();
  floatingPane.set(null);
  if (get(activeTab)?.id !== tabId) tabStore.setActiveTab(tabId);
  focusIn(tabId, paneId);
}

/** Show the entry floating over the current tab. An entry of the current tab
 *  has its own slot, and the grid has no floating window: both are focused
 *  instead. */
export function preview(entry: AttentionEntry, mode: LayoutMode) {
  if (mode !== 'focus' || entry.here) {
    jumpTo(entry.tabId, entry.pane.id);
    return;
  }
  remember();
  floatingPane.set({ tabId: entry.tabId, paneId: entry.pane.id });
  tabStore.focusPane(entry.tabId, entry.pane.id);
}

/** Ctrl+Shift+J: the next entry in rank order. */
export function next(mode: LayoutMode): AttentionEntry | null {
  const e = nextEntry(attentionList(get(allTabs), get(activeTab)?.id ?? ''), cursor);
  if (!e) return null;
  cursor = cursorOf(e);
  walkPaneId.set(e.pane.id);
  preview(e, mode);
  return e;
}

/** Ctrl+Shift+K: back to the pane the user was in before the walk. */
export function back() {
  const t = returnTo;
  returnTo = null;
  cursor = null;
  walkPaneId.set('');
  floatingPane.set(null);
  if (!t) return;
  if (get(activeTab)?.id !== t.tabId) tabStore.setActiveTab(t.tabId);
  if (t.paneId) focusIn(t.tabId, t.paneId);
}

/** Jump to the pane with this session, from a toast click. */
export function jumpToSession(ref: string) {
  const tab = get(allTabs).find((t) => t.panes.some((p) => p.sessionId === ref));
  const pane = tab?.panes.find((p) => p.sessionId === ref);
  if (tab && pane) jumpTo(tab.id, pane.id);
}

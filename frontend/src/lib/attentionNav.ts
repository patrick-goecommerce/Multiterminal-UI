import { get, writable } from 'svelte/store';
import { tabStore, allTabs, activeTab } from '../stores/tabs';
import { floatingPane, focusOrders, setFocusOrder } from '../stores/focusLayout';
import { promoteInOrder, reconcileOrder, type LayoutMode } from './focusLayout';
import { attentionList, nextEntry, cursorOf, type AttentionEntry, type AttentionCursor } from './attention';

// Moving through what waits: a click in the panel, Ctrl+Shift+J and
// Ctrl+Shift+K, a toast click. Module state on purpose: there is one walk per
// window, and the panel, the toolbar counter and the shortcuts share it.

interface PaneRef { tabId: string; paneId: string }

let returnTo: PaneRef | null = null;
/** Where the last jump or preview put the user. */
let landedOn: PaneRef | null = null;
let cursor: AttentionCursor | null = null;

/** The pane the keyboard walk is on, for the panel's highlight; '' = none. */
export const walkPaneId = writable('');

/** The user is still where the walk put them: the preview is open, or the
 *  pane it jumped to still has the focus. Anything else means they moved on
 *  by hand, and the way back is stale. */
function stillThere(): boolean {
  if (get(floatingPane)) return true;
  const tab = get(activeTab);
  return !!landedOn && !!tab && tab.id === landedOn.tabId && tab.focusedPaneId === landedOn.paneId;
}

function remember() {
  if (returnTo && stillThere()) return;
  const tab = get(activeTab);
  returnTo = tab ? { tabId: tab.id, paneId: tab.focusedPaneId } : null;
}

function endWalk() {
  cursor = null;
  walkPaneId.set('');
}

// Writes the tab's focus order in both layouts. The grid never reads it, and
// in the focus layout it does what a click on the pane would: a big pane
// stays where it is, a small one moves up to the left.
function focusIn(tabId: string, paneId: string) {
  const tab = get(allTabs).find((t) => t.id === tabId);
  if (tab) setFocusOrder(tabId, promoteInOrder(reconcileOrder(get(focusOrders)[tabId], tab.panes.map((p) => p.id)), paneId, false));
  tabStore.focusPane(tabId, paneId);
}

function goTo(tabId: string, paneId: string) {
  remember();
  floatingPane.set(null);
  if (get(activeTab)?.id !== tabId) tabStore.setActiveTab(tabId);
  focusIn(tabId, paneId);
  landedOn = { tabId, paneId };
}

function show(entry: AttentionEntry, mode: LayoutMode) {
  if (mode !== 'focus' || entry.here) {
    goTo(entry.tabId, entry.pane.id);
    return;
  }
  remember();
  floatingPane.set({ tabId: entry.tabId, paneId: entry.pane.id });
  tabStore.focusPane(entry.tabId, entry.pane.id);
  landedOn = { tabId: entry.tabId, paneId: entry.pane.id };
}

/** Switch to the pane's tab and give it the keyboard. A click, not the
 *  keyboard walk: the walk's position and highlight end here. */
export function jumpTo(tabId: string, paneId: string) {
  endWalk();
  goTo(tabId, paneId);
}

/** Show the entry floating over the current tab. An entry of the current tab
 *  has its own slot, and the grid has no floating window: both are focused
 *  instead. Like jumpTo, it ends the keyboard walk. */
export function preview(entry: AttentionEntry, mode: LayoutMode) {
  endWalk();
  show(entry, mode);
}

/** Ctrl+Shift+J: the next entry in rank order. */
export function next(mode: LayoutMode): AttentionEntry | null {
  const e = nextEntry(attentionList(get(allTabs), get(activeTab)?.id ?? ''), cursor);
  if (!e) return null;
  cursor = cursorOf(e);
  walkPaneId.set(e.pane.id);
  show(e, mode);
  return e;
}

/** Ctrl+Shift+K: back to the pane the user was in before the walk, as long
 *  as they are still where it put them. Otherwise it only forgets the walk. */
export function back() {
  const t = returnTo;
  const there = stillThere();
  returnTo = null;
  landedOn = null;
  endWalk();
  if (!t || !there) return;
  floatingPane.set(null);
  if (get(activeTab)?.id !== t.tabId) tabStore.setActiveTab(t.tabId);
  if (t.paneId) focusIn(t.tabId, t.paneId);
}

/** Jump to the pane with this session, from a toast click. */
export function jumpToSession(ref: string) {
  const tab = get(allTabs).find((t) => t.panes.some((p) => p.sessionId === ref));
  const pane = tab?.panes.find((p) => p.sessionId === ref);
  if (tab && pane) jumpTo(tab.id, pane.id);
}

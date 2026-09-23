// State of the focus layout (see lib/focusLayout.ts). The order is saved with
// the session (SavedTab.focus_order), so a restart keeps the arrangement; a
// floating window is something you open for one answer and is not saved.
import { writable, get } from 'svelte/store';
import * as App from '../../wailsjs/go/backend/App';
import { config } from './config';
import { tabStore, type Pane } from './tabs';
import { reconcileOrder, type FloatingRef } from '../lib/focusLayout';

/** Per tab: pane ids, most recently used first. Index 0 and 1 are big. */
export const focusOrders = writable<Record<string, string[]>>({});

export function setFocusOrder(tabId: string, order: string[]) {
  focusOrders.update((all) => ({ ...all, [tabId]: order }));
}

function paneIds(tabId: string): string[] {
  return tabStore.getState().tabs.find((t) => t.id === tabId)?.panes.map((p) => p.id) ?? [];
}

/** The pane's position in the tab's order right now, -1 if it has none. */
export function slotIndexOf(tabId: string, paneId: string): number {
  return reconcileOrder(get(focusOrders)[tabId], paneIds(tabId)).indexOf(paneId);
}

// Panes the app put somewhere on purpose. PaneGrid treats them as known, so
// they do not get the "newest pane goes left" treatment when focused.
const placed = new Set<string>();

/** Whether the app placed this pane itself; answers true once. */
export function consumePlaced(paneId: string): boolean {
  return placed.delete(paneId);
}

/** Put a pane at `index` of its tab's order (clamped). Used when a pane is
 *  replaced by a new one (restart, terminal/chat toggle, worktree finish), so
 *  the replacement takes the old one's place, and for panes an agent opened,
 *  which go to the small column instead of pushing the user's work aside. */
export function placePane(tabId: string, paneId: string, index: number) {
  if (index < 0) return;
  const rest = reconcileOrder(get(focusOrders)[tabId], paneIds(tabId)).filter((id) => id !== paneId);
  const at = Math.min(index, rest.length);
  setFocusOrder(tabId, [...rest.slice(0, at), paneId, ...rest.slice(at)]);
  placed.add(paneId);
}

/** The order as indices into `panes`, for the session file. Undefined when
 *  the tab never had an order, so a file written in grid mode stays as it was. */
export function focusOrderIndices(tabId: string, panes: Pane[]): number[] | undefined {
  const stored = get(focusOrders)[tabId];
  if (!stored) return undefined;
  const index = new Map(panes.map((p, i) => [p.id, i]));
  return reconcileOrder(stored, panes.map((p) => p.id)).map((id) => index.get(id)!);
}

/** Rebuild a tab's order from the saved indices. `restoredIds[i]` is the pane
 *  created for saved pane i, undefined where the restore skipped one. */
export function restoreFocusOrder(tabId: string, saved: number[] | undefined, restoredIds: (string | undefined)[]) {
  if (!saved || saved.length === 0) return;
  const order = saved.map((i) => restoredIds[i]).filter((id): id is string => !!id);
  if (order.length > 0) setFocusOrder(tabId, order);
}

/** The pane of another tab that is open as a floating window, if any. */
export const floatingPane = writable<FloatingRef | null>(null);

/** Remember where the floating window was dragged to. Saved with the config,
 *  so the next one opens in the same place, also after a restart. */
export async function saveFloatPosition(x: number, y: number) {
  config.update((c) => ({ ...c, layout: { ...c.layout, float_x: Math.round(x), float_y: Math.round(y) } }));
  try {
    await App.SaveConfig(get(config));
  } catch (err) {
    console.error('[saveFloatPosition]', err);
  }
}

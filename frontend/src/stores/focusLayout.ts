// State of the focus layout (see lib/focusLayout.ts). Kept in memory only:
// the order is a view preference that rebuilds itself from use, and a floating
// window is something you open for one answer.
import { writable, get } from 'svelte/store';
import * as App from '../../wailsjs/go/backend/App';
import { config } from './config';
import type { FloatingRef } from '../lib/focusLayout';

/** Per tab: pane ids, most recently used first. Index 0 and 1 are big. */
export const focusOrders = writable<Record<string, string[]>>({});

export function setFocusOrder(tabId: string, order: string[]) {
  focusOrders.update((all) => ({ ...all, [tabId]: order }));
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

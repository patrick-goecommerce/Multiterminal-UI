import { describe, it, expect, beforeEach, vi } from 'vitest';
import { get } from 'svelte/store';
import { tabStore } from './tabs';
import {
  focusOrders, placePane, slotIndexOf, consumePlaced, focusOrderIndices, restoreFocusOrder, setFocusOrder,
} from './focusLayout';

vi.mock('../../wailsjs/go/backend/App', () => ({ SaveConfig: vi.fn().mockResolvedValue(undefined) }));

let tab = '';
let ids: string[] = [];

beforeEach(() => {
  for (const t of tabStore.getState().tabs) tabStore.forceCloseTab(t.id);
  focusOrders.set({});
  tab = tabStore.addTab('T', '/t');
  ids = [1, 2, 3, 4].map((n) => tabStore.addPane(tab, `h1:${n}`, `p${n}`, 'claude', ''));
});

describe('placePane', () => {
  it('lets a replacement take the old pane\'s place', () => {
    setFocusOrder(tab, [ids[2], ids[0], ids[1], ids[3]]);
    const slot = slotIndexOf(tab, ids[0]);
    expect(slot).toBe(1);
    tabStore.closePane(tab, ids[0]);
    const fresh = tabStore.addPane(tab, 'h1:9', 'neu', 'claude', '');
    placePane(tab, fresh, slot);
    expect(get(focusOrders)[tab]).toEqual([ids[2], fresh, ids[1], ids[3]]);
  });

  it('marks the pane as placed exactly once', () => {
    placePane(tab, ids[3], 2);
    expect(consumePlaced(ids[3])).toBe(true);
    expect(consumePlaced(ids[3])).toBe(false);
  });

  it('clamps an index past the end and ignores -1', () => {
    placePane(tab, ids[0], 99);
    expect(get(focusOrders)[tab].at(-1)).toBe(ids[0]);
    const before = get(focusOrders)[tab];
    placePane(tab, ids[1], -1);
    expect(get(focusOrders)[tab]).toBe(before);
  });
});

describe('saving the order', () => {
  it('writes nothing for a tab that never had an order', () => {
    const t = tabStore.getState().tabs.find((x) => x.id === tab)!;
    expect(focusOrderIndices(tab, t.panes)).toBeUndefined();
  });

  it('writes indices into the pane list and maps them back on restore', () => {
    setFocusOrder(tab, [ids[3], ids[1]]);
    const t = tabStore.getState().tabs.find((x) => x.id === tab)!;
    const saved = focusOrderIndices(tab, t.panes)!;
    expect(saved).toEqual([3, 1, 0, 2]);

    // Restore: saved pane 1 could not be restored.
    restoreFocusOrder('tab-new', saved, ['a', undefined, 'c', 'd']);
    expect(get(focusOrders)['tab-new']).toEqual(['d', 'a', 'c']);
  });
});

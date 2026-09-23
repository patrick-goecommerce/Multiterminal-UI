import { describe, it, expect, afterEach, beforeEach, vi } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import { get } from 'svelte/store';
import { tick } from 'svelte';
import PaneGrid from './PaneGrid.svelte';
import { tabStore, type Pane } from '../stores/tabs';
import { focusOrders, floatingPane, placePane, slotIndexOf, setFocusOrder } from '../stores/focusLayout';

vi.mock('./TerminalPane.svelte', async () => ({ default: (await import('./__fixtures__/PaneStub.svelte')).default }));
vi.mock('../../wailsjs/go/backend/App', () => ({ SaveConfig: vi.fn().mockResolvedValue(undefined) }));

function panesOf(tabId: string): Pane[] {
  return tabStore.getState().tabs.find((t) => t.id === tabId)!.panes;
}

function slotKind(container: HTMLElement, paneId: string): string {
  const el = container.querySelector(`[data-testid="pane-${paneId}"]`);
  const slot = el?.closest('.slot');
  const kind = Array.from(slot?.classList ?? []).find((c) => c.startsWith('slot-'));
  return kind ?? '';
}

let tabA = '';
let tabB = '';
let a: string[] = [];
let b = '';

beforeEach(() => {
  for (const t of tabStore.getState().tabs) tabStore.forceCloseTab(t.id);
  focusOrders.set({});
  floatingPane.set(null);
  tabA = tabStore.addTab('Alpha', '/a');
  a = [1, 2, 3, 4].map((n) => tabStore.addPane(tabA, `h1:${n}`, `a${n}`, 'claude', ''));
  tabB = tabStore.addTab('Beta', '/b', false);
  b = tabStore.addPane(tabB, 'h1:9', 'b1', 'claude', '');
  tabStore.addPane(tabB, 'h1:10', 'b2', 'claude', '');
  tabStore.updateActivity('h1:9', 'waitingPermission', '', 100);
  tabStore.updateActivity('h1:10', 'active', '', 100);
});

afterEach(cleanup);

describe('PaneGrid in focus mode', () => {
  it('shows two panes big and the rest small', () => {
    const { container } = render(PaneGrid, { props: { panes: panesOf(tabA), tabId: tabA, layoutMode: 'focus' } });
    expect(slotKind(container, a[0])).toBe('slot-big');
    expect(slotKind(container, a[1])).toBe('slot-big');
    expect(slotKind(container, a[2])).toBe('slot-small');
    expect(slotKind(container, a[3])).toBe('slot-small');
  });

  it('moves a clicked small pane to the left', async () => {
    const { container } = render(PaneGrid, { props: { panes: panesOf(tabA), tabId: tabA, layoutMode: 'focus' } });
    const cover = container.querySelector(`[data-testid="pane-${a[3]}"]`)!.closest('.slot')!.querySelector('.small-cover')!;
    await fireEvent.click(cover);

    expect(get(focusOrders)[tabA]).toEqual([a[3], a[0], a[1], a[2]]);
    expect(slotKind(container, a[3])).toBe('slot-big');
    expect(slotKind(container, a[1])).toBe('slot-small');
  });

  it('lists only the waiting panes of other tabs', () => {
    const { container } = render(PaneGrid, { props: { panes: panesOf(tabA), tabId: tabA, layoutMode: 'focus' } });
    const rows = Array.from(container.querySelectorAll('.wl-row')).map((r) => r.textContent ?? '');
    expect(rows).toHaveLength(1);
    expect(rows[0]).toContain('Beta');
    expect(rows[0]).toContain('b1');
  });

  it('opens a waiting pane as floating window without switching the tab', async () => {
    const { container } = render(PaneGrid, { props: { panes: panesOf(tabA), tabId: tabA, layoutMode: 'focus' } });
    await fireEvent.click(container.querySelector('.wl-row')!);

    expect(get(floatingPane)).toEqual({ tabId: tabB, paneId: b });
    expect(tabStore.getState().activeTabId).toBe(tabA);
  });

  it('draws the floating pane in its own, hidden tab layer and makes it live', () => {
    floatingPane.set({ tabId: tabB, paneId: b });
    const { container, getByText } = render(PaneGrid, {
      props: { panes: panesOf(tabB), tabId: tabB, tabName: 'Beta', active: false, layoutMode: 'focus' },
    });
    expect(slotKind(container, b)).toBe('slot-float');
    expect(container.querySelector(`[data-testid="pane-${b}"]`)!.getAttribute('data-active')).toBe('true');
    expect(getByText('Zum Projekt wechseln')).toBeTruthy();
  });

  it('switches to the project from the floating window', async () => {
    floatingPane.set({ tabId: tabB, paneId: b });
    const { getByText } = render(PaneGrid, {
      props: { panes: panesOf(tabB), tabId: tabB, tabName: 'Beta', active: false, layoutMode: 'focus' },
    });
    await fireEvent.click(getByText('Zum Projekt wechseln'));

    expect(tabStore.getState().activeTabId).toBe(tabB);
    expect(get(floatingPane)).toBeNull();
    expect(get(focusOrders)[tabB][0]).toBe(b);
  });
});

describe('PaneGrid in grid mode', () => {
  it('has no waiting list and no small panes', () => {
    const { container } = render(PaneGrid, { props: { panes: panesOf(tabA), tabId: tabA } });
    expect(container.querySelector('.waiting-list')).toBeNull();
    expect(container.querySelector('.small-cover')).toBeNull();
    expect(slotKind(container, a[3])).toBe('slot-grid');
  });
});

describe('PaneGrid focus hand-off', () => {
  // TerminalPane will not take the keyboard from a focused button, so a
  // click that is meant to move typing into a terminal must release it.
  it('lets go of the clicked list button so the floating terminal gets the keyboard', async () => {
    const { container } = render(PaneGrid, { props: { panes: panesOf(tabA), tabId: tabA, layoutMode: 'focus' } });
    const row = container.querySelector('.wl-row') as HTMLButtonElement;
    row.focus();
    await fireEvent.click(row);
    expect(document.activeElement).not.toBe(row);
  });
});

describe('PaneGrid focus mode: panes the app places', () => {
  it('keeps a replaced pane in the old pane\'s slot', async () => {
    const { container, rerender } = render(PaneGrid, { props: { panes: panesOf(tabA), tabId: tabA, active: true, layoutMode: 'focus' } });
    const slot = slotIndexOf(tabA, a[1]);
    tabStore.closePane(tabA, a[1]);
    await rerender({ panes: panesOf(tabA) });
    const fresh = tabStore.addPane(tabA, 'h1:50', 'neu', 'claude', '');
    placePane(tabA, fresh, slot);
    await rerender({ panes: panesOf(tabA) });

    expect(get(focusOrders)[tabA][1]).toBe(fresh);
    expect(slotKind(container, fresh)).toBe('slot-big');
    expect(get(focusOrders)[tabA][0]).toBe(a[0]);
  });

  it('puts an agent pane small and leaves focus alone', async () => {
    tabStore.focusPane(tabA, a[0]);
    const { container, rerender } = render(PaneGrid, { props: { panes: panesOf(tabA), tabId: tabA, active: true, layoutMode: 'focus' } });
    const agent = tabStore.addPane(tabA, 'h1:60', 'agent', 'claude', '');
    placePane(tabA, agent, 2);
    tabStore.focusPane(tabA, a[0]);
    await rerender({ panes: panesOf(tabA) });

    expect(slotKind(container, agent)).toBe('slot-small');
    expect(get(focusOrders)[tabA].slice(0, 2)).toEqual([a[0], a[1]]);
  });

  it('does not promote the neighbour that gets focus when the focused pane closes', async () => {
    setFocusOrder(tabA, [a[1], a[0], a[2], a[3]]);
    tabStore.focusPane(tabA, a[1]);
    const { rerender } = render(PaneGrid, { props: { panes: panesOf(tabA), tabId: tabA, active: true, layoutMode: 'focus' } });
    // closePane hands focus to the array neighbour (a[2], small here).
    tabStore.closePane(tabA, a[1]);
    await rerender({ panes: panesOf(tabA) });

    expect(get(focusOrders)[tabA].slice(0, 2)).toEqual([a[1], a[0]]); // stored order untouched
    expect(panesOf(tabA).find((p) => p.focused)?.id).toBe(a[2]);
  });

  it('still puts a pane the user opens on the left', async () => {
    const { container, rerender } = render(PaneGrid, { props: { panes: panesOf(tabA), tabId: tabA, active: true, layoutMode: 'focus' } });
    const opened = tabStore.addPane(tabA, 'h1:70', 'neu', 'claude', '');
    await rerender({ panes: panesOf(tabA) });
    await tick();

    expect(get(focusOrders)[tabA][0]).toBe(opened);
    expect(slotKind(container, opened)).toBe('slot-big');
  });
});

describe('PaneGrid focus mode: review fixes', () => {
  it('keeps a placed pane in the small column when it takes the focus', async () => {
    const { container, rerender } = render(PaneGrid, { props: { panes: panesOf(tabA), tabId: tabA, active: true, layoutMode: 'focus' } });
    const fresh = tabStore.addPane(tabA, 'h1:80', 'ersatz', 'claude', '');
    placePane(tabA, fresh, 3);
    await rerender({ panes: panesOf(tabA) });
    await tick();

    expect(get(focusOrders)[tabA][3]).toBe(fresh);
    expect(slotKind(container, fresh)).toBe('slot-small');
  });

  it('shows the waiting list only in the visible grid', () => {
    const { container } = render(PaneGrid, { props: { panes: panesOf(tabA), tabId: tabA, active: false, layoutMode: 'focus' } });
    expect(container.querySelector('.waiting-list')).toBeNull();
  });

  it('does not save the window position on a click without a move', async () => {
    const App = await import('../../wailsjs/go/backend/App');
    (App.SaveConfig as any).mockClear();
    floatingPane.set({ tabId: tabB, paneId: b });
    const { container } = render(PaneGrid, {
      props: { panes: panesOf(tabB), tabId: tabB, tabName: 'Beta', active: false, layoutMode: 'focus' },
    });
    const bar = container.querySelector('.float-bar') as HTMLElement;
    bar.setPointerCapture = () => {};
    await fireEvent.pointerDown(bar, { clientX: 100, clientY: 100, pointerId: 1 });
    await fireEvent.pointerUp(bar, { clientX: 100, clientY: 100, pointerId: 1 });
    expect(App.SaveConfig).not.toHaveBeenCalled();

    await fireEvent.pointerDown(bar, { clientX: 100, clientY: 100, pointerId: 1 });
    await fireEvent.pointerMove(bar, { clientX: 160, clientY: 140, pointerId: 1 });
    await fireEvent.pointerUp(bar, { clientX: 160, clientY: 140, pointerId: 1 });
    expect(App.SaveConfig).toHaveBeenCalledTimes(1);
  });
});

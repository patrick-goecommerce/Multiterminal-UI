import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { tabStore, allTabs, activeTab } from '../stores/tabs';
import { floatingPane, focusOrders, setFocusOrder } from '../stores/focusLayout';
import { attentionList } from './attention';
import { jumpTo, preview, next, back, walkPaneId } from './attentionNav';

function setup() {
  for (const t of get(allTabs)) tabStore.forceCloseTab(t.id);
  const a = tabStore.addTab('alpha', 'C:\\a');
  const b = tabStore.addTab('beta', 'C:\\b', false);
  const pa = tabStore.addPane(a, 'h:1', 'A', 'claude', '');
  const pb = tabStore.addPane(b, 'h:2', 'B', 'claude', '');
  tabStore.applyAttention('h:2', { state: 'waitingPermission', rank: 0, seen: false, detail: '', source: '', since: 1 });
  tabStore.applyAttention('h:1', { state: 'done', rank: 3, seen: false, detail: '', source: '', since: 1 });
  tabStore.setActiveTab(a);
  return { a, b, pa, pb };
}

describe('attentionNav', () => {
  beforeEach(() => { floatingPane.set(null); back(); });

  it('next opens the most urgent entry as a preview in the focus layout', () => {
    const { a, b, pb } = setup();
    next('focus');
    expect(get(floatingPane)).toEqual({ tabId: b, paneId: pb });
    expect(get(activeTab)?.id).toBe(a);
  });

  it('back returns to where the walk started', () => {
    const { a, pa } = setup();
    next('grid');
    expect(get(activeTab)?.id).not.toBe(a);
    back();
    expect(get(activeTab)?.id).toBe(a);
    expect(get(activeTab)?.focusedPaneId).toBe(pa);
  });

  it('a preview of an entry in the current tab focuses it instead (Review Focus 5)', () => {
    const { a, pa } = setup();
    const here = attentionList(get(allTabs), a).find((e) => e.pane.id === pa)!;
    preview(here, 'focus');
    expect(get(floatingPane)).toBeNull();
    expect(get(activeTab)?.focusedPaneId).toBe(pa);
  });

  it('jumpTo switches tab and focuses the pane', () => {
    const { b, pb } = setup();
    jumpTo(b, pb);
    expect(get(activeTab)?.id).toBe(b);
    expect(get(activeTab)?.focusedPaneId).toBe(pb);
  });

  it('back closes the preview and returns while it is open', () => {
    const { a, pa } = setup();
    next('focus');
    back();
    expect(get(floatingPane)).toBeNull();
    expect(get(activeTab)?.id).toBe(a);
    expect(get(activeTab)?.focusedPaneId).toBe(pa);
  });

  // R17: once the user has gone somewhere by hand, Ctrl+Shift+K must not
  // drag them back to where a walk started long ago.
  it('back does nothing once the user left the pane the walk put them on', () => {
    setup();
    const c = tabStore.addTab('gamma', 'C:\\c', false);
    next('grid');
    tabStore.setActiveTab(c);
    back();
    expect(get(activeTab)?.id).toBe(c);
    expect(get(walkPaneId)).toBe('');
  });

  it('a walk that starts after the user moved on returns to where it started', () => {
    const { b, pb } = setup();
    const c = tabStore.addTab('gamma', 'C:\\c', false);
    const pc = tabStore.addPane(c, 'h:3', 'C', 'claude', '');
    jumpTo(b, pb);
    tabStore.setActiveTab(c);
    next('grid');
    expect(get(activeTab)?.id).toBe(b);
    back();
    expect(get(activeTab)?.id).toBe(c);
    expect(get(activeTab)?.focusedPaneId).toBe(pc);
  });

  it('a jump by click ends the keyboard walk, so the selection follows the click', () => {
    const { a, pa } = setup();
    next('focus');
    expect(get(walkPaneId)).not.toBe('');
    jumpTo(a, pa);
    expect(get(walkPaneId)).toBe('');
  });

  // R22: a pane already in a big slot stays where it is.
  it('does not swap the big panes when it focuses the right one', () => {
    const { a, pa } = setup();
    const pa2 = tabStore.addPane(a, 'h:4', 'A2', 'claude', '');
    setFocusOrder(a, [pa2, pa]);
    jumpTo(a, pa);
    expect(get(focusOrders)[a]).toEqual([pa2, pa]);
  });
});

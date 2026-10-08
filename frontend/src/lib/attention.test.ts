import { describe, it, expect } from 'vitest';
import {
  paneRank, glyphKind, attentionList, panelRows, nextEntry, cursorOf, tabRank, tabGlyphs, shouldToast,
} from './attention';

const pane = (id: string, activity: string, att?: Partial<{ rank: number; seen: boolean; since: number; state: string }>) =>
  ({ id, sessionId: 'h:' + id, name: id, activity, activitySince: 0,
     attention: att ? { state: activity, rank: 5, seen: false, detail: '', source: '', since: 0, ...att } : undefined }) as any;
const tab = (id: string, panes: any[]) => ({ id, name: id, dir: '', panes, focusedPaneId: '' }) as any;

describe('paneRank', () => {
  it('uses the host rank when there is one', () => {
    expect(paneRank(pane('a', 'done', { rank: 3 }))).toBe(3);
  });
  it('without a record, a finished pane counts as seen (older daemon)', () => {
    expect(paneRank(pane('a', 'done'))).toBe(5);
    expect(paneRank(pane('b', 'waitingPermission'))).toBe(0);
  });
});

describe('attentionList', () => {
  const tabs = [
    tab('t1', [pane('p1', 'done', { rank: 3, since: 100 }), pane('p2', 'active', { rank: 4 })]),
    tab('t2', [pane('p3', 'waitingAnswer', { rank: 1, since: 300 }), pane('p4', 'waitingPermission', { rank: 0, since: 500 })]),
  ];
  it('lists ranks 0-3 from every tab, most urgent first, the current tab marked', () => {
    const list = attentionList(tabs, 't1');
    expect(list.map((e) => e.pane.id)).toEqual(['p4', 'p3', 'p1']);
    expect(list.find((e) => e.pane.id === 'p1')?.here).toBe(true);
  });
  it('within a rank, the longest waiting first', () => {
    const list = attentionList([tab('t', [pane('x', 'done', { rank: 3, since: 900 }), pane('y', 'done', { rank: 3, since: 100 })])], '');
    expect(list.map((e) => e.pane.id)).toEqual(['y', 'x']);
  });
});

describe('panelRows', () => {
  it('keeps every blocking entry and collapses finished ones beyond three', () => {
    const list = attentionList([tab('t', [
      pane('q', 'waitingAnswer', { rank: 1, since: 1 }),
      ...[1, 2, 3, 4, 5].map((n) => pane('d' + n, 'done', { rank: 3, since: n })),
    ])], '');
    const r = panelRows(list, false);
    expect(r.rows).toHaveLength(4);
    expect(r.hiddenDone).toBe(2);
    expect(panelRows(list, true).hiddenDone).toBe(0);
  });
});

describe('nextEntry', () => {
  const list = attentionList([tab('t', [
    pane('a', 'waitingPermission', { rank: 0, since: 1 }),
    pane('b', 'waitingAnswer', { rank: 1, since: 1 }),
    pane('c', 'done', { rank: 3, since: 1 }),
  ])], '');
  it('walks in order and wraps', () => {
    let e = nextEntry(list, null);
    expect(e?.pane.id).toBe('a');
    e = nextEntry(list, cursorOf(e!));
    expect(e?.pane.id).toBe('b');
    expect(nextEntry(list, cursorOf(list[2]))?.pane.id).toBe('a');
  });
  it('an entry that left the list does not send the walk back to the top', () => {
    const cursor = cursorOf(list[1]); // b, then b gets answered
    const without = list.filter((e) => e.pane.id !== 'b');
    expect(nextEntry(without, cursor)?.pane.id).toBe('c');
  });
});

describe('tabRank and tabGlyphs', () => {
  it('a question beats a working pane in either order', () => {
    expect(tabRank([pane('a', 'waitingAnswer', { rank: 1 }), pane('b', 'active', { rank: 4 })])).toBe(1);
    expect(tabRank([pane('b', 'active', { rank: 4 }), pane('a', 'waitingAnswer', { rank: 1 })])).toBe(1);
  });
  it('shows at most six glyphs and counts the rest', () => {
    const many = Array.from({ length: 8 }, (_, i) => pane('p' + i, 'active', { rank: 4 }));
    const g = tabGlyphs(many);
    expect(g.kinds).toHaveLength(6);
    expect(g.more).toBe(2);
  });
  it('glyphKind tells seen from unseen', () => {
    expect(glyphKind(pane('a', 'done', { rank: 3, seen: false }))).toBe('done');
    expect(glyphKind(pane('a', 'done', { rank: 5, seen: true }))).toBe('doneSeen');
  });
});

describe('shouldToast', () => {
  const att = (rank: number, since = 1, seen = false) => ({ state: '', rank, seen, detail: '', source: '', since });
  it('toasts a new permission, question or error', () => {
    expect(shouldToast(undefined, att(0), false)).toBe(true);
    expect(shouldToast(att(4), att(2), false)).toBe(true);
  });
  it('never for done, never for the pane being looked at, not twice', () => {
    expect(shouldToast(undefined, att(3), false)).toBe(false);
    expect(shouldToast(undefined, att(0), true)).toBe(false);
    expect(shouldToast(att(0, 5), att(0, 5, true), false)).toBe(false);
  });
});

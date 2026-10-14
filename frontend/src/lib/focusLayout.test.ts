import { describe, it, expect } from 'vitest';
import type { Pane, Tab } from '../stores/tabs';
import {
  reconcileOrder, promoteInOrder, floatingStillValid,
  focusGeometry, floatingRect, slotFor, layoutModeOf,
} from './focusLayout';

function pane(id: string, activity: Pane['activity'] = 'idle', activitySince = 0): Pane {
  return {
    id, sessionId: `h1:${id}`, name: id, mode: 'claude', model: '', focused: false, activity,
    cost: '', running: true, maximized: false, issueNumber: null, issueTitle: '', issueBranch: '',
    worktreePath: '', branch: '', targetBranch: '', zoomDelta: 0, background: false,
    display: 'terminal', conversationId: '', claudeSessionId: '', mcpProfile: '', autoName: '', sessionName: '',
    oscTitle: '', autoNameSource: '', userRenamed: false, finishPhase: '', activitySince,
  };
}

function tab(id: string, panes: Pane[]): Tab {
  return { id, name: `Projekt ${id}`, dir: '/p', panes, focusedPaneId: '' };
}

describe('layoutModeOf', () => {
  it('reads anything but focus as grid', () => {
    expect(layoutModeOf('focus')).toBe('focus');
    expect(layoutModeOf('grid')).toBe('grid');
    expect(layoutModeOf(undefined)).toBe('grid');
    expect(layoutModeOf('Focus')).toBe('grid');
  });
});

describe('order', () => {
  it('drops duplicates, so no pane gets a slot that does not exist', () => {
    expect(reconcileOrder(['a', 'a', 'b'], ['a', 'b', 'c'])).toEqual(['a', 'b', 'c']);
  });

  it('drops closed panes and appends new ones', () => {
    expect(reconcileOrder(['c', 'a', 'x'], ['a', 'b', 'c'])).toEqual(['c', 'a', 'b']);
    expect(reconcileOrder(undefined, ['a', 'b'])).toEqual(['a', 'b']);
  });

  it('moves a small pane to the left; the old left moves right, the old right becomes small', () => {
    expect(promoteInOrder(['a', 'b', 'c', 'd'], 'd')).toEqual(['d', 'a', 'b', 'c']);
  });

  it('leaves the right big pane where it is, so clicking into it does not make it jump', () => {
    const order = ['a', 'b', 'c'];
    expect(promoteInOrder(order, 'b')).toBe(order);
    expect(promoteInOrder(order, 'a')).toBe(order);
  });

  it('puts a new pane on the left even when it landed in the right slot', () => {
    expect(promoteInOrder(['a', 'b'], 'b', true)).toEqual(['b', 'a']);
  });

  it('ignores an unknown id', () => {
    const order = ['a', 'b'];
    expect(promoteInOrder(order, 'zz')).toBe(order);
  });
});

describe('floatingStillValid', () => {
  const tabs = [tab('t1', []), tab('t2', [pane('a')])];

  it('holds while the pane exists in a tab that is not active', () => {
    expect(floatingStillValid({ tabId: 't2', paneId: 'a' }, tabs, 't1')).toBe(true);
  });

  it('ends when its own tab becomes active, or the pane is gone', () => {
    expect(floatingStillValid({ tabId: 't2', paneId: 'a' }, tabs, 't2')).toBe(false);
    expect(floatingStillValid({ tabId: 't2', paneId: 'gone' }, tabs, 't1')).toBe(false);
    expect(floatingStillValid(null, tabs, 't1')).toBe(false);
  });
});

describe('focusGeometry', () => {
  it('keeps the right column even with a single pane and nothing waiting', () => {
    const g = focusGeometry(2000, 1000, 1, 0);
    expect(g.big).toHaveLength(1);
    expect(g.column.w).toBeGreaterThan(0);
    expect(g.big[0].x + g.big[0].w).toBeLessThanOrEqual(g.column.x);
  });

  it('keeps the big panes the same size whether or not something waits elsewhere', () => {
    const quiet = focusGeometry(2000, 1000, 4, 0);
    const busy = focusGeometry(2000, 1000, 4, 5);
    expect(busy.big).toEqual(quiet.big);
  });

  it('splits two big panes and stacks the rest above the waiting list', () => {
    const g = focusGeometry(2000, 1000, 5, 1);
    expect(g.big).toHaveLength(2);
    expect(g.small).toHaveLength(3);
    const lastSmall = g.small[2];
    expect(lastSmall.y + lastSmall.h).toBeLessThanOrEqual(g.list.y);
  });

  it('shrinks the list to one line when nothing waits', () => {
    const g = focusGeometry(2000, 1000, 4, 0);
    expect(g.list.h).toBe(36);
  });

  it('caps the list at 40 % of the column', () => {
    const g = focusGeometry(2000, 1000, 4, 50);
    expect(g.list.h).toBeLessThanOrEqual(Math.round((1000 - 8) * 0.4));
  });
});

describe('floatingRect', () => {
  it('centres a window that was never moved', () => {
    const r = floatingRect(1000, 800, -1, -1);
    expect(r.x).toBe(Math.round((1000 - r.w) / 2));
    expect(r.y).toBe(Math.round((800 - r.h) / 2));
  });

  it('pulls a saved position back into a smaller area', () => {
    const r = floatingRect(1000, 800, 5000, 5000);
    expect(r.x + r.w).toBe(1000);
    expect(r.y + r.h).toBe(800);
  });
});

describe('slotFor', () => {
  const geo = focusGeometry(2000, 1000, 3, 0);
  const area = { w: 2000, h: 1000 };

  it('draws a small pane at the big size, scaled down, so the PTY never learns about it', () => {
    const s = slotFor('c', ['a', 'b', 'c'], geo, area, null, '');
    expect(s.kind).toBe('small');
    expect(s.logical).toEqual({ w: geo.big[1].w, h: geo.big[1].h });
    expect(s.scale).toBeCloseTo(geo.small[0].w / geo.big[1].w);
  });

  it('gives a floating pane the floating rect, ahead of everything else', () => {
    const rect = { x: 1, y: 2, w: 3, h: 4 };
    expect(slotFor('c', ['a', 'b', 'c'], geo, area, { id: 'c', rect }, 'a')).toEqual({ kind: 'float', rect });
  });

  it('hides the others while one pane is maximized', () => {
    expect(slotFor('a', ['a', 'b'], geo, area, null, 'b').kind).toBe('hidden');
    expect(slotFor('b', ['a', 'b'], geo, area, null, 'b').kind).toBe('max');
  });

  // A zero-size slot would fit the terminal, and its PTY, down to 2x1.
  it('keeps the hidden panes at their own size behind a maximized one', () => {
    const big = slotFor('a', ['a', 'b', 'c'], geo, area, null, 'b');
    expect(big.rect).toEqual(geo.big[0]);
    const small = slotFor('c', ['a', 'b', 'c'], geo, area, null, 'b');
    expect(small.kind).toBe('hidden');
    expect(small.logical).toEqual({ w: geo.big[1].w, h: geo.big[1].h });
  });
});

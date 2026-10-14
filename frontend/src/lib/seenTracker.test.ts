import { describe, it, expect } from 'vitest';
import { createSeenTracker } from './seenTracker';

function fakeTimers() {
  let fn: (() => void) | null = null;
  return {
    timers: { set: (f: () => void) => { fn = f; return 1; }, clear: () => { fn = null; } },
    fire: () => { const f = fn; fn = null; f?.(); },
    armed: () => fn !== null,
  };
}
const done = (ref = 'h:1', seen = false, since = 1) => ({ ref, rank: 3, seen, since });

describe('seenTracker', () => {
  it('marks a finished pane after it was looked at for the delay', () => {
    const marked: string[] = [];
    const ft = fakeTimers();
    const t = createSeenTracker((r) => marked.push(r), 1000, ft.timers);
    t.look(done());
    expect(marked).toEqual([]);
    ft.fire();
    expect(marked).toEqual(['h:1']);
  });
  it('does nothing for a permission request, which only the agent clears', () => {
    const ft = fakeTimers();
    const t = createSeenTracker(() => {}, 1000, ft.timers);
    t.look({ ref: 'h:1', rank: 0, seen: false, since: 1 });
    expect(ft.armed()).toBe(false);
  });
  it('a pane that finishes while being looked at is marked too', () => {
    const marked: string[] = [];
    const ft = fakeTimers();
    const t = createSeenTracker((r) => marked.push(r), 1000, ft.timers);
    t.look({ ref: 'h:1', rank: 4, seen: false, since: 1 });
    expect(ft.armed()).toBe(false);
    t.look(done());
    ft.fire();
    expect(marked).toEqual(['h:1']);
  });
  it('looking away or losing window focus cancels', () => {
    const marked: string[] = [];
    const ft = fakeTimers();
    const t = createSeenTracker((r) => marked.push(r), 1000, ft.timers);
    t.look(done());
    t.look(done('h:2', true));
    expect(ft.armed()).toBe(false);
    t.look(done());
    t.windowFocus(false);
    expect(ft.armed()).toBe(false);
    expect(marked).toEqual([]);
  });
  it('typing marks at once', () => {
    const marked: string[] = [];
    const t = createSeenTracker((r) => marked.push(r), 1000, fakeTimers().timers);
    t.typed(done());
    t.typed(done('h:2', true));
    expect(marked).toEqual(['h:1']);
  });
  it('typing twice into the same state marks once', () => {
    const marked: string[] = [];
    const t = createSeenTracker((r) => marked.push(r), 1000, fakeTimers().timers);
    t.typed(done());
    t.typed(done());
    expect(marked).toEqual(['h:1']);
  });
  it('does not arm again for a state it already marked', () => {
    const marked: string[] = [];
    const ft = fakeTimers();
    const t = createSeenTracker((r) => marked.push(r), 1000, ft.timers);
    t.look(done());
    ft.fire();
    t.look(done());
    expect(ft.armed()).toBe(false);
    expect(marked).toEqual(['h:1']);
  });
  it('marks the same pane again for a new state', () => {
    const marked: string[] = [];
    const ft = fakeTimers();
    const t = createSeenTracker((r) => marked.push(r), 1000, ft.timers);
    t.typed(done());
    t.look(done('h:1', false, 2));
    ft.fire();
    expect(marked).toEqual(['h:1', 'h:1']);
  });
});

import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { get } from 'svelte/store';
import { now, TICK_MS } from './clock';

describe('now store', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('advances on each tick while subscribed', () => {
    const seen: number[] = [];
    const stop = now.subscribe((v) => seen.push(v));
    const first = seen.length;

    vi.advanceTimersByTime(TICK_MS);
    expect(seen.length).toBe(first + 1);

    vi.advanceTimersByTime(TICK_MS);
    expect(seen.length).toBe(first + 2);
    stop();
  });

  it('runs a single interval no matter how many subscribers', () => {
    const spy = vi.spyOn(globalThis, 'setInterval');
    const a = now.subscribe(() => {});
    const b = now.subscribe(() => {});
    const c = now.subscribe(() => {});
    expect(spy).toHaveBeenCalledTimes(1);
    a(); b(); c();
    spy.mockRestore();
  });

  it('stops ticking once the last subscriber leaves', () => {
    const spy = vi.spyOn(globalThis, 'clearInterval');
    const stop = now.subscribe(() => {});
    stop();
    expect(spy).toHaveBeenCalled();
    spy.mockRestore();
  });
});

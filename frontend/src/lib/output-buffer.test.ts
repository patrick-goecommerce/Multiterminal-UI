import { describe, it, expect } from 'vitest';
import { PendingOutput } from './output-buffer';

const bytes = (...values: number[]) => new Uint8Array(values);
const filler = (n: number) => new Uint8Array(n).fill(0x61);

describe('PendingOutput', () => {
  it('drains chunks in arrival order as one contiguous buffer', () => {
    const buf = new PendingOutput(1024);
    buf.push(bytes(1, 2));
    buf.push(bytes(3));
    buf.push(bytes(4, 5));

    expect(buf.drain(1024)).toEqual(bytes(1, 2, 3, 4, 5));
    expect(buf.byteLength).toBe(0);
    expect(buf.isEmpty).toBe(true);
  });

  it('bounds a drain to maxBytes but always makes progress', () => {
    const buf = new PendingOutput(1024);
    buf.push(filler(30));
    buf.push(filler(30));

    expect(buf.drain(40)?.length).toBe(30); // second chunk would exceed 40
    expect(buf.byteLength).toBe(30);

    // A single chunk larger than the cap must still be handed over, otherwise
    // the pane would stall forever.
    const big = new PendingOutput(1024);
    big.push(filler(500));
    expect(big.drain(100)?.length).toBe(500);
  });

  it('returns null when there is nothing to drain', () => {
    expect(new PendingOutput(1024).drain(64)).toBeNull();
  });

  // Regression for #157: cutting the oldest chunks out of the backlog leaves a
  // hole in the middle of the VT100 stream. xterm.js cannot recover from that —
  // escape sequences get truncated and the pane stays garbled. On overflow the
  // whole backlog must go, so the caller can resync from a full repaint.
  it('discards the entire backlog on overflow instead of cutting a hole', () => {
    const buf = new PendingOutput(100);
    expect(buf.push(filler(60))).toBe(false);
    expect(buf.push(filler(60))).toBe(true); // 120 > 100 → overflow

    expect(buf.byteLength).toBe(0);
    expect(buf.isEmpty).toBe(true);
    expect(buf.drain(1024)).toBeNull();
  });

  it('reports overflow only for the push that crosses the cap', () => {
    const buf = new PendingOutput(100);
    buf.push(filler(60));
    expect(buf.push(filler(60))).toBe(true);
    expect(buf.push(filler(10))).toBe(false); // fresh backlog, well under cap
    expect(buf.byteLength).toBe(10);
  });

  it('treats a single oversized chunk as an overflow', () => {
    const buf = new PendingOutput(100);
    expect(buf.push(filler(400))).toBe(true);
    expect(buf.isEmpty).toBe(true);
  });

  it('clears on demand', () => {
    const buf = new PendingOutput(1024);
    buf.push(filler(10));
    buf.clear();
    expect(buf.isEmpty).toBe(true);
    expect(buf.byteLength).toBe(0);
  });
});

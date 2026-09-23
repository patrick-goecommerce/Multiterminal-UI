import { vi, describe, test, expect, beforeEach } from 'vitest';

// Capture the context-loss callback that attachWebglRenderer registers so we
// can simulate a GPU/display event (monitor power cycle, output-device switch)
// invalidating the WebGL context.
let capturedContextLossCb: (() => void) | null = null;

vi.mock('@xterm/addon-webgl', () => ({
  WebglAddon: class {
    onContextLoss(cb: () => void) {
      capturedContextLossCb = cb;
    }
    dispose() {}
  },
}));

import { attachWebglRenderer } from './terminal';

describe('attachWebglRenderer — WebGL context loss', () => {
  beforeEach(() => {
    capturedContextLossCb = null;
  });

  test('forces a full repaint when the WebGL context is lost', () => {
    const refresh = vi.fn();
    const terminal = { rows: 40, refresh, loadAddon: vi.fn() } as any;

    attachWebglRenderer(terminal);
    expect(capturedContextLossCb).toBeTypeOf('function');

    // Simulate the lost GL context (e.g. display power cycle / device switch).
    // Without a forced repaint the DOM-renderer keeps the stale, half-erased
    // frame until new PTY output arrives — an idle Claude pane stays frozen.
    capturedContextLossCb!();

    expect(refresh).toHaveBeenCalledWith(0, terminal.rows - 1);
  });
});

describe('attachWebglRenderer / detachWebglRenderer — context budget', () => {
  const fakeTerminal = () => ({ rows: 40, refresh: vi.fn(), loadAddon: vi.fn() }) as any;

  test('attaching twice holds one context, detaching gives it back', async () => {
    const { webglContextCount, detachWebglRenderer } = await import('./terminal');
    const t = fakeTerminal();
    const before = webglContextCount();

    attachWebglRenderer(t);
    attachWebglRenderer(t);
    expect(webglContextCount()).toBe(before + 1);
    expect(t.loadAddon).toHaveBeenCalledTimes(1);

    detachWebglRenderer(t);
    expect(webglContextCount()).toBe(before);
  });

  // A background pane gave its context back; a context-loss retry that was
  // already scheduled must not take it again.
  test('a context-loss retry does not re-attach a detached terminal', async () => {
    vi.useFakeTimers();
    try {
      const { webglContextCount, detachWebglRenderer } = await import('./terminal');
      const t = fakeTerminal();
      const before = webglContextCount();
      attachWebglRenderer(t);
      capturedContextLossCb!();
      detachWebglRenderer(t);
      vi.advanceTimersByTime(200);

      expect(webglContextCount()).toBe(before);
      expect(t.loadAddon).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });

  // On a host where creating the context always fails, each attempt used to
  // keep its slot, and after eight the window stopped trying WebGL for good.
  test('a failed loadAddon does not keep its slot', async () => {
    const { webglContextCount } = await import('./terminal');
    const before = webglContextCount();
    for (let i = 0; i < 10; i++) {
      const t = fakeTerminal();
      t.loadAddon = vi.fn(() => { throw new Error('no WebGL2'); });
      attachWebglRenderer(t);
    }
    expect(webglContextCount()).toBe(before);
  });
});

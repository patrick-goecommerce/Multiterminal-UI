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

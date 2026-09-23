import { describe, it, expect, vi } from 'vitest';
import { createGlobalKeyHandler, type ShortcutCallbacks } from './shortcuts';

function callbacks(): ShortcutCallbacks {
  return {
    onNewPane: vi.fn(), onNewTab: vi.fn(), onCloseTab: vi.fn(), onToggleSidebar: vi.fn(),
    onToggleMaximize: vi.fn(), onFocusPane: vi.fn(), onOpenIssues: vi.fn(),
    canAddPane: () => true, onNextWaiting: vi.fn(),
  };
}

describe('Ctrl+Shift+J', () => {
  it('jumps to the next waiting pane', () => {
    const cb = callbacks();
    const e = new KeyboardEvent('keydown', { key: 'J', ctrlKey: true, shiftKey: true });
    createGlobalKeyHandler(cb)(e);
    expect(cb.onNextWaiting).toHaveBeenCalledTimes(1);
  });

  // Plain Ctrl+J is a newline in the terminal (Claude Code uses it).
  it('leaves plain Ctrl+J alone', () => {
    const cb = callbacks();
    createGlobalKeyHandler(cb)(new KeyboardEvent('keydown', { key: 'j', ctrlKey: true }));
    expect(cb.onNextWaiting).not.toHaveBeenCalled();
  });
});

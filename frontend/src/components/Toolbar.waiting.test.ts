import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';
import Toolbar from './Toolbar.svelte';
import type { AttentionEntry } from '../lib/attention';

vi.mock('../../wailsjs/go/backend/App', () => ({
  MarkSessionSeen: vi.fn().mockResolvedValue(undefined),
  MarkSessionUnseen: vi.fn().mockResolvedValue(undefined),
  SaveConfig: vi.fn().mockResolvedValue(undefined),
}));

const entry = {
  tabId: 't1', tabName: 'Tab', here: false, rank: 1, since: 1,
  pane: { id: 'p1', sessionId: 'h:1', name: 'P', activity: 'waitingAnswer', attention: { state: 'question', rank: 1, seen: false, detail: '', source: '', since: 1 } },
} as unknown as AttentionEntry;

afterEach(() => cleanup());

describe('Toolbar waiting popover', () => {
  it('closes on a click outside, stays open on a click inside', async () => {
    const { container } = render(Toolbar, { props: { waiting: [entry], showCounter: true } });
    await fireEvent.click(container.querySelector('.wait-counter')!);
    await tick();
    expect(container.querySelector('.wait-pop')).not.toBeNull();
    await fireEvent.click(container.querySelector('.wait-pop')!);
    expect(container.querySelector('.wait-pop')).not.toBeNull();
    await fireEvent.click(document.body);
    await tick();
    expect(container.querySelector('.wait-pop')).toBeNull();
  });
});

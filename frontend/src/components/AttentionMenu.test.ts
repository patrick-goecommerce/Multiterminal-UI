import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';
import AttentionMenu from './AttentionMenu.svelte';
import * as App from '../../wailsjs/go/backend/App';
import type { Pane } from '../stores/tabs';

vi.mock('../../wailsjs/go/backend/App', () => ({
  MarkSessionSeen: vi.fn().mockResolvedValue(undefined),
  MarkSessionUnseen: vi.fn().mockResolvedValue(undefined),
  SaveConfig: vi.fn().mockResolvedValue(undefined),
}));

function donePane(seen: boolean): Pane {
  return {
    id: 'pane-1', sessionId: 'h:1', name: 'P', mode: 'claude', model: '', focused: false, activity: 'done',
    cost: '', running: true, maximized: false, issueNumber: null, issueTitle: '', issueBranch: '',
    worktreePath: '', branch: '', targetBranch: '', zoomDelta: 0, background: false,
    display: 'terminal', conversationId: '', claudeSessionId: '', mcpProfile: '', autoName: '', sessionName: '',
    oscTitle: '', autoNameSource: '', userRenamed: false, finishPhase: '', activitySince: 0,
    attention: { state: 'done', rank: seen ? 5 : 3, seen, detail: '', source: '', since: 1 },
  };
}

function menuBox(width: number, height: number) {
  return vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(
    { x: 0, y: 0, left: 0, top: 0, right: width, bottom: height, width, height, toJSON: () => ({}) } as DOMRect);
}

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.clearAllMocks();
});

describe('AttentionMenu placement (R20)', () => {
  it('opens where it was asked when it fits', async () => {
    menuBox(250, 120);
    const { container } = render(AttentionMenu, { props: { pane: donePane(false), x: 100, y: 80 } });
    await tick();
    const el = container.querySelector('.am') as HTMLElement;
    expect(el.style.left).toBe('100px');
    expect(el.style.top).toBe('80px');
  });

  it('opens upward and leftward near the bottom-right corner', async () => {
    menuBox(250, 120);
    const x = window.innerWidth - 20;
    const y = window.innerHeight - 10;
    const { container } = render(AttentionMenu, { props: { pane: donePane(false), x, y } });
    await tick();
    const el = container.querySelector('.am') as HTMLElement;
    expect(el.style.left).toBe(`${x - 250}px`);
    expect(el.style.top).toBe(`${y - 120}px`);
  });
});

describe('AttentionMenu actions', () => {
  // R23: a terminal only takes the keyboard when no button holds it, so the
  // menu has to be gone and the focus released before the action runs.
  it('closes and lets go of the keyboard before it acts', async () => {
    let focusAtCall: Element | null = null;
    vi.mocked(App.MarkSessionSeen).mockImplementation(async () => { focusAtCall = document.activeElement; });
    const { getByText, component } = render(AttentionMenu, { props: { pane: donePane(false), x: 10, y: 10 } });
    const closed = vi.fn();
    component.$on('close', closed);
    const btn = getByText('Als gesehen markieren');
    btn.focus();
    await fireEvent.click(btn);
    await tick();

    expect(App.MarkSessionSeen).toHaveBeenCalledWith('h:1');
    expect(focusAtCall).not.toBe(btn);
    expect(closed.mock.invocationCallOrder[0]).toBeLessThan(vi.mocked(App.MarkSessionSeen).mock.invocationCallOrder[0]);
  });

  // R24: a failed call is logged, not swallowed.
  it('logs a failed MarkSessionSeen', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    const err = new Error('boom');
    vi.mocked(App.MarkSessionSeen).mockRejectedValue(err);
    const { getByText } = render(AttentionMenu, { props: { pane: donePane(false), x: 10, y: 10 } });
    await fireEvent.click(getByText('Als gesehen markieren'));
    await vi.waitFor(() => expect(warn).toHaveBeenCalledWith('[attention] MarkSessionSeen failed', 'h:1', err));
  });

  it('logs a failed MarkSessionUnseen', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {});
    const err = new Error('boom');
    vi.mocked(App.MarkSessionUnseen).mockRejectedValue(err);
    const { getByText } = render(AttentionMenu, { props: { pane: donePane(true), x: 10, y: 10 } });
    await fireEvent.click(getByText('Als ungelesen markieren'));
    await vi.waitFor(() => expect(warn).toHaveBeenCalledWith('[attention] MarkSessionUnseen failed', 'h:1', err));
  });
});

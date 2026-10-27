import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';
import WaitingPanel from './WaitingPanel.svelte';
import { briefings } from '../stores/briefings';
import type { AttentionEntry } from '../lib/attention';

const browserOpenURL = vi.hoisted(() => vi.fn());
vi.mock('../../wailsjs/runtime/runtime', () => ({ BrowserOpenURL: browserOpenURL }));

const entry = (rank: number, detail = ''): AttentionEntry => ({
  tabId: 't1', tabName: 'Shop', here: false, rank, since: 1,
  pane: { id: 'p1', sessionId: 'h:1', name: 'P', activity: 'done',
    attention: { state: 'done', rank, seen: false, detail, source: '', since: 1 } },
}) as unknown as AttentionEntry;

const briefing = (over: object = {}) => ({
  warnings: [], closing: 'Fertig.', notes: [], checks: [], built_at: 1,
  git: { branch_files: 2, branch_added: 5, branch_deleted: 1, commits: 1, on_target: true, target: 'main' },
  pr: { number: 42, url: 'https://example.test/pr/42', state: 'OPEN', checks: 'green', checks_passed: 3, checks_total: 3 },
  ...over,
}) as any;

beforeEach(() => {
  // jsdom has no ResizeObserver, which bind:clientHeight needs.
  vi.stubGlobal('ResizeObserver', class { observe() {} unobserve() {} disconnect() {} });
  vi.useFakeTimers();
  briefings.set({});
});
afterEach(() => { cleanup(); vi.useRealTimers(); vi.unstubAllGlobals(); browserOpenURL.mockClear(); });

describe('WaitingPanel briefing', () => {
  it('shows the verdict on line 2 of a done entry', () => {
    briefings.set({ 'h:1': briefing({ warnings: ['dirty'] }) });
    const { container } = render(WaitingPanel, { props: { rows: [entry(3)], total: 1 } });
    const l2 = container.querySelector('.l2')!;
    expect(l2.textContent).toBe('⚠ ungesichert');
    expect(l2.classList.contains('warn')).toBe(true);
  });

  it('keeps the host detail for a question entry', () => {
    briefings.set({ 'h:1': briefing() });
    const { container } = render(WaitingPanel, { props: { rows: [entry(1, 'Welche Datei?')], total: 1 } });
    expect(container.querySelector('.l2')!.textContent).toBe('Welche Datei?');
  });

  it('shows the card after 300 ms of hover and hides it on leave', async () => {
    briefings.set({ 'h:1': briefing() });
    const { container } = render(WaitingPanel, { props: { rows: [entry(3)], total: 1 } });
    const row = container.querySelector('.wp-entry')!;
    await fireEvent.mouseEnter(row);
    vi.advanceTimersByTime(299);
    await tick();
    expect(document.querySelector('.card')).toBeNull();
    vi.advanceTimersByTime(2);
    await tick();
    expect(document.querySelector('.card')).not.toBeNull();
    await fireEvent.mouseLeave(row);
    vi.advanceTimersByTime(200);
    await tick();
    expect(document.querySelector('.card')).toBeNull();
  });

  it('shows the card on keyboard focus at once and opens the PR from its number', async () => {
    briefings.set({ 'h:1': briefing() });
    const { container } = render(WaitingPanel, { props: { rows: [entry(3)], total: 1 } });
    await fireEvent.focus(container.querySelector('.wp-entry')!);
    await tick();
    const btn = document.querySelector('.card .prlink') as HTMLButtonElement;
    expect(btn.textContent).toBe('#42');
    await fireEvent.click(btn);
    expect(browserOpenURL).toHaveBeenCalledWith('https://example.test/pr/42');
  });

  it('a mouse press focuses the entry without opening the card at once', async () => {
    briefings.set({ 'h:1': briefing() });
    const { container } = render(WaitingPanel, { props: { rows: [entry(3)], total: 1 } });
    const row = container.querySelector('.wp-entry')!;
    await fireEvent.mouseEnter(row);
    await fireEvent.mouseDown(row);
    await fireEvent.focus(row);
    await fireEvent.mouseUp(row);
    await fireEvent.click(row);
    await tick();
    expect(document.querySelector('.card')).toBeNull();
    // the pending hover timer was not cancelled
    vi.advanceTimersByTime(301);
    await tick();
    expect(document.querySelector('.card')).not.toBeNull();
  });

  it('a press released outside the row does not block keyboard focus afterwards', async () => {
    briefings.set({ 'h:1': briefing() });
    const { container } = render(WaitingPanel, { props: { rows: [entry(3)], total: 1 } });
    const row = container.querySelector('.wp-entry')!;
    await fireEvent.mouseDown(row);
    await fireEvent.mouseUp(window);
    await fireEvent.focus(row);
    await tick();
    expect(document.querySelector('.card')).not.toBeNull();
  });

  it('clicking a second row while the first has focus does not open its card at once', async () => {
    const second = { ...entry(3), pane: { ...entry(3).pane, id: 'p2', sessionId: 'h:2' } } as AttentionEntry;
    briefings.set({ 'h:1': briefing(), 'h:2': briefing() });
    const { container } = render(WaitingPanel, { props: { rows: [entry(3), second], total: 2 } });
    const [a, b] = Array.from(container.querySelectorAll('.wp-entry'));
    await fireEvent.mouseDown(b);
    await fireEvent.blur(a);
    await fireEvent.focus(b);
    await tick();
    expect(document.querySelector('.card')).toBeNull();
    await fireEvent.mouseUp(b);
    await fireEvent.mouseEnter(b);
    vi.advanceTimersByTime(301);
    await tick();
    expect(document.querySelector('.card')).not.toBeNull();
  });

  it('refreshes the open card when its briefing is updated', async () => {
    briefings.set({ 'h:1': briefing() });
    const { container } = render(WaitingPanel, { props: { rows: [entry(3)], total: 1 } });
    await fireEvent.focus(container.querySelector('.wp-entry')!);
    await tick();
    expect(document.querySelector('.card')!.textContent).not.toContain('Neuer Schluss');
    briefings.set({ 'h:1': briefing({ closing: 'Neuer Schluss' }) });
    await tick();
    expect(document.querySelector('.card')!.textContent).toContain('Neuer Schluss');
  });

  it('shows no card without a briefing or for a rank 0 entry', async () => {
    const { container } = render(WaitingPanel, { props: { rows: [entry(3)], total: 1 } });
    await fireEvent.focus(container.querySelector('.wp-entry')!);
    await tick();
    expect(document.querySelector('.card')).toBeNull();
  });
});

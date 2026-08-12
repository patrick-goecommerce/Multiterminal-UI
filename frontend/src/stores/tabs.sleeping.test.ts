import { describe, it, expect, afterEach, vi } from 'vitest';
import { tabStore, computeTabActivity } from './tabs';

// Pane sleeping (issue #180, step 3): 'sleeping' and 'resuming' are lifecycle
// states, not activity classifications. They must reach the pane instantly and
// must never make a background tab claim attention.

function paneOf(tabId: string, sessionId: number) {
  const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
  return tab!.panes.find((p) => p.sessionId === sessionId)!;
}

afterEach(() => {
  vi.useRealTimers();
});

describe('computeTabActivity with sleeping panes', () => {
  it('ignores a sleeping pane', () => {
    expect(computeTabActivity([{ activity: 'sleeping' } as any])).toBeNull();
  });

  it('ignores a resuming pane', () => {
    expect(computeTabActivity([{ activity: 'resuming' } as any])).toBeNull();
  });

  it('does not let a sleeping pane mask a waiting one', () => {
    const panes = [
      { activity: 'sleeping' } as any,
      { activity: 'waitingPermission' } as any,
    ];
    expect(computeTabActivity(panes)).toBe('waitingPermission');
  });

  it('keeps done when the other pane sleeps', () => {
    const panes = [
      { activity: 'done' } as any,
      { activity: 'sleeping' } as any,
    ];
    expect(computeTabActivity(panes)).toBe('done');
  });
});

describe('updateActivity — sleeping bypasses the calm debounce', () => {
  it('applies "sleeping" immediately', () => {
    vi.useFakeTimers();
    const tabId = tabStore.addTab('Sleep');
    tabStore.addPane(tabId, 9101, 'Claude', 'claude', '');

    tabStore.updateActivity(9101, 'sleeping', '');
    // No timer advanced on purpose: the state must already be applied.
    expect(paneOf(tabId, 9101).activity).toBe('sleeping');
  });

  it('applies "resuming" immediately', () => {
    vi.useFakeTimers();
    const tabId = tabStore.addTab('Wake');
    tabStore.addPane(tabId, 9102, 'Claude', 'claude', '');

    tabStore.updateActivity(9102, 'resuming', '');
    expect(paneOf(tabId, 9102).activity).toBe('resuming');
  });

  it('cancels a pending calm transition when the pane falls asleep', () => {
    vi.useFakeTimers();
    const tabId = tabStore.addTab('SleepRace');
    tabStore.addPane(tabId, 9103, 'Claude', 'claude', '');

    tabStore.updateActivity(9103, 'done', '');   // debounced
    tabStore.updateActivity(9103, 'sleeping', ''); // must win
    vi.advanceTimersByTime(5000);

    expect(paneOf(tabId, 9103).activity).toBe('sleeping');
  });

  it('still debounces "done" (regression guard for the calm path)', () => {
    vi.useFakeTimers();
    const tabId = tabStore.addTab('Calm');
    tabStore.addPane(tabId, 9104, 'Claude', 'claude', '');

    tabStore.updateActivity(9104, 'active', '');
    tabStore.updateActivity(9104, 'done', '');
    expect(paneOf(tabId, 9104).activity).toBe('active'); // not yet settled
    vi.advanceTimersByTime(1000);
    expect(paneOf(tabId, 9104).activity).toBe('done');
  });
});

describe('updateActivity — the wake-up indicator bridges the replay', () => {
  it('keeps "resuming" while claude replays the transcript (~12–15 s of output)', () => {
    vi.useFakeTimers();
    const tabId = tabStore.addTab('Resuming');
    tabStore.addPane(tabId, 9105, 'Claude', 'claude', '');

    tabStore.updateActivity(9105, 'sleeping', '');
    tabStore.updateActivity(9105, 'resuming', '');
    tabStore.updateActivity(9105, 'active', '$0.30'); // replay output starts

    const pane = paneOf(tabId, 9105);
    expect(pane.activity).toBe('resuming');
    expect(pane.cost).toBe('$0.30'); // cost still tracks live
  });

  it('ends "resuming" once a settled state arrives', () => {
    vi.useFakeTimers();
    const tabId = tabStore.addTab('ResumingDone');
    tabStore.addPane(tabId, 9106, 'Claude', 'claude', '');

    tabStore.updateActivity(9106, 'resuming', '');
    tabStore.updateActivity(9106, 'active', '');
    tabStore.updateActivity(9106, 'done', '');
    vi.advanceTimersByTime(1000);

    expect(paneOf(tabId, 9106).activity).toBe('done');
  });

  it('lets an attention state interrupt the wake-up immediately', () => {
    vi.useFakeTimers();
    const tabId = tabStore.addTab('ResumingAsk');
    tabStore.addPane(tabId, 9107, 'Claude', 'claude', '');

    tabStore.updateActivity(9107, 'resuming', '');
    tabStore.updateActivity(9107, 'waitingPermission', '');
    vi.advanceTimersByTime(1000);

    expect(paneOf(tabId, 9107).activity).toBe('waitingPermission');
  });

  it('does not hold "resuming" for a pane that never slept', () => {
    vi.useFakeTimers();
    const tabId = tabStore.addTab('NoSleep');
    tabStore.addPane(tabId, 9108, 'Claude', 'claude', '');

    tabStore.updateActivity(9108, 'active', '');
    expect(paneOf(tabId, 9108).activity).toBe('active');
  });
});

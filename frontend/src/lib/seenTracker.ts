import type { SessionRef } from './sessionRef';

export interface SeenTarget { ref: SessionRef; rank: number; seen: boolean; since: number }

const keyOf = (t: SeenTarget) => `${t.ref}|${t.rank}|${t.since}`;

/** Only "error" and "done" are cleared by looking; a permission or a question
 *  stays until the agent moves on (spec Teil A). */
export function needsSeen(t: SeenTarget | null | undefined): t is SeenTarget {
  return !!t && !!t.ref && !t.seen && (t.rank === 2 || t.rank === 3);
}

/** The seen target for a pane, null without an attention record. */
export function seenTargetOf(pane: { sessionId: SessionRef; attention?: { rank: number; seen: boolean; since: number } } | null | undefined): SeenTarget | null {
  if (!pane || !pane.sessionId || !pane.attention) return null;
  return { ref: pane.sessionId, rank: pane.attention.rank, seen: pane.attention.seen, since: pane.attention.since };
}

export interface Timers { set: (fn: () => void, ms: number) => unknown; clear: (h: unknown) => void }

const realTimers: Timers = { set: (fn, ms) => setTimeout(fn, ms), clear: (h) => clearTimeout(h as ReturnType<typeof setTimeout>) };

/** Marks a finished or failed pane as seen once the user looked at it for
 *  `delayMs` with the window focused, or typed into it. */
export function createSeenTracker(markSeen: (ref: SessionRef) => void, delayMs = 1000, timers: Timers = realTimers) {
  let target: SeenTarget | null = null;
  let windowFocused = true;
  let timer: unknown = null;
  // Key of the state last reported, so one state is reported once however
  // long the host takes to confirm it.
  let lastMarked = '';

  const disarm = () => {
    if (timer !== null) { timers.clear(timer); timer = null; }
  };
  const arm = () => {
    if (timer !== null || !windowFocused || !needsSeen(target) || keyOf(target) === lastMarked) return;
    const key = keyOf(target);
    timer = timers.set(() => {
      timer = null;
      if (windowFocused && needsSeen(target) && keyOf(target) === key) {
        lastMarked = key;
        markSeen(target.ref);
      }
    }, delayMs);
  };

  return {
    /** The pane the user is looking at, or its state, changed. */
    look(next: SeenTarget | null) {
      if (next?.ref !== target?.ref) disarm();
      target = next;
      if (needsSeen(target)) arm(); else disarm();
    },
    windowFocus(focused: boolean) {
      windowFocused = focused;
      if (focused) arm(); else disarm();
    },
    typed(t: SeenTarget | null) {
      if (!needsSeen(t) || keyOf(t) === lastMarked) return;
      lastMarked = keyOf(t);
      markSeen(t.ref);
    },
  };
}

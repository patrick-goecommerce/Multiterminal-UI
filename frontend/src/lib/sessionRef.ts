/**
 * A session as the backend names it: "hub:id" (hub.Ref in Go). The hub half
 * says which session host owns it, the number is only unique on that host.
 *
 * Treat it as opaque: store it, compare it with ===, hand it back to a
 * binding. Never compute with it. '' means "no session" (a chat pane, a failed
 * start), which is why a plain truthiness check is the right test for one.
 */
export type SessionRef = string;

export const NO_SESSION: SessionRef = '';

/** The host-local number, for display ("Session 3") and nothing else. */
export function sessionNumber(ref: SessionRef): number {
  const n = Number(ref.slice(ref.lastIndexOf(':') + 1));
  return Number.isInteger(n) && n > 0 ? n : 0;
}

/**
 * Whether a saved pane's ref names a live session. A pane saved before refs
 * carried a hub has a bare number; that can only have meant the one host of
 * that time, so it matches by number.
 */
export function sameSession(saved: SessionRef, live: SessionRef): boolean {
  if (!saved || !live) return false;
  if (saved === live) return true;
  return !saved.includes(':') && sessionNumber(saved) === sessionNumber(live);
}

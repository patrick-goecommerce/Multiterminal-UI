import { createSeenTracker } from './seenTracker';
import type { SessionRef } from './sessionRef';
import * as App from '../../wailsjs/go/backend/App';

/** Tells the host a pane was seen; a failure is logged, not swallowed. */
export function markSessionSeen(ref: SessionRef) {
  App.MarkSessionSeen(ref).catch((err) => console.warn('[attention] MarkSessionSeen failed', ref, err));
}

/** The one tracker of this window: App.svelte feeds it what is looked at,
 *  TerminalPane feeds it typing, so both share the once-per-state memory. */
export const seenTracker = createSeenTracker(markSessionSeen);

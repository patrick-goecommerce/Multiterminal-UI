import { createSeenTracker } from './seenTracker';
import * as App from '../../wailsjs/go/backend/App';

/** The one tracker of this window: App.svelte feeds it what is looked at,
 *  TerminalPane feeds it typing, so both share the once-per-state memory. */
export const seenTracker = createSeenTracker((ref) => { App.MarkSessionSeen(ref).catch(() => {}); });

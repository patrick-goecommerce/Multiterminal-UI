import { RANK_DONE, RANK_ERROR, RANK_WORKING } from './attention';
import type { SessionRef } from './sessionRef';

// Panes restored after an app restart in embedded mode.
//
// Their sessions are new processes (claude --resume) without hook data yet,
// so the screen scan confirms the transcript's last frame: "done" for a
// visible prompt, "waitingAnswer" for a line above it ending in "?". Every
// restored pane would land in the box as unseen and every restored question
// would toast, since its state began after the window started. The first
// settled record of a restored pane therefore never toasts; a finished one
// counts as seen at once, a question stays listed (spec, offener Punkt 4).
// An error is different: the screen scan never yields one, so a first record
// of rank error is a failed restart (exit code), and it is handled like any
// other pane's, unseen and free to toast. A re-attached daemon session is not
// restored here: its host kept the record.

export interface Settle {
  /** Whether this record may toast at all (shouldToast still decides). */
  toast: boolean;
  /** Mark the session seen now. */
  markSeen: boolean;
}

const PASS: Settle = { toast: true, markSeen: false };

export function createRestoreSettle() {
  const pending = new Set<SessionRef>();
  return {
    /** A pane the restore created a new session for. */
    markRestored(ref: SessionRef) {
      if (ref) pending.add(ref);
    },
    /** What to do with an attention record. A restored ref is forgotten
     *  after its first record that is not "working": the relaunched CLI
     *  works through its boot first, and only the state after that is the
     *  one the transcript ended in. */
    settleRestored(ref: SessionRef, att: { rank: number }): Settle {
      if (!pending.has(ref)) return PASS;
      if (att.rank === RANK_WORKING) return { toast: false, markSeen: false };
      pending.delete(ref);
      if (att.rank === RANK_ERROR) return PASS;
      return { toast: false, markSeen: att.rank === RANK_DONE };
    },
  };
}

const restored = createRestoreSettle();

export const markRestored = restored.markRestored;
export const settleRestored = restored.settleRestored;

import type { Tab } from '../stores/tabs';
import type { SessionRef } from './sessionRef';

/** Tells the host each pane's merge target once per change, so the briefing can
 *  compare against it. Panes without a session or a target are skipped. */
export function createTargetSync(
  send: (ref: SessionRef, branch: string) => Promise<void>,
): (tabs: Tab[]) => void {
  const sent = new Map<SessionRef, string>();
  return (tabs) => {
    for (const tab of tabs) {
      for (const pane of tab.panes) {
        const ref = pane.sessionId;
        const branch = pane.targetBranch;
        if (!ref || !branch || sent.get(ref) === branch) continue;
        sent.set(ref, branch);
        // A failed send is forgotten so the next tabs change tries again.
        Promise.resolve(send(ref, branch)).catch(() => {
          if (sent.get(ref) === branch) sent.delete(ref);
        });
      }
    }
  };
}

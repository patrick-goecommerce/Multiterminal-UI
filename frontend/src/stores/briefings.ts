import { writable, get } from 'svelte/store';
import type { hub } from '../../wailsjs/go/models';
import * as App from '../../wailsjs/go/backend/App';
import type { SessionRef } from '../lib/sessionRef';

/** The host's latest review briefing per session (terminal:briefing). */
export const briefings = writable<Record<string, hub.Briefing>>({});

export function applyBriefing(ref: SessionRef, b: hub.Briefing): void {
  if (!ref || !b) return;
  briefings.update((m) => ({ ...m, [ref]: b }));
}

export function forgetBriefing(ref: SessionRef): void {
  briefings.update((m) => {
    if (!(ref in m)) return m;
    const { [ref]: _gone, ...rest } = m;
    return rest;
  });
}

/** Pull the briefings that already exist (a window opened later in daemon mode).
 *  Null answers and failures are skipped; an event that arrived meanwhile wins. */
export async function loadBriefings(
  refs: SessionRef[],
  fetch: (ref: SessionRef) => Promise<hub.Briefing | null> = App.GetSessionBriefing,
): Promise<void> {
  const results = await Promise.all(refs.filter(Boolean).map(async (ref) => {
    try { return [ref, await fetch(ref)] as const; } catch { return [ref, null] as const; }
  }));
  for (const [ref, b] of results) {
    if (!b) continue;
    const have = get(briefings)[ref];
    if (!have || (have.built_at ?? 0) < (b.built_at ?? 0)) applyBriefing(ref, b);
  }
}

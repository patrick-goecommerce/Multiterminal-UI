import { paneDisplayName, type Pane, type PaneAttention, type Tab } from '../stores/tabs';

// The frontend half of the attention model (spec 2026-10-07). The host decides
// the rank; this file only sorts, groups and walks what the host said.

export const RANK_PERMISSION = 0;
export const RANK_QUESTION = 1;
export const RANK_ERROR = 2;
export const RANK_DONE = 3;
export const RANK_WORKING = 4;
export const RANK_QUIET = 5;

export const ENTRY_LABEL: Record<number, string> = { 0: 'Freigabe nötig', 1: 'Frage', 2: 'Fehler', 3: 'fertig' };

/** A pane's rank: the host's, or, without a record (a daemon from before),
 *  read from the activity with "done" and "error" taken as seen. That is how
 *  the app behaved before, so an old daemon does not flood the list. */
export function paneRank(pane: Pane): number {
  if (pane.attention) return pane.attention.rank;
  switch (pane.activity) {
    case 'waitingPermission': return RANK_PERMISSION;
    case 'waitingAnswer': return RANK_QUESTION;
    case 'active': return RANK_WORKING;
    default: return RANK_QUIET;
  }
}

export type GlyphKind = 'permission' | 'question' | 'error' | 'errorSeen' | 'done' | 'doneSeen' | 'working' | 'quiet';

export const GLYPH_LABEL: Record<GlyphKind, string> = {
  permission: 'braucht Freigabe', question: 'hat eine Frage', error: 'Fehler', errorSeen: 'Fehler, gesehen',
  done: 'fertig, ungesehen', doneSeen: 'fertig', working: 'läuft', quiet: 'ruht',
};

/** Which status glyph a pane shows. The shape carries the meaning. */
export function glyphKind(pane: Pane): GlyphKind {
  const state = pane.attention?.state || pane.activity;
  const seen = pane.attention ? pane.attention.seen : true;
  switch (state) {
    case 'waitingPermission': return 'permission';
    case 'waitingAnswer': return 'question';
    case 'error': return seen ? 'errorSeen' : 'error';
    case 'done': return seen ? 'doneSeen' : 'done';
    case 'active': return 'working';
    default: return 'quiet';
  }
}

export interface AttentionEntry {
  tabId: string;
  tabName: string;
  /** The entry belongs to the tab currently shown. */
  here: boolean;
  pane: Pane;
  rank: number;
  /** Unix seconds; 0 = unknown. */
  since: number;
}

const sinceKey = (since: number) => since || Number.MAX_SAFE_INTEGER;

/** Everything that waits, from every tab, most urgent first; within a rank the
 *  longest waiting first. An unknown start sorts after the known ones. */
export function attentionList(tabs: Tab[], currentTabId: string): AttentionEntry[] {
  const out: AttentionEntry[] = [];
  for (const tab of tabs) {
    for (const pane of tab.panes) {
      const rank = paneRank(pane);
      if (rank > RANK_DONE) continue;
      out.push({ tabId: tab.id, tabName: tab.name, here: tab.id === currentTabId, pane, rank,
        since: pane.attention?.since || pane.activitySince || 0 });
    }
  }
  return out.sort((a, b) => a.rank - b.rank || sinceKey(a.since) - sinceKey(b.since) || a.pane.id.localeCompare(b.pane.id));
}

export interface PanelRows {
  rows: AttentionEntry[];
  hiddenDone: number;
  /** More finished entries than `keep`: only then is there a toggle. An
   *  expanded panel whose finished entries dropped below that has none. */
  collapsible: boolean;
}

/** What the panel shows: everything blocking or failed, and at most `keep`
 *  finished entries unless expanded. */
export function panelRows(list: AttentionEntry[], expanded: boolean, keep = 3): PanelRows {
  const top = list.filter((e) => e.rank < RANK_DONE);
  const done = list.filter((e) => e.rank === RANK_DONE);
  const shown = expanded ? done : done.slice(0, keep);
  return { rows: [...top, ...shown], hiddenDone: done.length - shown.length, collapsible: done.length > keep };
}

export interface AttentionCursor { rank: number; since: number; paneId: string }

export const cursorOf = (e: AttentionEntry): AttentionCursor => ({ rank: e.rank, since: sinceKey(e.since), paneId: e.pane.id });

function isAfter(e: AttentionEntry, c: AttentionCursor): boolean {
  if (e.rank !== c.rank) return e.rank > c.rank;
  const s = sinceKey(e.since);
  if (s !== c.since) return s > c.since;
  return e.pane.id.localeCompare(c.paneId) > 0;
}

/** The entry after the cursor, wrapping to the first. An entry that left the
 *  list (seen, answered) does not send the walk back to the top. */
export function nextEntry(list: AttentionEntry[], cursor: AttentionCursor | null): AttentionEntry | null {
  if (list.length === 0) return null;
  if (!cursor) return list[0];
  return list.find((e) => isAfter(e, cursor)) ?? list[0];
}

/** A tab's rank: its most urgent pane, whatever the pane order. */
export function tabRank(panes: Pane[]): number {
  return panes.reduce((min, p) => Math.min(min, paneRank(p)), RANK_QUIET);
}

/** One glyph per pane, at most `max`; `more` counts the rest. */
export function tabGlyphs(panes: Pane[], max = 6): { kinds: GlyphKind[]; more: number } {
  return { kinds: panes.slice(0, max).map(glyphKind), more: Math.max(0, panes.length - max) };
}

/** The tab tooltip: full name, then every pane with its state. */
export function tabTooltip(tab: Tab): string {
  return [tab.name, ...tab.panes.map((p) => `${paneDisplayName(p)}: ${GLYPH_LABEL[glyphKind(p)]}`)].join('\n');
}

/** A toast only for something new that blocks or failed, never for "done"
 *  and never for the pane the user is looking at in a focused window. */
export function shouldToast(before: PaneAttention | undefined, next: PaneAttention, lookedAt: boolean, notBefore: number = 0): boolean {
  if (next.rank > RANK_ERROR || next.seen || lookedAt) return false;
  // No earlier record: a state that began before this window started (a
  // reattach re-sends it) or of unknown start shows in the box, not as a toast.
  if (!before) return next.since > 0 && next.since >= notBefore;
  return before.rank !== next.rank || before.since !== next.since;
}

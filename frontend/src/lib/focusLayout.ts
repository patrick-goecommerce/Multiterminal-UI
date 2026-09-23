// Focus layout: the opt-in alternative to the pane grid (config layout.mode).
//
// Two panes of a tab are shown big, the most recently used one on the left.
// The rest of the tab sits small in a column on the right, and below them a
// list of panes in OTHER tabs that wait for the user. One of those can be
// opened as a floating window over the current tab, so answering it does not
// mean leaving the project.
//
// Everything here is pure: PaneGrid feeds it sizes and ids and renders the
// result, which is what keeps the terminals mounted while they move around.
import type { Pane, Tab } from '../stores/tabs';

export type LayoutMode = 'grid' | 'focus';

export function layoutModeOf(mode: string | undefined): LayoutMode {
  return mode === 'focus' ? 'focus' : 'grid';
}

// --- Order ------------------------------------------------------------------

/** The saved order, minus panes that are gone and duplicates, plus new ones
 *  at the end. The result always holds every id exactly once: an order longer
 *  than the pane list would give the last pane a slot that does not exist. */
export function reconcileOrder(order: string[] | undefined, ids: string[]): string[] {
  const present = new Set(ids);
  const known = new Set<string>();
  const kept: string[] = [];
  for (const id of order ?? []) {
    if (present.has(id) && !known.has(id)) {
      known.add(id);
      kept.push(id);
    }
  }
  return [...kept, ...ids.filter((id) => !known.has(id))];
}

/** Bring a pane to the big left slot. The old left one moves right, and the
 *  old right one becomes the first small one. A pane that is already big stays
 *  where it is (clicking into the right pane must not make it jump), unless it
 *  is new: the most recently opened pane belongs on the left. */
export function promoteInOrder(order: string[], id: string, isNew = false): string[] {
  const idx = order.indexOf(id);
  if (idx <= 0) return order;
  if (idx === 1 && !isNew) return order;
  return [id, ...order.slice(0, idx), ...order.slice(idx + 1)];
}

// --- Waiting panes in other tabs --------------------------------------------

export interface FloatingRef {
  tabId: string;
  paneId: string;
}

export interface WaitingEntry {
  tabId: string;
  tabName: string;
  pane: Pane;
}

export function isWaiting(activity: Pane['activity']): boolean {
  return activity === 'waitingPermission' || activity === 'waitingAnswer';
}

/** Panes outside `currentTabId` that wait for the user: a permission request
 *  or a question. Finished, working and failed panes are left out on purpose;
 *  the list is for what blocks an agent right now. Permission requests first,
 *  then whoever has been waiting longest. */
export function waitingElsewhere(tabs: Tab[], currentTabId: string): WaitingEntry[] {
  const out: WaitingEntry[] = [];
  for (const tab of tabs) {
    if (tab.id === currentTabId) continue;
    for (const pane of tab.panes) {
      if (isWaiting(pane.activity)) out.push({ tabId: tab.id, tabName: tab.name, pane });
    }
  }
  const rank = (p: Pane) => (p.activity === 'waitingPermission' ? 0 : 1);
  // 0 means "unknown since when"; sort those after the ones with a time.
  const since = (p: Pane) => p.activitySince || Number.MAX_SAFE_INTEGER;
  return out.sort((a, b) => rank(a.pane) - rank(b.pane) || since(a.pane) - since(b.pane));
}

/** The entry after `current`, wrapping around; the first one if `current` is
 *  not in the list. Null when nothing waits. */
export function nextWaiting(list: WaitingEntry[], current: FloatingRef | null): FloatingRef | null {
  if (list.length === 0) return null;
  const i = current ? list.findIndex((e) => e.tabId === current.tabId && e.pane.id === current.paneId) : -1;
  const next = list[(i + 1) % list.length];
  return { tabId: next.tabId, paneId: next.pane.id };
}

/** Whether the floating reference still points at something worth showing:
 *  the pane exists, and its tab is not the active one (there it has its slot). */
export function floatingStillValid(ref: FloatingRef | null, tabs: Tab[], activeTabId: string): boolean {
  if (!ref || ref.tabId === activeTabId) return false;
  const tab = tabs.find((t) => t.id === ref.tabId);
  return !!tab && tab.panes.some((p) => p.id === ref.paneId);
}

// --- Geometry ---------------------------------------------------------------

export interface Rect { x: number; y: number; w: number; h: number }

export interface FocusGeometry {
  big: Rect[];
  small: Rect[];
  column: Rect;
  list: Rect;
}

// Keep in sync with the .pane-grid padding and gap in PaneGrid.svelte.
export const FOCUS_PAD = 4;
export const FOCUS_GAP = 4;
const LIST_HEAD = 30;
const LIST_ROW = 46;
export const FLOAT_BAR = 32;

/** Where the big slots, the small slots and the waiting list go in a pane area
 *  of `w` x `h`. The right column is always there in focus mode: letting it
 *  appear only when something waits elsewhere would resize the big panes every
 *  time an agent in another project asked a question. */
export function focusGeometry(w: number, h: number, paneCount: number, waitingCount: number): FocusGeometry {
  const P = FOCUS_PAD, G = FOCUS_GAP;
  const innerH = Math.max(0, h - 2 * P);
  const colW = Math.round(Math.min(420, Math.max(240, w * 0.22)));
  const column: Rect = { x: Math.max(P, w - P - colW), y: P, w: colW, h: innerH };
  const bigW = Math.max(0, column.x - G - P);

  const bigCount = Math.min(2, paneCount);
  const big: Rect[] = [];
  if (bigCount === 1) big.push({ x: P, y: P, w: bigW, h: innerH });
  if (bigCount === 2) {
    const half = Math.floor((bigW - G) / 2);
    big.push({ x: P, y: P, w: half, h: innerH });
    big.push({ x: P + half + G, y: P, w: bigW - half - G, h: innerH });
  }

  const listH = Math.round(Math.min(innerH * 0.45, LIST_HEAD + Math.max(1, waitingCount) * LIST_ROW + 8));
  const list: Rect = { x: column.x, y: P + innerH - listH, w: colW, h: listH };

  const smallCount = Math.max(0, paneCount - 2);
  const small: Rect[] = [];
  const areaH = Math.max(0, innerH - listH - G);
  if (smallCount > 0) {
    const each = (areaH - (smallCount - 1) * G) / smallCount;
    for (let i = 0; i < smallCount; i++) {
      small.push({ x: column.x, y: Math.round(P + i * (each + G)), w: colW, h: Math.max(0, Math.round(each)) });
    }
  }
  return { big, small, column, list };
}

/** The floating window: 60 % x 72 % of the area, at the remembered position
 *  (negative = never moved, so centred), clamped so it stays fully reachable
 *  even when the window got smaller since the position was saved. */
export function floatingRect(w: number, h: number, x: number, y: number): Rect {
  const fw = Math.round(Math.min(w, Math.max(Math.min(480, w), w * 0.6)));
  const fh = Math.round(Math.min(h, Math.max(Math.min(300, h), h * 0.72)));
  const cx = x < 0 ? (w - fw) / 2 : x;
  const cy = y < 0 ? (h - fh) / 2 : y;
  return {
    x: Math.round(Math.max(0, Math.min(w - fw, cx))),
    y: Math.round(Math.max(0, Math.min(h - fh, cy))),
    w: fw,
    h: fh,
  };
}

export type SlotKind = 'big' | 'small' | 'float' | 'max' | 'hidden';

export interface Slot {
  kind: SlotKind;
  rect: Rect;
  /** small (and a small one hidden behind a maximized pane): the logical size
   *  the terminal keeps (the big slot's), and the factor it is drawn at. The PTY never learns about the small size, so a
   *  pane moving between big and small does not reflow its TUI. */
  logical?: { w: number; h: number };
  scale?: number;
}

const NOWHERE: Rect = { x: 0, y: 0, w: 0, h: 0 };

export function slotFor(
  id: string,
  order: string[],
  geo: FocusGeometry,
  area: { w: number; h: number },
  floating: { id: string; rect: Rect } | null,
  maximizedId: string,
): Slot {
  if (floating && floating.id === id) return { kind: 'float', rect: floating.rect };
  const full: Rect = { x: FOCUS_PAD, y: FOCUS_PAD, w: area.w - 2 * FOCUS_PAD, h: area.h - 2 * FOCUS_PAD };
  if (maximizedId === id) return { kind: 'max', rect: full };
  const own = ownSlot(id, order, geo);
  // Behind a maximized pane the others keep their size and are only hidden.
  // A zero-size slot would make the terminal fit itself, and its PTY, down to
  // two columns, and the TUI would come back rewrapped.
  return maximizedId ? { ...own, kind: 'hidden' } : own;
}

function ownSlot(id: string, order: string[], geo: FocusGeometry): Slot {
  const idx = order.indexOf(id);
  if (idx < 0) return { kind: 'hidden', rect: NOWHERE };
  if (idx < 2) return { kind: 'big', rect: geo.big[idx] ?? NOWHERE };
  const rect = geo.small[idx - 2] ?? NOWHERE;
  const ref = geo.big[1] ?? geo.big[0] ?? rect;
  const scale = ref.w > 0 ? rect.w / ref.w : 1;
  return { kind: 'small', rect, logical: { w: ref.w, h: ref.h }, scale };
}

/** The state as the small cards and the waiting list spell it. */
export function stateLabel(activity: Pane['activity']): string {
  switch (activity) {
    case 'waitingPermission': return 'braucht Freigabe';
    case 'waitingAnswer': return 'hat eine Frage';
    case 'active': return 'läuft';
    case 'done': return 'fertig';
    case 'error': return 'Fehler';
    case 'sleeping': return 'schläft';
    case 'resuming': return 'wacht auf';
    case 'starting': return 'startet';
    default: return 'bereit';
  }
}

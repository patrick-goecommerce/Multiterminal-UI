import { writable, type Readable } from 'svelte/store';
import type { Rect } from './briefing';

export interface CardAnchor { paneId: string; entry: Rect; panel: Rect }

/** Hover/focus timing of the briefing card: the pointer waits `showMs` before
 *  the card shows, keyboard focus shows it at once, leaving hides it after a
 *  short grace so the pointer can cross onto the card (its PR button). */
export function createCardHover(showMs: number, hideMs: number) {
  const shown = writable<CardAnchor | null>(null);
  let showTimer: ReturnType<typeof setTimeout> | undefined;
  let hideTimer: ReturnType<typeof setTimeout> | undefined;
  const clear = () => { clearTimeout(showTimer); clearTimeout(hideTimer); };
  const rect = (r: DOMRect): Rect => ({ left: r.left, top: r.top, width: r.width, height: r.height });

  return {
    shown: shown as Readable<CardAnchor | null>,
    enter(paneId: string, measure: () => { entry: DOMRect; panel: DOMRect }, delay: boolean) {
      clear();
      const open = () => { const m = measure(); shown.set({ paneId, entry: rect(m.entry), panel: rect(m.panel) }); };
      if (delay) showTimer = setTimeout(open, showMs); else open();
    },
    /** The pointer is on the card: do not hide. */
    keep() { clearTimeout(hideTimer); },
    leave() {
      clearTimeout(showTimer);
      clearTimeout(hideTimer);
      hideTimer = setTimeout(() => shown.set(null), hideMs);
    },
    destroy: clear,
  };
}

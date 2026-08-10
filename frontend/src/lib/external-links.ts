import { BrowserOpenURL } from '../../wailsjs/runtime/runtime';

// Every external link in MTUI must leave through here. WebView2 has no browser
// chrome: a plain `<a target="_blank">` (or window.open) makes Wails v3 spawn a
// bare WebView popup — the "web viewer" — because it never subscribes to
// NewWindowRequested. BrowserOpenURL hands the URL to the OS default browser.
const EXTERNAL_SCHEME = /^(?:https?|mailto):/i;

/**
 * True for schemes we are willing to hand to the OS. Deliberately a strict
 * allowlist: `javascript:`, `file:` and friends must never reach the shell,
 * and relative hrefs belong to the app itself.
 */
export function isExternalUrl(url: string): boolean {
  return EXTERNAL_SCHEME.test(url.trim());
}

// A single user activation can reach us more than once (WebView2 delivering a
// click alongside its own navigation attempt, a handler that also fires on an
// ancestor) — which shows up as the same page opening in two browser tabs.
// Collapse repeats of the same URL inside a short window.
const DUPLICATE_WINDOW_MS = 500;
let lastURL = '';
let lastOpenedAt = 0;

/** Open a URL in the user's default browser (no-op for non-external URLs). */
export function openExternal(url: string): void {
  if (!isExternalUrl(url)) return;
  const target = url.trim();

  const now = Date.now();
  if (target === lastURL && now - lastOpenedAt < DUPLICATE_WINDOW_MS) return;
  lastURL = target;
  lastOpenedAt = now;

  BrowserOpenURL(target);
}

const INTERCEPTED_EVENTS = ['click', 'auxclick'] as const;

/**
 * Catch every anchor click in the document and route external targets to the
 * default browser instead of letting WebView2 navigate or pop up. Installed
 * once per window; returns an uninstall function.
 */
export function installExternalLinkInterceptor(root: Document = document): () => void {
  const handle = (event: Event) => {
    const e = event as MouseEvent;
    if (e.defaultPrevented) return;
    // Left click and middle click only — right click opens the context menu.
    if (e.button !== 0 && e.button !== 1) return;

    const target = e.target;
    if (!(target instanceof Element)) return;
    const anchor = target.closest('a[href]');
    if (!anchor) return;

    // getAttribute, not .href: the DOM property resolves relative hrefs against
    // the WebView origin, which would make every in-app link look external.
    const href = anchor.getAttribute('href') ?? '';
    if (!isExternalUrl(href)) return;

    e.preventDefault();
    openExternal(href);
  };

  for (const type of INTERCEPTED_EVENTS) {
    root.addEventListener(type, handle, true);
  }
  return () => {
    for (const type of INTERCEPTED_EVENTS) {
      root.removeEventListener(type, handle, true);
    }
  };
}

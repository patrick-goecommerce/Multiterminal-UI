import { ClipboardGetText, ClipboardSetText } from '../../wailsjs/runtime/runtime';
import { encodeForPty } from './claude';
import * as App from '../../wailsjs/go/backend/App';
import type { Terminal } from '@xterm/xterm';

/** Wrap text in bracketed paste sequences if the terminal has the mode enabled. */
function bracketForPaste(text: string, terminal: Terminal | null): string {
  if (terminal && terminal.modes?.bracketedPasteMode) {
    return `\x1b[200~${text}\x1b[201~`;
  }
  return text;
}

/** Write text to the system clipboard, returning whether it ACTUALLY succeeded.
 *  Prefers the WebView2-native async Clipboard API (reliable and awaitable) and
 *  falls back to the Wails runtime. Callers MUST check the result before
 *  reporting success: the old fire-and-forget `ClipboardSetText(x)` silently
 *  kept the previous clipboard content on failure while the UI still showed
 *  "kopiert!" — the exact bug this replaces. */
export async function writeClipboard(text: string): Promise<boolean> {
  try {
    if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch (err) {
    console.warn('[clipboard] navigator.clipboard.writeText failed, falling back to Wails:', err);
  }
  try {
    await ClipboardSetText(text);
    return true;
  } catch (err) {
    console.error('[clipboard] ClipboardSetText failed:', err);
    return false;
  }
}

/** Read clipboard and write its content to the given PTY session. */
export async function pasteToSession(sessionId: number, terminal: Terminal | null = null): Promise<void> {
  try {
    const text = await ClipboardGetText();
    if (text) App.WriteToSession(sessionId, encodeForPty(bracketForPaste(text, terminal)));
  } catch (err) {
    console.error('[clipboard] paste failed:', err);
  }
}

/** Copy the current terminal selection to the clipboard. Returns whether the
 *  copy actually succeeded; the selection is cleared ONLY on success so a
 *  failed copy stays selected and can be retried. */
export async function copySelection(terminal: Terminal): Promise<boolean> {
  if (!terminal.hasSelection()) return false;
  const ok = await writeClipboard(terminal.getSelection());
  if (ok) terminal.clearSelection();
  return ok;
}

/** Encode and write arbitrary text to a PTY session. */
export function writeTextToSession(sessionId: number, text: string): void {
  App.WriteToSession(sessionId, encodeForPty(text));
}

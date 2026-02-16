import { ClipboardGetText, ClipboardSetText } from '../../wailsjs/runtime/runtime';
import { encodeForPty } from './claude';
import * as App from '../../wailsjs/go/backend/App';
import type { Terminal } from '@xterm/xterm';

/** Read clipboard and write its content to the given PTY session. */
export async function pasteToSession(sessionId: number): Promise<void> {
  try {
    const text = await ClipboardGetText();
    if (text) App.WriteToSession(sessionId, encodeForPty(text));
  } catch (err) {
    console.error('[clipboard] paste failed:', err);
  }
}

/** Copy the current terminal selection to clipboard, return true if copied. */
export function copySelection(terminal: Terminal): boolean {
  if (terminal.hasSelection()) {
    ClipboardSetText(terminal.getSelection());
    terminal.clearSelection();
    return true;
  }
  return false;
}

/** Encode and write arbitrary text to a PTY session. */
export function writeTextToSession(sessionId: number, text: string): void {
  App.WriteToSession(sessionId, encodeForPty(text));
}

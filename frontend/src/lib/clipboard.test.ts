import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock the Wails boundary so we can assert exactly what crosses it. vi.hoisted
// lets the spies be referenced inside the hoisted vi.mock factories.
const { writeToSession, clipboardGet, clipboardSet } = vi.hoisted(() => ({
  writeToSession: vi.fn(),
  clipboardGet: vi.fn(),
  clipboardSet: vi.fn(),
}));

vi.mock('../../wailsjs/go/backend/App', () => ({ WriteToSession: writeToSession }));
vi.mock('../../wailsjs/runtime/runtime', () => ({
  ClipboardGetText: clipboardGet,
  ClipboardSetText: clipboardSet,
}));

import { pasteToSession, copySelection, writeTextToSession } from './clipboard';
import { encodeForPty } from './claude';

// Mirror the backend WriteToSession decode (base64.StdEncoding.DecodeString +
// raw bytes → UTF-8) so a test asserts the exact string the PTY would receive.
function decodeAsBackend(b64: string): string {
  const binary = atob(b64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return new TextDecoder().decode(bytes);
}

/** The single base64 arg passed to the mocked WriteToSession. */
function lastSent(): string {
  const call = writeToSession.mock.calls.at(-1);
  return call ? call[1] : '';
}

function terminalWithBracketMode(on: boolean) {
  return { modes: { bracketedPasteMode: on } } as any;
}

beforeEach(() => {
  writeToSession.mockClear();
  clipboardGet.mockClear();
  clipboardSet.mockClear();
});

describe('encodeForPty (the encode half of the paste chain)', () => {
  // The whole point of the chain: whatever the user copies must reach the PTY
  // byte-for-byte. base64(UTF-8) must round-trip through the backend decode.
  const cases: Record<string, string> = {
    ascii: 'hello world',
    umlauts: 'Grüße über Ötzi',
    cjk_and_emoji: '世界 🚀 café',
    unix_newlines: 'Zeile1\nZeile2\nZeile3',
    crlf_newlines: 'Zeile1\r\nZeile2',
    bracketed_paste_escapes: '\x1b[200~payload\x1b[201~',
    empty: '',
    tabs_and_spaces: '\tא\t  b',
  };

  for (const [name, input] of Object.entries(cases)) {
    it(`round-trips ${name} through the backend decode`, () => {
      expect(decodeAsBackend(encodeForPty(input))).toBe(input);
    });
  }

  it('matches the shared cross-language vector (Go app_write_test.go decodes the same literal)', () => {
    // If encodeForPty ever changes encoding (e.g. base64url, or dropping UTF-8),
    // this and the Go-side TestWriteToSessionDecode_CrossLanguageVector both break.
    expect(encodeForPty('Grüße über Ötzi')).toBe('R3LDvMOfZSDDvGJlciDDlnR6aQ==');
  });
});

describe('pasteToSession (clipboard → PTY)', () => {
  it('sends clipboard text to the session, decoding back to the original', async () => {
    clipboardGet.mockResolvedValue('foo\nbar');
    await pasteToSession(7, null);
    expect(writeToSession).toHaveBeenCalledTimes(1);
    expect(writeToSession.mock.calls[0][0]).toBe(7);
    expect(decodeAsBackend(lastSent())).toBe('foo\nbar');
  });

  it('preserves unicode across the boundary (no mojibake)', async () => {
    clipboardGet.mockResolvedValue('café ☕ 世界 🚀');
    await pasteToSession(1, terminalWithBracketMode(false));
    expect(decodeAsBackend(lastSent())).toBe('café ☕ 世界 🚀');
  });

  it('wraps the paste in bracketed-paste escapes when the terminal has the mode on', async () => {
    clipboardGet.mockResolvedValue('rm -rf');
    await pasteToSession(1, terminalWithBracketMode(true));
    expect(decodeAsBackend(lastSent())).toBe('\x1b[200~rm -rf\x1b[201~');
  });

  it('does NOT wrap when bracketed-paste mode is off', async () => {
    clipboardGet.mockResolvedValue('rm -rf');
    await pasteToSession(1, terminalWithBracketMode(false));
    expect(decodeAsBackend(lastSent())).toBe('rm -rf');
  });

  it('does nothing on empty clipboard (no write)', async () => {
    clipboardGet.mockResolvedValue('');
    await pasteToSession(1, null);
    expect(writeToSession).not.toHaveBeenCalled();
  });

  it('swallows clipboard errors without throwing (paste never crashes the pane)', async () => {
    clipboardGet.mockRejectedValue(new Error('clipboard unavailable'));
    await expect(pasteToSession(1, null)).resolves.toBeUndefined();
    expect(writeToSession).not.toHaveBeenCalled();
  });
});

describe('copySelection (terminal → clipboard)', () => {
  it('copies the selection, clears it, and reports success', () => {
    const clearSelection = vi.fn();
    const term = { hasSelection: () => true, getSelection: () => 'selected text', clearSelection } as any;
    expect(copySelection(term)).toBe(true);
    expect(clipboardSet).toHaveBeenCalledWith('selected text');
    expect(clearSelection).toHaveBeenCalledTimes(1);
  });

  it('does nothing and reports false when there is no selection', () => {
    const clearSelection = vi.fn();
    const term = { hasSelection: () => false, getSelection: () => '', clearSelection } as any;
    expect(copySelection(term)).toBe(false);
    expect(clipboardSet).not.toHaveBeenCalled();
    expect(clearSelection).not.toHaveBeenCalled();
  });
});

describe('writeTextToSession (programmatic inject)', () => {
  it('encodes text so the backend decodes it unchanged', () => {
    writeTextToSession(3, 'echo "Grüße 🚀"\n');
    expect(writeToSession).toHaveBeenCalledTimes(1);
    expect(writeToSession.mock.calls[0][0]).toBe(3);
    expect(decodeAsBackend(lastSent())).toBe('echo "Grüße 🚀"\n');
  });
});

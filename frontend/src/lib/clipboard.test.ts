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

import { pasteToSession, copySelection, writeTextToSession, writeClipboard, decodeOsc52, registerOsc52Handler } from './clipboard';
import { encodeForPty } from './claude';

// The WebView2-native clipboard API is the preferred write path; jsdom has no
// real one, so install a controllable stub.
const clipboardWriteText = vi.fn();

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
  clipboardSet.mockReset();
  clipboardSet.mockResolvedValue(undefined);
  clipboardWriteText.mockReset();
  clipboardWriteText.mockResolvedValue(undefined);
  Object.defineProperty(globalThis.navigator, 'clipboard', {
    value: { writeText: clipboardWriteText },
    configurable: true,
    writable: true,
  });
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

describe('writeClipboard (the actual clipboard write)', () => {
  it('prefers navigator.clipboard and does not fall back on success', async () => {
    expect(await writeClipboard('hello')).toBe(true);
    expect(clipboardWriteText).toHaveBeenCalledWith('hello');
    expect(clipboardSet).not.toHaveBeenCalled();
  });

  it('falls back to the Wails runtime when navigator.clipboard rejects', async () => {
    clipboardWriteText.mockRejectedValue(new Error('not allowed'));
    expect(await writeClipboard('hello')).toBe(true);
    expect(clipboardSet).toHaveBeenCalledWith('hello');
  });

  it('reports FAILURE when both write paths fail (no false success)', async () => {
    clipboardWriteText.mockRejectedValue(new Error('not allowed'));
    clipboardSet.mockRejectedValue(new Error('wails failed'));
    expect(await writeClipboard('hello')).toBe(false);
  });
});

describe('copySelection (terminal → clipboard)', () => {
  it('copies the selection, clears it, and reports success', async () => {
    const clearSelection = vi.fn();
    const term = { hasSelection: () => true, getSelection: () => 'selected text', clearSelection } as any;
    expect(await copySelection(term)).toBe(true);
    expect(clipboardWriteText).toHaveBeenCalledWith('selected text');
    expect(clearSelection).toHaveBeenCalledTimes(1);
  });

  it('reports failure and KEEPS the selection when the clipboard write fails', async () => {
    // The reported bug: UI claimed success while the clipboard kept its old
    // content. A failed write must return false AND leave the selection intact
    // so the user can retry — never a silent false success.
    clipboardWriteText.mockRejectedValue(new Error('not allowed'));
    clipboardSet.mockRejectedValue(new Error('wails failed'));
    const clearSelection = vi.fn();
    const term = { hasSelection: () => true, getSelection: () => 'x', clearSelection } as any;
    expect(await copySelection(term)).toBe(false);
    expect(clearSelection).not.toHaveBeenCalled();
  });

  it('does nothing and reports false when there is no selection', async () => {
    const clearSelection = vi.fn();
    const term = { hasSelection: () => false, getSelection: () => '', clearSelection } as any;
    expect(await copySelection(term)).toBe(false);
    expect(clipboardWriteText).not.toHaveBeenCalled();
    expect(clearSelection).not.toHaveBeenCalled();
  });
});

describe('decodeOsc52 (OSC 52 payload decoding)', () => {
  it('decodes a base64 clipboard payload', () => {
    expect(decodeOsc52('c;aGVsbG8=')).toBe('hello');
  });

  it('decodes UTF-8 multi-byte content', () => {
    expect(decodeOsc52('c;Y2Fmw6k=')).toBe('café');
  });

  it('returns null for a query ("?"), which callers must not write back', () => {
    expect(decodeOsc52('c;?')).toBeNull();
  });

  it('returns null for data with no selection separator', () => {
    expect(decodeOsc52('aGVsbG8=')).toBeNull();
  });

  it('returns null for invalid base64 instead of throwing', () => {
    expect(decodeOsc52('c;not-valid-base64!!!')).toBeNull();
  });
});

describe('registerOsc52Handler (PTY-issued OSC 52 -> real clipboard)', () => {
  // The reported bug: a CLI running inside the pane (e.g. Claude Code's own
  // "copied N chars to clipboard" status line) writes via the OSC 52 escape
  // sequence. xterm.js does not act on OSC 52 out of the box, so without a
  // registered handler the sequence is silently dropped: the CLI believes it
  // succeeded while nothing reaches the real OS clipboard.
  function fakeTerminalCapturingOscHandler() {
    let handler: ((data: string) => boolean | Promise<boolean>) | undefined;
    const terminal = {
      parser: {
        registerOscHandler: vi.fn((_ident: number, cb: typeof handler) => {
          handler = cb;
          return { dispose: vi.fn() };
        }),
      },
    } as any;
    return { terminal, invoke: (data: string) => handler!(data) };
  }

  it('writes the decoded payload to the clipboard', async () => {
    const { terminal, invoke } = fakeTerminalCapturingOscHandler();
    registerOsc52Handler(terminal);
    invoke('c;aGVsbG8=');
    await Promise.resolve();
    expect(clipboardWriteText).toHaveBeenCalledWith('hello');
  });

  it('registers on OSC ident 52', () => {
    const { terminal } = fakeTerminalCapturingOscHandler();
    registerOsc52Handler(terminal);
    expect(terminal.parser.registerOscHandler).toHaveBeenCalledWith(52, expect.any(Function));
  });

  it('ignores a clipboard query without writing', async () => {
    const { terminal, invoke } = fakeTerminalCapturingOscHandler();
    registerOsc52Handler(terminal);
    invoke('c;?');
    await Promise.resolve();
    expect(clipboardWriteText).not.toHaveBeenCalled();
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

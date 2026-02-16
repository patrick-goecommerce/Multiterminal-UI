import { WebLinksAddon } from '@xterm/addon-web-links';
import type { Terminal, ILinkProvider, ILink, IDisposable } from '@xterm/xterm';

// URL regex — NO g flag! The WebLinksAddon adds it internally;
// a duplicate 'g' causes a SyntaxError and breaks all link detection.
export const URL_REGEX = /https?:\/\/[^\s'")\]>]+[^\s'")\]>.,;:!?]/;

// Localhost URL regex (with g flag) — used for auto-open detection in terminal output
export const LOCALHOST_REGEX = /https?:\/\/(?:localhost|127\.0\.0\.1|0\.0\.0\.0)(?::\d+)(?:[^\s'")\]>]*[^\s'")\]>.,;:!?])?/g;

// File path regex: directory separator + file extension, optional :line:col
export const FILE_PATH_REGEX = /(?:[A-Z]:\\|\/(?!\/)|\.{0,2}[\\/])?[\w.-]+(?:[\\/][\w.-]+)+\.\w+(?::\d+(?::\d+)?)?/;

// Combined regex (with g flag) — only used in tests
export const LINK_REGEX = new RegExp(`(${URL_REGEX.source}|${FILE_PATH_REGEX.source})`, 'g');

export function isUrl(uri: string): boolean {
  return /^https?:\/\//.test(uri);
}

export type LinkHandler = (event: MouseEvent, uri: string) => void;

export function createWebLinksAddon(handler: LinkHandler): WebLinksAddon {
  return new WebLinksAddon(
    (event: MouseEvent, uri: string) => {
      if (!event.ctrlKey) return;
      handler(event, uri);
    },
    { urlRegex: URL_REGEX }
  );
}

/**
 * Link provider for file paths. The WebLinksAddon rejects non-URL matches
 * via its internal isUrl() check, so file paths need a separate provider.
 */
class FileLinkProvider implements ILinkProvider {
  constructor(
    private _terminal: Terminal,
    private _regex: RegExp,
    private _handler: LinkHandler
  ) {}

  provideLinks(y: number, callback: (links: ILink[] | undefined) => void): void {
    const line = this._terminal.buffer.active.getLine(y - 1);
    if (!line) { callback(undefined); return; }

    const text = line.translateToString(true);
    const rex = new RegExp(this._regex.source, 'g');
    let match;
    const links: ILink[] = [];

    while ((match = rex.exec(text)) !== null) {
      if (isUrl(match[0])) continue; // URLs handled by WebLinksAddon
      const m = match[0];
      links.push({
        range: {
          start: { x: match.index + 1, y },
          end: { x: match.index + m.length, y },
        },
        text: m,
        activate: (event: MouseEvent, uri: string) => {
          if (!event.ctrlKey) return;
          this._handler(event, uri);
        },
      });
    }

    callback(links.length > 0 ? links : undefined);
  }
}

export function registerFileLinkProvider(terminal: Terminal, handler: LinkHandler): IDisposable {
  return terminal.registerLinkProvider(new FileLinkProvider(terminal, FILE_PATH_REGEX, handler));
}

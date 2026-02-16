import { WebLinksAddon } from '@xterm/addon-web-links';

// Combined regex: HTTP(S) URLs | file paths (with optional :line:col)
// File paths must contain at least one directory separator and a file extension.
export const LINK_REGEX = /(https?:\/\/[^\s'")\]>]+|(?:[A-Z]:\\|\.{0,2}[\\/])?[\w.-]+(?:[\\/][\w.-]+)+\.\w+(?::\d+(?::\d+)?)?)/g;

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
    { urlRegex: LINK_REGEX }
  );
}

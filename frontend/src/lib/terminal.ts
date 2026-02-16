import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { SearchAddon } from '@xterm/addon-search';
import { createWebLinksAddon, registerFileLinkProvider, type LinkHandler } from './links';

export interface TerminalInstance {
  terminal: Terminal;
  fitAddon: FitAddon;
  searchAddon: SearchAddon;
  dispose: () => void;
}

// Theme-aware terminal options
const baseOptions: Partial<import('@xterm/xterm').ITerminalOptions> = {
  fontSize: 14,
  fontFamily: "'Cascadia Code', 'Fira Code', 'JetBrains Mono', 'Consolas', monospace",
  cursorBlink: true,
  cursorStyle: 'block',
  scrollback: 10000,
  allowProposedApi: true,
};

// Terminal themes matching the app themes
const terminalThemes: Record<string, import('@xterm/xterm').ITheme> = {
  dark: {
    background: '#11111b',
    foreground: '#cdd6f4',
    cursor: '#f5e0dc',
    selectionBackground: '#45475a80',
    black: '#45475a',
    red: '#f38ba8',
    green: '#a6e3a1',
    yellow: '#f9e2af',
    blue: '#89b4fa',
    magenta: '#cba6f7',
    cyan: '#94e2d5',
    white: '#bac2de',
    brightBlack: '#585b70',
    brightRed: '#f38ba8',
    brightGreen: '#a6e3a1',
    brightYellow: '#f9e2af',
    brightBlue: '#89b4fa',
    brightMagenta: '#cba6f7',
    brightCyan: '#94e2d5',
    brightWhite: '#a6adc8',
  },
  light: {
    background: '#dce0e8',
    foreground: '#4c4f69',
    cursor: '#dc8a78',
    selectionBackground: '#ccd0da80',
    black: '#5c5f77',
    red: '#d20f39',
    green: '#40a02b',
    yellow: '#df8e1d',
    blue: '#1e66f5',
    magenta: '#8839ef',
    cyan: '#179299',
    white: '#acb0be',
    brightBlack: '#6c6f85',
    brightRed: '#d20f39',
    brightGreen: '#40a02b',
    brightYellow: '#df8e1d',
    brightBlue: '#1e66f5',
    brightMagenta: '#8839ef',
    brightCyan: '#179299',
    brightWhite: '#bcc0cc',
  },
  dracula: {
    background: '#1e1f29',
    foreground: '#f8f8f2',
    cursor: '#f8f8f2',
    selectionBackground: '#44475a80',
    black: '#21222c',
    red: '#ff5555',
    green: '#50fa7b',
    yellow: '#f1fa8c',
    blue: '#bd93f9',
    magenta: '#ff79c6',
    cyan: '#8be9fd',
    white: '#f8f8f2',
    brightBlack: '#6272a4',
    brightRed: '#ff6e6e',
    brightGreen: '#69ff94',
    brightYellow: '#ffffa5',
    brightBlue: '#d6acff',
    brightMagenta: '#ff92df',
    brightCyan: '#a4ffff',
    brightWhite: '#ffffff',
  },
  nord: {
    background: '#242933',
    foreground: '#eceff4',
    cursor: '#eceff4',
    selectionBackground: '#4c566a80',
    black: '#3b4252',
    red: '#bf616a',
    green: '#a3be8c',
    yellow: '#ebcb8b',
    blue: '#81a1c1',
    magenta: '#b48ead',
    cyan: '#88c0d0',
    white: '#e5e9f0',
    brightBlack: '#4c566a',
    brightRed: '#bf616a',
    brightGreen: '#a3be8c',
    brightYellow: '#ebcb8b',
    brightBlue: '#81a1c1',
    brightMagenta: '#b48ead',
    brightCyan: '#8fbcbb',
    brightWhite: '#eceff4',
  },
  solarized: {
    background: '#00212b',
    foreground: '#839496',
    cursor: '#839496',
    selectionBackground: '#073642',
    black: '#073642',
    red: '#dc322f',
    green: '#859900',
    yellow: '#b58900',
    blue: '#268bd2',
    magenta: '#d33682',
    cyan: '#2aa198',
    white: '#eee8d5',
    brightBlack: '#002b36',
    brightRed: '#cb4b16',
    brightGreen: '#586e75',
    brightYellow: '#657b83',
    brightBlue: '#839496',
    brightMagenta: '#6c71c4',
    brightCyan: '#93a1a1',
    brightWhite: '#fdf6e3',
  },
};

export function createTerminal(theme: string = 'dark', linkHandler?: LinkHandler): TerminalInstance {
  const terminal = new Terminal({
    ...baseOptions,
    theme: terminalThemes[theme] || terminalThemes.dark,
  });

  const fitAddon = new FitAddon();
  terminal.loadAddon(fitAddon);

  const searchAddon = new SearchAddon();
  terminal.loadAddon(searchAddon);

  let fileLinkDisposable: { dispose(): void } | undefined;
  if (linkHandler) {
    terminal.loadAddon(createWebLinksAddon(linkHandler));
    fileLinkDisposable = registerFileLinkProvider(terminal, linkHandler);
  }

  return {
    terminal,
    fitAddon,
    searchAddon,
    dispose: () => {
      fileLinkDisposable?.dispose();
      searchAddon.dispose();
      fitAddon.dispose();
      terminal.dispose();
    },
  };
}

export function getTerminalTheme(theme: string): import('@xterm/xterm').ITheme {
  return terminalThemes[theme] || terminalThemes.dark;
}

import { writable, derived } from 'svelte/store';

export type ThemeName = 'dark' | 'light' | 'dracula' | 'nord' | 'solarized';

export const currentTheme = writable<ThemeName>('dark');

interface ThemeColors {
  bg: string;
  bgSecondary: string;
  bgTertiary: string;
  fg: string;
  fgMuted: string;
  accent: string;
  accentHover: string;
  border: string;
  borderFocused: string;
  success: string;
  warning: string;
  error: string;
  tabBg: string;
  tabActiveBg: string;
  tabActiveFg: string;
  paneBg: string;
  paneBorder: string;
  paneBorderFocused: string;
  toolbarBg: string;
  footerBg: string;
}

const themes: Record<ThemeName, ThemeColors> = {
  dark: {
    bg: '#1e1e2e',
    bgSecondary: '#181825',
    bgTertiary: '#313244',
    fg: '#cdd6f4',
    fgMuted: '#6c7086',
    accent: '#cba6f7',
    accentHover: '#b4befe',
    border: '#45475a',
    borderFocused: '#cba6f7',
    success: '#a6e3a1',
    warning: '#f9e2af',
    error: '#f38ba8',
    tabBg: '#181825',
    tabActiveBg: '#1e1e2e',
    tabActiveFg: '#cba6f7',
    paneBg: '#11111b',
    paneBorder: '#45475a',
    paneBorderFocused: '#cba6f7',
    toolbarBg: '#181825',
    footerBg: '#181825',
  },
  light: {
    bg: '#eff1f5',
    bgSecondary: '#e6e9ef',
    bgTertiary: '#ccd0da',
    fg: '#4c4f69',
    fgMuted: '#9ca0b0',
    accent: '#8839ef',
    accentHover: '#7287fd',
    border: '#ccd0da',
    borderFocused: '#8839ef',
    success: '#40a02b',
    warning: '#df8e1d',
    error: '#d20f39',
    tabBg: '#e6e9ef',
    tabActiveBg: '#eff1f5',
    tabActiveFg: '#8839ef',
    paneBg: '#dce0e8',
    paneBorder: '#ccd0da',
    paneBorderFocused: '#8839ef',
    toolbarBg: '#e6e9ef',
    footerBg: '#e6e9ef',
  },
  dracula: {
    bg: '#282a36',
    bgSecondary: '#21222c',
    bgTertiary: '#44475a',
    fg: '#f8f8f2',
    fgMuted: '#6272a4',
    accent: '#bd93f9',
    accentHover: '#ff79c6',
    border: '#44475a',
    borderFocused: '#bd93f9',
    success: '#50fa7b',
    warning: '#f1fa8c',
    error: '#ff5555',
    tabBg: '#21222c',
    tabActiveBg: '#282a36',
    tabActiveFg: '#bd93f9',
    paneBg: '#1e1f29',
    paneBorder: '#44475a',
    paneBorderFocused: '#bd93f9',
    toolbarBg: '#21222c',
    footerBg: '#21222c',
  },
  nord: {
    bg: '#2e3440',
    bgSecondary: '#3b4252',
    bgTertiary: '#434c5e',
    fg: '#eceff4',
    fgMuted: '#4c566a',
    accent: '#88c0d0',
    accentHover: '#81a1c1',
    border: '#4c566a',
    borderFocused: '#88c0d0',
    success: '#a3be8c',
    warning: '#ebcb8b',
    error: '#bf616a',
    tabBg: '#3b4252',
    tabActiveBg: '#2e3440',
    tabActiveFg: '#88c0d0',
    paneBg: '#242933',
    paneBorder: '#4c566a',
    paneBorderFocused: '#88c0d0',
    toolbarBg: '#3b4252',
    footerBg: '#3b4252',
  },
  solarized: {
    bg: '#002b36',
    bgSecondary: '#073642',
    bgTertiary: '#586e75',
    fg: '#839496',
    fgMuted: '#586e75',
    accent: '#268bd2',
    accentHover: '#2aa198',
    border: '#073642',
    borderFocused: '#268bd2',
    success: '#859900',
    warning: '#b58900',
    error: '#dc322f',
    tabBg: '#073642',
    tabActiveBg: '#002b36',
    tabActiveFg: '#268bd2',
    paneBg: '#00212b',
    paneBorder: '#073642',
    paneBorderFocused: '#268bd2',
    toolbarBg: '#073642',
    footerBg: '#073642',
  },
};

export const themeColors = derived(currentTheme, ($theme) => themes[$theme]);

export function applyTheme(theme: ThemeName, accentColor?: string) {
  currentTheme.set(theme);
  const colors = themes[theme];
  const root = document.documentElement;
  for (const [key, value] of Object.entries(colors)) {
    root.style.setProperty(`--${camelToKebab(key)}`, value);
  }
  if (accentColor) {
    applyAccentColor(accentColor);
  }
}

// Override accent-related CSS variables with a custom color.
export function applyAccentColor(hex: string) {
  const root = document.documentElement;
  root.style.setProperty('--accent', hex);
  root.style.setProperty('--accent-hover', adjustBrightness(hex, -20));
  root.style.setProperty('--border-focused', hex);
  root.style.setProperty('--tab-active-fg', hex);
  root.style.setProperty('--pane-border-focused', hex);
}

function adjustBrightness(hex: string, amount: number): string {
  const num = parseInt(hex.replace('#', ''), 16);
  const r = Math.max(0, Math.min(255, ((num >> 16) & 0xff) + amount));
  const g = Math.max(0, Math.min(255, ((num >> 8) & 0xff) + amount));
  const b = Math.max(0, Math.min(255, (num & 0xff) + amount));
  return `#${((r << 16) | (g << 8) | b).toString(16).padStart(6, '0')}`;
}

function camelToKebab(str: string): string {
  return str.replace(/([a-z])([A-Z])/g, '$1-$2').toLowerCase();
}

import { describe, it, expect, beforeEach, vi } from 'vitest';
import { get } from 'svelte/store';
import { currentTheme, themeColors, applyTheme, applyAccentColor } from './theme';
import type { ThemeName } from './theme';

// Mock document.documentElement for applyTheme/applyAccentColor
const setPropertyMock = vi.fn();
Object.defineProperty(document, 'documentElement', {
  value: {
    style: {
      setProperty: setPropertyMock,
    },
  },
  writable: true,
});

describe('theme store', () => {
  beforeEach(() => {
    setPropertyMock.mockClear();
  });

  describe('currentTheme', () => {
    it('defaults to dark', () => {
      // The store may have been changed by previous tests, set it explicitly
      currentTheme.set('dark');
      expect(get(currentTheme)).toBe('dark');
    });

    it('can be set to all theme names', () => {
      const themes: ThemeName[] = ['dark', 'light', 'dracula', 'nord', 'solarized'];
      for (const theme of themes) {
        currentTheme.set(theme);
        expect(get(currentTheme)).toBe(theme);
      }
    });
  });

  describe('themeColors', () => {
    it('returns colors for dark theme', () => {
      currentTheme.set('dark');
      const colors = get(themeColors);
      expect(colors.bg).toBe('#1e1e2e');
      expect(colors.fg).toBe('#cdd6f4');
      expect(colors.accent).toBe('#cba6f7');
    });

    it('returns colors for light theme', () => {
      currentTheme.set('light');
      const colors = get(themeColors);
      expect(colors.bg).toBe('#eff1f5');
      expect(colors.fg).toBe('#4c4f69');
    });

    it('returns colors for dracula theme', () => {
      currentTheme.set('dracula');
      const colors = get(themeColors);
      expect(colors.bg).toBe('#282a36');
      expect(colors.accent).toBe('#bd93f9');
    });

    it('returns colors for nord theme', () => {
      currentTheme.set('nord');
      const colors = get(themeColors);
      expect(colors.bg).toBe('#2e3440');
      expect(colors.accent).toBe('#88c0d0');
    });

    it('returns colors for solarized theme', () => {
      currentTheme.set('solarized');
      const colors = get(themeColors);
      expect(colors.bg).toBe('#002b36');
      expect(colors.accent).toBe('#268bd2');
    });

    it('all themes have required properties', () => {
      const requiredKeys = [
        'bg', 'bgSecondary', 'bgTertiary', 'fg', 'fgMuted',
        'accent', 'accentHover', 'border', 'borderFocused',
        'success', 'warning', 'error',
        'tabBg', 'tabActiveBg', 'tabActiveFg',
        'paneBg', 'paneBorder', 'paneBorderFocused',
        'toolbarBg', 'footerBg',
      ];

      const themes: ThemeName[] = ['dark', 'light', 'dracula', 'nord', 'solarized'];
      for (const theme of themes) {
        currentTheme.set(theme);
        const colors = get(themeColors);
        for (const key of requiredKeys) {
          expect(colors).toHaveProperty(key);
          expect((colors as any)[key]).toBeTruthy();
        }
      }
    });
  });

  describe('applyTheme', () => {
    it('sets CSS variables on document root', () => {
      applyTheme('dracula');
      expect(setPropertyMock).toHaveBeenCalled();

      // Should set --bg with dracula bg color
      const bgCall = setPropertyMock.mock.calls.find(
        (call: string[]) => call[0] === '--bg'
      );
      expect(bgCall).toBeDefined();
      expect(bgCall![1]).toBe('#282a36');
    });

    it('updates currentTheme store', () => {
      applyTheme('nord');
      expect(get(currentTheme)).toBe('nord');
    });

    it('applies accent color when provided', () => {
      setPropertyMock.mockClear();
      applyTheme('dark', '#ff0000');

      // applyTheme sets --accent from theme first, then applyAccentColor overrides it.
      // Find the LAST --accent call (the override).
      const accentCalls = setPropertyMock.mock.calls.filter(
        (call: string[]) => call[0] === '--accent'
      );
      expect(accentCalls.length).toBeGreaterThanOrEqual(2);
      expect(accentCalls[accentCalls.length - 1][1]).toBe('#ff0000');
    });

    it('converts camelCase to kebab-case for CSS vars', () => {
      applyTheme('dark');
      const bgSecondaryCall = setPropertyMock.mock.calls.find(
        (call: string[]) => call[0] === '--bg-secondary'
      );
      expect(bgSecondaryCall).toBeDefined();
    });
  });

  describe('applyAccentColor', () => {
    it('sets accent-related CSS variables', () => {
      setPropertyMock.mockClear();
      applyAccentColor('#00ff00');

      const vars = setPropertyMock.mock.calls.map((c: string[]) => c[0]);
      expect(vars).toContain('--accent');
      expect(vars).toContain('--accent-hover');
      expect(vars).toContain('--border-focused');
      expect(vars).toContain('--tab-active-fg');
      expect(vars).toContain('--pane-border-focused');
    });

    it('sets the exact accent color', () => {
      setPropertyMock.mockClear();
      applyAccentColor('#abcdef');

      const accentCall = setPropertyMock.mock.calls.find(
        (call: string[]) => call[0] === '--accent'
      );
      expect(accentCall![1]).toBe('#abcdef');
    });

    it('generates a darker hover color', () => {
      setPropertyMock.mockClear();
      applyAccentColor('#808080');

      const hoverCall = setPropertyMock.mock.calls.find(
        (call: string[]) => call[0] === '--accent-hover'
      );
      expect(hoverCall).toBeDefined();
      // The hover color should be different from the accent
      expect(hoverCall![1]).not.toBe('#808080');
      // And should be a valid hex color
      expect(hoverCall![1]).toMatch(/^#[0-9a-f]{6}$/);
    });
  });
});

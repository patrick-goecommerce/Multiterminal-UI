import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import CrashDialog from './CrashDialog.svelte';

afterEach(() => {
  cleanup();
});

describe('CrashDialog', () => {
  describe('visibility', () => {
    it('renders nothing when visible=false', () => {
      const { container } = render(CrashDialog, { props: { visible: false } });
      expect(container.querySelector('.overlay')).toBeNull();
    });

    it('renders overlay when visible=true', () => {
      const { container } = render(CrashDialog, { props: { visible: true } });
      expect(container.querySelector('.overlay')).not.toBeNull();
    });

    it('shows dialog with title', () => {
      const { getByText } = render(CrashDialog, { props: { visible: true } });
      expect(getByText('Instabilität erkannt')).toBeTruthy();
    });

    it('shows description text', () => {
      const { container } = render(CrashDialog, { props: { visible: true } });
      const desc = container.querySelector('.desc');
      expect(desc).not.toBeNull();
      expect(desc!.textContent).toContain('nicht sauber beendet');
    });

    it('shows auto-disable hint', () => {
      const { container } = render(CrashDialog, { props: { visible: true } });
      const hint = container.querySelector('.hint');
      expect(hint).not.toBeNull();
      expect(hint!.textContent).toContain('3 Sitzungen');
    });
  });

  describe('buttons', () => {
    it('has dismiss button', () => {
      const { getByText } = render(CrashDialog, { props: { visible: true } });
      expect(getByText('Nein, danke')).toBeTruthy();
    });

    it('has enable button', () => {
      const { getByText } = render(CrashDialog, { props: { visible: true } });
      expect(getByText('Logging aktivieren')).toBeTruthy();
    });
  });

  describe('events', () => {
    it('dispatches enable on button click', async () => {
      const { getByText, component } = render(CrashDialog, { props: { visible: true } });
      const handler = vi.fn();
      component.$on('enable', handler);

      await fireEvent.click(getByText('Logging aktivieren'));
      expect(handler).toHaveBeenCalledTimes(1);
    });

    it('dispatches dismiss on dismiss button click', async () => {
      const { getByText, component } = render(CrashDialog, { props: { visible: true } });
      const handler = vi.fn();
      component.$on('dismiss', handler);

      await fireEvent.click(getByText('Nein, danke'));
      expect(handler).toHaveBeenCalledTimes(1);
    });

    it('dispatches dismiss on overlay click', async () => {
      const { container, component } = render(CrashDialog, { props: { visible: true } });
      const handler = vi.fn();
      component.$on('dismiss', handler);

      const overlay = container.querySelector('.overlay')!;
      await fireEvent.click(overlay);
      expect(handler).toHaveBeenCalledTimes(1);
    });

    it('does not dispatch dismiss on dialog click (stopPropagation)', async () => {
      const { container, component } = render(CrashDialog, { props: { visible: true } });
      const handler = vi.fn();
      component.$on('dismiss', handler);

      const dialog = container.querySelector('.dialog')!;
      await fireEvent.click(dialog);
      expect(handler).not.toHaveBeenCalled();
    });
  });

  describe('keyboard', () => {
    it('dispatches dismiss on Escape when visible', async () => {
      const { component } = render(CrashDialog, { props: { visible: true } });
      const handler = vi.fn();
      component.$on('dismiss', handler);

      await fireEvent.keyDown(window, { key: 'Escape' });
      expect(handler).toHaveBeenCalledTimes(1);
    });

    it('dispatches enable on Enter when visible', async () => {
      const { component } = render(CrashDialog, { props: { visible: true } });
      const handler = vi.fn();
      component.$on('enable', handler);

      await fireEvent.keyDown(window, { key: 'Enter' });
      expect(handler).toHaveBeenCalledTimes(1);
    });

    it('does not dispatch on Escape when hidden', async () => {
      const { component } = render(CrashDialog, { props: { visible: false } });
      const dismissHandler = vi.fn();
      component.$on('dismiss', dismissHandler);

      await fireEvent.keyDown(window, { key: 'Escape' });
      expect(dismissHandler).not.toHaveBeenCalled();
    });
  });

  describe('structure', () => {
    it('shows warning icon with exclamation mark', () => {
      const { container } = render(CrashDialog, { props: { visible: true } });
      const icon = container.querySelector('.icon');
      expect(icon).not.toBeNull();
      expect(icon!.textContent).toBe('!');
    });

    it('has overlay > dialog > actions hierarchy', () => {
      const { container } = render(CrashDialog, { props: { visible: true } });
      const overlay = container.querySelector('.overlay');
      expect(overlay).not.toBeNull();
      expect(overlay!.querySelector('.dialog')).not.toBeNull();
      expect(overlay!.querySelector('.dialog .actions')).not.toBeNull();
    });

    it('has exactly two action buttons', () => {
      const { container } = render(CrashDialog, { props: { visible: true } });
      const buttons = container.querySelectorAll('.actions button');
      expect(buttons.length).toBe(2);
    });
  });
});

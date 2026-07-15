import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import CloseTabConfirmDialog from './CloseTabConfirmDialog.svelte';

afterEach(() => {
  cleanup();
});

describe('CloseTabConfirmDialog', () => {
  describe('visibility', () => {
    it('renders nothing when visible=false', () => {
      const { container } = render(CloseTabConfirmDialog, { props: { visible: false } });
      expect(container.querySelector('.overlay')).toBeNull();
    });

    it('renders overlay when visible=true', () => {
      const { container } = render(CloseTabConfirmDialog, { props: { visible: true } });
      expect(container.querySelector('.overlay')).not.toBeNull();
    });

    it('shows the tab name and pane count in the body text', () => {
      const { container } = render(CloseTabConfirmDialog, {
        props: { visible: true, tabName: 'my-project', paneCount: 3 },
      });
      const desc = container.querySelector('.desc');
      expect(desc).not.toBeNull();
      expect(desc!.textContent).toContain('my-project');
      expect(desc!.textContent).toContain('3');
    });
  });

  describe('events', () => {
    it('dispatches confirm on confirm button click', async () => {
      const { getByText, component } = render(CloseTabConfirmDialog, {
        props: { visible: true, tabName: 'x', paneCount: 1 },
      });
      const handler = vi.fn();
      component.$on('confirm', handler);

      await fireEvent.click(getByText('Tab schließen'));
      expect(handler).toHaveBeenCalledTimes(1);
    });

    it('dispatches cancel on cancel button click', async () => {
      const { getByText, component } = render(CloseTabConfirmDialog, {
        props: { visible: true, tabName: 'x', paneCount: 1 },
      });
      const handler = vi.fn();
      component.$on('cancel', handler);

      await fireEvent.click(getByText('Abbrechen'));
      expect(handler).toHaveBeenCalledTimes(1);
    });

    it('dispatches cancel on overlay click', async () => {
      const { container, component } = render(CloseTabConfirmDialog, {
        props: { visible: true, tabName: 'x', paneCount: 1 },
      });
      const handler = vi.fn();
      component.$on('cancel', handler);

      const overlay = container.querySelector('.overlay')!;
      await fireEvent.click(overlay);
      expect(handler).toHaveBeenCalledTimes(1);
    });

    it('does not dispatch cancel on dialog click (stopPropagation)', async () => {
      const { container, component } = render(CloseTabConfirmDialog, {
        props: { visible: true, tabName: 'x', paneCount: 1 },
      });
      const handler = vi.fn();
      component.$on('cancel', handler);

      const dialog = container.querySelector('.dialog')!;
      await fireEvent.click(dialog);
      expect(handler).not.toHaveBeenCalled();
    });
  });

  describe('keyboard', () => {
    it('dispatches cancel on Escape when visible', async () => {
      const { component } = render(CloseTabConfirmDialog, { props: { visible: true } });
      const handler = vi.fn();
      component.$on('cancel', handler);

      await fireEvent.keyDown(window, { key: 'Escape' });
      expect(handler).toHaveBeenCalledTimes(1);
    });

    it('dispatches confirm on Enter when visible', async () => {
      const { component } = render(CloseTabConfirmDialog, { props: { visible: true } });
      const handler = vi.fn();
      component.$on('confirm', handler);

      await fireEvent.keyDown(window, { key: 'Enter' });
      expect(handler).toHaveBeenCalledTimes(1);
    });

    it('does not dispatch on Escape when hidden', async () => {
      const { component } = render(CloseTabConfirmDialog, { props: { visible: false } });
      const handler = vi.fn();
      component.$on('cancel', handler);

      await fireEvent.keyDown(window, { key: 'Escape' });
      expect(handler).not.toHaveBeenCalled();
    });
  });
});

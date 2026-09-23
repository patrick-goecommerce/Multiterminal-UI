import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import SettingsDialog from './SettingsDialog.svelte';
import { config } from '../stores/config';

const { saveConfig } = vi.hoisted(() => ({
  saveConfig: vi.fn().mockResolvedValue(undefined),
}));

vi.mock('../../wailsjs/go/backend/App', () => ({
  SaveConfig: saveConfig,
  GetLogPath: vi.fn().mockResolvedValue(''),
  GetMCPServerPort: vi.fn().mockResolvedValue(0),
  UsesSessionDaemon: vi.fn().mockResolvedValue(false),
}));
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(() => () => {}) }));

afterEach(() => {
  cleanup();
  saveConfig.mockClear();
  config.update((c) => ({ ...c, layout: { mode: 'grid', float_x: -1, float_y: -1 } }));
});

function layoutSelect(container: HTMLElement): HTMLSelectElement {
  const el = container.querySelector('#layout-mode-select');
  if (!el) throw new Error('no layout select');
  return el as HTMLSelectElement;
}

describe('SettingsDialog — pane layout', () => {
  it('defaults to the grid', () => {
    const { container } = render(SettingsDialog, { props: { visible: true } });
    expect(layoutSelect(container).value).toBe('grid');
  });

  it('saves focus mode and keeps the remembered window position', async () => {
    config.update((c) => ({ ...c, layout: { mode: 'grid', float_x: 300, float_y: 120 } }));
    const { container, getByText } = render(SettingsDialog, { props: { visible: true } });
    const sel = layoutSelect(container);
    sel.value = 'focus';
    await fireEvent.change(sel);
    await fireEvent.click(getByText('Speichern'));

    expect(saveConfig).toHaveBeenCalledTimes(1);
    expect(saveConfig.mock.calls[0][0].layout).toEqual({ mode: 'focus', float_x: 300, float_y: 120 });
  });

  // Recurring-bug guard: changing one control must not reset another.
  it('does not reset the theme when the layout changes', async () => {
    const { container } = render(SettingsDialog, { props: { visible: true } });
    const theme = container.querySelector('#theme-select') as HTMLSelectElement;
    theme.value = 'nord';
    await fireEvent.change(theme);
    const sel = layoutSelect(container);
    sel.value = 'focus';
    await fireEvent.change(sel);
    expect((container.querySelector('#theme-select') as HTMLSelectElement).value).toBe('nord');
    expect(layoutSelect(container).value).toBe('focus');
  });
});

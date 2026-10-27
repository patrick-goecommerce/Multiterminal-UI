import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import { readFileSync } from 'node:fs';
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
  config.update((c) => ({ ...c, briefing: { big_diff_files: 15, big_diff_lines: 500, ci_watch: true } }));
});

function groupFor(container: HTMLElement, label: string): Element {
  const g = Array.from(container.querySelectorAll('.setting-group'))
    .find((el) => el.textContent?.includes(label));
  if (!g) throw new Error(`no setting group for ${label}`);
  return g;
}

const CI = 'CI und PRs beobachten';

describe('SettingsDialog: Leitstand', () => {
  it('shows the defaults', () => {
    const { container, getByLabelText } = render(SettingsDialog, { props: { visible: true } });
    expect(groupFor(container, CI).querySelector('.toggle-btn')!.className).toContain('toggle-on');
    expect((getByLabelText('Großer Diff ab Dateien') as HTMLInputElement).value).toBe('15');
    expect((getByLabelText('Großer Diff ab geänderten Zeilen') as HTMLInputElement).value).toBe('500');
  });

  it('saves the changed values as briefing', async () => {
    const { container, getByLabelText, getByText } = render(SettingsDialog, { props: { visible: true } });
    await fireEvent.click(groupFor(container, CI).querySelector('.toggle-btn')!);
    await fireEvent.input(getByLabelText('Großer Diff ab Dateien'), { target: { value: '20' } });
    await fireEvent.input(getByLabelText('Großer Diff ab geänderten Zeilen'), { target: { value: '800' } });
    await fireEvent.click(getByText('Speichern'));

    expect(saveConfig).toHaveBeenCalledTimes(1);
    expect(saveConfig.mock.calls[0][0].briefing).toEqual({
      big_diff_files: 20, big_diff_lines: 800, ci_watch: false,
    });
  });

  it('shows the stored values when opened again', () => {
    config.update((c) => ({ ...c, briefing: { big_diff_files: 20, big_diff_lines: 800, ci_watch: false } }));
    const { container, getByLabelText } = render(SettingsDialog, { props: { visible: true } });
    expect(groupFor(container, CI).querySelector('.toggle-btn')!.className).not.toContain('toggle-on');
    expect((getByLabelText('Großer Diff ab Dateien') as HTMLInputElement).value).toBe('20');
    expect((getByLabelText('Großer Diff ab geänderten Zeilen') as HTMLInputElement).value).toBe('800');
  });

  it('assigns none of the three inside a $: block (recurring-bug guard)', () => {
    const src = readFileSync('src/components/SettingsDialog.svelte', 'utf8');
    const reactive = src.split('\n').filter((l) => l.trim().startsWith('$:')).join('\n');
    expect(reactive).not.toMatch(/briefing(BigFiles|BigLines|CIWatch)/);
  });
});

import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import SettingsDialog from './SettingsDialog.svelte';
import { config } from '../stores/config';
import * as App from '../../wailsjs/go/backend/App';

vi.mock('../../wailsjs/go/backend/App', () => ({
  SaveConfig: vi.fn().mockResolvedValue(undefined),
  GetLogPath: vi.fn().mockResolvedValue(''),
  GetMCPServerPort: vi.fn().mockResolvedValue(0),
  GetProjectForceWorktrees: vi.fn().mockResolvedValue('inherit'),
  SetProjectForceWorktrees: vi.fn().mockResolvedValue(undefined),
}));
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(() => () => {}) }));

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  config.update((c) => ({ ...c, force_worktrees: false }));
});

/** The toggle button sits in the setting-group whose label is "Worktree-Pflicht". */
function worktreeToggle(container: HTMLElement): HTMLButtonElement {
  const group = Array.from(container.querySelectorAll('.setting-group')).find((el) =>
    el.querySelector('.setting-label')?.textContent?.includes('Worktree-Pflicht'),
  );
  if (!group) throw new Error('Worktree-Pflicht setting group not found');
  return group.querySelector('.toggle-btn') as HTMLButtonElement;
}

describe('SettingsDialog — Worktree-Pflicht', () => {
  it('reflects force_worktrees from config', () => {
    config.update((c) => ({ ...c, force_worktrees: true }));
    const { container } = render(SettingsDialog, { props: { visible: true } });
    expect(worktreeToggle(container).classList.contains('toggle-on')).toBe(true);
  });

  it('shows inactive when force_worktrees is off', () => {
    const { container } = render(SettingsDialog, { props: { visible: true } });
    expect(worktreeToggle(container).classList.contains('toggle-on')).toBe(false);
  });

  // The regression test for the original bug: the toggle used to write a
  // `use_worktrees` key that does not exist in the Go config struct, so
  // SaveConfig silently dropped it and the control did nothing at all.
  it('saves force_worktrees and never the dead use_worktrees key', async () => {
    const { container, getByText } = render(SettingsDialog, { props: { visible: true } });

    await fireEvent.click(worktreeToggle(container));
    await fireEvent.click(getByText('Speichern'));

    expect(App.SaveConfig).toHaveBeenCalledTimes(1);
    const saved = (App.SaveConfig as any).mock.calls[0][0];
    expect(saved.force_worktrees).toBe(true);
    expect(saved).not.toHaveProperty('use_worktrees');
  });

  it('does not reset the toggle when an unrelated control changes', async () => {
    const { container } = render(SettingsDialog, { props: { visible: true } });
    const toggle = worktreeToggle(container);
    await fireEvent.click(toggle);
    expect(toggle.classList.contains('toggle-on')).toBe(true);

    const checkbox = container.querySelector('input[type="checkbox"]') as HTMLInputElement;
    if (checkbox) await fireEvent.click(checkbox);

    expect(worktreeToggle(container).classList.contains('toggle-on')).toBe(true);
  });
});

import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import SettingsDialog from './SettingsDialog.svelte';
import { config } from '../stores/config';

const { saveConfig, usesSessionDaemon } = vi.hoisted(() => ({
  saveConfig: vi.fn().mockResolvedValue(undefined),
  usesSessionDaemon: vi.fn().mockResolvedValue(false),
}));

vi.mock('../../wailsjs/go/backend/App', () => ({
  SaveConfig: saveConfig,
  GetLogPath: vi.fn().mockResolvedValue(''),
  GetMCPServerPort: vi.fn().mockResolvedValue(0),
  UsesSessionDaemon: usesSessionDaemon,
}));
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(() => () => {}) }));

afterEach(() => {
  cleanup();
  saveConfig.mockClear();
  usesSessionDaemon.mockResolvedValue(false);
  config.update((c) => ({ ...c, session_host: 'embedded' } as any));
});

/** toggleFor finds the on/off button that belongs to a labelled setting. */
function toggleFor(container: HTMLElement, label: string): HTMLButtonElement {
  const groups = Array.from(container.querySelectorAll('.setting-group'));
  const group = groups.find((g) => g.querySelector('.setting-label')?.textContent?.includes(label));
  if (!group) throw new Error(`no setting group for ${label}`);
  const btn = group.querySelector('.toggle-btn');
  if (!btn) throw new Error(`no toggle in the group for ${label}`);
  return btn as HTMLButtonElement;
}

const DAEMON = 'Sitzungen im Hintergrunddienst';

describe('SettingsDialog — session host', () => {
  it('reflects the configured host', () => {
    config.update((c) => ({ ...c, session_host: 'daemon' } as any));
    const { container } = render(SettingsDialog, { props: { visible: true } });
    expect(toggleFor(container, DAEMON).className).toContain('toggle-on');
  });

  it('is off by default, because the sessions belong to the window unless asked otherwise', () => {
    const { container } = render(SettingsDialog, { props: { visible: true } });
    expect(toggleFor(container, DAEMON).className).not.toContain('toggle-on');
  });

  it('saves the switch as session_host', async () => {
    const { container, getByText } = render(SettingsDialog, { props: { visible: true } });
    await fireEvent.click(toggleFor(container, DAEMON));
    await fireEvent.click(getByText('Speichern'));

    expect(saveConfig).toHaveBeenCalledTimes(1);
    expect(saveConfig.mock.calls[0][0].session_host).toBe('daemon');
  });

  it('saves the switch off again', async () => {
    config.update((c) => ({ ...c, session_host: 'daemon' } as any));
    const { container, getByText } = render(SettingsDialog, { props: { visible: true } });
    await fireEvent.click(toggleFor(container, DAEMON));
    await fireEvent.click(getByText('Speichern'));

    expect(saveConfig.mock.calls[0][0].session_host).toBe('embedded');
  });

  // The recurring SettingsDialog bug: a reactive block that assigns variables
  // re-runs on any change and resets every control to the config's values.
  // Toggling this one must leave the others where the user put them.
  it('does not reset another setting when it is toggled (recurring-bug guard)', async () => {
    const { container } = render(SettingsDialog, { props: { visible: true } });

    const naming = toggleFor(container, 'Automatische Pane-Benennung');
    const namingWasOn = naming.className.includes('toggle-on');
    await fireEvent.click(naming);
    expect(naming.className.includes('toggle-on')).toBe(!namingWasOn);

    await fireEvent.click(toggleFor(container, DAEMON));

    expect(toggleFor(container, 'Automatische Pane-Benennung').className.includes('toggle-on'))
      .toBe(!namingWasOn);
  });

  // The setting is what the user asked for; what the backend ended up doing
  // can differ until the app restarts, and saying so is the whole point of
  // the line under the switch.
  it('reports that the daemon is actually in use', async () => {
    usesSessionDaemon.mockResolvedValue(true);
    config.update((c) => ({ ...c, session_host: 'daemon' } as any));
    const { container, findByText } = render(SettingsDialog, { props: { visible: true } });

    expect(await findByText(/Läuft: die Sitzungen dieses Fensters/)).toBeTruthy();
    expect(toggleFor(container, DAEMON).className).toContain('toggle-on');
  });

  it('says a switched-on daemon is not live yet', async () => {
    config.update((c) => ({ ...c, session_host: 'daemon' } as any));
    const { findByText } = render(SettingsDialog, { props: { visible: true } });

    expect(await findByText(/Noch nicht aktiv/)).toBeTruthy();
  });
});

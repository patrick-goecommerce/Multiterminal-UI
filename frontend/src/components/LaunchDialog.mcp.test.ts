import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import LaunchDialog from './LaunchDialog.svelte';
import { config } from '../stores/config';

vi.mock('../../wailsjs/go/backend/App', () => ({}));

const initialProfiles = [{ name: 'Nur MTUI', servers: ['mtui'] }];

afterEach(() => {
  cleanup();
  config.update((c) => ({ ...c, mcp_profiles: initialProfiles, default_mcp_profile: '' }));
});

function mcpSelect(container: HTMLElement): HTMLSelectElement {
  const el = container.querySelector('#mcp-profile-select') as HTMLSelectElement;
  if (!el) throw new Error('MCP profile picker not rendered');
  return el;
}

describe('LaunchDialog — MCP-Profil (issue #179)', () => {
  it('lists the sentinels plus every configured profile', () => {
    config.update((c) => ({
      ...c,
      mcp_profiles: [{ name: 'Nur MTUI', servers: ['mtui'] }, { name: 'Web', config_path: '/w.json' }],
    }));
    const { container } = render(LaunchDialog, { props: { visible: true } });

    const values = Array.from(mcpSelect(container).options).map((o) => o.value);
    expect(values).toEqual(['', 'none', 'Nur MTUI', 'Web']);
  });

  it('preselects the configured default profile', async () => {
    config.update((c) => ({ ...c, default_mcp_profile: 'Nur MTUI' }));
    const { container } = render(LaunchDialog, { props: { visible: true } });

    expect(mcpSelect(container).value).toBe('Nur MTUI');
  });

  it('falls back to "all" when the default names a deleted profile', () => {
    config.update((c) => ({ ...c, default_mcp_profile: 'Deleted', mcp_profiles: initialProfiles }));
    const { container } = render(LaunchDialog, { props: { visible: true } });

    expect(mcpSelect(container).value).toBe('');
  });

  it('dispatches the chosen profile with the launch event', async () => {
    const { container, getByText, component } = render(LaunchDialog, { props: { visible: true } });
    const launched = vi.fn();
    component.$on('launch', (e: CustomEvent) => launched(e.detail));

    await fireEvent.change(mcpSelect(container), { target: { value: 'none' } });
    await fireEvent.click(getByText('Claude Code'));

    expect(launched).toHaveBeenCalledTimes(1);
    expect(launched.mock.calls[0][0].mcpProfile).toBe('none');
  });

  it('keeps the user selection when the dialog is reopened', async () => {
    const { container, component } = render(LaunchDialog, { props: { visible: true } });
    await fireEvent.change(mcpSelect(container), { target: { value: 'Nur MTUI' } });

    await component.$set({ visible: false });
    await component.$set({ visible: true });

    expect(mcpSelect(container).value).toBe('Nur MTUI');
  });
});

import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import SettingsDialog from './SettingsDialog.svelte';
import { config } from '../stores/config';

vi.mock('../../wailsjs/go/backend/App', () => ({
  SaveConfig: vi.fn().mockResolvedValue(undefined),
  GetLogPath: vi.fn().mockResolvedValue(''),
}));
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(() => () => {}) }));

afterEach(() => {
  cleanup();
  config.update((c) => ({ ...c, finish_prep_prompt: '', quick_actions: [] }));
});

describe('SettingsDialog — quick actions section', () => {
  it('shows the finish-prompt textarea seeded from config', () => {
    config.update((c) => ({ ...c, finish_prep_prompt: 'my custom finish prompt' }));
    const { container } = render(SettingsDialog, { props: { visible: true } });
    const textarea = container.querySelector('.finish-prompt-input') as HTMLTextAreaElement;
    expect(textarea).not.toBeNull();
    expect(textarea.value).toBe('my custom finish prompt');
  });

  it('lists existing quick actions', () => {
    config.update((c) => ({
      ...c,
      quick_actions: [{ label: '🔁', prompt: 'loop it' }, { label: '🧪', prompt: 'test it' }],
    }));
    const { container } = render(SettingsDialog, { props: { visible: true } });
    const rows = container.querySelectorAll('.quick-action-row');
    expect(rows.length).toBe(2);
  });

  it('adds a new quick action row, capped at 5', async () => {
    config.update((c) => ({
      ...c,
      quick_actions: [
        { label: '1', prompt: 'p1' }, { label: '2', prompt: 'p2' }, { label: '3', prompt: 'p3' },
        { label: '4', prompt: 'p4' }, { label: '5', prompt: 'p5' },
      ],
    }));
    const { container, getByText } = render(SettingsDialog, { props: { visible: true } });
    const addBtn = getByText('+ Quick Action') as HTMLButtonElement;
    expect(addBtn.disabled).toBe(true);
    expect(container.querySelectorAll('.quick-action-row').length).toBe(5);
  });

  it('removes a quick action row on delete click', async () => {
    config.update((c) => ({ ...c, quick_actions: [{ label: '🔁', prompt: 'loop it' }] }));
    const { container } = render(SettingsDialog, { props: { visible: true } });
    expect(container.querySelectorAll('.quick-action-row').length).toBe(1);
    await fireEvent.click(container.querySelector('.quick-action-remove')!);
    expect(container.querySelectorAll('.quick-action-row').length).toBe(0);
  });

  it('does not reset an edited field when an unrelated field changes (recurring-bug guard)', async () => {
    config.update((c) => ({ ...c, finish_prep_prompt: 'original' }));
    const { container } = render(SettingsDialog, { props: { visible: true } });
    const textarea = container.querySelector('.finish-prompt-input') as HTMLTextAreaElement;
    await fireEvent.input(textarea, { target: { value: 'edited by user' } });
    // Toggling an unrelated checkbox must NOT re-run initDialog() and wipe the edit.
    const loggingCheckbox = container.querySelector('input[type="checkbox"]') as HTMLInputElement;
    if (loggingCheckbox) await fireEvent.click(loggingCheckbox);
    expect(textarea.value).toBe('edited by user');
  });
});

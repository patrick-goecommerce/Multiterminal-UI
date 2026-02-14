import { writable } from 'svelte/store';

export interface ModelEntry {
  label: string;
  id: string;
}

export interface AppConfig {
  default_shell: string;
  default_dir: string;
  theme: string;
  terminal_color: string;
  max_panes_per_tab: number;
  sidebar_width: number;
  claude_command: string;
  claude_models: ModelEntry[];
  commit_reminder_minutes: number;
}

export const config = writable<AppConfig>({
  default_shell: '',
  default_dir: '',
  theme: 'dark',
  terminal_color: '#39ff14',
  max_panes_per_tab: 12,
  sidebar_width: 30,
  claude_command: 'claude',
  claude_models: [
    { label: 'Default', id: '' },
    { label: 'Opus 4.6', id: 'claude-opus-4-6' },
    { label: 'Sonnet 4.5', id: 'claude-sonnet-4-5-20250929' },
    { label: 'Haiku 4.5', id: 'claude-haiku-4-5-20251001' },
  ],
  commit_reminder_minutes: 30,
});

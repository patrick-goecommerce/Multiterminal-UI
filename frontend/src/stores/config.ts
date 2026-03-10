import { writable } from 'svelte/store';

export interface ModelEntry {
  label: string;
  id: string;
}

export interface CommandEntry {
  name: string;
  text: string;
}

export interface AudioConfig {
  enabled?: boolean;
  volume: number;
  when_focused?: boolean;
  done_sound: string;
  input_sound: string;
  error_sound: string;
}

export interface KeepAliveConfig {
  enabled?: boolean;
  interval_minutes: number;
  message: string;
}

export interface StatusLineConfig {
  enabled: boolean;
  template: string;
  show_model: boolean;
  show_context: boolean;
  show_cost: boolean;
  show_git_branch: boolean;
  show_duration: boolean;
}

export interface BackgroundAgentsConfig {
  review_enabled?: boolean;
  review_tool: string;
  review_model: string;
  review_prompt: string;
  test_enabled?: boolean;
  test_command: string;
}

export interface OrchestratorConfig {
  max_parallel_agents: number;
  default_auto_merge: boolean;
  default_auto_start: boolean;
  max_retries: number;
  review_command: string;
  sync_subtasks_to_github: boolean;
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
  claude_enabled?: boolean;
  codex_command: string;
  codex_models: ModelEntry[];
  codex_enabled?: boolean;
  gemini_command: string;
  gemini_models: ModelEntry[];
  gemini_enabled?: boolean;
  commit_reminder_minutes: number;
  restore_session?: boolean;
  logging_enabled?: boolean;
  commands: CommandEntry[];
  audio: AudioConfig;
  keep_alive: KeepAliveConfig;
  status_line: StatusLineConfig;
  localhost_auto_open: string;
  sidebar_pinned: boolean;
  font_family: string;
  font_size: number;
  favorites: Record<string, string[]>;
  background_agents: BackgroundAgentsConfig;
  orchestrator: OrchestratorConfig;
  language: string;
  setup_done: boolean;
}

export const config = writable<AppConfig>({
  default_shell: '',
  default_dir: '',
  theme: 'dark',
  terminal_color: '#39ff14',
  max_panes_per_tab: 9,
  sidebar_width: 30,
  claude_command: 'claude',
  claude_enabled: true,
  codex_command: 'codex',
  codex_models: [
    { label: 'Default', id: '' },
    { label: 'o4-mini', id: 'o4-mini' },
    { label: 'o3', id: 'o3' },
    { label: 'GPT-4.1', id: 'gpt-4.1' },
  ],
  codex_enabled: false,
  gemini_command: 'gemini',
  gemini_models: [
    { label: 'Default', id: '' },
    { label: 'Gemini 2.5 Pro', id: 'gemini-2.5-pro' },
    { label: 'Gemini 2.5 Flash', id: 'gemini-2.5-flash' },
  ],
  gemini_enabled: false,
  claude_models: [
    { label: 'Default', id: '' },
    { label: 'Opus 4.6', id: 'claude-opus-4-6' },
    { label: 'Sonnet 4.5', id: 'claude-sonnet-4-5-20250929' },
    { label: 'Haiku 4.5', id: 'claude-haiku-4-5-20251001' },
  ],
  commit_reminder_minutes: 30,
  commands: [
    { name: 'Commit & Push', text: "git add -A && git commit -m 'update' && git push" },
  ],
  keep_alive: {
    enabled: true,
    interval_minutes: 300,
    message: 'Hi!',
  },
  audio: {
    enabled: true,
    volume: 50,
    when_focused: true,
    done_sound: '',
    input_sound: '',
    error_sound: '',
  },
  status_line: {
    enabled: false,
    template: 'standard',
    show_model: true,
    show_context: true,
    show_cost: true,
    show_git_branch: false,
    show_duration: false,
  },
  background_agents: {
    review_enabled: false,
    review_tool: 'claude',
    review_model: 'claude-haiku-4-5-20251001',
    review_prompt: 'Review the following commit diff. Flag bugs, security issues, and code quality problems:\n\n{diff}',
    test_enabled: false,
    test_command: '',
  },
  localhost_auto_open: 'notify',
  sidebar_pinned: false,
  font_family: '',
  font_size: 10,
  favorites: {},
  orchestrator: {
    max_parallel_agents: 3,
    default_auto_merge: false,
    default_auto_start: false,
    max_retries: 2,
    review_command: '',
    sync_subtasks_to_github: false,
  },
  language: 'de',
  setup_done: false,
});

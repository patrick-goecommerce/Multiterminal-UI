import { writable } from 'svelte/store';

export interface ModelEntry {
  label: string;
  id: string;
}

export interface CommandEntry {
  name: string;
  text: string;
}

export interface QuickAction {
  label: string;
  prompt: string;
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

/** A named subset of MCP servers a Claude pane may load (see internal/config/mcp_profile.go). */
export interface MCPProfile {
  name: string;
  /** MCP server names looked up in the user's claude registration. */
  servers?: string[];
  /** Ready-made .mcp.json handed to `claude --mcp-config` (wins over servers). */
  config_path?: string;
}

export interface IssueTrackingConfig {
  auto_comment_on_start: boolean;
  auto_comment_on_done: boolean;
  auto_comment_on_close: boolean;
  auto_close_issue: boolean;
  include_cost_in_report: boolean;
}

export interface STTCloudConfig {
  base_url: string;
  model: string;
  api_key: string;
}

export interface STTConfig {
  provider: string;
  language: string;
  cloud: STTCloudConfig;
}

export interface AutoNamingConfig {
  enabled?: boolean;
  model: string;
}

export interface IdleSuspendConfig {
  enabled?: boolean;
  timeout_minutes: number;
}

export interface MCPServerConfig {
  enabled?: boolean;
  port: number;
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
  auto_branch_on_issue?: boolean;
  force_worktrees?: boolean;
  commands: CommandEntry[];
  finish_prep_prompt: string;
  quick_actions: QuickAction[];
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
  mcp_profiles: MCPProfile[];
  /** Profile preselected in the launch dialog ('' = all global MCP servers). */
  default_mcp_profile: string;
  last_opened_dir: string;
  issue_tracking: IssueTrackingConfig;
  chat_style: string;
  stt: STTConfig;
  auto_naming: AutoNamingConfig;
  mcp_server: MCPServerConfig;
  idle_suspend: IdleSuspendConfig;
  update_channel: string;
  auto_update_check_minutes: number;
  terminal_scrollback: number;
  /**
   * Where the sessions live. "daemon" hands them to mtuid so they survive
   * closing the window; anything else keeps them in this process.
   */
  session_host: string;
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
    { label: 'Opus (latest)', id: 'opus' },
    { label: 'Sonnet (latest)', id: 'sonnet' },
    { label: 'Fable (latest)', id: 'fable' },
    { label: 'Haiku (latest)', id: 'haiku' },
  ],
  commit_reminder_minutes: 30,
  force_worktrees: false,
  commands: [
    { name: 'Commit & Push', text: "git add -A && git commit -m 'update' && git push" },
  ],
  finish_prep_prompt: '',
  quick_actions: [],
  keep_alive: {
    enabled: true,
    interval_minutes: 60,
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
  mcp_profiles: [
    { name: 'Nur MTUI', servers: ['mtui'] },
  ],
  default_mcp_profile: '',
  last_opened_dir: '',
  issue_tracking: {
    auto_comment_on_start: false,
    auto_comment_on_done: false,
    auto_comment_on_close: false,
    auto_close_issue: false,
    include_cost_in_report: false,
  },
  chat_style: '',
  stt: {
    provider: '',
    language: '',
    cloud: { base_url: '', model: '', api_key: '' },
  },
  auto_naming: { model: '' },
  mcp_server: { port: 0 },
  idle_suspend: { timeout_minutes: 0 },
  update_channel: '',
  auto_update_check_minutes: 0,
  terminal_scrollback: 0,
  // Empty rather than 'embedded': these defaults are only what the store holds
  // before GetConfig answers, and the backend decides what an unset value
  // means. Writing 'embedded' here would look like a decision this file made.
  session_host: '',
});

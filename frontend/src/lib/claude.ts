import type { PaneMode } from '../stores/tabs';

export const MODE_TO_INDEX: Record<string, number> = {
  shell: 0, claude: 1, 'claude-yolo': 2,
  codex: 3, 'codex-auto': 4,
  gemini: 5, 'gemini-yolo': 6,
  'claude-auto': 7,
};
export const INDEX_TO_MODE: PaneMode[] = [
  'shell', 'claude', 'claude-yolo',
  'codex', 'codex-auto',
  'gemini', 'gemini-yolo',
  'claude-auto',
];

/** Options for pinning/resuming a claude session id (claude modes only). */
export interface SessionOpts {
  /** Resume an existing claude conversation by session id (`--resume`). Wins over sessionId. */
  resumeId?: string;
  /** Launch a fresh claude session with a fixed id (`--session-id`) so it can be resumed later. */
  sessionId?: string;
  /**
   * MCP profile name for this pane (issue #179). Every claude pane spawns its
   * own stdio child process per MCP server, so panes that need none should say so:
   *   - MCP_PROFILE_GLOBAL ('' / undefined) — no MCP flags, inherit every
   *     globally registered server (pre-#179 behaviour).
   *   - MCP_PROFILE_NONE ('none') — `--strict-mcp-config` alone, which loads
   *     ZERO servers and needs no file on disk.
   *   - any other name — `--strict-mcp-config --mcp-config <mcpConfigPath>`.
   */
  mcpProfile?: string;
  /** Path of the .mcp.json the backend resolved for mcpProfile (AppService.ResolveMCPProfile). */
  mcpConfigPath?: string;
}

/** Inherit every globally registered MCP server (no MCP flags at all). */
export const MCP_PROFILE_GLOBAL = '';
/** Load zero MCP servers, without writing a temporary config file. */
export const MCP_PROFILE_NONE = 'none';

/**
 * MCP flags for a claude pane. Mirrors app_pane_name.go's naming call:
 * `--strict-mcp-config` with no `--mcp-config` loads ZERO MCP servers while
 * leaving OAuth/model/settings intact.
 *
 * A named profile whose config path could not be resolved deliberately falls
 * back to "no servers" rather than "all servers" — the pane the user asked to
 * keep small must never silently become the expensive one.
 */
export function mcpArgs(profile?: string, configPath?: string): string[] {
  const name = (profile || '').trim();
  if (name === MCP_PROFILE_GLOBAL) return [];
  if (name === MCP_PROFILE_NONE) return ['--strict-mcp-config'];
  const path = (configPath || '').trim();
  return path ? ['--strict-mcp-config', '--mcp-config', path] : ['--strict-mcp-config'];
}

/** Modes backed by the claude CLI, which understands --session-id / --resume. */
export const CLAUDE_MODES = new Set<PaneMode>(['claude', 'claude-yolo', 'claude-auto']);

/** Generate a fresh session id (UUID v4) for pinning a claude session. */
export function genSessionId(): string {
  return crypto.randomUUID();
}

/**
 * Map a pane mode to the chat `--permission-mode` equivalent so a terminal
 * pane keeps its permission posture when toggled to chat display. Mirrors the
 * flags buildClaudeArgv adds: claude-yolo↔--dangerously-skip-permissions,
 * claude-auto↔--permission-mode auto, plain claude↔default.
 */
export function modeToPermissionMode(mode: PaneMode): string {
  switch (mode) {
    case 'claude-yolo': return 'bypassPermissions';
    case 'claude-auto': return 'auto';
    default: return 'default';
  }
}

/** Build the argv array for launching a CLI session. */
export function buildClaudeArgv(mode: PaneMode, model: string, claudeCmd: string, codexCmd?: string, geminiCmd?: string, opts?: SessionOpts): string[] {
  let argv: string[];
  switch (mode) {
    case 'claude':
      argv = model ? [claudeCmd, '--model', model] : [claudeCmd];
      break;
    case 'claude-yolo':
      argv = model
        ? [claudeCmd, '--dangerously-skip-permissions', '--model', model]
        : [claudeCmd, '--dangerously-skip-permissions'];
      break;
    case 'claude-auto':
      argv = model
        ? [claudeCmd, '--permission-mode', 'auto', '--model', model]
        : [claudeCmd, '--permission-mode', 'auto'];
      break;
    case 'codex':
      argv = model ? [codexCmd || 'codex', '--model', model] : [codexCmd || 'codex'];
      break;
    case 'codex-auto':
      argv = model
        ? [codexCmd || 'codex', '--full-auto', '--model', model]
        : [codexCmd || 'codex', '--full-auto'];
      break;
    case 'gemini':
      argv = model ? [geminiCmd || 'gemini', '--model', model] : [geminiCmd || 'gemini'];
      break;
    case 'gemini-yolo':
      argv = model
        ? [geminiCmd || 'gemini', '--sandbox', '--model', model]
        : [geminiCmd || 'gemini', '--sandbox'];
      break;
    default:
      return [];
  }

  // Pin or resume the claude session so terminal⇄chat toggles keep the same
  // conversation. --resume wins over --session-id (they are mutually exclusive).
  if (opts && CLAUDE_MODES.has(mode)) {
    if (opts.resumeId) argv.push('--resume', opts.resumeId);
    else if (opts.sessionId) argv.push('--session-id', opts.sessionId);
    // MCP flags are claude-CLI-only; codex/gemini don't understand them.
    argv.push(...mcpArgs(opts.mcpProfile, opts.mcpConfigPath));
  }
  return argv;
}

/** Generate a display name for a pane. */
export function getClaudeName(mode: PaneMode, model: string): string {
  switch (mode) {
    case 'claude': return `Claude ${model ? `(${model})` : ''}`.trim();
    case 'claude-yolo': return `YOLO ${model ? `(${model})` : ''}`.trim();
    case 'claude-auto': return `Claude Auto ${model ? `(${model})` : ''}`.trim();
    case 'codex': return `Codex ${model ? `(${model})` : ''}`.trim();
    case 'codex-auto': return `Codex Auto ${model ? `(${model})` : ''}`.trim();
    case 'gemini': return `Gemini ${model ? `(${model})` : ''}`.trim();
    case 'gemini-yolo': return `Gemini Sandbox ${model ? `(${model})` : ''}`.trim();
    default: return 'Shell';
  }
}

/** Encode a string as base64 for PTY transmission. */
export function encodeForPty(text: string): string {
  const encoder = new TextEncoder();
  const bytes = encoder.encode(text);
  let binary = '';
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

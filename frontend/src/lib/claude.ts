import type { PaneMode } from '../stores/tabs';

export const MODE_TO_INDEX: Record<string, number> = { shell: 0, claude: 1, 'claude-yolo': 2 };
export const INDEX_TO_MODE: PaneMode[] = ['shell', 'claude', 'claude-yolo'];

/** Build the argv array for launching a Claude session. */
export function buildClaudeArgv(mode: PaneMode, model: string, claudeCmd: string): string[] {
  switch (mode) {
    case 'claude':
      return model ? [claudeCmd, '--model', model] : [claudeCmd];
    case 'claude-yolo':
      return model
        ? [claudeCmd, '--dangerously-skip-permissions', '--model', model]
        : [claudeCmd, '--dangerously-skip-permissions'];
    default:
      return [];
  }
}

/** Generate a display name for a Claude pane. */
export function getClaudeName(mode: PaneMode, model: string): string {
  switch (mode) {
    case 'claude': return `Claude ${model ? `(${model})` : ''}`.trim();
    case 'claude-yolo': return `YOLO ${model ? `(${model})` : ''}`.trim();
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

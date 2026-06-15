import { describe, it, expect } from 'vitest';
import { buildClaudeArgv, getClaudeName, MODE_TO_INDEX, INDEX_TO_MODE, modeToPermissionMode } from './claude';

describe('modeToPermissionMode', () => {
  it('maps claude-yolo to bypassPermissions (matches --dangerously-skip-permissions)', () => {
    expect(modeToPermissionMode('claude-yolo')).toBe('bypassPermissions');
  });

  it('maps claude-auto to auto (matches --permission-mode auto)', () => {
    expect(modeToPermissionMode('claude-auto')).toBe('auto');
  });

  it('maps plain claude to default', () => {
    expect(modeToPermissionMode('claude')).toBe('default');
  });

  it('falls back to default for non-claude modes', () => {
    expect(modeToPermissionMode('shell')).toBe('default');
    expect(modeToPermissionMode('codex')).toBe('default');
  });
});

describe('buildClaudeArgv session opts', () => {
  it('pins a fresh session via --session-id (claude)', () => {
    expect(buildClaudeArgv('claude', '', 'claude', undefined, undefined, { sessionId: 'sid-1' }))
      .toEqual(['claude', '--session-id', 'sid-1']);
  });

  it('resumes via --resume (claude)', () => {
    expect(buildClaudeArgv('claude', 'opus', 'claude', undefined, undefined, { resumeId: 'sess-9' }))
      .toEqual(['claude', '--model', 'opus', '--resume', 'sess-9']);
  });

  it('--resume wins over --session-id', () => {
    expect(buildClaudeArgv('claude-auto', '', 'claude', undefined, undefined, { sessionId: 'sid', resumeId: 'res' }))
      .toEqual(['claude', '--permission-mode', 'auto', '--resume', 'res']);
  });

  it('ignores session opts for non-claude modes (codex)', () => {
    expect(buildClaudeArgv('codex', '', 'claude', 'codex', undefined, { sessionId: 'sid' }))
      .toEqual(['codex']);
  });

  it('emits nothing extra when opts are empty', () => {
    expect(buildClaudeArgv('claude', '', 'claude', undefined, undefined, {}))
      .toEqual(['claude']);
  });
});

describe('claude-auto mode', () => {
  it('builds argv with --permission-mode auto, no model', () => {
    expect(buildClaudeArgv('claude-auto', '', 'claude')).toEqual([
      'claude', '--permission-mode', 'auto',
    ]);
  });

  it('builds argv with --permission-mode auto and model', () => {
    expect(buildClaudeArgv('claude-auto', 'claude-opus-4-7', 'claude')).toEqual([
      'claude', '--permission-mode', 'auto', '--model', 'claude-opus-4-7',
    ]);
  });

  it('produces a display name', () => {
    expect(getClaudeName('claude-auto', '')).toBe('Claude Auto');
    expect(getClaudeName('claude-auto', 'opus')).toBe('Claude Auto (opus)');
  });

  it('appends claude-auto at the end of the index table (persistence safety)', () => {
    expect(MODE_TO_INDEX['shell']).toBe(0);
    expect(MODE_TO_INDEX['claude']).toBe(1);
    expect(MODE_TO_INDEX['claude-yolo']).toBe(2);
    expect(MODE_TO_INDEX['codex']).toBe(3);
    expect(MODE_TO_INDEX['codex-auto']).toBe(4);
    expect(MODE_TO_INDEX['gemini']).toBe(5);
    expect(MODE_TO_INDEX['gemini-yolo']).toBe(6);
    expect(MODE_TO_INDEX['claude-auto']).toBe(7);
    expect(INDEX_TO_MODE[7]).toBe('claude-auto');
  });
});

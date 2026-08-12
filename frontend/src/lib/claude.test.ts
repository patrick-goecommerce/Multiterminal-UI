import { describe, it, expect } from 'vitest';
import {
  buildClaudeArgv, getClaudeName, MODE_TO_INDEX, INDEX_TO_MODE, modeToPermissionMode,
  mcpArgs, MCP_PROFILE_GLOBAL, MCP_PROFILE_NONE,
} from './claude';

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

describe('mcpArgs (MCP profiles, issue #179)', () => {
  it('adds nothing for the global profile (inherit every registered server)', () => {
    expect(mcpArgs(MCP_PROFILE_GLOBAL)).toEqual([]);
    expect(mcpArgs(undefined)).toEqual([]);
    expect(mcpArgs('  ')).toEqual([]);
  });

  it('uses --strict-mcp-config alone for "kein MCP" — no temp file involved', () => {
    expect(mcpArgs(MCP_PROFILE_NONE)).toEqual(['--strict-mcp-config']);
    expect(mcpArgs('none', '/ignored/path.json')).toEqual(['--strict-mcp-config']);
  });

  it('passes a resolved profile config via --mcp-config', () => {
    expect(mcpArgs('Nur MTUI', '/home/u/.multiterminal/mcp-profiles/nur_mtui-1234abcd.json'))
      .toEqual(['--strict-mcp-config', '--mcp-config', '/home/u/.multiterminal/mcp-profiles/nur_mtui-1234abcd.json']);
  });

  it('falls back to ZERO servers (not all) when a profile could not be resolved', () => {
    expect(mcpArgs('Nur MTUI', '')).toEqual(['--strict-mcp-config']);
    expect(mcpArgs('Nur MTUI', undefined)).toEqual(['--strict-mcp-config']);
  });
});

describe('buildClaudeArgv MCP profiles', () => {
  it('leaves argv untouched without a profile (pre-#179 behaviour)', () => {
    expect(buildClaudeArgv('claude', '', 'claude')).toEqual(['claude']);
    expect(buildClaudeArgv('claude', '', 'claude', undefined, undefined, { sessionId: 'sid' }))
      .toEqual(['claude', '--session-id', 'sid']);
  });

  it('appends --strict-mcp-config for the "none" profile', () => {
    expect(buildClaudeArgv('claude', '', 'claude', undefined, undefined, { mcpProfile: 'none' }))
      .toEqual(['claude', '--strict-mcp-config']);
  });

  it('appends --mcp-config after the session flags', () => {
    expect(buildClaudeArgv('claude', 'opus', 'claude', undefined, undefined,
      { sessionId: 'sid-1', mcpProfile: 'Nur MTUI', mcpConfigPath: '/tmp/p.json' }))
      .toEqual(['claude', '--model', 'opus', '--session-id', 'sid-1',
        '--strict-mcp-config', '--mcp-config', '/tmp/p.json']);
  });

  it('applies to yolo and auto claude modes too', () => {
    expect(buildClaudeArgv('claude-yolo', '', 'claude', undefined, undefined, { mcpProfile: 'none' }))
      .toEqual(['claude', '--dangerously-skip-permissions', '--strict-mcp-config']);
    expect(buildClaudeArgv('claude-auto', '', 'claude', undefined, undefined,
      { mcpProfile: 'P', mcpConfigPath: '/tmp/p.json' }))
      .toEqual(['claude', '--permission-mode', 'auto', '--strict-mcp-config', '--mcp-config', '/tmp/p.json']);
  });

  it('never adds MCP flags to codex/gemini (they do not understand them)', () => {
    expect(buildClaudeArgv('codex', '', 'claude', 'codex', undefined,
      { mcpProfile: 'none' })).toEqual(['codex']);
    expect(buildClaudeArgv('gemini', '', 'claude', undefined, 'gemini',
      { mcpProfile: 'P', mcpConfigPath: '/tmp/p.json' })).toEqual(['gemini']);
    expect(buildClaudeArgv('shell', '', 'claude', undefined, undefined,
      { mcpProfile: 'none' })).toEqual([]);
  });

  it('combines --resume with a profile', () => {
    expect(buildClaudeArgv('claude', '', 'claude', undefined, undefined,
      { resumeId: 'res', mcpProfile: 'none' }))
      .toEqual(['claude', '--resume', 'res', '--strict-mcp-config']);
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

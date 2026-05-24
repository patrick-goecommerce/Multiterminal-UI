import { describe, it, expect } from 'vitest';
import { buildClaudeArgv, getClaudeName, MODE_TO_INDEX, INDEX_TO_MODE } from './claude';

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

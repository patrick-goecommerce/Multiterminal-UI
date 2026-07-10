import { describe, it, expect } from 'vitest';
import { config } from './config';
import { get } from 'svelte/store';

describe('config store — quick actions defaults', () => {
  it('defaults quick_actions to an empty array', () => {
    expect(get(config).quick_actions).toEqual([]);
  });

  it('defaults finish_prep_prompt to an empty string', () => {
    expect(get(config).finish_prep_prompt).toBe('');
  });
});

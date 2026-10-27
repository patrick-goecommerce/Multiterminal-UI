import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { briefings, applyBriefing, forgetBriefing, loadBriefings } from './briefings';

const b = (id: string) => ({ prompt_id: id, built_at: 1, prompt: '', closing: '', checks: [], warnings: [], notes: [] }) as any;

describe('briefings store', () => {
  beforeEach(() => briefings.set({}));
  it('applies and forgets', () => {
    applyBriefing('h:1', b('a'));
    expect(get(briefings)['h:1'].prompt_id).toBe('a');
    applyBriefing('h:1', b('b'));
    expect(get(briefings)['h:1'].prompt_id).toBe('b');
    forgetBriefing('h:1');
    expect(get(briefings)).toEqual({});
  });
  it('loadBriefings keeps only the ones that came back', async () => {
    const fetch = async (ref: string) => {
      if (ref === 'h:1') return b('one');
      if (ref === 'h:3') throw new Error('boom');
      return null;
    };
    await loadBriefings(['h:1', 'h:2', 'h:3'], fetch);
    expect(Object.keys(get(briefings))).toEqual(['h:1']);
  });
  it('does not replace a briefing that arrived by event meanwhile', async () => {
    applyBriefing('h:1', { ...b('new'), built_at: 5 });
    await loadBriefings(['h:1'], async () => ({ ...b('old'), built_at: 2 }));
    expect(get(briefings)['h:1'].prompt_id).toBe('new');
  });
});

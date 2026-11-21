import { describe, it, expect, vi } from 'vitest';
import { createTargetSync } from './targetSync';

const tab = (panes: { sessionId: string; targetBranch: string }[]) => ({ id: 't', panes }) as any;

describe('createTargetSync', () => {
  it('sends once per change and skips empty values', () => {
    const send = vi.fn().mockResolvedValue(undefined);
    const sync = createTargetSync(send);
    const tabs = [tab([
      { sessionId: 'h:1', targetBranch: 'main' },
      { sessionId: '', targetBranch: 'main' },
      { sessionId: 'h:2', targetBranch: '' },
    ])];
    sync(tabs);
    sync(tabs);
    expect(send).toHaveBeenCalledTimes(1);
    expect(send).toHaveBeenCalledWith('h:1', 'main');
    sync([tab([{ sessionId: 'h:1', targetBranch: 'alpha-main' }])]);
    expect(send).toHaveBeenCalledTimes(2);
    expect(send).toHaveBeenLastCalledWith('h:1', 'alpha-main');
  });
  it('retries after a failed send', async () => {
    const send = vi.fn().mockRejectedValueOnce(new Error('x')).mockResolvedValue(undefined);
    const sync = createTargetSync(send);
    const tabs = [tab([{ sessionId: 'h:1', targetBranch: 'main' }])];
    sync(tabs);
    await Promise.resolve();
    await Promise.resolve();
    sync(tabs);
    expect(send).toHaveBeenCalledTimes(2);
  });
});

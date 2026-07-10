import { describe, it, expect, vi } from 'vitest';

const { addToQueue } = vi.hoisted(() => ({ addToQueue: vi.fn() }));
vi.mock('../../wailsjs/go/backend/App', () => ({ AddToQueue: addToQueue }));

import { sendQuickAction } from './quickActionQueue';

describe('sendQuickAction', () => {
  it('forwards sessionId and prompt to App.AddToQueue', async () => {
    await sendQuickAction(42, 'rebase feat/x onto alpha-main');
    expect(addToQueue).toHaveBeenCalledWith(42, 'rebase feat/x onto alpha-main');
  });
});

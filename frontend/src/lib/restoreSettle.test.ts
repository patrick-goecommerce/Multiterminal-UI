import { describe, it, expect } from 'vitest';
import { createRestoreSettle } from './restoreSettle';

const att = (rank: number) => ({ rank });

describe('restoreSettle', () => {
  it('passes a pane the restore did not create', () => {
    const s = createRestoreSettle();
    expect(s.settleRestored('h:1', att(1))).toEqual({ toast: true, markSeen: false });
  });

  it('a restored done is seen at once and never toasts', () => {
    const s = createRestoreSettle();
    s.markRestored('h:1');
    expect(s.settleRestored('h:1', att(3))).toEqual({ toast: false, markSeen: true });
  });

  it('a restored error is a failed restart: it stays unseen and may toast', () => {
    const s = createRestoreSettle();
    s.markRestored('h:1');
    expect(s.settleRestored('h:1', att(2))).toEqual({ toast: true, markSeen: false });
    // Forgotten: a later "done" is not marked seen like a restored one.
    expect(s.settleRestored('h:1', att(3))).toEqual({ toast: true, markSeen: false });
  });

  it('a restored question stays listed without a toast', () => {
    const s = createRestoreSettle();
    s.markRestored('h:1');
    expect(s.settleRestored('h:1', att(1))).toEqual({ toast: false, markSeen: false });
  });

  it('a permission or a quiet first record settles without a toast and without marking', () => {
    const s = createRestoreSettle();
    s.markRestored('h:1');
    s.markRestored('h:2');
    expect(s.settleRestored('h:1', att(0))).toEqual({ toast: false, markSeen: false });
    expect(s.settleRestored('h:2', att(5))).toEqual({ toast: false, markSeen: false });
    expect(s.settleRestored('h:2', att(1))).toEqual({ toast: true, markSeen: false });
  });

  it('the boot\'s "working" waits for the settled state', () => {
    const s = createRestoreSettle();
    s.markRestored('h:1');
    expect(s.settleRestored('h:1', att(4))).toEqual({ toast: false, markSeen: false });
    expect(s.settleRestored('h:1', att(4))).toEqual({ toast: false, markSeen: false });
    expect(s.settleRestored('h:1', att(3))).toEqual({ toast: false, markSeen: true });
  });

  it('only the first settled record: a later question toasts as usual', () => {
    const s = createRestoreSettle();
    s.markRestored('h:1');
    s.settleRestored('h:1', att(3));
    expect(s.settleRestored('h:1', att(1))).toEqual({ toast: true, markSeen: false });
  });

  it('ignores an empty ref', () => {
    const s = createRestoreSettle();
    s.markRestored('');
    expect(s.settleRestored('', att(3))).toEqual({ toast: true, markSeen: false });
  });
});

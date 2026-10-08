import { describe, it, expect } from 'vitest';
import { normPath, tabForProject } from './projectTabs';

const tab = (id: string, dir: string) => ({ id, name: id, dir, panes: [], focusedPaneId: '' }) as any;

describe('tabForProject', () => {
  const tabs = [tab('a', 'C:\\repos\\polaris'), tab('b', 'C:\\repos\\goki\\src')];
  const roots = { 'C:\\repos\\polaris': 'C:\\repos\\polaris', 'C:\\repos\\goki\\src': 'C:\\repos\\goki' };

  it('finds the tab whose project root matches, ignoring case and slashes', () => {
    expect(tabForProject(tabs, roots, 'c:/Repos/goki/')?.id).toBe('b');
  });
  it('returns null when no tab belongs to the project', () => {
    expect(tabForProject(tabs, roots, 'C:\\repos\\other')).toBeNull();
  });
  it('falls back to the tab dir while its root is unknown', () => {
    expect(tabForProject([tab('c', 'D:\\x')], {}, 'd:\\x')?.id).toBe('c');
  });
  it('normalises paths', () => {
    expect(normPath('C:\\A\\b\\')).toBe('c:/a/b');
  });
});

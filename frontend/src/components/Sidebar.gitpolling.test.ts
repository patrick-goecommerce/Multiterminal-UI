import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';

const { getGitFileStatuses } = vi.hoisted(() => ({ getGitFileStatuses: vi.fn() }));

vi.mock('../../wailsjs/go/backend/App', () => ({
  GetGitFileStatuses: getGitFileStatuses,
  ListDirectory: vi.fn().mockResolvedValue([]),
  GetFavorites: vi.fn().mockResolvedValue([]),
  AddFavorite: vi.fn(),
  RemoveFavorite: vi.fn(),
  SearchFiles: vi.fn().mockResolvedValue([]),
  GetIssues: vi.fn().mockResolvedValue([]),
  GetIssueDetail: vi.fn(),
  AddIssueComment: vi.fn(),
  UpdateIssue: vi.fn(),
  CheckGitHubCLI: vi.fn().mockResolvedValue(''),
  AddToGitignore: vi.fn(),
}));

import Sidebar from './Sidebar.svelte';

beforeEach(() => {
  vi.useFakeTimers();
  getGitFileStatuses.mockReset().mockResolvedValue({});
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

// `git status -uall` every 5 s used to run whether or not anybody could see
// the result: the sidebar stays mounted while it is closed.
describe('Sidebar git polling', () => {
  it('does not run git while the sidebar is closed', async () => {
    render(Sidebar, { props: { visible: false, dir: '/repo' } });
    await tick();
    vi.advanceTimersByTime(20_000);
    await tick();

    expect(getGitFileStatuses).not.toHaveBeenCalled();
  });

  it('fetches once on opening, then every 5 s while open', async () => {
    const { rerender } = render(Sidebar, { props: { visible: false, dir: '/repo' } });
    await tick();

    await rerender({ visible: true, dir: '/repo' });
    await tick();
    expect(getGitFileStatuses).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(10_000);
    expect(getGitFileStatuses).toHaveBeenCalledTimes(3);

    await rerender({ visible: false, dir: '/repo' });
    getGitFileStatuses.mockClear();
    await vi.advanceTimersByTimeAsync(10_000);
    expect(getGitFileStatuses).not.toHaveBeenCalled();
  });

  // On a big repo one run can outlast the interval; a second run started
  // behind it only doubles the load.
  it('never runs two git status at once for the same directory', async () => {
    getGitFileStatuses.mockReturnValue(new Promise(() => {})); // never settles
    render(Sidebar, { props: { visible: true, dir: '/repo' } });
    await tick();
    await vi.advanceTimersByTimeAsync(20_000);

    expect(getGitFileStatuses).toHaveBeenCalledTimes(1);
  });
});

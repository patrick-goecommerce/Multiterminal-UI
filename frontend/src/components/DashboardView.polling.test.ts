import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';

const { getDashboardStats, getDashboardPanes } = vi.hoisted(() => ({
  getDashboardStats: vi.fn(),
  getDashboardPanes: vi.fn(),
}));

vi.mock('../../wailsjs/go/backend/App', () => ({
  GetDashboardStats: getDashboardStats,
  GetDashboardPanes: getDashboardPanes,
}));

import DashboardView from './DashboardView.svelte';
import { workspace } from '../stores/workspace';

beforeEach(() => {
  vi.useFakeTimers();
  getDashboardStats.mockReset().mockResolvedValue({ projects: [], total_cost: '' });
  getDashboardPanes.mockReset().mockResolvedValue([]);
  workspace.setView('dashboard');
});
afterEach(() => {
  cleanup();
  workspace.setView('terminals');
  vi.useRealTimers();
});

describe('DashboardView polling', () => {
  it('keeps one 5 s poll however often the workspace store emits', async () => {
    render(DashboardView);
    await tick();
    for (let i = 0; i < 4; i++) {
      workspace.toggleCollapsed();
      await tick();
    }
    getDashboardStats.mockClear();

    vi.advanceTimersByTime(5000);
    expect(getDashboardStats).toHaveBeenCalledTimes(1);
  });

  it('stops polling when the dashboard is left', async () => {
    render(DashboardView);
    await tick();
    workspace.toggleCollapsed();
    await tick();
    workspace.setView('terminals');
    await tick();
    getDashboardStats.mockClear();

    vi.advanceTimersByTime(15000);
    expect(getDashboardStats).not.toHaveBeenCalled();
  });
});

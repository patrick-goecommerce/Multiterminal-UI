import { describe, it, expect, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import PaneTitlebar from './PaneTitlebar.svelte';
import { config } from '../stores/config';
import { get } from 'svelte/store';
import type { Pane } from '../stores/tabs';

afterEach(() => {
  cleanup();
  config.update((c) => ({ ...c, quick_actions: [] }));
});

function basePane(overrides: Partial<Pane> = {}): Pane {
  return {
    id: 'pane-1',
    sessionId: 42,
    name: 'Claude',
    mode: 'claude',
    model: '',
    focused: true,
    activity: 'idle',
    cost: '',
    running: true,
    maximized: false,
    issueNumber: null,
    issueTitle: '',
    issueBranch: '',
    worktreePath: '',
    branch: 'feat/x',
    targetBranch: 'alpha-main',
    zoomDelta: 0,
    background: false,
    display: 'terminal',
    conversationId: '',
    claudeSessionId: '',
    autoName: '',
    oscTitle: '',
    autoNameSource: '',
    userRenamed: false,
    finishPhase: '',
    ...overrides,
  };
}

describe('PaneTitlebar — quick actions', () => {
  it('renders one button per configured quick action on a claude pane', () => {
    config.update((c) => ({
      ...c,
      quick_actions: [
        { label: '🔁', prompt: 'loop it' },
        { label: '🧪', prompt: 'test it' },
      ],
    }));
    const { container } = render(PaneTitlebar, { props: { pane: basePane() } });
    const buttons = container.querySelectorAll('.quick-action-btn');
    expect(buttons.length).toBe(2);
    expect(buttons[0].textContent).toBe('🔁');
    expect(buttons[1].textContent).toBe('🧪');
  });

  it('does not render quick actions on a shell pane', () => {
    config.update((c) => ({ ...c, quick_actions: [{ label: '🔁', prompt: 'loop it' }] }));
    const { container } = render(PaneTitlebar, { props: { pane: basePane({ mode: 'shell', worktreePath: '' }) } });
    expect(container.querySelectorAll('.quick-action-btn').length).toBe(0);
  });

  it('renders quick actions on claude-auto and claude-yolo panes too', () => {
    config.update((c) => ({ ...c, quick_actions: [{ label: '🔁', prompt: 'loop it' }] }));
    const auto = render(PaneTitlebar, { props: { pane: basePane({ mode: 'claude-auto' }) } });
    expect(auto.container.querySelectorAll('.quick-action-btn').length).toBe(1);
    cleanup();
    const yolo = render(PaneTitlebar, { props: { pane: basePane({ mode: 'claude-yolo' }) } });
    expect(yolo.container.querySelectorAll('.quick-action-btn').length).toBe(1);
  });

  it('dispatches quickAction with the rendered prompt and session id on click', async () => {
    config.update((c) => ({
      ...c,
      quick_actions: [{ label: '🔁', prompt: 'rebase {{branch}} onto {{targetBranch}}' }],
    }));
    const { container, component } = render(PaneTitlebar, { props: { pane: basePane() } });
    const handler = (e: CustomEvent) => received.push(e.detail);
    const received: any[] = [];
    component.$on('quickAction', handler);

    await fireEvent.click(container.querySelector('.quick-action-btn')!);

    expect(received).toEqual([{ sessionId: 42, prompt: 'rebase feat/x onto alpha-main' }]);
  });

  it('renders duplicate quick actions without an each_key_duplicate crash', () => {
    config.update((c) => ({
      ...c,
      quick_actions: [
        { label: '⭐', prompt: '' },
        { label: '⭐', prompt: '' },
      ],
    }));
    const { container } = render(PaneTitlebar, { props: { pane: basePane() } });
    expect(container.querySelectorAll('.quick-action-btn').length).toBe(2);
  });
});

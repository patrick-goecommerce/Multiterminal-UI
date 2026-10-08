import { describe, it, expect, afterEach } from 'vitest';
import { render, fireEvent, createEvent, cleanup } from '@testing-library/svelte';
import { tick } from 'svelte';
import PaneTitlebar from './PaneTitlebar.svelte';
import type { Pane } from '../stores/tabs';

afterEach(cleanup);

function pane(): Pane {
  return {
    id: 'pane-1', sessionId: 'h1:42', name: 'Claude', mode: 'claude', model: '', focused: true, activity: 'done',
    cost: '', running: true, maximized: false, issueNumber: null, issueTitle: '', issueBranch: '',
    worktreePath: '', branch: '', targetBranch: '', zoomDelta: 0, background: false,
    display: 'terminal', conversationId: '', claudeSessionId: '', mcpProfile: '', autoName: '', sessionName: '',
    oscTitle: '', autoNameSource: '', userRenamed: false, finishPhase: '', activitySince: 0,
    attention: { state: 'done', rank: 3, seen: false, detail: '', source: '', since: 1 },
  };
}

describe('PaneTitlebar attention', () => {
  it('draws the state glyph', () => {
    const { container } = render(PaneTitlebar, { props: { pane: pane() } });
    expect(container.querySelector('.pane-titlebar svg.status-glyph')?.getAttribute('aria-label')).toBe('fertig, ungesehen');
  });

  it('opens the attention menu on a right-click on the bar', async () => {
    const { container } = render(PaneTitlebar, { props: { pane: pane() } });
    await fireEvent.contextMenu(container.querySelector('.pane-name')!);
    expect(container.querySelector('.am')).not.toBeNull();
  });

  // R18: the rename field keeps the native menu (copy, paste).
  it('leaves a right-click in the rename field to the field', async () => {
    const { container } = render(PaneTitlebar, { props: { pane: pane() } });
    await fireEvent.dblClick(container.querySelector('.pane-name')!);
    await tick();
    const input = container.querySelector('.rename-input') as HTMLInputElement;
    expect(input).not.toBeNull();
    const ev = createEvent.contextMenu(input);
    await fireEvent(input, ev);
    expect(ev.defaultPrevented).toBe(false);
    expect(container.querySelector('.am')).toBeNull();
  });
});

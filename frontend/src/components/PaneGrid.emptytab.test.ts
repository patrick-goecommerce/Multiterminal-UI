import { describe, it, expect, afterEach } from 'vitest';
import { render, cleanup } from '@testing-library/svelte';
import PaneGrid from './PaneGrid.svelte';

afterEach(cleanup);

// A tab with no panes is a legitimate state: the user closes the last pane, or
// a restored session carries a tab that was saved without one. PaneGrid must
// render its empty state for it. Getting the grid shape wrong here throws
// during the reactive pass, which aborts the whole App.svelte update flush —
// every other tab's grid stays unrendered and the footer freezes.
describe('PaneGrid with a tab that has no panes', () => {
  it('renders the empty state instead of throwing', () => {
    const { getByText } = render(PaneGrid, { props: { panes: [], tabId: 'tab-1' } });

    expect(getByText('Kein Terminal offen.')).toBeTruthy();
  });

  it('renders the empty state when a stale saved sizing is present', () => {
    const { getByText } = render(PaneGrid, {
      props: { panes: [], tabId: 'tab-1', colFractions: [1, 1], rowFractions: [1] },
    });

    expect(getByText('Kein Terminal offen.')).toBeTruthy();
  });
});

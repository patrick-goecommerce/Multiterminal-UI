import { describe, it, expect, afterEach, vi } from 'vitest';
import { render, cleanup } from '@testing-library/svelte';
import BoundaryHarness from './__fixtures__/BoundaryHarness.svelte';

afterEach(cleanup);

// App.svelte renders a PaneGrid for every tab, so a throw inside one grid used
// to abort the whole update flush and leave the window blank. Each tab layer is
// wrapped in <svelte:boundary>; these tests pin down that the boundary really
// does contain a throw coming out of a child's reactive statement — the
// assumption the containment rests on.
describe('svelte:boundary around a tab layer', () => {
  it('contains a throw from a child reactive statement', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {});

    const { getByTestId } = render(BoundaryHarness, { props: { cols: 0 } });

    expect(getByTestId('failed').textContent).toContain('Invalid array length');
  });

  it('keeps siblings outside the boundary rendered', () => {
    vi.spyOn(console, 'error').mockImplementation(() => {});

    const { getByTestId } = render(BoundaryHarness, { props: { cols: 0 } });

    expect(getByTestId('sibling')).toBeTruthy();
  });
});

import { readable } from 'svelte/store';

/**
 * How often the shared clock advances. Durations are read in minutes and
 * hours, so a coarse tick is enough — and it keeps one interval from waking
 * every pane badge every second.
 */
export const TICK_MS = 30_000;

/**
 * A single shared clock for every duration label in the UI.
 *
 * readable's start function runs on the first subscriber and its teardown on
 * the last, so exactly one interval exists no matter how many panes are open,
 * and none runs while nothing is listening.
 */
export const now = readable(Date.now(), (set) => {
  const timer = setInterval(() => set(Date.now()), TICK_MS);
  return () => clearInterval(timer);
});

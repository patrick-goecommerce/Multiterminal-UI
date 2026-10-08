import type { Tab } from '../stores/tabs';

/** For comparing paths: forward slashes, no trailing slash, lower case
 *  (Windows paths are case-insensitive). */
export function normPath(p: string): string {
  return (p || '').replace(/\\/g, '/').replace(/\/+$/, '').toLowerCase();
}

/** The tab that belongs to a project. `rootsByDir` maps a tab dir to its
 *  project root as the backend resolved it; a dir not resolved yet stands
 *  for itself. */
export function tabForProject(tabs: Tab[], rootsByDir: Record<string, string>, projectRoot: string): Tab | null {
  const want = normPath(projectRoot);
  if (!want) return null;
  return tabs.find((t) => normPath(rootsByDir[t.dir] ?? t.dir) === want) ?? null;
}

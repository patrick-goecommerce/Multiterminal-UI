import * as App from '../../wailsjs/go/backend/App';

export async function fetchBranch(dir: string): Promise<string> {
  try {
    return await App.GetGitBranch(dir || '.');
  } catch {
    return '';
  }
}

// How long a branch lookup is reused. Short enough that a checkout shows up
// in the badge soon, long enough that many panes on one directory share one
// git process.
export const BRANCH_TTL_MS = 30_000;

const branchCache = new Map<string, { at: number; value: Promise<string> }>();

/**
 * The current branch of dir, from a cache shared by every caller.
 *
 * Callers that run on every render (a pane titlebar) must use this instead of
 * fetchBranch: each fetch is a git process, and on Windows a console host
 * with it. A lookup already in flight is shared, not repeated.
 */
export function cachedBranch(dir: string, now: number = Date.now()): Promise<string> {
  const hit = branchCache.get(dir);
  if (hit && now - hit.at < BRANCH_TTL_MS) return hit.value;
  const value = fetchBranch(dir);
  branchCache.set(dir, { at: now, value });
  return value;
}

/** Test hook: forget every cached branch. */
export function resetBranchCache(): void {
  branchCache.clear();
}

export async function fetchCommitAge(dir: string): Promise<number> {
  try {
    const ts = await App.GetLastCommitTime(dir || '.');
    return ts > 0 ? Math.floor((Math.floor(Date.now() / 1000) - ts) / 60) : -1;
  } catch {
    return -1;
  }
}

export interface ConflictInfo {
  count: number;
  files: string[];
  operation: string;
}

export async function fetchConflicts(dir: string): Promise<ConflictInfo> {
  if (!dir) return { count: 0, files: [], operation: '' };
  try {
    const info = await App.GetMergeConflicts(dir);
    return {
      count: info.count,
      files: info.files || [],
      operation: info.operation || '',
    };
  } catch {
    return { count: 0, files: [], operation: '' };
  }
}

export async function fetchRepoURL(dir: string): Promise<string> {
  if (!dir) return '';
  try {
    return await App.GetRepoURL(dir);
  } catch {
    return '';
  }
}

export async function fetchIssueCount(dir: string): Promise<number> {
  if (!dir) return 0;
  try {
    const issues = await App.GetIssues(dir, 'open');
    return issues ? issues.length : 0;
  } catch {
    return 0;
  }
}

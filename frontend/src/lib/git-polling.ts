import * as App from '../../wailsjs/go/backend/App';

export async function fetchBranch(dir: string): Promise<string> {
  try {
    return await App.GetGitBranch(dir || '.');
  } catch {
    return '';
  }
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

export async function fetchIssueCount(dir: string): Promise<number> {
  if (!dir) return 0;
  try {
    const issues = await App.GetIssues(dir, 'open');
    return issues ? issues.length : 0;
  } catch {
    return 0;
  }
}

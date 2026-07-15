import * as App from '../../wailsjs/go/backend/App';

export interface IssueContext {
  number: number;
  title: string;
  body: string;
  labels: string[];
}

/** Build the text prompt to auto-send when launching Claude for an issue. */
export function buildIssuePrompt(issue: IssueContext): string {
  let text = `Closes #${issue.number}: ${issue.title}`;
  if (issue.labels.length > 0) text += `\nLabels: ${issue.labels.join(', ')}`;
  if (issue.body) {
    const desc = issue.body.length > 500 ? issue.body.slice(0, 500).trimEnd() + '...' : issue.body;
    text += `\n\n${desc}`;
  }
  text += `\n\nRef: #${issue.number}`;
  return text;
}

export interface BranchConflict {
  currentBranch: string;
  currentIssueNumber: number;
  dirtyWorkingTree: boolean;
}

export interface BranchSetupResult {
  issueBranch: string;
  worktreePath: string;
  targetBranch: string;
  sessionDir: string;
  conflict?: BranchConflict;
  cancelled?: boolean;
}

/**
 * Set up isolation for an issue before launching a session. Every issue pane
 * gets its own deterministic worktree (App.CreateIssueWorktree) — the main
 * repo's own checked-out branch is never touched by this default path.
 * GetOrCreateIssueBranch/BranchConflictDialog remain only as a fallback for
 * the rare case where the main repo is already manually sitting on a
 * DIFFERENT issue's branch (state MTUI itself no longer creates).
 */
export async function setupIssueBranch(
  sessionDir: string,
  issue: IssueContext,
  autoBranch: boolean,
): Promise<BranchSetupResult> {
  const result: BranchSetupResult = { issueBranch: '', worktreePath: '', targetBranch: '', sessionDir };

  if (!autoBranch) return result;

  const branchInfo = await App.IsOnIssueBranch(sessionDir, issue.number);

  if (branchInfo.on_issue_branch && !branchInfo.is_same_issue) {
    const dirty = !(await App.HasCleanWorkingTree(sessionDir));
    result.conflict = {
      currentBranch: branchInfo.branch_name,
      currentIssueNumber: branchInfo.issue_number,
      dirtyWorkingTree: dirty,
    };
    return result;
  }

  if (branchInfo.is_same_issue) {
    // Legacy/manual state: main repo already checked out on this exact issue
    // branch (predates worktree isolation, or done manually outside MTUI).
    result.issueBranch = branchInfo.branch_name;
    return result;
  }

  try {
    const wt = await App.CreateIssueWorktree(sessionDir, issue.number, issue.title);
    result.sessionDir = wt.path;
    result.issueBranch = wt.branch;
    result.worktreePath = wt.path;
    result.targetBranch = wt.target_branch;
  } catch (err: any) {
    const msg = err?.message || String(err);
    if (!confirm(`Worktree-Erstellung fehlgeschlagen:\n${msg}\n\nTrotzdem im Hauptrepo-Verzeichnis starten?`)) {
      result.cancelled = true;
    }
  }

  return result;
}

/**
 * Resolve a branch conflict after the user chose an action.
 */
export async function resolveBranchConflict(
  action: 'switch' | 'stay' | 'worktree',
  sessionDir: string,
  issue: IssueContext,
): Promise<{ issueBranch: string; worktreePath: string; targetBranch: string; sessionDir: string; cancelled?: boolean }> {
  const result = { issueBranch: '', worktreePath: '', targetBranch: '', sessionDir, cancelled: false };

  if (action === 'switch') {
    try {
      result.issueBranch = await App.GetOrCreateIssueBranch(sessionDir, issue.number, issue.title);
    } catch (err: any) {
      const msg = err?.message || String(err);
      if (!confirm(`Branch-Wechsel fehlgeschlagen:\n${msg}\n\nTrotzdem ohne Branch starten?`)) {
        result.cancelled = true;
      }
    }
  } else if (action === 'worktree') {
    try {
      const wt = await App.CreateIssueWorktree(sessionDir, issue.number, issue.title);
      if (wt) {
        result.sessionDir = wt.path;
        result.issueBranch = wt.branch;
        result.worktreePath = wt.path;
        result.targetBranch = wt.target_branch;
      }
    } catch (err: any) {
      const msg = err?.message || String(err);
      if (!confirm(`Worktree-Erstellung fehlgeschlagen:\n${msg}\n\nTrotzdem ohne Worktree starten?`)) {
        result.cancelled = true;
      }
    }
  }

  return result;
}

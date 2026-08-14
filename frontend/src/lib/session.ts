import { tabStore } from '../stores/tabs';
import { INDEX_TO_MODE, MODE_TO_INDEX, buildClaudeArgv, genSessionId } from './claude';
import type { SessionOpts } from './claude';
import { resolveMCPConfigPath } from './mcp';
import * as App from '../../wailsjs/go/backend/App';

/** Restore saved tabs/panes from the backend session file. */
export async function restoreSession(claudePath: string, codexPath?: string, geminiPath?: string): Promise<boolean> {
  try {
    const saved = await App.LoadTabs();
    if (!saved || !saved.tabs || saved.tabs.length === 0) return false;

    for (const savedTab of saved.tabs) {
      // setActive=false: avoid triggering xterm.js creation for each tab during
      // restore. Only the final setActiveTab() call below mounts the active tab.
      const tabId = tabStore.addTab(savedTab.name, savedTab.dir, false);

      // Mirror the launch-time setup call (App.svelte handleLaunch): restored
      // sessions read their memory files on process start too, so a project
      // whose CLAUDE.local.md/settings.local.json setup never ran (or is
      // stale) must be re-checked here, not just on the next fresh launch.
      const hasNonShellPane = savedTab.panes.some((p) => (INDEX_TO_MODE[p.mode] || 'shell') !== 'shell');
      if (hasNonShellPane && savedTab.dir) {
        try {
          await App.EnsureProjectWorktreeSetup(savedTab.dir);
        } catch (err) {
          console.error('[EnsureProjectWorktreeSetup]', err);
        }
      }

      for (const savedPane of savedTab.panes) {
        const mode = INDEX_TO_MODE[savedPane.mode] || 'shell';
        const display = (savedPane as any).display || 'terminal';
        const conversationId = (savedPane as any).conversation_id || '';

        const mcpProfile = (savedPane as any).mcp_profile || '';

        if (display === 'chat') {
          // Chat panes have no PTY; the backend chat process restarts lazily on next message (with --resume).
          const chatPaneId = tabStore.addPane(tabId, 0, savedPane.name, mode, savedPane.model || '', null, '', '', '', '', '', false, 'chat', conversationId, '', mcpProfile);
          if ((savedPane as any).user_renamed) tabStore.renamePane(tabId, chatPaneId, savedPane.name);
          continue;
        }

        // Resume the pinned claude session so restored terminal panes keep their
        // context (and stay toggle-able to chat). Pin a fresh id if none saved.
        let claudeSessionId = (savedPane as any).claude_session_id || '';
        if (mode.startsWith('claude') && !claudeSessionId) claudeSessionId = genSessionId();
        const sessOpts: SessionOpts | undefined = mode.startsWith('claude')
          ? ((savedPane as any).claude_session_id ? { resumeId: claudeSessionId } : { sessionId: claudeSessionId })
          : undefined;

        // Worktree panes MUST restore into their worktree, not the tab dir
        // (spec 4.2 — otherwise badge/finish point at the worktree while the
        // session runs in the main repo).
        let sessionDir = savedTab.dir || '';
        const wtPath = (savedPane as any).worktree_path || '';
        let wtBranch = (savedPane as any).worktree_branch || '';
        let wtTarget = (savedPane as any).target_branch || '';
        if (wtPath) {
          const exists = await App.WorktreeDirExists(wtPath).catch(() => false);
          if (exists) {
            sessionDir = wtPath;
          } else {
            console.warn('[restoreSession] worktree missing, falling back to main repo:', wtPath);
            wtBranch = ''; wtTarget = '';
          }
        }

        // The MCP profile is resolved against the pane's FINAL directory
        // (project-scope .mcp.json lives in the worktree), so argv is built
        // only after the worktree fallback above settled sessionDir.
        if (sessOpts) {
          sessOpts.mcpProfile = mcpProfile;
          sessOpts.mcpConfigPath = await resolveMCPConfigPath(sessionDir, mcpProfile);
        }
        const argv = buildClaudeArgv(mode, savedPane.model || '', claudePath, codexPath || 'codex', geminiPath || 'gemini', sessOpts);

        try {
          const sessionId = await App.CreateSession(argv, sessionDir, 24, 80, mode);
          if (sessionId > 0) {
            // Restore the pane's state-start timestamp so its duration badge
            // keeps counting from where it was, instead of starting over on
            // the first confirmed activity after restart (#189). A session
            // file predating this field has no key here, so `?? 0` (not
            // `||`) is what turns "missing" into 0 rather than undefined.
            const activitySince = (savedPane as any).activity_since ?? 0;
            if (activitySince) App.SeedActivitySince(sessionId, activitySince);

            const issueNum = (savedPane as any).issue_number || 0;
            const issueBranch = (savedPane as any).issue_branch || '';
            const paneId = tabStore.addPane(tabId, sessionId, savedPane.name, mode, savedPane.model || '', issueNum || null, '', issueBranch, wtPath && sessionDir === wtPath ? wtPath : '', wtBranch, wtTarget, false, 'terminal', '', claudeSessionId, mcpProfile);
            if ((savedPane as any).user_renamed) tabStore.renamePane(tabId, paneId, savedPane.name);
            const zd = (savedPane as any).zoom_delta || 0;
            if (zd !== 0) {
              tabStore.setZoomDelta(tabId, paneId, zd);
            }
            if (issueNum) App.LinkSessionIssue(sessionId, issueNum, '', issueBranch, savedTab.dir || '');
          }
        } catch (err) {
          console.error('[restoreSession] failed to create session:', err);
        }
      }
      // Restore pane-grid column/row sizing (resize handles). The store's
      // own reactive length-check (PaneGrid.svelte) is the actual safety
      // net if the pane count doesn't match what was saved.
      const savedCols = (savedTab as any).col_fractions;
      const savedRows = (savedTab as any).row_fractions;
      if (savedCols || savedRows) {
        tabStore.setGridFractions(tabId, savedCols, savedRows);
      }

      // Restore focused pane (addPane always focuses the last-added pane)
      if (savedTab.focus_idx >= 0) {
        const curState = tabStore.getState();
        const tab = curState.tabs.find(t => t.id === tabId);
        if (tab && savedTab.focus_idx < tab.panes.length) {
          tabStore.focusPane(tabId, tab.panes[savedTab.focus_idx].id);
        }
      }
    }

    const state = tabStore.getState();
    if (saved.active_tab >= 0 && saved.active_tab < state.tabs.length) {
      tabStore.setActiveTab(state.tabs[saved.active_tab].id);
    }
    return true;
  } catch (err) {
    console.error('[restoreSession]', err);
    return false;
  }
}

/** Pure mapping Pane → SavedPane shape (testbar, eine Quelle der Wahrheit). */
export function paneToSaved(pane: any) {
  return {
    name: pane.name,
    mode: MODE_TO_INDEX[pane.mode] ?? 0,
    model: pane.model || '',
    issue_number: pane.issueNumber || 0,
    issue_branch: pane.issueBranch || '',
    zoom_delta: pane.zoomDelta || 0,
    display: pane.display || 'terminal',
    conversation_id: pane.conversationId || '',
    claude_session_id: pane.claudeSessionId || '',
    mcp_profile: pane.mcpProfile || '',
    user_renamed: pane.userRenamed || false,
    worktree_path: pane.worktreePath || '',
    worktree_branch: pane.branch || '',
    target_branch: pane.targetBranch || '',
    activity_since: pane.activitySince || 0,
  };
}

/** Persist current tab/pane layout to the backend session file. */
export function saveSession(): void {
  const state = tabStore.getState();
  if (!state.tabs.length) return;
  const activeIdx = state.tabs.findIndex((t) => t.id === state.activeTabId);
  const tabs = state.tabs.map((tab) => ({
    name: tab.name,
    dir: tab.dir,
    focus_idx: tab.panes.findIndex((p) => p.focused),
    panes: tab.panes.map(paneToSaved),
    col_fractions: tab.colFractions,
    row_fractions: tab.rowFractions,
  }));
  App.SaveTabs({ active_tab: Math.max(activeIdx, 0), tabs } as any);
}

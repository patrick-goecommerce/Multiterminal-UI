import { get } from 'svelte/store';
import { tabStore } from '../stores/tabs';
import type { Pane } from '../stores/tabs';
import { INDEX_TO_MODE, MODE_TO_INDEX, buildClaudeArgv, genSessionId } from './claude';
import type { SessionOpts } from './claude';
import { resolveMCPConfigPath } from './mcp';
import * as App from '../../wailsjs/go/backend/App';
import { NO_SESSION, sameSession, sessionNumber, type SessionRef } from './sessionRef';
import { focusOrderIndices, restoreFocusOrder } from '../stores/focusLayout';

/**
 * Sessions the backend still holds.
 *
 * Empty unless the session daemon owns them: with the in-process host they
 * died with the last window, and a saved id then names nothing.
 */
async function liveSessions(): Promise<any[]> {
  try {
    return (await App.ListLiveSessions()) || [];
  } catch (err) {
    console.error('[restoreSession] ListLiveSessions failed:', err);
    return [];
  }
}

/** Restore saved tabs/panes from the backend session file. */
export async function restoreSession(claudePath: string, codexPath?: string, geminiPath?: string): Promise<boolean> {
  try {
    const saved = await App.LoadTabs();
    // Sessions that outlived the last window have to be claimed by the panes
    // that own them, or the restore would launch a second agent next to each
    // one that is still running.
    const alive = await liveSessions();
    const live = new Set<SessionRef>(alive.map((s: any) => s.id));
    if ((!saved || !saved.tabs || saved.tabs.length === 0) && live.size === 0) return false;

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

      // restoredIds[i] is the pane created for saved pane i (undefined when
      // it could not be restored), so the focus order can be mapped back.
      const restoredIds: (string | undefined)[] = [];
      for (const savedPane of savedTab.panes) {
        restoredIds.push(undefined);
        const mode = INDEX_TO_MODE[savedPane.mode] || 'shell';
        const display = (savedPane as any).display || 'terminal';
        const conversationId = (savedPane as any).conversation_id || '';

        const mcpProfile = (savedPane as any).mcp_profile || '';

        if (display === 'chat') {
          // Chat panes have no PTY; the backend chat process restarts lazily on next message (with --resume).
          const chatPaneId = tabStore.addPane(tabId, NO_SESSION, savedPane.name, mode, savedPane.model || '', null, '', '', '', '', '', false, 'chat', conversationId, '', mcpProfile);
          restoredIds[restoredIds.length - 1] = chatPaneId;
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
          // A pane whose session is still running re-attaches to it: same
          // process, same context, and the screen comes back from the host's
          // replay buffer instead of blank.
          // The saved ref is matched against what the host reports rather than
          // passed on as is: a pane saved before refs carried a hub has a bare
          // number, which the backend refuses to guess about.
          const savedSessionId: SessionRef = String((savedPane as any).session_id || '');
          const liveMatch = savedSessionId ? [...live].find((l) => sameSession(savedSessionId, l)) : undefined;
          let sessionId: SessionRef = NO_SESSION;
          let reattached = false;
          if (liveMatch) {
            reattached = await App.AttachSession(liveMatch, 24, 80).catch(() => false);
            if (reattached) {
              sessionId = liveMatch;
              live.delete(liveMatch);
            }
          }
          if (!reattached) {
            sessionId = await App.CreateSession(argv, sessionDir, 24, 80, mode);
          }
          if (sessionId) {
            // Restore the pane's state-start timestamp so its duration badge
            // keeps counting from where it was, instead of starting over on
            // the first confirmed activity after restart (#189). A session
            // file predating this field has no key here, so `?? 0` (not
            // `||`) is what turns "missing" into 0 rather than undefined.
            //
            // The state travels with it: the relaunched CLI produces output
            // for its whole boot, so the first state the backend confirms
            // after a restart is almost always a transient "active". Without
            // the state the seed would attach to that one and show hours of
            // "läuft" on a session two seconds old.
            //
            // A re-attached pane needs none of this: its session never
            // stopped, so the backend's own state is the current one and a
            // saved timestamp would only overwrite it with something older.
            const activitySince = (savedPane as any).activity_since ?? 0;
            const activityState = (savedPane as any).activity_state ?? '';
            if (!reattached && activitySince && activityState) {
              App.SeedActivitySince(sessionId, activitySince, activityState).catch((err) => {
                console.error('[restoreSession] SeedActivitySince failed:', err);
              });
            }

            const issueNum = (savedPane as any).issue_number || 0;
            const issueBranch = (savedPane as any).issue_branch || '';
            const paneId = tabStore.addPane(tabId, sessionId, savedPane.name, mode, savedPane.model || '', issueNum || null, '', issueBranch, wtPath && sessionDir === wtPath ? wtPath : '', wtBranch, wtTarget, false, 'terminal', '', claudeSessionId, mcpProfile);
            // A re-attached session kept running while no window was up; its
            // state is only sent on the next change. Ask for it now that the
            // pane exists, or it shows "startet" until the agent moves.
            if (reattached) App.ResendActivity(sessionId).catch(() => {});
            restoredIds[restoredIds.length - 1] = paneId;
            if ((savedPane as any).user_renamed) tabStore.renamePane(tabId, paneId, savedPane.name);
            if ((savedPane as any).auto_name) tabStore.setAutoName(sessionId, (savedPane as any).auto_name, 'llm');
            tabStore.setSessionName(sessionId, (savedPane as any).agent_name || '');
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

      restoreFocusOrder(tabId, savedTab.focus_order, restoredIds);

      // Restore focused pane (addPane always focuses the last-added pane)
      if (savedTab.focus_idx >= 0) {
        const curState = tabStore.getState();
        const tab = curState.tabs.find(t => t.id === tabId);
        if (tab && savedTab.focus_idx < tab.panes.length) {
          tabStore.focusPane(tabId, tab.panes[savedTab.focus_idx].id);
        }
      }
    }

    await adoptOrphanSessions(alive, live);

    const state = tabStore.getState();
    if (saved && saved.active_tab >= 0 && saved.active_tab < state.tabs.length) {
      tabStore.setActiveTab(state.tabs[saved.active_tab].id);
    }
    return true;
  } catch (err) {
    console.error('[restoreSession]', err);
    return false;
  }
}

/**
 * Give every still-running session left over after the restore a pane.
 *
 * These are sessions no saved pane claimed: an agent that opened one over the
 * control API while no window was up, or a pane whose tab was closed without
 * closing its session. Without this they keep running with nothing on screen
 * pointing at them, which is the state herdr's sidebar exists to prevent.
 * Grouped by directory so a project's leftovers land in one tab.
 */
async function adoptOrphanSessions(alive: any[], unclaimed: Set<SessionRef>): Promise<void> {
  if (unclaimed.size === 0) return;

  const byDir = new Map<string, any[]>();
  for (const s of alive) {
    if (!unclaimed.has(s.id)) continue;
    const list = byDir.get(s.dir || '') || [];
    list.push(s);
    byDir.set(s.dir || '', list);
  }

  for (const [dir, sessions] of byDir) {
    const tabId = tabStore.addTab(dirLabel(dir), dir, false);
    for (const s of sessions) {
      const attached = await App.AttachSession(s.id, 24, 80).catch(() => false);
      if (!attached) continue;
      tabStore.addPane(tabId, s.id, s.name || `Session ${sessionNumber(s.id)}`, (s.mode || 'shell') as any, '');
      App.ResendActivity(s.id).catch(() => {});
    }
  }
}

/** dirLabel names a tab after the directory's last segment. */
/**
 * Ends the processes behind one pane. A chat pane runs its conversation as a
 * claude process keyed by conversation ID, a terminal pane as a host session.
 */
export function closePaneSession(pane: Pane): void {
  const done = pane.display === 'chat'
    ? (pane.conversationId ? App.CloseChatSession(pane.conversationId) : undefined)
    : (pane.sessionId ? App.CloseSession(pane.sessionId) : undefined);
  Promise.resolve(done).catch((err) => console.error('[closePaneSession]', pane.id, err));
}

/**
 * Closes a tab and ends every session in it.
 *
 * tabStore.closeTab only drops the tab from the store, and for a long time
 * that was all closing a tab did: each pane kept its claude tree, its MCP
 * servers and its console running, invisibly, until the app quit, and in
 * daemon mode for good. Moving a tab to another window goes through the store
 * directly and must keep its sessions, which is why this lives here and not
 * in closeTab.
 *
 * The store never closes a window's last tab, so neither does this, and that
 * tab's sessions are left alone.
 */
export function closeTabWithSessions(tabId: string): boolean {
  const { tabs } = get(tabStore);
  if (tabs.length <= 1) return false;
  const tab = tabs.find((t) => t.id === tabId);
  if (!tab) return false;
  for (const pane of tab.panes) closePaneSession(pane);
  tabStore.closeTab(tabId);
  return true;
}

export function dirLabel(dir: string): string {
  if (!dir) return 'Wiederhergestellt';
  const trimmed = dir.replace(/[\\/]+$/, '');
  const parts = trimmed.split(/[\\/]/);
  return parts[parts.length - 1] || trimmed;
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
    // Paired with the timestamp: on restore the seed only counts if the pane
    // confirms this same state again (see restoreSession).
    activity_state: pane.activity || '',
    // Only useful while the session outlives the window, i.e. with the session
    // daemon; the restore checks it against what the host still holds.
    session_id: pane.sessionId || '',
    // Names that came from the agent or from MTUI's naming call. Without them
    // a restored pane shows its launch name until the agent reports again.
    auto_name: pane.autoName || '',
    agent_name: pane.sessionName || '',
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
    focus_order: focusOrderIndices(tab.id, tab.panes),
  }));
  App.SaveTabs({ active_tab: Math.max(activeIdx, 0), tabs } as any);
}

import { tabStore } from '../stores/tabs';
import { INDEX_TO_MODE, MODE_TO_INDEX, buildClaudeArgv } from './claude';
import * as App from '../../wailsjs/go/backend/App';

/** Restore saved tabs/panes from the backend session file. */
export async function restoreSession(claudePath: string): Promise<boolean> {
  try {
    const saved = await App.LoadTabs();
    if (!saved || !saved.tabs || saved.tabs.length === 0) return false;

    for (const savedTab of saved.tabs) {
      const tabId = tabStore.addTab(savedTab.name, savedTab.dir);
      for (const savedPane of savedTab.panes) {
        const mode = INDEX_TO_MODE[savedPane.mode] || 'shell';
        const argv = buildClaudeArgv(mode, savedPane.model || '', claudePath);
        try {
          const sessionId = await App.CreateSession(argv, savedTab.dir || '', 24, 80);
          if (sessionId > 0) {
            const issueNum = (savedPane as any).issue_number || 0;
            const issueBranch = (savedPane as any).issue_branch || '';
            tabStore.addPane(tabId, sessionId, savedPane.name, mode, savedPane.model || '', issueNum || null, '', issueBranch);
            if (issueNum) App.LinkSessionIssue(sessionId, issueNum, '', issueBranch, savedTab.dir || '');
          }
        } catch (err) {
          console.error('[restoreSession] failed to create session:', err);
        }
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

/** Persist current tab/pane layout to the backend session file. */
export function saveSession(): void {
  const state = tabStore.getState();
  if (!state.tabs.length) return;
  const activeIdx = state.tabs.findIndex((t) => t.id === state.activeTabId);
  const tabs = state.tabs.map((tab) => ({
    name: tab.name,
    dir: tab.dir,
    focus_idx: tab.panes.findIndex((p) => p.focused),
    panes: tab.panes.map((pane) => ({
      name: pane.name,
      mode: MODE_TO_INDEX[pane.mode] ?? 0,
      model: pane.model || '',
      issue_number: pane.issueNumber || 0,
      issue_branch: pane.issueBranch || '',
    })),
  }));
  App.SaveTabs({ active_tab: Math.max(activeIdx, 0), tabs } as any);
}

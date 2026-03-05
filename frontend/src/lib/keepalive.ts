import { tabStore } from '../stores/tabs';
import { buildClaudeArgv, encodeForPty } from './claude';
import * as App from '../../wailsjs/go/backend/App';
import { isMainWindow } from './window';

export interface KeepAliveConfig {
  enabled: boolean | null;
  interval_minutes: number;
  message: string;
}

/** Find the first running Claude or claude-yolo pane across all tabs. */
function findFirstClaudePane(): { sessionId: number; tabId: string } | null {
  const state = tabStore.getState();
  for (const tab of state.tabs) {
    for (const pane of tab.panes) {
      if ((pane.mode === 'claude' || pane.mode === 'claude-yolo') && pane.running) {
        return { sessionId: pane.sessionId, tabId: tab.id };
      }
    }
  }
  return null;
}

/**
 * Start the keep-alive loop after session restore.
 * Returns a cleanup function to stop the loop (call in onDestroy).
 *
 * Behaviour:
 * 1. If no Claude pane exists after restore → create one in the first tab.
 * 2. Every `interval_minutes` minutes: if no activity in any session for that
 *    interval, write the keep-alive message to the first Claude pane found.
 */
export async function startKeepAliveLoop(
  cfg: KeepAliveConfig,
  claudePath: string,
): Promise<() => void> {
  if (!isMainWindow()) return () => {};
  if (!cfg.enabled || cfg.interval_minutes <= 0) return () => {};

  // Auto-start: create a Claude pane if none exists
  if (!findFirstClaudePane()) {
    const state = tabStore.getState();
    if (state.tabs.length > 0) {
      const firstTab = state.tabs[0];
      const argv = buildClaudeArgv('claude', '', claudePath);
      try {
        const sessionId = await App.CreateSession(argv, firstTab.dir || '', 24, 80, 'claude');
        if (sessionId > 0) {
          tabStore.addPane(firstTab.id, sessionId, 'Claude', 'claude', '');
        }
      } catch (err) {
        console.error('[keepalive] auto-start failed:', err);
      }
    }
  }

  const intervalMs = cfg.interval_minutes * 60 * 1000;
  const intervalSec = cfg.interval_minutes * 60;

  const timer = setInterval(async () => {
    try {
      const lastActivity = await App.GetGlobalLastActivityUnix();
      const nowSec = Math.floor(Date.now() / 1000);

      if (lastActivity > 0 && nowSec - lastActivity < intervalSec) {
        return; // activity within window
      }

      const pane = findFirstClaudePane();
      if (!pane) return; // no Claude pane to ping

      await App.WriteToSession(pane.sessionId, encodeForPty(cfg.message + '\n'));
    } catch (err) {
      console.error('[keepalive] ping failed:', err);
    }
  }, intervalMs);

  return () => clearInterval(timer);
}

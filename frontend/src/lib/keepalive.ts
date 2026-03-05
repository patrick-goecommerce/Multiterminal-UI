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

  async function sendPing() {
    const pane = findFirstClaudePane();
    if (!pane) return;
    // Send message text and Enter as separate writes (mimics real typing)
    await App.WriteToSession(pane.sessionId, encodeForPty(cfg.message));
    await new Promise(r => setTimeout(r, 100));
    await App.WriteToSession(pane.sessionId, encodeForPty('\r'));
  }

  async function ping() {
    try {
      const lastActivity = await App.GetGlobalLastActivityUnix();
      const nowSec = Math.floor(Date.now() / 1000);
      if (lastActivity > 0 && nowSec - lastActivity < intervalSec) {
        return; // activity within window
      }
      await sendPing();
    } catch (err) {
      console.error('[keepalive] ping failed:', err);
    }
  }

  // Send once at startup — wait until Claude's startup output has settled
  // (no PTY output for 2s), then send. Gives up after 60s.
  async function startupPing() {
    const timeoutMs = 60_000;
    const idleMs = 2_000;
    const pollMs = 500;
    const start = Date.now();
    let lastSeen = await App.GetGlobalLastActivityUnix();
    let lastChangeAt = Date.now();

    while (Date.now() - start < timeoutMs) {
      await new Promise(r => setTimeout(r, pollMs));
      const cur = await App.GetGlobalLastActivityUnix();
      if (cur !== lastSeen) {
        lastSeen = cur;
        lastChangeAt = Date.now();
      } else if (Date.now() - lastChangeAt >= idleMs) {
        await sendPing();
        return;
      }
    }
  }
  startupPing().catch(err => console.error('[keepalive] startup ping failed:', err));

  // Then repeat every interval.
  const timer = setInterval(ping, intervalMs);

  return () => clearInterval(timer);
}

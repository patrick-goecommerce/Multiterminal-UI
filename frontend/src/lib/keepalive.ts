import { tabStore } from '../stores/tabs';
import { buildClaudeArgv, encodeForPty } from './claude';
import * as App from '../../wailsjs/go/backend/App';
import { isMainWindow } from './window';

export interface KeepAliveConfig {
  enabled: boolean | null;
  interval_minutes: number;
  message: string;
}

/**
 * Start the keep-alive's window half after session restore.
 * Returns a cleanup function (call in onDestroy).
 *
 * The periodic nudge is NOT here any more. It runs on the session host
 * (internal/hub/keepalive.go), because in a window it stops the moment the
 * window closes, and that is exactly the stretch in which a session goes cold
 * unnoticed. The host also sees every session rather than one window's.
 *
 * What is left is the half that needs a window:
 *
 * 1. If no Claude session exists anywhere after restore, create one in the
 *    first tab. That adds a pane to a tab, which only a window can do.
 * 2. Ping once at startup, after Claude's own boot output has settled. This
 *    is about *this app starting*, not about a stretch of silence, so it has
 *    no counterpart on the host.
 */
export async function startKeepAliveLoop(
  cfg: KeepAliveConfig,
  claudePath: string,
): Promise<() => void> {
  if (!isMainWindow()) return () => {};
  if (!cfg.enabled || cfg.interval_minutes <= 0) return () => {};

  // Auto-start: create a Claude session if none exists in ANY window.
  const existing = await App.GetFirstClaudeSessionID().catch(() => -1);
  if (existing < 0) {
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

  // Set by the returned cleanup, so a window that closes mid-wait stops
  // polling instead of finishing its minute on a host nobody is watching.
  let cancelled = false;

  async function sendPing(sessionId: number) {
    // Message text and Enter as separate writes: an agent's redraw can swallow
    // a Return that arrives in the same chunk as the text.
    await App.WriteToSession(sessionId, encodeForPty(cfg.message));
    await new Promise(r => setTimeout(r, 100));
    await App.WriteToSession(sessionId, encodeForPty('\r'));
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

    while (!cancelled && Date.now() - start < timeoutMs) {
      await new Promise(r => setTimeout(r, pollMs));
      const cur = await App.GetGlobalLastActivityUnix();
      if (cur !== lastSeen) {
        lastSeen = cur;
        lastChangeAt = Date.now();
      } else if (Date.now() - lastChangeAt >= idleMs) {
        const sessionId = await App.GetFirstClaudeSessionID();
        if (sessionId >= 0) await sendPing(sessionId);
        return;
      }
    }
  }
  startupPing().catch(err => console.error('[keepalive] startup ping failed:', err));

  return () => {
    cancelled = true;
  };
}

import { tabStore } from '../stores/tabs';
import { buildClaudeArgv, encodeForPty } from './claude';
import * as App from '../../wailsjs/go/backend/App';
import { isMainWindow } from './window';

export interface KeepAliveConfig {
  enabled: boolean | null;
  interval_minutes: number;
  message: string;
}

// How often we poll for idle time, independent of the configured threshold —
// keeps the actual ping within ~1 minute of crossing the threshold instead of
// only being checked once per (potentially multi-hour) interval.
const POLL_MS = 60_000;

/**
 * Start the keep-alive loop after session restore.
 * Returns a cleanup function to stop the loop (call in onDestroy).
 *
 * Behaviour:
 * 1. If no Claude session exists anywhere (any window) after restore →
 *    create one in the first tab of the main window.
 * 2. Every minute: if no activity in any session for `interval_minutes`,
 *    write the keep-alive message to the oldest running Claude session,
 *    wherever it lives (main window or a detached one) — the target is
 *    resolved via the backend, which tracks all sessions process-wide.
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

  const thresholdSec = cfg.interval_minutes * 60;
  let lastPingAtSec = 0;

  async function sendPing(sessionId: number) {
    // Send message text and Enter as separate writes (mimics real typing)
    await App.WriteToSession(sessionId, encodeForPty(cfg.message));
    await new Promise(r => setTimeout(r, 100));
    await App.WriteToSession(sessionId, encodeForPty('\r'));
    lastPingAtSec = Math.floor(Date.now() / 1000);
  }

  async function tick() {
    try {
      const sessionId = await App.GetFirstClaudeSessionID();
      if (sessionId < 0) return;

      const nowSec = Math.floor(Date.now() / 1000);
      if (nowSec - lastPingAtSec < thresholdSec) return; // already pinged recently

      const lastActivity = await App.GetGlobalLastActivityUnix();
      if (lastActivity > 0 && nowSec - lastActivity < thresholdSec) return; // still active

      await sendPing(sessionId);
    } catch (err) {
      console.error('[keepalive] tick failed:', err);
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
        const sessionId = await App.GetFirstClaudeSessionID();
        if (sessionId >= 0) await sendPing(sessionId);
        return;
      }
    }
  }
  startupPing().catch(err => console.error('[keepalive] startup ping failed:', err));

  // Poll frequently; tick() itself gates on the configured threshold.
  const timer = setInterval(tick, POLL_MS);

  return () => clearInterval(timer);
}

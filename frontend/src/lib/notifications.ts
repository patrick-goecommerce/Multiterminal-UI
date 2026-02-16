import * as App from '../../wailsjs/go/backend/App';

const recentNotifications = new Map<string, number>();
const DEDUP_WINDOW_MS = 5000;

/** Send a native OS notification via the Go backend (deduplicated). */
export function sendNotification(title: string, body: string) {
  const key = `${title}\0${body}`;
  const now = Date.now();
  const last = recentNotifications.get(key);
  if (last && now - last < DEDUP_WINDOW_MS) return;
  recentNotifications.set(key, now);
  // Cleanup alte Einträge
  if (recentNotifications.size > 50) {
    for (const [k, t] of recentNotifications) {
      if (now - t >= DEDUP_WINDOW_MS) recentNotifications.delete(k);
    }
  }
  App.SendNotification(title, body);
}

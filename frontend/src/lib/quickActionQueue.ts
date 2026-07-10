import * as App from '../../wailsjs/go/backend/App';

/** Send a quick-action's already-rendered prompt into a session's prompt
 *  queue. Thin wrapper so App.svelte's event handler and its test share one
 *  implementation instead of App.svelte re-implementing the AddToQueue call. */
export async function sendQuickAction(sessionId: number, prompt: string): Promise<void> {
  await App.AddToQueue(sessionId, prompt);
}

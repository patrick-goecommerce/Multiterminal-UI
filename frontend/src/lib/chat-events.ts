import { EventsOn } from '../../wailsjs/runtime/runtime';
import { chat } from '../stores/chat';

/**
 * Subscribes to backend chat:* events.
 * Call once on app mount; returns an unsubscribe function.
 */
export function subscribeChatEvents(): () => void {
  const offs: Array<() => void> = [];

  // Streaming assistant text delta — append to buffer, keep streaming flag true
  offs.push(EventsOn('chat:stream', (event: any) => {
    const { conversationId, delta } = event.data || {};
    if (!conversationId) return;
    chat.appendStream(conversationId, delta ?? '');
  }));

  // Tool-use/tool-result message arriving mid-stream — append as a card, do NOT stop streaming
  offs.push(EventsOn('chat:tool', (event: any) => {
    const { conversationId, message } = event.data || {};
    if (!conversationId || !message) return;
    chat.addMessage(conversationId, message);
  }));

  // Finalized assistant message — ends the turn, clears stream buffer
  offs.push(EventsOn('chat:done', (event: any) => {
    const { conversationId, message } = event.data || {};
    if (!conversationId || !message) return;
    chat.completeStream(conversationId, message);
  }));

  // Error during generation — stop streaming state
  offs.push(EventsOn('chat:error', (event: any) => {
    const { conversationId } = event.data || {};
    if (!conversationId) return;
    chat.streamError(conversationId);
  }));

  return () => offs.forEach(off => off && off());
}

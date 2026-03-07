import { writable, derived } from 'svelte/store';

export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant' | 'ask_user';
  content: string;
  timestamp: string;
  cost: string;
  tokens: number;
}

export interface Conversation {
  id: string;
  title: string;
  provider: string;
  model: string;
  scope: string;
  created_at: string;
  updated_at: string;
  messages: ChatMessage[];
}

export interface ChatStore {
  conversations: Conversation[];
  activeConvId: string | null;
  loading: boolean;
  streaming: boolean;
  streamBuffer: string;
  dir: string;
}

const initialStore: ChatStore = {
  conversations: [],
  activeConvId: null,
  loading: false,
  streaming: false,
  streamBuffer: '',
  dir: '',
};

function createChatStore() {
  const { subscribe, set, update } = writable<ChatStore>(initialStore);

  return {
    subscribe,

    /** Set the project directory */
    setDir(dir: string) {
      update(s => ({ ...s, dir, conversations: [], activeConvId: null, loading: true }));
    },

    /** Load conversations from backend */
    setConversations(convs: Conversation[]) {
      update(s => ({ ...s, conversations: convs, loading: false }));
    },

    /** Set the active conversation */
    setActive(convId: string | null) {
      update(s => ({ ...s, activeConvId: convId, streamBuffer: '' }));
    },

    /** Add a new conversation */
    addConversation(conv: Conversation) {
      update(s => ({
        ...s,
        conversations: [conv, ...s.conversations],
        activeConvId: conv.id,
      }));
    },

    /** Remove a conversation */
    removeConversation(convId: string) {
      update(s => ({
        ...s,
        conversations: s.conversations.filter(c => c.id !== convId),
        activeConvId: s.activeConvId === convId ? null : s.activeConvId,
      }));
    },

    /** Add a user message to the active conversation */
    addUserMessage(msg: ChatMessage) {
      update(s => ({
        ...s,
        conversations: s.conversations.map(c =>
          c.id === s.activeConvId
            ? { ...c, messages: [...c.messages, msg], updated_at: msg.timestamp }
            : c
        ),
        streaming: true,
        streamBuffer: '',
      }));
    },

    /** Append streaming delta */
    appendStream(convId: string, delta: string) {
      update(s => {
        if (s.activeConvId !== convId) return s;
        return { ...s, streamBuffer: s.streamBuffer + delta };
      });
    },

    /** Complete streaming with final message */
    completeStream(convId: string, msg: ChatMessage) {
      update(s => ({
        ...s,
        conversations: s.conversations.map(c =>
          c.id === convId
            ? { ...c, messages: [...c.messages, msg], updated_at: msg.timestamp }
            : c
        ),
        streaming: false,
        streamBuffer: '',
      }));
    },

    /** Handle stream error */
    streamError(convId: string) {
      update(s => ({ ...s, streaming: false, streamBuffer: '' }));
    },

    /** Update conversation title */
    renameConversation(convId: string, title: string) {
      update(s => ({
        ...s,
        conversations: s.conversations.map(c =>
          c.id === convId ? { ...c, title } : c
        ),
      }));
    },

    /** Reset store */
    reset() {
      set(initialStore);
    },
  };
}

export const chat = createChatStore();

/** Derived: active conversation */
export const activeConversation = derived(chat, $c => {
  if (!$c.activeConvId) return null;
  return $c.conversations.find(c => c.id === $c.activeConvId) ?? null;
});

/** Derived: unread count (conversations with new messages while not active) */
export const chatUnreadCount = derived(chat, $c => {
  // Simple heuristic: count conversations updated in last 5 minutes that aren't active
  return 0; // Will be enhanced with proper unread tracking
});

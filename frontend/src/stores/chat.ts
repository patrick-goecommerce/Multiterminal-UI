import { writable, derived } from 'svelte/store';

export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant' | 'tool' | 'ask_user';
  content: string;
  timestamp: string;
  cost: string;
  tokens: number;
  tool_name?: string;
  tool_input?: string;
  tool_result?: string;
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
  session_id?: string;
  permission_mode?: string;
}

export interface ConvStreamState {
  streaming: boolean;
  buffer: string;
}

export interface ChatStore {
  conversations: Conversation[];
  activeConvId: string | null;
  loading: boolean;
  streams: Record<string, ConvStreamState>;
  dir: string;
}

const initialStore: ChatStore = {
  conversations: [],
  activeConvId: null,
  loading: false,
  streams: {},
  dir: '',
};

function setStream(s: ChatStore, convId: string, next: ConvStreamState): ChatStore {
  return { ...s, streams: { ...s.streams, [convId]: next } };
}

function createChatStore() {
  const { subscribe, set, update } = writable<ChatStore>(initialStore);

  return {
    subscribe,

    setDir(dir: string) {
      update(s => ({ ...s, dir, conversations: [], activeConvId: null, loading: true }));
    },

    setConversations(convs: Conversation[]) {
      update(s => ({ ...s, conversations: convs, loading: false }));
    },

    setActive(convId: string | null) {
      update(s => ({ ...s, activeConvId: convId }));
    },

    addConversation(conv: Conversation) {
      update(s => ({ ...s, conversations: [conv, ...s.conversations], activeConvId: conv.id }));
    },

    removeConversation(convId: string) {
      update(s => {
        const streams = { ...s.streams };
        delete streams[convId];
        return {
          ...s,
          conversations: s.conversations.filter(c => c.id !== convId),
          activeConvId: s.activeConvId === convId ? null : s.activeConvId,
          streams,
        };
      });
    },

    /** Append a user message to a conversation and mark it streaming. */
    addUserMessage(convId: string, msg: ChatMessage) {
      update(s => {
        const withMsg = {
          ...s,
          conversations: s.conversations.map(c =>
            c.id === convId ? { ...c, messages: [...c.messages, msg], updated_at: msg.timestamp } : c
          ),
        };
        return setStream(withMsg, convId, { streaming: true, buffer: '' });
      });
    },

    /** Append a streaming delta for a conversation. */
    appendStream(convId: string, delta: string) {
      update(s => {
        const prev = s.streams[convId] ?? { streaming: true, buffer: '' };
        return setStream(s, convId, { streaming: true, buffer: prev.buffer + delta });
      });
    },

    /** Append a message to a conversation without changing its stream state. */
    addMessage(convId: string, msg: ChatMessage) {
      update(s => ({
        ...s,
        conversations: s.conversations.map(c =>
          c.id === convId ? { ...c, messages: [...c.messages, msg], updated_at: msg.timestamp } : c
        ),
      }));
    },

    /** Finalize streaming: append the completed message and clear the buffer. */
    completeStream(convId: string, msg: ChatMessage) {
      update(s => {
        const withMsg = {
          ...s,
          conversations: s.conversations.map(c =>
            c.id === convId ? { ...c, messages: [...c.messages, msg], updated_at: msg.timestamp } : c
          ),
        };
        return setStream(withMsg, convId, { streaming: false, buffer: '' });
      });
    },

    /** Mark streaming stopped after an error. */
    streamError(convId: string) {
      update(s => setStream(s, convId, { streaming: false, buffer: '' }));
    },

    renameConversation(convId: string, title: string) {
      update(s => ({
        ...s,
        conversations: s.conversations.map(c => (c.id === convId ? { ...c, title } : c)),
      }));
    },

    reset() {
      set(initialStore);
    },
  };
}

export const chat = createChatStore();

/** Derived: active conversation. */
export const activeConversation = derived(chat, $c => {
  if (!$c.activeConvId) return null;
  return $c.conversations.find(c => c.id === $c.activeConvId) ?? null;
});

/** Returns a derived store: is the given conversation currently streaming? */
export const isStreaming = (convId: string | null) =>
  derived(chat, $c => (convId ? $c.streams[convId]?.streaming ?? false : false));

/** Returns a derived store: the current streaming buffer for a conversation. */
export const streamBuffer = (convId: string | null) =>
  derived(chat, $c => (convId ? $c.streams[convId]?.buffer ?? '' : ''));

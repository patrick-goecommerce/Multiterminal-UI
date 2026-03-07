import { writable, derived } from 'svelte/store';

/** Column IDs matching the Go backend constants */
export const COLUMN_IDS = ['backlog', 'planned', 'in_progress', 'review', 'done'] as const;
export type ColumnID = typeof COLUMN_IDS[number];

/** Column display labels (German UI) */
export const COLUMN_LABELS: Record<ColumnID, string> = {
  backlog: 'Backlog',
  planned: 'Geplant',
  in_progress: 'In Arbeit',
  review: 'Review',
  done: 'Erledigt',
};

/** Column accent colors */
export const COLUMN_COLORS: Record<ColumnID, string> = {
  backlog: '#a6adc8',
  planned: '#89b4fa',
  in_progress: '#39ff14',
  review: '#f5a623',
  done: '#22c55e',
};

export interface KanbanCard {
  id: string;
  issue_number: number;
  title: string;
  labels: string[];
  dir: string;
  session_id: number;
  priority: number;
  dependencies: number[];
  plan_id: string;
  schedule_id: string;
  created_at: string;
}

export interface PlanStep {
  issue_number: number;
  card_id: string;
  title: string;
  order: number;
  parallel: boolean;
  session_id: number;
  status: string;
  prompt: string;
}

export interface Plan {
  id: string;
  dir: string;
  created_at: string;
  steps: PlanStep[];
  status: string;
}

export interface ScheduledTask {
  id: string;
  name: string;
  dir: string;
  prompt: string;
  schedule: string;
  mode: string;
  model: string;
  enabled: boolean;
  last_run: string;
  next_run: string;
}

export interface KanbanState {
  columns: Record<string, KanbanCard[]>;
  plans: Plan[];
  schedules: ScheduledTask[];
}

export interface KanbanStore {
  state: KanbanState;
  loading: boolean;
  dir: string;
  activeTab: 'board' | 'schedules';
  dragCard: KanbanCard | null;
  dragSourceCol: string | null;
}

const emptyState: KanbanState = {
  columns: {
    backlog: [],
    planned: [],
    in_progress: [],
    review: [],
    done: [],
  },
  plans: [],
  schedules: [],
};

const initialStore: KanbanStore = {
  state: emptyState,
  loading: false,
  dir: '',
  activeTab: 'board',
  dragCard: null,
  dragSourceCol: null,
};

function createKanbanStore() {
  const { subscribe, set, update } = writable<KanbanStore>(initialStore);

  return {
    subscribe,

    /** Set the project directory and reset state */
    setDir(dir: string) {
      update(s => ({ ...s, dir, state: emptyState, loading: true }));
    },

    /** Update state from backend response */
    setState(state: KanbanState) {
      update(s => ({
        ...s,
        state: {
          columns: state.columns || emptyState.columns,
          plans: state.plans || [],
          schedules: state.schedules || [],
        },
        loading: false,
      }));
    },

    /** Set loading state */
    setLoading(loading: boolean) {
      update(s => ({ ...s, loading }));
    },

    /** Switch between board and schedules tab */
    setActiveTab(tab: 'board' | 'schedules') {
      update(s => ({ ...s, activeTab: tab }));
    },

    /** Start dragging a card */
    startDrag(card: KanbanCard, sourceCol: string) {
      update(s => ({ ...s, dragCard: card, dragSourceCol: sourceCol }));
    },

    /** End drag operation */
    endDrag() {
      update(s => ({ ...s, dragCard: null, dragSourceCol: null }));
    },

    /** Optimistically move a card between columns */
    moveCard(cardId: string, fromCol: string, toCol: string, position: number) {
      update(s => {
        const newState = { ...s.state, columns: { ...s.state.columns } };

        // Remove from source
        const sourceCards = [...(newState.columns[fromCol] || [])];
        const cardIdx = sourceCards.findIndex(c => c.id === cardId);
        if (cardIdx === -1) return s;
        const [card] = sourceCards.splice(cardIdx, 1);
        newState.columns[fromCol] = sourceCards;

        // Insert at target
        const targetCards = [...(newState.columns[toCol] || [])];
        const insertAt = Math.min(position, targetCards.length);
        targetCards.splice(insertAt, 0, card);
        newState.columns[toCol] = targetCards;

        return { ...s, state: newState, dragCard: null, dragSourceCol: null };
      });
    },

    /** Add a card to backlog */
    addCard(card: KanbanCard) {
      update(s => {
        const newState = { ...s.state, columns: { ...s.state.columns } };
        newState.columns.backlog = [...(newState.columns.backlog || []), card];
        return { ...s, state: newState };
      });
    },

    /** Remove a card from the board */
    removeCard(cardId: string) {
      update(s => {
        const newState = { ...s.state, columns: { ...s.state.columns } };
        for (const col of Object.keys(newState.columns)) {
          newState.columns[col] = newState.columns[col].filter(c => c.id !== cardId);
        }
        return { ...s, state: newState };
      });
    },

    /** Reset to initial state */
    reset() {
      set(initialStore);
    },
  };
}

export const kanban = createKanbanStore();

/** Derived: total card count */
export const totalCards = derived(kanban, $k => {
  let count = 0;
  for (const cards of Object.values($k.state.columns)) {
    count += cards.length;
  }
  return count;
});

/** Derived: active plans (not done/cancelled) */
export const activePlans = derived(kanban, $k =>
  $k.state.plans.filter(p => p.status === 'draft' || p.status === 'approved' || p.status === 'running')
);

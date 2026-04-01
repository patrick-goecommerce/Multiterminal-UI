import { writable, derived } from 'svelte/store';
import { board } from '../../wailsjs/go/models';

// Column IDs for the v3 state machine board
export const COLUMN_IDS = ['backlog', 'planning', 'review', 'executing', 'qa', 'done'] as const;
export type ColumnID = typeof COLUMN_IDS[number];

// German labels
export const COLUMN_LABELS: Record<ColumnID, string> = {
  backlog: 'Backlog',
  planning: 'Planung',
  review: 'Review',
  executing: 'Ausführung',
  qa: 'Qualität',
  done: 'Erledigt',
};

export const COLUMN_COLORS: Record<ColumnID, string> = {
  backlog: '#9ca3af',
  planning: '#8b5cf6',
  review: '#3b82f6',
  executing: '#f97316',
  qa: '#06b6d4',
  done: '#22c55e',
};

// States that show as badges, not columns
export const BADGE_STATES: Record<string, { label: string; color: string }> = {
  triage: { label: 'Triage', color: '#f59e0b' },
  stuck: { label: 'Blockiert', color: '#ef4444' },
  human_review: { label: 'Review nötig', color: '#f97316' },
  merging: { label: 'Merge', color: '#06b6d4' },
};

// Map task state to display column
export function stateToColumn(state: board.TaskState): ColumnID {
  switch (state) {
    case 'backlog':
    case 'triage':
      return 'backlog';
    case 'planning':
      return 'planning';
    case 'review':
      return 'review';
    case 'executing':
    case 'stuck':
    case 'human_review':
      return 'executing';
    case 'qa':
    case 'merging':
      return 'qa';
    case 'done':
      return 'done';
    default:
      return 'backlog';
  }
}

// Check if a state should show a badge
export function getBadge(state: board.TaskState): { label: string; color: string } | null {
  return BADGE_STATES[state] || null;
}

interface KanbanStoreState {
  tasks: board.TaskCard[];
  loading: boolean;
  dir: string;
  dragCard: board.TaskCard | null;
  dragSourceCol: ColumnID | null;
}

function createKanbanStore() {
  const { subscribe, set, update } = writable<KanbanStoreState>({
    tasks: [],
    loading: false,
    dir: '',
    dragCard: null,
    dragSourceCol: null,
  });

  return {
    subscribe,
    setDir: (dir: string) => update(s => ({ ...s, dir })),
    setTasks: (tasks: board.TaskCard[]) => update(s => ({ ...s, tasks, loading: false })),
    setLoading: (loading: boolean) => update(s => ({ ...s, loading })),
    addTask: (task: board.TaskCard) => update(s => ({ ...s, tasks: [...s.tasks, task] })),
    removeTask: (id: string) => update(s => ({ ...s, tasks: s.tasks.filter(t => t.id !== id) })),
    updateTask: (updated: board.TaskCard) => update(s => ({
      ...s,
      tasks: s.tasks.map(t => t.id === updated.id ? updated : t),
    })),
    startDrag: (card: board.TaskCard, col: ColumnID) => update(s => ({ ...s, dragCard: card, dragSourceCol: col })),
    endDrag: () => update(s => ({ ...s, dragCard: null, dragSourceCol: null })),
    reset: () => set({ tasks: [], loading: false, dir: '', dragCard: null, dragSourceCol: null }),
  };
}

export const kanban = createKanbanStore();

// Derived: tasks grouped by display column
export const tasksByColumn = derived(kanban, ($k) => {
  const grouped: Record<ColumnID, board.TaskCard[]> = {
    backlog: [], planning: [], review: [], executing: [], qa: [], done: [],
  };
  for (const task of $k.tasks) {
    const col = stateToColumn(task.state as board.TaskState);
    grouped[col].push(task);
  }
  return grouped;
});

// Derived: total task count
export const totalTasks = derived(kanban, ($k) => $k.tasks.length);

// Backward-compat stubs for components that import these
export type KanbanCard = board.TaskCard;
export type Plan = board.Plan;
export type PlanStep = board.PlanStep;

export const activePlans = derived(kanban, () => [] as board.Plan[]);
export const parentIssueProgress = derived(kanban, () => ({} as Record<number, { done: number; total: number }>));

import type { Tab, Pane } from '../stores/tabs';

export type PaneWithContext = Pane & {
  tabId: string;
  tabName: string;
};

export type ActivityGroups = {
  starting: PaneWithContext[];       // starting (just launched, not yet scanned)
  needsAttention: PaneWithContext[]; // waitingPermission | waitingAnswer | error
  active: PaneWithContext[];         // active
  done: PaneWithContext[];           // done
  idle: PaneWithContext[];           // idle (including shell panes, stopped)
};

export function groupPanesByActivity(tabs: Tab[]): ActivityGroups {
  const groups: ActivityGroups = {
    starting: [],
    needsAttention: [],
    active: [],
    done: [],
    idle: [],
  };

  for (const tab of tabs) {
    for (const pane of tab.panes) {
      const ctx: PaneWithContext = { ...pane, tabId: tab.id, tabName: tab.name };
      if (pane.activity === 'starting' && pane.running) {
        groups.starting.push(ctx);
      } else if (
        pane.activity === 'waitingPermission' ||
        pane.activity === 'waitingAnswer' ||
        pane.activity === 'error'
      ) {
        groups.needsAttention.push(ctx);
      } else if (pane.activity === 'active') {
        groups.active.push(ctx);
      } else if (pane.activity === 'done') {
        groups.done.push(ctx);
      } else {
        groups.idle.push(ctx);
      }
    }
  }

  return groups;
}

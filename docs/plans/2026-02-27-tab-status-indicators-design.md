# Tab Status Indicators — Design

**Date:** 2026-02-27
**Branch:** alpha-main
**Status:** Approved

## Goal

Show the aggregate activity state of all panes in each tab directly in the tab bar.
Users can see at a glance which tabs need attention (input required, all done) without
switching to them. Once a tab is clicked, the indicator clears — it re-appears only
when new activity happens while the tab is not focused.

## Behaviour

### Indicator states

| State | Visual | Meaning |
|---|---|---|
| `needsInput` | Red pulsing dot | At least one pane is waiting for user confirmation |
| `active` | Accent-color slowly pulsing dot | Claude is running, no input needed yet |
| `done` | Green static dot | All Claude panes finished, none active/waiting |
| `null` | — | Nothing noteworthy (all idle or current tab) |

### Aggregation rule

Scan all panes in the tab; apply the highest-priority state:

```
needsInput  >  active  >  done  >  null
```

Shell panes stay `idle` permanently — they do not affect the aggregate.

### Clear-on-click

When a tab becomes the active tab (`setActiveTab()`), its `unreadActivity` is reset
to `null`. The indicator reappears only when a pane in that tab emits a new activity
event while the tab is not focused.

### Active tab

The active (currently visible) tab never shows an indicator. The user can see the
pane status directly.

## Data Model Change

`tabs.ts` — `Tab` interface:

```typescript
export interface Tab {
  // ... existing fields ...
  unreadActivity: 'needsInput' | 'active' | 'done' | null;  // NEW
}
```

Default value: `null`.

## Logic Changes

### `tabs.ts` — `updateActivity(sessionId, activity, cost)`

After updating the pane's activity, check whether the pane's tab is the active tab.
If not, recompute the tab's `unreadActivity`:

```
for each pane in tab:
  if needsInput → aggregate = needsInput, break
  if active     → aggregate = active (continue scanning for needsInput)
  if done       → aggregate = done   (continue scanning for active/needsInput)
→ tab.unreadActivity = aggregate (or null if all idle)
```

### `tabs.ts` — `setActiveTab(tabId)`

```typescript
tab.unreadActivity = null;
```

## Visual — `TabBar.svelte`

Small dot rendered to the right of the tab label text, inside the tab button.

```
┌─────────────────┐  ┌───────────────────●┐
│   Tab 1         │  │   Tab 2           🔴│  ← needsInput (pulse)
└─────────────────┘  └────────────────────┘

┌─────────────────┐  ┌───────────────────●┐
│   Tab 1         │  │   Tab 2           🟢│  ← done (static)
└─────────────────┘  └────────────────────┘

┌─────────────────┐  ┌───────────────────●┐
│   Tab 1         │  │   Tab 2           ◉ │  ← active (slow pulse)
└─────────────────┘  └────────────────────┘
```

### CSS

- `needsInput`: `#ef4444`, animation `tab-dot-pulse` (same rhythm as pane border)
- `active`: accent color (CSS var), animation `tab-dot-slow-pulse` (2s, subtle)
- `done`: `#22c55e`, no animation

Dot size: ~7px diameter, vertically centred in the tab button.

## Files Changed

| File | Change |
|---|---|
| `frontend/src/stores/tabs.ts` | Add `unreadActivity` to `Tab`; update `updateActivity()`; clear in `setActiveTab()` |
| `frontend/src/components/TabBar.svelte` | Render dot + CSS animations |

## Out of Scope

- No server-side changes (activity event payload unchanged)
- No indicator on the active/currently-visible tab
- Shell panes do not contribute to tab status

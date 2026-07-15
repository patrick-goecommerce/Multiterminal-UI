# Configurable Quick Actions & Finish Prompt Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let each user customize the worktree-finish prep prompt (what happens when a worktree is "finished") and define up to 5 additional quick-action buttons in the Claude-pane titlebar, each sending a user-defined, placeholder-templated prompt into the pane's prompt queue.

**Architecture:** Two new `config.Config` fields (`FinishPrepPrompt string`, `QuickActions []QuickAction`) drive both features. A shared placeholder-substitution scheme (`{{branch}}`, `{{targetBranch}}`, `{{worktreePath}}`) is implemented once in Go (for the finish prompt, rendered server-side where the pane's git context lives) and once in TypeScript (for quick actions, rendered client-side where the pane object already lives). Both delivery paths reuse the existing `App.AddToQueue(sessionId, prompt)` Wails binding — no new backend RPC, no new state machine, no step/workflow engine. Quick-action buttons render only for Claude-family panes (`claude`, `claude-auto`, `claude-yolo`).

**Tech Stack:** Go 1.21+ (`internal/config`, `internal/backend`), Svelte 4 + TypeScript (`frontend/src`), Vitest + `@testing-library/svelte` for tests.

## Global Constraints

- Go structs exposed to the frontend need both `yaml` and `json` tags (CLAUDE.md).
- `frontend/wailsjs/go/models.ts` must be manually kept in sync with any new/changed Go struct field returned to the frontend — property declaration AND constructor assignment (CLAUDE.md: silent `undefined` bug otherwise).
- `SettingsDialog.svelte`: the `$: if (visible) initDialog();` block must stay the ONLY reactive statement touching dialog-local variables; all sync-from-config logic lives inside `initDialog()` (CLAUDE.md recurring-bug rule).
- UI text in `SettingsDialog.svelte` is hardcoded German (the file does not use the `$t()` i18n system) — new UI text follows that existing convention, no i18n key additions needed.
- No workflow/step engine: both prompt fields are opaque text with placeholder substitution only. Multiterminal never parses or validates slash commands inside them.
- Quick actions render only when `pane.mode` is one of `claude`, `claude-auto`, `claude-yolo` — never for `shell`, `codex*`, `gemini*`.
- Max 5 custom quick actions, enforced in the Settings UI only (no backend limit).

---

## Task 1: Backend config model — `QuickAction` + `FinishPrepPrompt`

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_quickactions_test.go` (new)

**Interfaces:**
- Produces: `config.QuickAction{ Label string, Prompt string }`, `Config.FinishPrepPrompt string`, `Config.QuickActions []QuickAction`.
- Consumes: nothing (leaf task).

- [ ] **Step 1: Write the failing test**

Create `internal/config/config_quickactions_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultConfig_QuickActionsEmpty(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.FinishPrepPrompt != "" {
		t.Errorf("FinishPrepPrompt default = %q, want empty (falls back to built-in prompt)", cfg.FinishPrepPrompt)
	}
	if len(cfg.QuickActions) != 0 {
		t.Errorf("QuickActions default = %+v, want empty", cfg.QuickActions)
	}
}

func TestConfig_QuickActions_YAMLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-config.yaml")

	original := DefaultConfig()
	original.FinishPrepPrompt = "Push {{branch}}, open a PR against {{targetBranch}}, then /loop until merged."
	original.QuickActions = []QuickAction{
		{Label: "🔁", Prompt: "/loop 5m check status"},
		{Label: "🧪", Prompt: "Run the test suite and report failures"},
	}

	if err := writeDefaults(path, original); err != nil {
		t.Fatalf("writeDefaults failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var loaded Config
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if loaded.FinishPrepPrompt != original.FinishPrepPrompt {
		t.Errorf("loaded FinishPrepPrompt = %q, want %q", loaded.FinishPrepPrompt, original.FinishPrepPrompt)
	}
	if len(loaded.QuickActions) != 2 {
		t.Fatalf("loaded QuickActions len = %d, want 2", len(loaded.QuickActions))
	}
	if loaded.QuickActions[0].Label != "🔁" || loaded.QuickActions[0].Prompt != "/loop 5m check status" {
		t.Errorf("loaded QuickActions[0] = %+v, want {🔁, /loop 5m check status}", loaded.QuickActions[0])
	}
	if loaded.QuickActions[1].Label != "🧪" || loaded.QuickActions[1].Prompt != "Run the test suite and report failures" {
		t.Errorf("loaded QuickActions[1] = %+v, want {🧪, Run the test suite and report failures}", loaded.QuickActions[1])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/... -run TestDefaultConfig_QuickActionsEmpty -v` and `go test ./internal/config/... -run TestConfig_QuickActions_YAMLRoundTrip -v`
Expected: FAIL with `cfg.FinishPrepPrompt undefined (type Config has no field or method FinishPrepPrompt)` (compile error).

- [ ] **Step 3: Implement the minimal code**

In `internal/config/config.go`, add the new struct type right after `CommandEntry` (around line 85):

```go
// QuickAction represents a user-defined pane-titlebar button that sends a
// (placeholder-templated) prompt into the pane's prompt queue. Placeholders
// {{branch}}, {{targetBranch}}, {{worktreePath}} are substituted at send time.
type QuickAction struct {
	Label  string `yaml:"label" json:"label"`
	Prompt string `yaml:"prompt" json:"prompt"`
}
```

Add two fields to the `Config` struct (after the `Commands []CommandEntry` line):

```go
	Commands              []CommandEntry `yaml:"commands" json:"commands"`
	// FinishPrepPrompt overrides the built-in worktree-finish prep prompt
	// (see app_worktree_finish.go prepPromptTemplate) when non-empty. Supports
	// the same {{branch}}/{{targetBranch}}/{{worktreePath}} placeholders.
	FinishPrepPrompt      string         `yaml:"finish_prep_prompt" json:"finish_prep_prompt"`
	QuickActions          []QuickAction  `yaml:"quick_actions" json:"quick_actions"`
```

No changes needed in `DefaultConfig()` (zero-value `""` / `nil` slice are the correct defaults) or in `Load()` (no bounds to enforce server-side per the design).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/config/... -v`
Expected: PASS (all tests in the package, including the two new ones).

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_quickactions_test.go
git commit -m "feat(config): add configurable finish prompt and quick actions"
```

---

## Task 2: Backend placeholder rendering + wire into worktree-finish prep

**Files:**
- Create: `internal/backend/app_prompt_template.go`
- Modify: `internal/backend/app_worktree_finish.go:25-30` (the `prepPromptTemplate` constant) and `internal/backend/app_worktree_finish.go:162` (the `fmt.Sprintf` call inside `StartWorktreeFinish`)
- Test: `internal/backend/app_prompt_template_test.go` (new)

**Interfaces:**
- Consumes: `config.Config.FinishPrepPrompt` (Task 1), `a.cfg` field (`internal/backend/app.go:33`), `newTestApp()` helper (`internal/backend/app_queue_test.go:10`).
- Produces: `renderPlaceholders(tpl, branch, target, worktreePath string) string` (package-level function), `(a *AppService) renderFinishPrompt(branch, target, worktreePath string) string` (method).

- [ ] **Step 1: Write the failing test**

Create `internal/backend/app_prompt_template_test.go`:

```go
package backend

import "testing"

func TestRenderPlaceholders_AllThree(t *testing.T) {
	got := renderPlaceholders("push {{branch}} to {{targetBranch}} from {{worktreePath}}", "feat/x", "alpha-main", `C:\wt`)
	want := `push feat/x to alpha-main from C:\wt`
	if got != want {
		t.Errorf("renderPlaceholders() = %q, want %q", got, want)
	}
}

func TestRenderPlaceholders_NoPlaceholders(t *testing.T) {
	got := renderPlaceholders("just do the thing, no vars here", "feat/x", "alpha-main", `C:\wt`)
	if got != "just do the thing, no vars here" {
		t.Errorf("renderPlaceholders() = %q, want unchanged", got)
	}
}

func TestRenderPlaceholders_RepeatedPlaceholder(t *testing.T) {
	got := renderPlaceholders("{{branch}}...{{branch}}", "feat/x", "alpha-main", `C:\wt`)
	if got != "feat/x...feat/x" {
		t.Errorf("renderPlaceholders() = %q, want repeated substitution", got)
	}
}

func TestRenderFinishPrompt_DefaultWhenEmpty(t *testing.T) {
	a := newTestApp()
	got := a.renderFinishPrompt("feat/x", "alpha-main", `C:\wt`)
	want := "Rebase dann feat/x auf den lokalen alpha-main."
	if !contains(got, want) {
		t.Errorf("renderFinishPrompt() default = %q, missing %q", got, want)
	}
}

func TestRenderFinishPrompt_CustomTemplate(t *testing.T) {
	a := newTestApp()
	a.cfg.FinishPrepPrompt = "Push {{branch}}, open a PR against {{targetBranch}}, then clean up {{worktreePath}}."
	got := a.renderFinishPrompt("feat/y", "main", `C:\wt\feat-y`)
	want := `Push feat/y, open a PR against main, then clean up C:\wt\feat-y.`
	if got != want {
		t.Errorf("renderFinishPrompt() = %q, want %q", got, want)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i+len(substr) <= len(s); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/backend/... -run TestRenderPlaceholders -v` and `go test ./internal/backend/... -run TestRenderFinishPrompt -v`
Expected: FAIL with `undefined: renderPlaceholders` (compile error).

- [ ] **Step 3: Implement the minimal code**

Create `internal/backend/app_prompt_template.go`:

```go
// Shared placeholder rendering for user-configurable prompt templates (the
// worktree-finish prep prompt today; any future pane-context prompt reuses
// the same three placeholders).
package backend

import "strings"

// renderPlaceholders replaces {{branch}}, {{targetBranch}} and
// {{worktreePath}} in tpl with their actual values. Placeholders the
// template doesn't use are simply not substituted; the template may use
// each placeholder zero, one, or multiple times.
func renderPlaceholders(tpl, branch, target, worktreePath string) string {
	r := strings.NewReplacer(
		"{{branch}}", branch,
		"{{targetBranch}}", target,
		"{{worktreePath}}", worktreePath,
	)
	return r.Replace(tpl)
}

// renderFinishPrompt builds the prep prompt sent to a pane before a worktree
// finish: the user's own template (Config.FinishPrepPrompt) when set,
// otherwise the built-in default (prepPromptTemplate).
func (a *AppService) renderFinishPrompt(branch, target, worktreePath string) string {
	tpl := a.cfg.FinishPrepPrompt
	if tpl == "" {
		tpl = prepPromptTemplate
	}
	return renderPlaceholders(tpl, branch, target, worktreePath)
}
```

In `internal/backend/app_worktree_finish.go`, change the constant (lines 25-30) from positional `%s` to named placeholders — the rendered default output stays byte-for-byte identical, only the templating mechanism changes:

```go
const prepPromptTemplate = "Committe alle offenen Änderungen in nachvollziehbaren Commits. " +
	"Committe keine Secrets, .env-Dateien oder Build-Artefakte — ergänze für solche Dateien " +
	".gitignore-Einträge oder lass sie untracked und erwähne sie. " +
	"Rebase dann {{branch}} auf den lokalen {{targetBranch}}. Bei Rebase-Konflikten: nicht selbst lösen, " +
	"`git rebase --abort` ausführen und die Konfliktdateien nennen. " +
	"Merge nicht selbst, pushe nicht, erstelle keinen PR."
```

In the same file, inside `StartWorktreeFinish` (around line 162), replace:

```go
	prompt := fmt.Sprintf(prepPromptTemplate, branch, target)
```

with:

```go
	prompt := a.renderFinishPrompt(branch, target, worktreePath)
```

`fmt` is very likely now an unused import in `app_worktree_finish.go` if `fmt.Sprintf` was its only remaining use besides `fmt.Sprintf` in `setFinishBlocked`'s reason string — check with `go vet`/`go build`; the file also uses `fmt.Sprintf` at line 138 (`"Queue nicht leer (%d Prompts)..."`), so the import stays needed. No import changes required.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./... && go test ./internal/backend/... -v`
Expected: PASS for all tests in the package, including the 5 new ones. Also run `go test ./internal/backend/... -run TestStartFinish` to confirm the existing finish-flow tests (`app_worktree_finish_test.go`) still pass unchanged — they assert on `Phase`/`PrepItemID`, not on prompt text, so they are unaffected.

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_prompt_template.go internal/backend/app_prompt_template_test.go internal/backend/app_worktree_finish.go
git commit -m "feat(worktree): make the finish prep prompt user-configurable"
```

---

## Task 3: Frontend placeholder rendering for quick actions

**Files:**
- Create: `frontend/src/lib/quickActions.ts`
- Test: `frontend/src/lib/quickActions.test.ts` (new)

**Interfaces:**
- Consumes: nothing (pure function, no dependency on Pane type to keep it trivially testable — callers pass primitive strings).
- Produces: `renderQuickActionPrompt(template: string, branch: string, targetBranch: string, worktreePath: string): string`.

- [ ] **Step 1: Write the failing test**

Create `frontend/src/lib/quickActions.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { renderQuickActionPrompt } from './quickActions';

describe('renderQuickActionPrompt', () => {
  it('substitutes all three placeholders', () => {
    const result = renderQuickActionPrompt(
      'push {{branch}} to {{targetBranch}} from {{worktreePath}}',
      'feat/x', 'alpha-main', '/repo/.worktrees/feat-x',
    );
    expect(result).toBe('push feat/x to alpha-main from /repo/.worktrees/feat-x');
  });

  it('returns the template unchanged when it has no placeholders', () => {
    const result = renderQuickActionPrompt('/code-review', 'feat/x', 'alpha-main', '/repo/.worktrees/feat-x');
    expect(result).toBe('/code-review');
  });

  it('substitutes a repeated placeholder every time it appears', () => {
    const result = renderQuickActionPrompt('{{branch}}...{{branch}}', 'feat/x', '', '');
    expect(result).toBe('feat/x...feat/x');
  });

  it('substitutes empty string for placeholders with no value (non-worktree pane)', () => {
    const result = renderQuickActionPrompt('target={{targetBranch}} path={{worktreePath}}', 'main', '', '');
    expect(result).toBe('target= path=');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/lib/quickActions.test.ts`
Expected: FAIL — `Failed to resolve import "./quickActions"`.

- [ ] **Step 3: Implement the minimal code**

Create `frontend/src/lib/quickActions.ts`:

```ts
/** Substitute {{branch}}, {{targetBranch}} and {{worktreePath}} in a
 *  quick-action prompt template with the values of the pane the button was
 *  clicked on. Panes without a worktree pass '' for targetBranch/worktreePath
 *  — the placeholders simply resolve to an empty string. */
export function renderQuickActionPrompt(
  template: string,
  branch: string,
  targetBranch: string,
  worktreePath: string,
): string {
  return template
    .split('{{branch}}').join(branch)
    .split('{{targetBranch}}').join(targetBranch)
    .split('{{worktreePath}}').join(worktreePath);
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd frontend && npx vitest run src/lib/quickActions.test.ts`
Expected: PASS (4 tests).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/quickActions.ts frontend/src/lib/quickActions.test.ts
git commit -m "feat(quick-actions): add placeholder rendering for quick-action prompts"
```

---

## Task 4: Frontend config store + Wails models sync

**Files:**
- Modify: `frontend/src/stores/config.ts`
- Modify: `frontend/wailsjs/go/models.ts`

**Interfaces:**
- Consumes: `config.QuickAction` YAML/JSON shape from Task 1 (`{label, prompt}`).
- Produces: `QuickAction` TS interface (in `stores/config.ts`) and `QuickAction` TS class (in `models.ts`); `AppConfig.quick_actions: QuickAction[]`, `AppConfig.finish_prep_prompt: string`; `models.ts`'s `Config` class gains matching fields.

- [ ] **Step 1: Write the failing test**

Create `frontend/src/stores/config.quickactions.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { config } from './config';
import { get } from 'svelte/store';

describe('config store — quick actions defaults', () => {
  it('defaults quick_actions to an empty array', () => {
    expect(get(config).quick_actions).toEqual([]);
  });

  it('defaults finish_prep_prompt to an empty string', () => {
    expect(get(config).finish_prep_prompt).toBe('');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/stores/config.quickactions.test.ts`
Expected: FAIL — `expect(received).toEqual(expected)` with `received: undefined` (TypeScript would also flag `quick_actions` as not existing on `AppConfig` once typed, but at runtime the field is simply `undefined`).

- [ ] **Step 3: Implement the minimal code**

In `frontend/src/stores/config.ts`, add the interface (after `CommandEntry`, around line 11):

```ts
export interface QuickAction {
  label: string;
  prompt: string;
}
```

Add two fields to `AppConfig` (after `commands: CommandEntry[];`, around line 75):

```ts
  commands: CommandEntry[];
  finish_prep_prompt: string;
  quick_actions: QuickAction[];
```

Add matching defaults to the `config` writable's initial value (after `commands: [...]`, around line 123):

```ts
  commands: [
    { name: 'Commit & Push', text: "git add -A && git commit -m 'update' && git push" },
  ],
  finish_prep_prompt: '',
  quick_actions: [],
```

In `frontend/wailsjs/go/models.ts`, add a new class right after `CommandEntry` (around line 793):

```ts
	export class QuickAction {
	    label: string;
	    prompt: string;

	    static createFrom(source: any = {}) {
	        return new QuickAction(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.prompt = source["prompt"];
	    }
	}
```

In the same file's `Config` class, add the two fields to the property list (after `commands: CommandEntry[];`, around line 857):

```ts
	    commands: CommandEntry[];
	    finish_prep_prompt: string;
	    quick_actions: QuickAction[];
```

And add the two assignments to the constructor (after `this.commands = this.convertValues(source["commands"], CommandEntry);`, around line 888):

```ts
	        this.commands = this.convertValues(source["commands"], CommandEntry);
	        this.finish_prep_prompt = source["finish_prep_prompt"];
	        this.quick_actions = this.convertValues(source["quick_actions"], QuickAction);
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd frontend && npx vitest run src/stores/config.quickactions.test.ts`
Expected: PASS (2 tests).

Also run: `cd frontend && npx vitest run` to confirm nothing else broke (both files are additive changes to existing structures).
Expected: all existing tests still PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/stores/config.ts frontend/wailsjs/go/models.ts frontend/src/stores/config.quickactions.test.ts
git commit -m "feat(config): sync quick actions and finish prompt fields to frontend"
```

---

## Task 5: PaneTitlebar quick-action buttons

**Files:**
- Modify: `frontend/src/lib/claude.ts:25` (export the existing claude-family mode set)
- Modify: `frontend/src/components/PaneTitlebar.svelte`
- Test: `frontend/src/components/PaneTitlebar.quickactions.test.ts` (new)

**Interfaces:**
- Consumes: `CLAUDE_MODES` (exported from `claude.ts`), `renderQuickActionPrompt` (Task 3), `$config.quick_actions: QuickAction[]` (Task 4), `Pane` type (`stores/tabs.ts`).
- Produces: a `quickAction` custom event dispatched by `PaneTitlebar`, detail shape `{ sessionId: number, prompt: string }`.

- [ ] **Step 1: Write the failing test**

Create `frontend/src/components/PaneTitlebar.quickactions.test.ts`:

```ts
import { describe, it, expect, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import PaneTitlebar from './PaneTitlebar.svelte';
import { config } from '../stores/config';
import { get } from 'svelte/store';
import type { Pane } from '../stores/tabs';

afterEach(() => {
  cleanup();
  config.update((c) => ({ ...c, quick_actions: [] }));
});

function basePane(overrides: Partial<Pane> = {}): Pane {
  return {
    id: 'pane-1',
    sessionId: 42,
    name: 'Claude',
    mode: 'claude',
    model: '',
    focused: true,
    activity: 'idle',
    cost: '',
    running: true,
    maximized: false,
    issueNumber: null,
    issueTitle: '',
    issueBranch: '',
    worktreePath: '/repo/.worktrees/feat-x',
    branch: 'feat/x',
    targetBranch: 'alpha-main',
    zoomDelta: 0,
    background: false,
    display: 'terminal',
    conversationId: '',
    claudeSessionId: '',
    autoName: '',
    oscTitle: '',
    autoNameSource: '',
    userRenamed: false,
    finishPhase: '',
    ...overrides,
  };
}

describe('PaneTitlebar — quick actions', () => {
  it('renders one button per configured quick action on a claude pane', () => {
    config.update((c) => ({
      ...c,
      quick_actions: [
        { label: '🔁', prompt: 'loop it' },
        { label: '🧪', prompt: 'test it' },
      ],
    }));
    const { container } = render(PaneTitlebar, { props: { pane: basePane() } });
    const buttons = container.querySelectorAll('.quick-action-btn');
    expect(buttons.length).toBe(2);
    expect(buttons[0].textContent).toBe('🔁');
    expect(buttons[1].textContent).toBe('🧪');
  });

  it('does not render quick actions on a shell pane', () => {
    config.update((c) => ({ ...c, quick_actions: [{ label: '🔁', prompt: 'loop it' }] }));
    const { container } = render(PaneTitlebar, { props: { pane: basePane({ mode: 'shell', worktreePath: '' }) } });
    expect(container.querySelectorAll('.quick-action-btn').length).toBe(0);
  });

  it('renders quick actions on claude-auto and claude-yolo panes too', () => {
    config.update((c) => ({ ...c, quick_actions: [{ label: '🔁', prompt: 'loop it' }] }));
    const auto = render(PaneTitlebar, { props: { pane: basePane({ mode: 'claude-auto' }) } });
    expect(auto.container.querySelectorAll('.quick-action-btn').length).toBe(1);
    cleanup();
    const yolo = render(PaneTitlebar, { props: { pane: basePane({ mode: 'claude-yolo' }) } });
    expect(yolo.container.querySelectorAll('.quick-action-btn').length).toBe(1);
  });

  it('dispatches quickAction with the rendered prompt and session id on click', async () => {
    config.update((c) => ({
      ...c,
      quick_actions: [{ label: '🔁', prompt: 'rebase {{branch}} onto {{targetBranch}}' }],
    }));
    const { container, component } = render(PaneTitlebar, { props: { pane: basePane() } });
    const handler = (e: CustomEvent) => received.push(e.detail);
    const received: any[] = [];
    component.$on('quickAction', handler);

    await fireEvent.click(container.querySelector('.quick-action-btn')!);

    expect(received).toEqual([{ sessionId: 42, prompt: 'rebase feat/x onto alpha-main' }]);
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/components/PaneTitlebar.quickactions.test.ts`
Expected: FAIL — `container.querySelectorAll('.quick-action-btn').length` is `0` where `2`/`1` expected (no such buttons exist yet), and the dispatch test times out/fails with an empty `received` array.

- [ ] **Step 3: Implement the minimal code**

In `frontend/src/lib/claude.ts`, export the existing set (line 25) so `PaneTitlebar` can reuse it instead of re-declaring the claude-family mode list a third time:

```ts
/** Modes backed by the claude CLI, which understands --session-id / --resume. */
export const CLAUDE_MODES = new Set<PaneMode>(['claude', 'claude-yolo', 'claude-auto']);
```

In `frontend/src/components/PaneTitlebar.svelte`, add two imports (near the top, after the existing `git-polling` import at line 6):

```ts
  import { CLAUDE_MODES } from '../lib/claude';
  import { renderQuickActionPrompt } from '../lib/quickActions';
  import { config } from '../stores/config';
```

Add a click handler function (near `handleContextAction`/other handlers, or right before the markup — place it after `startRename`, around line 45):

```ts
  function handleQuickAction(prompt: string) {
    const rendered = renderQuickActionPrompt(prompt, pane.branch, pane.targetBranch, pane.worktreePath);
    dispatch('quickAction', { sessionId: pane.sessionId, prompt: rendered });
  }
```

In the markup, inside `.pane-title-right`, the existing structure (lines 174-185) is one `{#if pane.worktreePath} ... {:else if fallbackBranch} ... {/if}` block, followed by a separate `{#if pane.issueNumber}` block at line 186. Insert the quick-actions block as its own sibling **after line 185's closing `{/if}` and before line 186** — NOT nested inside the `pane.worktreePath` block, since quick actions must also appear on Claude panes running directly in the main repo (no worktree, `fallbackBranch` case):

```svelte
    {#if pane.worktreePath}
      <span class="wt-badge" ...>...</span>
      {#if pane.finishPhase === 'preparing' || ...}
        ...
      {/if}
    {:else if fallbackBranch}
      <span class="wt-badge wt-badge-main" ...>...</span>
    {/if}
    {#if CLAUDE_MODES.has(pane.mode)}
      {#each $config.quick_actions as qa (qa.label + qa.prompt)}
        ...
      {/each}
    {/if}
    {#if pane.issueNumber}
      ...
```

```svelte
    {#if CLAUDE_MODES.has(pane.mode)}
      {#each $config.quick_actions as qa (qa.label + qa.prompt)}
        <button
          class="pane-btn quick-action-btn"
          title={qa.prompt}
          on:click|stopPropagation={() => handleQuickAction(qa.prompt)}
        >{qa.label}</button>
      {/each}
    {/if}
```

Add a CSS rule near the other button styles (after `.finish-btn.spinning { ... }` around line 379):

```css
  .quick-action-btn { font-size: 12px; }
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd frontend && npx vitest run src/components/PaneTitlebar.quickactions.test.ts`
Expected: PASS (4 tests).

Run: `cd frontend && npx vitest run` to confirm no regressions in other suites (this file has no prior test file, so no existing PaneTitlebar tests can break; `claude.ts`'s existing tests must still pass since `export` was only added, nothing renamed).
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/claude.ts frontend/src/components/PaneTitlebar.svelte frontend/src/components/PaneTitlebar.quickactions.test.ts
git commit -m "feat(quick-actions): render configurable quick-action buttons in the pane titlebar"
```

---

## Task 6: Wire the quickAction event through to the prompt queue

**Files:**
- Modify: `frontend/src/components/TerminalPane.svelte:557` (event forwarding block)
- Modify: `frontend/src/components/PaneGrid.svelte`
- Modify: `frontend/src/App.svelte`

**Interfaces:**
- Consumes: `quickAction` event, detail `{ sessionId: number, prompt: string }` (Task 5); `App.AddToQueue(sessionId: number, prompt: string): Promise<backend.QueueItem>` (existing Wails binding, `frontend/wailsjs/go/backend/App.d.ts:14`).
- Produces: nothing further downstream (terminal task in the event chain).

- [ ] **Step 1: Write the failing test**

Create `frontend/src/App.quickaction.test.ts`. This test targets the handler function directly rather than mounting the full `App.svelte` (which requires extensive Wails/window/session mocking already set up elsewhere) — to keep the test focused, the handler is written as an exported pure-ish function that takes the AddToQueue call as a parameter. Since `App.svelte`'s handlers are private closures, the test instead verifies the **wiring** at the `PaneGrid` level (which is a plain, mountable component) and separately verifies `App.svelte`'s handler behavior via a small extracted helper. Extract the mapping first:

Create `frontend/src/lib/quickActionQueue.ts`:

```ts
import * as App from '../../wailsjs/go/backend/App';

/** Send a quick-action's already-rendered prompt into a session's prompt
 *  queue. Thin wrapper so App.svelte's event handler and its test share one
 *  implementation instead of App.svelte re-implementing the AddToQueue call. */
export async function sendQuickAction(sessionId: number, prompt: string): Promise<void> {
  await App.AddToQueue(sessionId, prompt);
}
```

Create `frontend/src/lib/quickActionQueue.test.ts`:

```ts
import { describe, it, expect, vi } from 'vitest';

const { addToQueue } = vi.hoisted(() => ({ addToQueue: vi.fn() }));
vi.mock('../../wailsjs/go/backend/App', () => ({ AddToQueue: addToQueue }));

import { sendQuickAction } from './quickActionQueue';

describe('sendQuickAction', () => {
  it('forwards sessionId and prompt to App.AddToQueue', async () => {
    await sendQuickAction(42, 'rebase feat/x onto alpha-main');
    expect(addToQueue).toHaveBeenCalledWith(42, 'rebase feat/x onto alpha-main');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/lib/quickActionQueue.test.ts`
Expected: FAIL — `Failed to resolve import "./quickActionQueue"`.

- [ ] **Step 3: Implement the minimal code**

Create the two files exactly as shown in Step 1 (`quickActionQueue.ts` is the implementation, not just test scaffolding).

Wire the event through the component tree. In `frontend/src/components/TerminalPane.svelte`, add a forwarding line next to the existing ones (around line 558):

```svelte
    on:commitPush
    on:finishWorktree
    on:quickAction
```

In `frontend/src/components/PaneGrid.svelte`, add a handler function (after `handleFinishWorktree`, around line 45):

```ts
  function handleQuickAction(e: CustomEvent) {
    dispatch('quickAction', e.detail);
  }
```

And bind it on the `<TerminalPane>` element (after `on:finishWorktree={handleFinishWorktree}`, around line 92):

```svelte
        on:finishWorktree={handleFinishWorktree}
        on:quickAction={handleQuickAction}
```

In `frontend/src/App.svelte`, add the import (near the other lib imports at the top of the `<script>` block, alongside wherever `encodeForPty` is imported):

```ts
  import { sendQuickAction } from './lib/quickActionQueue';
```

Add the handler function (after `handleFinishWorktree`, around line 1073):

```ts
  async function handleQuickAction(e: CustomEvent<{ sessionId: number; prompt: string }>) {
    try {
      await sendQuickAction(e.detail.sessionId, e.detail.prompt);
    } catch (err) {
      console.error('[handleQuickAction] AddToQueue failed:', err);
    }
  }
```

And bind it on `<PaneGrid>` (after `on:finishWorktree={handleFinishWorktree}`, around line 1172):

```svelte
              on:finishWorktree={handleFinishWorktree}
              on:quickAction={handleQuickAction}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd frontend && npx vitest run src/lib/quickActionQueue.test.ts`
Expected: PASS (1 test).

Run: `cd frontend && npx svelte-check --tsconfig ./tsconfig.json 2>&1 | grep -E "TerminalPane.svelte|PaneGrid.svelte|App.svelte"`
Expected: no new errors attributable to this change (pre-existing unrelated errors in other files, e.g. `KanbanPlanDialog.svelte`, may still appear — ignore those).

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/quickActionQueue.ts frontend/src/lib/quickActionQueue.test.ts frontend/src/components/TerminalPane.svelte frontend/src/components/PaneGrid.svelte frontend/src/App.svelte
git commit -m "feat(quick-actions): wire quickAction events through to the prompt queue"
```

---

## Task 7: Settings UI for the finish prompt and quick-action list

**Files:**
- Modify: `frontend/src/components/SettingsDialog.svelte`

**Interfaces:**
- Consumes: `QuickAction` interface (Task 4), `$config.finish_prep_prompt` / `$config.quick_actions` (Task 4), existing `initDialog()` / `save()` functions (`SettingsDialog.svelte:94` / `:237`).
- Produces: nothing further downstream (leaf UI task).

- [ ] **Step 1: Write the failing test**

Create `frontend/src/components/SettingsDialog.quickactions.test.ts`:

```ts
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, fireEvent, cleanup } from '@testing-library/svelte';
import SettingsDialog from './SettingsDialog.svelte';
import { config } from '../stores/config';

vi.mock('../../wailsjs/go/backend/App', () => ({
  SaveConfig: vi.fn().mockResolvedValue(undefined),
  GetLogPath: vi.fn().mockResolvedValue(''),
}));
vi.mock('../../wailsjs/runtime/runtime', () => ({ EventsOn: vi.fn(() => () => {}) }));

afterEach(() => {
  cleanup();
  config.update((c) => ({ ...c, finish_prep_prompt: '', quick_actions: [] }));
});

describe('SettingsDialog — quick actions section', () => {
  it('shows the finish-prompt textarea seeded from config', () => {
    config.update((c) => ({ ...c, finish_prep_prompt: 'my custom finish prompt' }));
    const { container } = render(SettingsDialog, { props: { visible: true } });
    const textarea = container.querySelector('.finish-prompt-input') as HTMLTextAreaElement;
    expect(textarea).not.toBeNull();
    expect(textarea.value).toBe('my custom finish prompt');
  });

  it('lists existing quick actions', () => {
    config.update((c) => ({
      ...c,
      quick_actions: [{ label: '🔁', prompt: 'loop it' }, { label: '🧪', prompt: 'test it' }],
    }));
    const { container } = render(SettingsDialog, { props: { visible: true } });
    const rows = container.querySelectorAll('.quick-action-row');
    expect(rows.length).toBe(2);
  });

  it('adds a new quick action row, capped at 5', async () => {
    config.update((c) => ({
      ...c,
      quick_actions: [
        { label: '1', prompt: 'p1' }, { label: '2', prompt: 'p2' }, { label: '3', prompt: 'p3' },
        { label: '4', prompt: 'p4' }, { label: '5', prompt: 'p5' },
      ],
    }));
    const { container, getByText } = render(SettingsDialog, { props: { visible: true } });
    const addBtn = getByText('+ Quick Action') as HTMLButtonElement;
    expect(addBtn.disabled).toBe(true);
    expect(container.querySelectorAll('.quick-action-row').length).toBe(5);
  });

  it('removes a quick action row on delete click', async () => {
    config.update((c) => ({ ...c, quick_actions: [{ label: '🔁', prompt: 'loop it' }] }));
    const { container } = render(SettingsDialog, { props: { visible: true } });
    expect(container.querySelectorAll('.quick-action-row').length).toBe(1);
    await fireEvent.click(container.querySelector('.quick-action-remove')!);
    expect(container.querySelectorAll('.quick-action-row').length).toBe(0);
  });

  it('does not reset an edited field when an unrelated field changes (recurring-bug guard)', async () => {
    config.update((c) => ({ ...c, finish_prep_prompt: 'original' }));
    const { container } = render(SettingsDialog, { props: { visible: true } });
    const textarea = container.querySelector('.finish-prompt-input') as HTMLTextAreaElement;
    await fireEvent.input(textarea, { target: { value: 'edited by user' } });
    // Toggling an unrelated checkbox must NOT re-run initDialog() and wipe the edit.
    const loggingCheckbox = container.querySelector('input[type="checkbox"]') as HTMLInputElement;
    if (loggingCheckbox) await fireEvent.click(loggingCheckbox);
    expect(textarea.value).toBe('edited by user');
  });
});
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd frontend && npx vitest run src/components/SettingsDialog.quickactions.test.ts`
Expected: FAIL — `container.querySelector('.finish-prompt-input')` is `null` (element doesn't exist yet), subsequent assertions fail.

- [ ] **Step 3: Implement the minimal code**

In `frontend/src/components/SettingsDialog.svelte`, add local state variables (after `let orchSyncSubtasks = ...;` around line 65):

```ts
  let finishPrepPrompt = $config.finish_prep_prompt || '';
  let quickActions: { label: string; prompt: string }[] = [];
```

In `initDialog()` (after `orchSyncSubtasks = $config.orchestrator?.sync_subtasks_to_github ?? false;` around line 129), add the sync lines — reassigning the whole array (not mutating in place) so Svelte's each-block updates correctly:

```ts
    finishPrepPrompt = $config.finish_prep_prompt || '';
    quickActions = ($config.quick_actions || []).map((qa) => ({ ...qa }));
```

Add two helper functions near the other handlers (after `initDialog()`, before `$: if (visible && sttProvider ...)` around line 133):

```ts
  function addQuickAction() {
    if (quickActions.length >= 5) return;
    quickActions = [...quickActions, { label: '⭐', prompt: '' }];
  }

  function removeQuickAction(index: number) {
    quickActions = quickActions.filter((_, i) => i !== index);
  }
```

In `save()` (inside the `updated` object, after `commands: ...` — note `commands` itself isn't in the current `save()` object per the earlier read of the file, so add these as new top-level keys, e.g. right after `use_worktrees: useWorktrees,` around line 244):

```ts
      use_worktrees: useWorktrees,
      finish_prep_prompt: finishPrepPrompt,
      quick_actions: quickActions,
```

Add the markup section. Place it in the dialog body wherever a new settings section reads naturally (e.g. after the existing "Orchestrator" section, matching the file's section-per-`<h3>`/`<div class="section">` convention — inspect the surrounding markup for the exact wrapper class used by neighboring sections and reuse it verbatim):

```svelte
      <div class="section">
        <h3>Quick Actions</h3>
        <p class="hint">
          Platzhalter: <code>{'{{branch}}'}</code>, <code>{'{{targetBranch}}'}</code>,
          <code>{'{{worktreePath}}'}</code> (leer, wenn kein Worktree aktiv ist).
        </p>

        <label class="field-label">Fertigstellen-Prompt (✓-Button)</label>
        <textarea
          class="finish-prompt-input"
          rows="4"
          placeholder="Leer lassen für das Standardverhalten (lokal mergen &amp; aufräumen)"
          bind:value={finishPrepPrompt}
        ></textarea>

        <label class="field-label">Eigene Quick-Actions ({quickActions.length}/5)</label>
        {#each quickActions as qa, i (i)}
          <div class="quick-action-row">
            <input class="quick-action-label" maxlength="2" bind:value={qa.label} placeholder="🔁" />
            <input class="quick-action-prompt" bind:value={qa.prompt} placeholder="Prompt-Text..." />
            <button class="quick-action-remove" on:click={() => removeQuickAction(i)}>✕</button>
          </div>
        {/each}
        <button class="add-quick-action" on:click={addQuickAction} disabled={quickActions.length >= 5}>
          + Quick Action
        </button>
      </div>
```

Add matching CSS (near the other section/input styles at the end of the `<style>` block):

```css
  .finish-prompt-input {
    width: 100%; box-sizing: border-box; font-family: inherit; font-size: 12px;
    background: var(--bg-tertiary); border: 1px solid var(--border); border-radius: 6px;
    color: var(--fg); padding: 8px; resize: vertical;
  }
  .quick-action-row { display: flex; gap: 6px; margin: 4px 0; align-items: center; }
  .quick-action-label { width: 40px; text-align: center; }
  .quick-action-prompt { flex: 1; }
  .quick-action-row input {
    background: var(--bg-tertiary); border: 1px solid var(--border); border-radius: 4px;
    color: var(--fg); padding: 5px 8px; font-size: 12px;
  }
  .quick-action-remove {
    background: none; border: none; color: var(--fg-muted); cursor: pointer; font-size: 13px;
  }
  .quick-action-remove:hover { color: var(--error); }
  .add-quick-action {
    margin-top: 6px; padding: 6px 12px; background: var(--bg-tertiary); border: 1px solid var(--border);
    border-radius: 6px; color: var(--fg); cursor: pointer; font-size: 12px;
  }
  .add-quick-action:disabled { opacity: 0.5; cursor: not-allowed; }
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd frontend && npx vitest run src/components/SettingsDialog.quickactions.test.ts`
Expected: PASS (5 tests).

Run the CLAUDE.md recurring-bug guard explicitly:
`grep -n '\$:' D:/repos/Multiterminal/frontend/src/components/SettingsDialog.svelte`
Expected output: only the two pre-existing lines — `$: if (visible) initDialog();` and `$: if (visible && sttProvider !== 'cloud-whisper') refreshSttStatus(sttProvider);` — no new `$:` block was introduced by this task.

Run: `cd frontend && npx vitest run` (full suite) to confirm no regressions.
Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/SettingsDialog.svelte frontend/src/components/SettingsDialog.quickactions.test.ts
git commit -m "feat(settings): add UI for the finish prompt and custom quick actions"
```

---

## Task 8: Build & manual verification

**Files:** none (verification only).

**Interfaces:**
- Consumes: everything from Tasks 1-7.
- Produces: a working alpha build proving the feature end-to-end.

- [ ] **Step 1: Run the full backend test suite**

Run: `export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH" && go build ./... && go vet ./... && go test ./...`
Expected: build succeeds, `go vet` reports nothing new, all tests PASS.

- [ ] **Step 2: Run the full frontend test suite**

Run: `cd frontend && npx vitest run`
Expected: all tests PASS (existing suite plus the ~16 new tests added across Tasks 1-7... Go side, plus the frontend-side new test files).

- [ ] **Step 3: Build the frontend bundle**

Run: `cd frontend && npm run build`
Expected: `✓ built in ...` with no new errors (pre-existing a11y warnings from unrelated components are fine).

- [ ] **Step 4: Build the Go binary**

Run: `export PATH="/c/Program Files/Go/bin:$HOME/go/bin:$PATH" && go build -o build/bin/multiterminal.exe -tags desktop .`
Expected: exits 0, `build/bin/multiterminal.exe` timestamp updated.

- [ ] **Step 5: Manual smoke test**

Launch `build/bin/multiterminal.exe`, open Settings, and verify:
1. The "Quick Actions" section appears with an empty finish-prompt textarea and no rows.
2. Type a custom finish prompt (e.g. `Push {{branch}}, open a PR against {{targetBranch}}.`), add one quick action (label `🔁`, prompt `/loop 5m check {{branch}} status`), save, reopen Settings — both persist.
3. Open a worktree-backed Claude pane: the 🔁 button appears in the titlebar next to ✓; clicking it queues the rendered prompt (branch/target substituted) — confirm via the queue panel or by watching it appear in the pane.
4. Click ✓ (Fertigstellen): confirm the custom finish prompt (not the old hardcoded one) is what gets sent.
5. Open a plain shell pane: confirm no quick-action buttons render.

- [ ] **Step 6: Commit (if the smoke test surfaced fixes)**

Only if Step 5 required code changes:

```bash
git add -A
git commit -m "fix(quick-actions): address issues found in manual verification"
```

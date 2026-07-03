# Worktree EnterWorktree — Rev. 2: MTUI-seitiger Merge-Trigger reaktiviert — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reaktiviere den ✓-Finish-Button, `WorktreeFinishDialog.svelte` und die Finish-State-Machine-Verdrahtung im Frontend, umverdrahtet auf per Hook erkannte (`EnterWorktree`) Worktrees, plus eine verschärfte Zustimmungspflicht im Projekt-Memory-Text.

**Architecture:** Das komplette Finish-Backend (`StartWorktreeFinish`, `CheckWorktreeFinish`, `FinishWorktree`, `CancelWorktreeFinish`, `GetWorktreeFinishStatus`, `mergeWorktreeBranch`, `cleanupWorktree`, alle `worktree:finish-*`-Events) und **alle zugehörigen Wails-Bindings sind bereits unverändert vorhanden** — es sind keine Backend-Code- oder Binding-Änderungen für den Finish-Flow nötig. Diese Änderung ist im Kern ein Revert der Frontend-Entfernung aus den Commits `cdf218e` (App.svelte + Dialog gelöscht) und `cb5a905` (Button aus PaneTitlebar entfernt), wobei die neuen `worktree:detected`/`worktree:cleared`-Listener aus dem Detection-Design **erhalten bleiben** und parallel zu den reaktivierten `worktree:finish-*`-Listenern laufen. Die zwei Event-Familien kollidieren nicht: `detected`/`cleared` pflegen „ist ein Worktree aktiv" (`worktreePath`/`branch`/`targetBranch`), die `finish-*`-Events pflegen „läuft gerade ein Finish-Vorgang" (`finishPhase`). Der Button löst ausschließlich einen lokalen ff-only-Merge im Haupt-Worktree aus (kein Push, kein PR durch MTUI). Zusätzlich wird der Memory-Text so verschärft, dass Claudes eigenständiges Push/PR/Merge/Fast-Forward immer vorherige Nutzer-Zustimmung erfordert.

**Tech Stack:** Go 1.21+ (Backend), Svelte 4 + TypeScript + Vite (Frontend), vitest (Frontend-Unit-Tests), Wails v3 alpha.

## Global Constraints

- **UI-Text ist Deutsch, Code/Kommentare Englisch** (Backend-Fehler-/Prompt-Strings bleiben Deutsch, wie im vorhandenen Finish-Backend).
- **Max. 300 Zeilen pro Go-Datei.**
- **Keine neuen Wails-Bindings nötig** — alle aufgerufenen `App.*`-Funktionen (`StartWorktreeFinish`, `CancelWorktreeFinish`, `CheckWorktreeFinish`, `FinishWorktree`, `GetWorktreeChangedFiles`, `CommitWorktreeFiles`, `RebaseWorktreeOntoTarget`, `AbortWorktreeRebase`, `EnsureProjectWorktreeSetup`) existieren bereits in `frontend/wailsjs/go/backend/App.d.ts` und `App.js`. Nichts an den Bindings anfassen.
- **Der ✓-Button löst NUR einen lokalen ff-only-Merge aus** — kein Push, kein PR durch MTUI (Spec §11.2).
- **`finishPhase` ist ein UNABHÄNGIGES Pane-Feld** neben `worktreePath`/`branch`/`targetBranch` — nicht deren Ersatz (Spec §11.1).
- **Verbatim-Restore, kein Redesign:** Die State-Machine und der Dialog werden unverändert aus der Git-Historie wiederhergestellt. Vereinfachungen der State-Machine sind explizit verworfen (Spec §11.1 „Verworfene Alternativen") — sie würden bereits gefixte Red-Team-Probleme (Rebase-Konflikt, Claude beschäftigt, Cleanup-Retry) erneut aufmachen.
- **Die `worktree:detected`/`worktree:cleared`-Listener bleiben erhalten** — die `finish-*`-Listener kommen ZUSÄTZLICH hinzu (Spec §11.1).
- **Svelte `$:`-Reset-Falle:** Niemals Variablenzuweisungen in einen `$:`-Block legen, der Dialog-Variablen liest UND schreibt (siehe CLAUDE.md). Der wiederhergestellte Dialog nutzt bewusst `const selected = () => …` statt eines reaktiven `$:`-Blocks — so beibehalten.
- **Bestehende `CLAUDE.local.md`-Dateien in anderen Projekten werden NICHT rückwirkend aktualisiert** (`EnsureProjectWorktreeSetup` überschreibt nie bestehende Dateien) — bekannte, dokumentierte Abweichung, kein Nachzieh-Automatismus (Spec §11.3).

---

### Task 1: Backend — Memory-Text auf Zustimmungspflicht verschärfen

Ändert die eingebettete Projekt-Memory-Anweisung: Push, PR-Erstellung und jeder Merge/Fast-Forward durch Claude selbst erfordern **immer** vorherige Nutzer-Zustimmung (statt der bisherigen „oder frage nach"-Kann-Option). Die `discard_changes`/Force-Klausel bleibt unverändert bestehen.

**Files:**
- Modify: `internal/backend/app_worktree_setup.go:16-29` (Konstante `projectWorktreeMemoryContent`)
- Test: `internal/backend/app_worktree_setup_test.go:23` (Assertion-Liste erweitern)

**Interfaces:**
- Consumes: nichts Neues.
- Produces: keine Signatur-Änderung — nur der Inhalt der Konstante `projectWorktreeMemoryContent` ändert sich. `EnsureProjectWorktreeSetup(dir string) error` bleibt unverändert.

- [ ] **Step 1: Test um Zustimmungs-Assertion erweitern (soll fehlschlagen)**

In `internal/backend/app_worktree_setup_test.go`, in `TestEnsureProjectWorktreeSetup_CreatesFilesOnce`, die `want`-Liste erweitern (Zeile 23):

```go
	for _, want := range []string{"EnterWorktree", "discard_changes", "NIEMALS", "Zustimmung"} {
		if !strings.Contains(string(mem), want) {
			t.Errorf("memory file missing %q", want)
		}
	}
```

- [ ] **Step 2: Test ausführen, Fehlschlag bestätigen**

Run: `go test ./internal/backend/ -run TestEnsureProjectWorktreeSetup_CreatesFilesOnce -v`
Expected: FAIL mit `memory file missing "Zustimmung"` (der alte Text enthält das Wort noch nicht).

- [ ] **Step 3: Memory-Text verschärfen**

In `internal/backend/app_worktree_setup.go` die Konstante `projectWorktreeMemoryContent` (Zeilen 16-29) ersetzen durch:

```go
const projectWorktreeMemoryContent = `# MTUI-Projekt: Worktree-Isolation

Nutze für Aufgaben mit eigenem, klar abgegrenztem Arbeitsbereich das
` + "`EnterWorktree`" + `-Tool, um deine Änderungen zu isolieren. Der Haupt-Branch
dieses Projekts darf dabei nie direkt gewechselt werden.

Wenn eine Aufgabe erledigt ist: committe deine Arbeit in nachvollziehbaren
Commits. Push, das Erstellen eines Pull Requests (` + "`gh pr create`" + `) und jeder
Merge oder Fast-Forward durch dich selbst erfordern IMMER vorherige Zustimmung
des Nutzers — frage vorher nach, statt eigenständig zu pushen, zu mergen oder
einen PR zu öffnen.

Nutze ` + "`ExitWorktree`" + ` mit ` + "`discard_changes: true`" + ` oder erzwungenem Entfernen
NIEMALS eigenständig — nur nach ausdrücklicher Rückfrage beim Nutzer und
dessen Bestätigung.
`
```

- [ ] **Step 4: Tests ausführen, grün bestätigen**

Run: `go test ./internal/backend/ -run TestEnsureProjectWorktreeSetup -v`
Expected: PASS (beide Setup-Tests grün — `CreatesFilesOnce` und `DoesNotOverwriteExisting`).

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_worktree_setup.go internal/backend/app_worktree_setup_test.go
git commit -m "feat(worktree): require user consent for push/PR/merge in memory text

Spec 2026-07-03 rev 2 section 11.3: tighten projectWorktreeMemoryContent
from an optional 'or ask' to a mandatory consent requirement for any
push/PR/merge/fast-forward performed by Claude itself.

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XJsKHQ9MnWXGC2aEScJwpN"
```

---

### Task 2: Store — `finishPhase`-Feld + `setFinishPhase` in tabs.ts wiederherstellen

Bringt das unabhängige `finishPhase`-Feld zurück auf `Pane` und die `setFinishPhase`-Methode auf den Store. `finishPhase` beschreibt den laufenden Finish-Vorgang; es ist getrennt von `worktreePath`/`branch`/`targetBranch` (die weiterhin von der Hook-Erkennung gesetzt werden).

**Files:**
- Modify: `frontend/src/stores/tabs.ts:37` (Pane-Interface), `:239-242` (addPane-Initialisierung), `:337` (nach `setWorktree`)
- Test: `frontend/src/stores/tabs.test.ts`

**Interfaces:**
- Consumes: nichts Neues.
- Produces:
  - Neues Pane-Feld: `finishPhase: '' | 'preparing' | 'ready' | 'blocked' | 'merging' | 'cleanup'`
  - Neue Store-Methode: `setFinishPhase(sessionId: number, phase: string): void` — setzt `finishPhase` auf allen Panes mit passender `sessionId`.

- [ ] **Step 1: Failing test für `setFinishPhase` + Default schreiben**

In `frontend/src/stores/tabs.test.ts` einen neuen `describe`-Block hinzufügen (z. B. nach dem `updateActivity`-Block). Muster wie die bestehenden Tests (`tabStore.addTab()` → `tabStore.addPane()` → Pane suchen):

```typescript
  describe('finishPhase', () => {
    it('initializes finishPhase to empty string on a new pane', () => {
      const tabId = tabStore.addTab('T', '/d');
      tabStore.addPane(tabId, 8100, 'Claude', 'claude', '');
      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      const pane = tab!.panes.find((p) => p.sessionId === 8100);
      expect(pane!.finishPhase).toBe('');
    });

    it('setFinishPhase sets the phase by session id', () => {
      const tabId = tabStore.addTab('T', '/d');
      tabStore.addPane(tabId, 8101, 'Claude', 'claude', '');
      tabStore.setFinishPhase(8101, 'preparing');
      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      const pane = tab!.panes.find((p) => p.sessionId === 8101);
      expect(pane!.finishPhase).toBe('preparing');
    });

    it('setFinishPhase leaves worktree fields untouched', () => {
      const tabId = tabStore.addTab('T', '/d');
      tabStore.addPane(tabId, 8102, 'Claude', 'claude', '');
      tabStore.setWorktree(8102, '/wt', 'worktree-x', 'alpha-main');
      tabStore.setFinishPhase(8102, 'merging');
      const tab = tabStore.getState().tabs.find((t) => t.id === tabId);
      const pane = tab!.panes.find((p) => p.sessionId === 8102);
      expect(pane!.worktreePath).toBe('/wt');
      expect(pane!.branch).toBe('worktree-x');
      expect(pane!.finishPhase).toBe('merging');
    });
  });
```

- [ ] **Step 2: Tests ausführen, Fehlschlag bestätigen**

Run: `cd frontend && npx vitest run src/stores/tabs.test.ts`
Expected: FAIL — `finishPhase` existiert nicht auf dem Pane bzw. `setFinishPhase is not a function`.

- [ ] **Step 3: `finishPhase` zum Pane-Interface hinzufügen**

In `frontend/src/stores/tabs.ts` im `Pane`-Interface (nach `userRenamed: boolean;`, Zeile 36) einfügen:

```typescript
  /** True once the user manually renamed the pane — suppresses all auto names. */
  userRenamed: boolean;
  /** Active worktree-finish flow phase. Independent of worktreePath/branch/
   *  targetBranch: those track "a worktree is active", this tracks "a finish
   *  run is in progress". Empty when no finish flow is active. */
  finishPhase: '' | 'preparing' | 'ready' | 'blocked' | 'merging' | 'cleanup';
```

- [ ] **Step 4: `finishPhase` in `addPane` initialisieren**

In `frontend/src/stores/tabs.ts` im `addPane`-Objektliteral (nach `userRenamed: false,`, Zeile 242) einfügen:

```typescript
          autoNameSource: '',
          userRenamed: false,
          finishPhase: '',
        });
```

- [ ] **Step 5: `setFinishPhase`-Methode hinzufügen**

In `frontend/src/stores/tabs.ts` direkt nach der `setWorktree`-Methode (nach Zeile 337, vor `clearWorktree`) einfügen:

```typescript
    setFinishPhase(sessionId: number, phase: string) {
      update((state) => {
        for (const tab of state.tabs) {
          const pane = tab.panes.find((p) => p.sessionId === sessionId);
          if (pane) {
            pane.finishPhase = phase as Pane['finishPhase'];
          }
        }
        return state;
      });
    },
```

- [ ] **Step 6: Tests ausführen, grün bestätigen**

Run: `cd frontend && npx vitest run src/stores/tabs.test.ts`
Expected: PASS (alle drei neuen Tests + die bestehenden tabs-Tests grün).

- [ ] **Step 7: Commit**

```bash
git add frontend/src/stores/tabs.ts frontend/src/stores/tabs.test.ts
git commit -m "feat(worktree): restore finishPhase field + setFinishPhase on tabStore

Spec 2026-07-03 rev 2 section 11.1: finishPhase is an independent pane
field alongside worktreePath/branch/targetBranch, tracking an in-progress
finish run separately from worktree presence.

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XJsKHQ9MnWXGC2aEScJwpN"
```

---

### Task 3: `WorktreeFinishDialog.svelte` aus der Git-Historie wiederherstellen

Stellt die Finish-Dialog-Komponente unverändert aus dem Commit unmittelbar vor ihrer Löschung wieder her (`cdf218e~1`). Die Komponente rendert vier Zustände: `ready` (Commits + Diff-Stat, „Mergen & Aufräumen"), `blocked` (mit Untervarianten Rebase-Konflikt / Cleanup-Fehler / regulär), `staging` (Shell-Datei-Auswahl) und den `cleanupOnly`-Sonderfall.

**Files:**
- Create: `frontend/src/components/WorktreeFinishDialog.svelte` (verbatim aus `cdf218e~1`)

**Interfaces:**
- Consumes: nichts (reine Präsentationskomponente mit Props).
- Produces: Svelte-Komponente `WorktreeFinishDialog` mit Props `visible`, `state`, `sessionId`, `worktreePath`, `targetBranch`, `commits`, `stat`, `untracked`, `cleanupOnly`, `reason`, `files`, `commitMessage`, `rebaseConflict`, `cleanupFailed` und Events `confirm`, `retry`, `retryCleanup`, `cancel`, `stageCommit`, `rebaseOnly`, `abortRebase`, `resolveInTerminal`, `close`. Diese exakten Namen werden in Task 5 verdrahtet.

- [ ] **Step 1: Datei verbatim aus der Historie wiederherstellen**

Run (im Repo-Root):
```bash
git show cdf218e~1:frontend/src/components/WorktreeFinishDialog.svelte > frontend/src/components/WorktreeFinishDialog.svelte
```

Dies schreibt die Komponente byte-genau in ihrer letzten Form vor der Löschung. **Inhalt nicht manuell verändern** — dieser Restore ist bewusst verbatim (Global Constraints). Zur Kontrolle: die Datei beginnt mit `<!-- frontend/src/components/WorktreeFinishDialog.svelte -->` und enthält den Kommentar „avoids the known SettingsDialog `$:`-reset trap" bei `const selected = () => …`.

- [ ] **Step 2: Verifizieren, dass die Datei existiert und syntaktisch baut**

Run: `cd frontend && npm run build`
Expected: Build erfolgreich (kein Svelte-/TS-Fehler). Die Komponente wird noch nicht importiert — der Build prüft nur, dass die neue Datei für sich valide ist. Falls der Build wegen einer noch nicht existierenden Referenz fehlschlägt, ist das ein Fehler in dieser Task und nicht erwartet (die Komponente ist selbst-enthalten).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/WorktreeFinishDialog.svelte
git commit -m "feat(worktree): restore WorktreeFinishDialog component

Spec 2026-07-03 rev 2 section 11.1: verbatim restore from cdf218e~1 (the
commit before it was removed). Renders ready/blocked/staging/cleanup-only
states for the MTUI-side worktree finish flow.

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XJsKHQ9MnWXGC2aEScJwpN"
```

---

### Task 4: ✓/◌-Finish-Button in PaneTitlebar.svelte wiederherstellen

Bringt den Finish-Button neben das `wt-badge` zurück (Revert von `cb5a905`): ein grünes ✓ zum Starten des Finish-Vorgangs, das während `preparing`/`merging`/`cleanup` zu einem rotierenden ◌ (Abbrechen) wird. Sichtbar unter derselben Bedingung wie das Badge (`pane.worktreePath`).

**Files:**
- Modify: `frontend/src/components/PaneTitlebar.svelte:204-206` (Button-Block), `:423` (CSS nach `.wt-badge`)

**Interfaces:**
- Consumes: `pane.finishPhase` (Task 2), `pane.worktreePath`, `pane.sessionId`, `pane.id`.
- Produces: Component-Events `finishWorktree` (`{ paneId, sessionId }`) und `cancelFinish` (`{ sessionId }`) — in Task 5 in App.svelte verdrahtet.

- [ ] **Step 1: Button-Block hinzufügen**

In `frontend/src/components/PaneTitlebar.svelte` den bestehenden `wt-badge`-Block (Zeilen 204-206):

```svelte
    {#if pane.worktreePath}
      <span class="wt-badge" title={`Worktree: ${pane.worktreePath}\nZiel: ${pane.targetBranch || '?'}`}>⎇ {pane.branch}</span>
    {/if}
```

ersetzen durch:

```svelte
    {#if pane.worktreePath}
      <span class="wt-badge" title={`Worktree: ${pane.worktreePath}\nZiel: ${pane.targetBranch || '?'}`}>⎇ {pane.branch}</span>
      {#if pane.finishPhase === 'preparing' || pane.finishPhase === 'merging' || pane.finishPhase === 'cleanup'}
        <button class="pane-btn finish-btn spinning" title="Fertigstellen läuft – klicken zum Abbrechen"
          on:click|stopPropagation={() => dispatch('cancelFinish', { sessionId: pane.sessionId })}>◌</button>
      {:else}
        <button class="pane-btn finish-btn" title="Worktree fertigstellen: mergen & aufräumen"
          on:click|stopPropagation={() => dispatch('finishWorktree', { paneId: pane.id, sessionId: pane.sessionId })}>✓</button>
      {/if}
    {/if}
```

- [ ] **Step 2: CSS für `.finish-btn` hinzufügen**

In `frontend/src/components/PaneTitlebar.svelte` direkt nach der `.wt-badge`-Regel (Zeile 423) einfügen:

```css
  .wt-badge { font-size: 10px; color: var(--accent); background: var(--bg-tertiary); border: 1px solid var(--border); border-radius: 4px; padding: 1px 6px; max-width: 140px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .finish-btn { color: #4ade80; font-weight: 700; }
  .finish-btn.spinning { animation: wt-spin 1s linear infinite; }
  @keyframes wt-spin { to { transform: rotate(360deg); } }
```

- [ ] **Step 3: Build verifizieren**

Run: `cd frontend && npm run build`
Expected: Build erfolgreich. `pane.finishPhase` ist dank Task 2 typisiert; kein TS-Fehler.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/PaneTitlebar.svelte
git commit -m "feat(titlebar): restore worktree finish button (revert cb5a905)

Spec 2026-07-03 rev 2 section 11.1: bring back the green checkmark finish
button + spinning-cancel state next to the worktree badge, gated on
pane.worktreePath. Dispatches finishWorktree/cancelFinish.

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XJsKHQ9MnWXGC2aEScJwpN"
```

---

### Task 5: App.svelte — Finish-Flow verdrahten (neben der Detection)

Verdrahtet die Finish-State-Machine im Frontend wieder ein: Dialog-Import, `finishDialog`-State, die drei `worktree:finish-*`-Listener (ZUSÄTZLICH zu den bestehenden `detected`/`cleared`), die Helfer-Funktionen (`startFinish`, Shell-Staging, Cancel/Retry, Relaunch), die Pane-Event-Handler `finishWorktree`/`cancelFinish` und die `<WorktreeFinishDialog>`-Markup. Zusätzlich wird die Pane-Schließen-Warnung `finishPhase`-bewusst gemacht.

**Files:**
- Modify: `frontend/src/App.svelte` — Import (nach Zeile 21), State (nach Zeile ~90), Listener (nach Zeile 291), Close-Handler (Zeilen 623-628), Helfer (nach `handleCommitPush`, ~Zeile 897), Pane-Event-Bindings (nach Zeile 970), Dialog-Markup (nach Zeile 1015).

**Interfaces:**
- Consumes: aus Task 2 `tabStore.setFinishPhase(sessionId, phase)`; aus Task 3 die `WorktreeFinishDialog`-Props/Events; aus Task 4 die Component-Events `finishWorktree`/`cancelFinish`. Bereits vorhanden im Modul: `ownsSession`, `findPaneBySession`? (NEU anzulegen), `genSessionId`, `buildClaudeArgv`, `resolvedClaudePath`/`resolvedCodexPath`/`resolvedGeminiPath`, `branch`, `App.*`-Bindings.
- Produces: nichts für spätere Tasks (letzte Task).

- [ ] **Step 1: Dialog importieren**

In `frontend/src/App.svelte` nach dem `AskUserDialog`-Import (Zeile 21) einfügen:

```typescript
  import AskUserDialog from './components/AskUserDialog.svelte';
  import WorktreeFinishDialog from './components/WorktreeFinishDialog.svelte';
```

- [ ] **Step 2: `finishDialog`-State-Objekt anlegen**

In `frontend/src/App.svelte` unmittelbar vor `let editIssueData:` (aktuell Zeile ~91, direkt nach `let previewFilePath = '';`) einfügen:

```typescript
  let previewFilePath = '';
  let finishDialog: {
    visible: boolean;
    sessionId: number;
    state: 'ready' | 'blocked' | 'staging';
    worktreePath: string;
    targetBranch: string;
    commits: string[];
    stat: string;
    untracked: string[];
    cleanupOnly: boolean;
    reason: string;
    files: { path: string; status: string; selected: boolean }[];
    commitMessage: string;
    rebaseConflict: boolean;
    cleanupFailed: boolean;
  } = {
    visible: false,
    sessionId: 0,
    state: 'ready',
    worktreePath: '',
    targetBranch: '',
    commits: [],
    stat: '',
    untracked: [],
    cleanupOnly: false,
    reason: '',
    files: [],
    commitMessage: '',
    rebaseConflict: false,
    cleanupFailed: false,
  };
```

- [ ] **Step 3: Die drei `worktree:finish-*`-Listener hinzufügen (neben detected/cleared)**

In `frontend/src/App.svelte` direkt NACH dem bestehenden `worktree:cleared`-Listener (nach Zeile 291, vor dem `pane:autoname`-Kommentar) einfügen. **Die `detected`/`cleared`-Listener NICHT entfernen:**

```typescript
    EventsOn('worktree:cleared', (event: any) => {
      const p = event.data ?? event;
      if (!ownsSession(p.id)) return;
      tabStore.clearWorktree(p.id);
    });
    // Worktree finish flow: confirm overlay + phase tracking + pane relaunch.
    // Runs ALONGSIDE the detection listeners above (spec 2026-07-03 rev 2 §11.1):
    // detected/cleared track "worktree active"; these track "finish in progress".
    // Payloads are filtered to sessions owned by a pane of THIS window.
    EventsOn('worktree:finish-ready', (event: any) => {
      const p = event.data ?? event;
      if (!ownsSession(p.sessionId)) return;
      tabStore.setFinishPhase(p.sessionId, 'ready');
      finishDialog = {
        visible: true,
        sessionId: p.sessionId,
        state: 'ready',
        worktreePath: '',
        targetBranch: p.targetBranch || '',
        commits: p.commits || [],
        stat: p.stat || '',
        untracked: p.untracked || [],
        cleanupOnly: !!p.cleanupOnly,
        reason: '',
        files: [],
        commitMessage: '',
        rebaseConflict: false,
        cleanupFailed: false,
      };
    });
    EventsOn('worktree:finish-blocked', (event: any) => {
      const p = event.data ?? event;
      if (!ownsSession(p.sessionId)) return;
      tabStore.setFinishPhase(p.sessionId, p.phase || '');
      // A real block ('blocked') and a post-merge cleanup failure (phase
      // 'cleanup' + cleanupFailed) both surface the overlay. 'preparing' is an
      // informative phase update (prep still running), and '' means cancel/abort
      // (e.g. CancelWorktreeFinish) — neither should reopen the confirm dialog.
      if (p.phase === 'blocked' || p.cleanupFailed) {
        finishDialog = {
          ...finishDialog,
          visible: true,
          sessionId: p.sessionId,
          state: 'blocked',
          reason: p.reason || '',
          rebaseConflict: false,
          cleanupFailed: !!p.cleanupFailed,
        };
      }
    });
    EventsOn('worktree:finish-done', async (event: any) => {
      const p = event.data ?? event;
      if (!ownsSession(p.sessionId)) return;
      finishDialog = { ...finishDialog, visible: false };
      await relaunchPaneAfterFinish(p.sessionId, p.mainRoot, p.mode);
    });
```

- [ ] **Step 4: Helfer-Funktionen hinzufügen**

In `frontend/src/App.svelte` direkt VOR `async function handleIssueAction` (aktuell Zeile ~899) einfügen. Diese Funktionen entsprechen dem Stand vor `cdf218e`:

```typescript
  function findPaneBySession(sessionId: number) {
    for (const tab of get(allTabs)) {
      const pane = tab.panes.find((p) => p.sessionId === sessionId);
      if (pane) return pane;
    }
    return null;
  }

  function findPaneLocation(sessionId: number) {
    for (const tab of get(allTabs)) {
      const pane = tab.panes.find((p) => p.sessionId === sessionId);
      if (pane) return { tab, pane };
    }
    return null;
  }

  function startFinish(sessionId: number) {
    const pane = findPaneBySession(sessionId);
    if (!pane?.worktreePath) return;
    // Detected worktrees carry a target branch (from hook detection); the prompt
    // is a v1 fallback for legacy worktrees without a stored target.
    const t = pane.targetBranch || window.prompt('Ziel-Branch für den Merge:', branch || 'alpha-main') || '';
    if (!t) return;
    const mode = pane.mode === 'shell' ? 'shell' : 'claude';
    tabStore.setFinishPhase(sessionId, 'preparing');
    App.StartWorktreeFinish(sessionId, pane.worktreePath, pane.branch, t, mode);
    if (mode === 'shell') {
      // Shell panes have no Claude to prepare the merge — MTUI stages, commits
      // and rebases itself via the staging dialog (backend set phase 'preparing').
      openShellStaging(sessionId, pane.worktreePath, t);
    }
  }

  // Files matching build artefacts / secrets are deselected by default so a shell
  // finish never blindly commits them.
  const stagingDeselect = /\.env|node_modules|dist\/|build\//;

  async function openShellStaging(sessionId: number, wtPath: string, target: string) {
    let files: { path: string; status: string; selected: boolean }[] = [];
    try {
      const changed = await App.GetWorktreeChangedFiles(wtPath);
      files = (changed || []).map((f: any) => ({
        path: f.path,
        status: f.status,
        selected: !stagingDeselect.test(f.path),
      }));
    } catch (err) {
      console.error('GetWorktreeChangedFiles failed', err);
    }
    finishDialog = {
      ...finishDialog,
      visible: true,
      sessionId,
      state: 'staging',
      worktreePath: wtPath,
      targetBranch: target,
      files,
      commitMessage: '',
      rebaseConflict: false,
      reason: '',
      cleanupFailed: false,
    };
  }

  // Runs the commit (optional) + rebase, then hands back to the shared verify gate.
  // A rebase conflict flips the dialog into a rebase-specific blocked state.
  async function runShellStage(sessionId: number, wtPath: string, target: string,
                               paths: string[], message: string) {
    try {
      if (paths.length > 0) {
        await App.CommitWorktreeFiles(wtPath, paths, message);
      }
      await App.RebaseWorktreeOntoTarget(wtPath, target);
    } catch (err) {
      console.error('shell stage/rebase failed', err);
      finishDialog = {
        ...finishDialog,
        visible: true,
        state: 'blocked',
        rebaseConflict: true,
        reason: `Rebase auf ${target} fehlgeschlagen — Konflikte im Terminal auflösen oder Rebase abbrechen.`,
      };
      return;
    }
    // Success: the shared verify gate emits finish-ready/blocked and reopens the
    // dialog in the matching state.
    App.CheckWorktreeFinish(sessionId);
  }

  function handleFinishWorktree(e: CustomEvent<{ paneId: string; sessionId: number }>) {
    startFinish(e.detail.sessionId);
  }

  function handleRetryFinish(sessionId: number) {
    startFinish(sessionId);
  }

  function handleCancelFinish(e: CustomEvent<{ sessionId: number }>) {
    App.CancelWorktreeFinish(e.detail.sessionId);
    tabStore.setFinishPhase(e.detail.sessionId, '');
  }

  async function relaunchPaneAfterFinish(sessionId: number, mainRoot: string, mode: string) {
    const loc = findPaneLocation(sessionId);
    if (!loc) return;
    const { tab, pane } = loc;
    tabStore.closePane(tab.id, pane.id);
    const sid = mode !== 'shell' ? genSessionId() : '';
    const argv = mode !== 'shell'
      ? buildClaudeArgv(pane.mode, pane.model, resolvedClaudePath, resolvedCodexPath, resolvedGeminiPath, { sessionId: sid })
      : [];
    try {
      const newId = await App.CreateSession(argv, mainRoot, 24, 80, pane.mode);
      if (newId > 0) {
        tabStore.addPane(tab.id, newId, pane.name, pane.mode, pane.model,
          null, '', '', '', '', '', false, 'terminal', '', sid);
      }
    } catch (err) { console.error('[relaunchPaneAfterFinish] failed:', err); }
  }
```

- [ ] **Step 5: Pane-Event-Bindings hinzufügen**

In `frontend/src/App.svelte` bei der Pane-Komponente, nach `on:commitPush={handleCommitPush}` (Zeile 970), einfügen:

```svelte
              on:commitPush={handleCommitPush}
              on:finishWorktree={handleFinishWorktree}
              on:cancelFinish={handleCancelFinish}
```

- [ ] **Step 6: `<WorktreeFinishDialog>`-Markup hinzufügen**

In `frontend/src/App.svelte` direkt nach dem schließenden `/>` von `<AskUserDialog … />` (nach Zeile 1015, vor dem finalen `</div>`) einfügen:

```svelte
    on:dismiss={handleAskUserDismiss}
  />
  <WorktreeFinishDialog
    {...finishDialog}
    on:confirm={() => { tabStore.setFinishPhase(finishDialog.sessionId, 'merging'); finishDialog.visible = false; App.FinishWorktree(finishDialog.sessionId); }}
    on:retry={() => { finishDialog.visible = false; handleRetryFinish(finishDialog.sessionId); }}
    on:retryCleanup={() => { tabStore.setFinishPhase(finishDialog.sessionId, 'merging'); finishDialog.visible = false; App.FinishWorktree(finishDialog.sessionId); }}
    on:cancel={() => { finishDialog.visible = false; App.CancelWorktreeFinish(finishDialog.sessionId); tabStore.setFinishPhase(finishDialog.sessionId, ''); }}
    on:stageCommit={(e) => runShellStage(finishDialog.sessionId, finishDialog.worktreePath, finishDialog.targetBranch, e.detail.files, e.detail.message)}
    on:rebaseOnly={() => runShellStage(finishDialog.sessionId, finishDialog.worktreePath, finishDialog.targetBranch, [], '')}
    on:abortRebase={() => { finishDialog.visible = false; App.AbortWorktreeRebase(finishDialog.worktreePath); App.CancelWorktreeFinish(finishDialog.sessionId); tabStore.setFinishPhase(finishDialog.sessionId, ''); }}
    on:resolveInTerminal={() => (finishDialog.visible = false)}
    on:close={() => (finishDialog.visible = false)}
  />
```

- [ ] **Step 7: Pane-Schließen-Warnung `finishPhase`-bewusst machen**

In `frontend/src/App.svelte` den Close-Handler-Block (Zeilen 623-628) ersetzen durch:

```svelte
    // A pane with an active worktree that hasn't been finished (✓) is not removed
    // automatically — the worktree stays on disk and remains reachable via the ⎇
    // dropdown. Warn so the user does not silently orphan it (spec 5.6).
    if (pane.worktreePath && pane.finishPhase === '') {
      if (!confirm('Dieses Pane hat einen aktiven Worktree. Trotzdem schließen?\n\n(Der Worktree bleibt liegen und ist weiterhin über das ⎇-Dropdown erreichbar. Zum Mergen/Aufräumen den ✓-Button nutzen.)')) return;
    }
```

- [ ] **Step 8: Build verifizieren**

Run: `cd frontend && npm run build`
Expected: Build erfolgreich, kein TS-/Svelte-Fehler. Insbesondere müssen `WorktreeFinishDialog`, `finishDialog`, `setFinishPhase`, `handleFinishWorktree`, `handleCancelFinish`, `relaunchPaneAfterFinish` und alle referenzierten `App.*`-Bindings auflösen.

- [ ] **Step 9: Volle Frontend-Testsuite ausführen**

Run: `cd frontend && npx vitest run`
Expected: PASS — alle bestehenden Tests grün (App.svelte hat keine eigenen Unit-Tests; die Suite prüft, dass tabs/claude/session/… unverändert funktionieren).

- [ ] **Step 10: Backend-Tests als Regressionsschutz ausführen**

Run: `go test ./internal/backend/...`
Expected: PASS — der Finish-Flow-Backend-Code ist unverändert; dies bestätigt, dass Task 1 nichts gebrochen hat und der Finish-Flow weiterhin durchläuft.

- [ ] **Step 11: Commit**

```bash
git add frontend/src/App.svelte
git commit -m "feat(app): rewire worktree finish flow alongside EnterWorktree detection

Spec 2026-07-03 rev 2 section 11.1: restore the finish dialog, worktree:finish-*
listeners, helpers and pane handlers removed in cdf218e — now running ALONGSIDE
(not replacing) the worktree:detected/cleared listeners. The ✓-button triggers a
local ff-only merge via the existing (unchanged) backend finish state machine.

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XJsKHQ9MnWXGC2aEScJwpN"
```

---

## Self-Review

**1. Spec coverage (§11):**
- §11.1 „✓-Button + Dialog + State-Machine reaktivieren, auf erkannte Worktrees umverdrahtet" → Tasks 3 (Dialog), 4 (Button), 5 (Verdrahtung). Backend „keine Code-Änderung nötig" → bestätigt: alle Finish-Funktionen/Bindings/Events vorhanden, Tasks fassen sie nicht an. ✓
- §11.1 „`finishPhase` als unabhängiges Feld neben worktreePath/branch/targetBranch" → Task 2. ✓
- §11.1 „`worktree:finish-*`-Listener ZUSÄTZLICH zu detected/cleared" → Task 5 Step 3 (explizit „NICHT entfernen"). ✓
- §11.1 „Dialog aus Git-Historie wiederherstellen" → Task 3 (`git show cdf218e~1:…`). ✓
- §11.2 „Button löst nur lokalen ff-only-Merge aus, kein Push/PR" → in Global Constraints + Architecture verankert; Backend `mergeWorktreeBranch` nutzt `merge --ff-only`, kein Push. Kein Push/PR-Aufruf in den Handlern. ✓
- §11.2 „bereits extern gemerged → 0 neue Commits → nur Cleanup" → durch unveränderten `GetWorktreeFinishStatus` (`count==0` → `cleanup_only`) abgedeckt, kein neuer Code nötig. ✓
- §11.3 „Memory-Text: Push/PR/Merge/FF durch Claude immer mit Zustimmung" → Task 1. ✓
- §11.3 „bestehende CLAUDE.local.md nicht rückwirkend aktualisiert" → Global Constraint dokumentiert; `EnsureProjectWorktreeSetup` überschreibt nie (Test `DoesNotOverwriteExisting` bleibt grün). ✓
- §11.4 „Wiederherstellung, kein Neubau" → alle Tasks sind Reverts/Restores. ✓

**2. Placeholder-Scan:** Keine TBD/TODO/„handle edge cases". Jeder Code-Step enthält vollständigen Code oder einen exakten `git show`-Restore-Befehl. ✓

**3. Typ-Konsistenz:** `setFinishPhase(sessionId, phase)` (Task 2) wird in Task 5 mit genau dieser Signatur aufgerufen. `finishPhase`-Werte (`''|preparing|ready|blocked|merging|cleanup`) stimmen zwischen Interface (Task 2), Button-Bedingung (Task 4: `preparing|merging|cleanup`) und Handlern (Task 5) überein. Die `WorktreeFinishDialog`-Props/Events (Task 3) stimmen mit der `{...finishDialog}`-Spread- und Event-Verdrahtung (Task 5 Step 6) überein. Alle `App.*`-Bindings verifiziert vorhanden. ✓

**Bekannte, bewusst akzeptierte Punkte (nicht blockierend):**
- `relaunchPaneAfterFinish` schließt und relauncht das Pane im Haupt-Verzeichnis nach erfolgreichem Merge/Cleanup — konsistent mit dem alten Design; die von MTUI beendete Claude-Session löst kein konkurrierendes `worktree:cleared` aus (kein `ExitWorktree`-Hook), also keine Event-Kollision.
- Offene Follow-ups aus dem 13-Task-Review (`clearWorktree` blankt `pane.branch`; Orphan-Pfad-Case-Sensitivity) sind hier NICHT adressiert — sie interagieren zwar mit `finishPhase`, gehören aber zu einem separaten Aufräum-Ticket und würden diesen Restore aufblähen.

# Leitstand Teil 2: Review-Briefing — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every finished pane gets a briefing built on the host (prompt, checks, git numbers for turn and branch, PR/CI, closing words, warnings); line 2 of a waiting entry shows the verdict, a card opens next to the box, and a CI run or PR that turns red later brings the pane back as an error.

**Architecture:** `internal/hub` gains a `turnBook` (what happened in the current turn, fed by hook events), a `briefingDesk` (builds the briefing at `Stop`/exit/hang with git via `internal/gitx`, stores it, emits `session.briefing`) and a `ciWatch` (polls `gh` for branches with sessions, re-raises attention on a red transition). The briefing travels like attention: event over the sink and the wire, a `GET` for late readers, a backend event `terminal:briefing`, a frontend store. The frontend only formats; urgency still comes from the host's rank.

**Tech Stack:** Go 1.25, Wails v3 beta.25, Svelte 5 (legacy syntax), Vitest, git, gh.

**Spec:** `docs/superpowers/specs/2026-10-19-leitstand-review-briefing-design.md` (read it; this plan argues from it). Part 1: `docs/superpowers/specs/2026-10-07-leitstand-aufmerksamkeit-design.md`.

**Worktree:** C:\Users\Patrick.Hennig\repos\Multiterminal-UI\.claude\worktrees\leitstand-review-briefing (branch `worktree-leitstand-review-briefing`, from `alpha-main` 3e60317, no upstream set)

## Global Constraints

- Max 300 lines per Go production file. `internal/hub/server.go` (452 lines) must not grow; `internal/hub/embedded.go` (296) must stay ≤ 300.
- No production file in `internal/backend` imports `internal/terminal` (`grep -rl internal/terminal internal/backend --include='*.go' | grep -v _test.go` must be empty).
- Go structs the frontend sees carry both `json` and `yaml` tags; mirrored by hand in `frontend/wailsjs/go/models.ts` (class body and constructor), config fields also in `frontend/src/stores/config.ts` (interface and default object). `go test ./internal/tsmodels/` checks both and the binding IDs.
- Bindings are written by hand in `frontend/wailsjs/go/backend/App.js` and `App.d.ts`. IDs (FNV-1a 32 of `github.com/patrick-goecommerce/Multiterminal-UI/internal/backend.AppService.<Name>`): `GetSessionBriefing` = **3334881243**, `SetSessionTarget` = **2176172594**.
- The frontend holds sessions as `hub.Ref` strings; new bindings take a ref, translate with `a.local(ref)`; a foreign ref resolves to 0 and must not reach the host.
- Every non-PTY child process calls `procs.HideConsole(cmd)` before start (`gitx.Cmd` already does; `gh` must too). `gh` runs with `GH_PROMPT_DISABLED=1` and a 15 s limit and only reads.
- No new Claude Code hook events; `hookEvents` stays as it is.
- Git: never fetch, never touch the real index or the work tree; all git through `gitx`.
- UI text German, code and comments English. SettingsDialog: never assign variables inside a `$:` block; initialise in `initDialog()`.
- Never write `focusOrders` or other state from inside a `$:` statement.
- Values pinned by the spec: prompt ≤ 200 runes (`clipDetail`), closing ≤ 600 runes, `output_tail` ≤ 300 runes, build limit 5 s, snapshot limit 5 s, at most 4 builds at once, `big_diff_files` 15, `big_diff_lines` 500, CI cadence 1 min / 5 min after 30 min / fast cadence ends after 3 h / 15 min for an open PR with finished checks / 15 min pause on a `gh` error, card hover delay 300 ms, card width ≈ 340 px.
- Commit messages English, ending with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`; write multi-line messages to a file and use `git commit -F`.
- Known red test, not caused by this work: `internal/launch TestWorktreeEnvVars_NonRepoGetsNone` fails inside any linked worktree.

## Rulings made while planning (deviations from the spec text)

1. **`command` field.** The hook passes the raw Bash command in a new `command` field (≤ 1000 runes, one line) in addition to the usual `detail` (`toolDetail`, "Bash: …", ≤ 200). The classifier needs the unclipped command; a pipe after rune 200 would otherwise be missed. — Cost if wrong: one extra JSON field per Bash event.
2. **`GET …/briefing` answers 200 with `{"briefing": null}`** when the session has none, instead of 404. `Remote.call` maps every 404 to `ErrNoSession`, so 404 can only mean "unknown session or older daemon"; the backend treats any error as "no briefing". — Cost: none visible.
3. **`PRFacts.Review` (raw `reviewDecision`) replaces `ChangesRequested`.** The card needs "Review offen"/"freigegeben", the flag is derivable. Spec D is updated in Task 15.
4. **Config fields and host policy come in Task 7 (stage a), the SettingsDialog section in Task 14 (stage c).** The thresholds are needed as soon as `big_diff` exists; only the UI is stage c.
5. **After 3 h of pending checks the fast cadence ends and an open PR falls back to the 15-min cadence** (conflicts and requested changes still matter); the spec's "nicht mehr" refers to the fast cadence.
6. **`headRepositoryOwner` is an object** `{"id","name","login"}` (measured 2026-10-19); the owner match compares `login`, case-insensitively.
7. **Toast wording stays as Part 1 builds it** (`"Fehler · <tab>"`, body `"<pane>: CI rot: backend"`); no toast code changes. The detail strings carry the spec's content.

## Review Focus

1. **`Stop` arrives while the turn-start snapshot is still running** → the briefing has `turn_known: false` and no turn numbers; it never diffs against the previous turn's tree. Test in Task 6.
2. **The session is closed or exits while a build is in flight** → no `session.briefing` event after the session is gone, no stored briefing, no panic. Test in Task 6.
3. **Two sessions on one branch, CI turns red, one of them is `active`** → the idle one is raised exactly once, the active one only gets the updated briefing; a second poll with the same red head raises nobody. Test in Task 13.
4. **Repo path containing a space** (Windows user dirs) → snapshot, numstat and branch numbers still work. Test in Task 3 and Task 4 (temp dir name contains a space).
5. **Daemon re-attach: a window opens after briefings already exist** → it shows them for existing panes via `GetSessionBriefing`, not only after the next `Stop`. Test in Task 10.

---

## Stage a — turn facts, git, record and wire, verdict and card

### Task 1: `mtui-hook` passes turn facts

**Files:**
- Modify: `cmd/mtui-hook/main.go` (`claudeEvent`, `hookLine`, `run`)
- Create: `cmd/mtui-hook/bash_output.go`
- Modify: `internal/hooks/watcher.go:24-46` (`Event`, keep tags in sync)
- Test: `cmd/mtui-hook/line_test.go` (helper `runHook(t, event, payload)`; payload must use `"session_id":"s1"`)

**Interfaces:**
- Produces on `hooks.Event` (and `hookLine`, same tags):
  `PromptID string \`json:"prompt_id,omitempty"\``, `Command string \`json:"command,omitempty"\``, `Interrupt bool \`json:"is_interrupt,omitempty"\``, `OutputTail string \`json:"output_tail,omitempty"\``.
- `claudeEvent` gains `PromptID string \`json:"prompt_id"\``, `IsInterrupt bool \`json:"is_interrupt"\``, `Error string \`json:"error"\``.

- [ ] **Step 1: Write the failing tests** (payloads are the measured shapes from the spec, "Gemessen")

```go
func TestBashFailureCarriesTurnFacts(t *testing.T) {
	ev := runHook(t, "PostToolUseFailure", `{"session_id":"s1","prompt_id":"p-1","tool_name":"Bash",
		"tool_input":{"command":"echo vorher; exit 3","description":"x"},"error":"Exit code 3\nvorher","is_interrupt":false}`)
	// PromptID "p-1", Command "echo vorher; exit 3", Detail "Bash: echo vorher; exit 3",
	// OutputTail "Exit code 3\nvorher", Interrupt false
}
func TestBashSuccessTailIsStdoutAndStderr(t *testing.T) {
	// PostToolUse, tool_response {"stdout":"ok","stderr":"warn","interrupted":false}
	// → OutputTail "ok\nwarn"; Command "echo ok"
}
func TestOutputTailKeepsLast300Runes(t *testing.T) { /* stdout of 1000 'x' + "END" → len([]rune(tail)) == 300, ends with "END" */ }
func TestInterruptIsPassedOn(t *testing.T)        { /* is_interrupt true → Interrupt true */ }
func TestPromptAndStopCarryPromptID(t *testing.T)  { /* UserPromptSubmit and Stop with prompt_id "p-2" → PromptID "p-2" */ }
func TestNonBashPostToolUseHasNoCommand(t *testing.T) { /* tool_name "Edit" → Command "", OutputTail "" */ }
```

- [ ] **Step 2:** Run `go test ./cmd/mtui-hook/ -run 'Bash|OutputTail|Interrupt|PromptID|NonBash'` — FAIL (unknown fields).
- [ ] **Step 3:** Implement. `bash_output.go`: `func bashOutputTail(eventType string, response json.RawMessage, errText string) string` — failure: `errText`; success: `stdout` + `"\n"` + `stderr` (skip empty parts); keep the last 300 runes via the existing `tail`. `func bashCommand(input json.RawMessage) string` — `command` collapsed to one line, ≤ 1000 runes. In `run`, set `Detail` also for `PostToolUse`/`PostToolUseFailure` when `ToolName == "Bash"`; set `PromptID` for every event.
- [ ] **Step 4:** Run `go test ./cmd/mtui-hook/ ./internal/hooks/` — PASS.
- [ ] **Step 5:** Commit `feat(hook): pass prompt id, command and output tail of Bash calls`.

### Task 2: Recognise checks

**Files:**
- Create: `internal/hub/turn_checks.go`
- Test: `internal/hub/turn_checks_test.go`

**Interfaces:**
- Produces: `const (CheckOK = "ok"; CheckFail = "fail"; CheckUnsure = "unsure")`;
  `type CheckResult struct { Label string \`json:"label" yaml:"label"\`; Result string \`json:"result" yaml:"result"\` }`;
  `func classifyCheck(command string, failed bool, outputTail string) (CheckResult, bool)`;
  `func publishCommand(command string) bool` (true for a segment starting `git push`, `gh pr create`, `gh pr merge`).

Rules (spec A3): split at `&&`, `;`, `||`; strip leading `cd …`, `NAME=value`, `timeout N`; prefix table verbatim from spec A3; several checks in one line → one result, labels joined with `" + "`; a `|` after a check → result from exit only if no pipe, else `unsure` unless the tail carries a failure mark (`fail`). Failure marks, line-anchored: `^--- FAIL:`, `^FAIL$`, `^FAIL\t`, `error TS\d+:`, `^\s*Tests\s+\d+ failed`, `^\s*Test Files\s+\d+ failed`.

- [ ] **Step 1: Write the failing table test** `TestClassifyCheck` with these rows (command, failed, tail → label, result, ok):

| command | failed | tail | want |
|---|---|---|---|
| `cd internal && go test ./...` | false | `ok  x 0.1s` | `go test`, ok |
| `FOO=1 npm run build` | true | `` | `build`, fail |
| `go vet ./... && go test ./...` | true | `` | `go vet + go test`, fail |
| `go test ./... \| tail -20` | false | `--- FAIL: TestFails (0.00s)\n    fail_test.go:5: boom\nFAIL\nFAIL\tfailprobe\t1.473s\nFAIL` | `go test`, fail |
| `go test ./... \| tail -20` | false | `ok  \tfailprobe\t0.2s` | `go test`, unsure |
| `npx tsc --noEmit \| head` | false | `bad.ts(1,7): error TS2322: Type 'string' is not assignable to type 'number'.` | `tsc`, fail |
| `npx vitest run \| tail` | false | ` Test Files  1 failed (1)\n      Tests  1 failed \| 1 passed (2)` | `test`, fail |
| `npm run build \| tail` | false | `> build\n> node -e "process.exit(2)"` | `build`, unsure |
| `timeout 60 pytest -q` | false | `` | `pytest`, ok |
| `grep -rn foo .` | true | `` | not a check |
| `git status` | false | `` | not a check |

  Plus `TestPublishCommand`: `cd x && git push -u origin b` → true; `gh pr create --fill` → true; `gh pr view 1` → false; `git pull` → false.
- [ ] **Step 2:** `go test ./internal/hub/ -run 'ClassifyCheck|PublishCommand'` — FAIL.
- [ ] **Step 3:** Implement in `turn_checks.go`.
- [ ] **Step 4:** Same command — PASS.
- [ ] **Step 5:** Commit `feat(hub): recognise check commands and their result`.

### Task 3: Git snapshot primitives

**Files:**
- Create: `internal/gitx/snapshot.go`
- Test: `internal/gitx/snapshot_test.go`

**Interfaces:**
- Produces: `func CmdContext(ctx context.Context, dir string, args ...string) *exec.Cmd` (same env/flags/`HideConsole` as `Cmd`);
  `func SnapshotTree(ctx context.Context, dir string) (string, error)` (spec B1: copy `git rev-parse --git-path index` to a temp file, `GIT_INDEX_FILE=<copy> git add -A`, `git write-tree`, remove the copy; if the index does not exist yet, start from an empty temp index);
  `type FileStat struct { Path string; Added, Deleted int; Binary bool }`;
  `func NumStat(ctx context.Context, dir, from, to string) ([]FileStat, error)` (`git diff --numstat -z <from> <to>`; `-`/`-` → `Binary`).

- [ ] **Step 1: Write failing tests** in a temp repo whose path contains a space (`filepath.Join(t.TempDir(), "repo mit leer")`), real `git` (`t.Skip` if not on PATH):
  - `TestSnapshotLeavesIndexUntouched`: commit `a.txt`; modify `a.txt` by writing the file (no git); add untracked `b.txt`; read `.git/index` bytes and mtime; `SnapshotTree`; bytes equal, mtime equal, `git status --porcelain` output unchanged.
  - `TestNumStatBetweenSnapshots`: tree1 = snapshot; append 2 lines to `a.txt`, delete 1, create `c.txt` with 3 lines; tree2 = snapshot; `NumStat(tree1, tree2)` = `{a.txt 2 1}`, `{c.txt 3 0}` (order by path).
  - `TestSnapshotHonoursGitignore`: `.gitignore` with `node_modules/`; files there do not appear in a numstat against HEAD's tree.
  - `TestSnapshotCancelled`: an already cancelled ctx returns an error.
- [ ] **Step 2:** `go test ./internal/gitx/` — FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** `go test ./internal/gitx/` — PASS.
- [ ] **Step 5:** Commit `feat(gitx): snapshot the work tree into a tree without touching the index`.

### Task 4: Git facts for a briefing

**Files:**
- Create: `internal/hub/briefing_git.go`, `internal/hub/briefing_types.go`
- Test: `internal/hub/briefing_git_test.go`

**Interfaces:**
- Produces in `briefing_types.go` (all with `json`/`yaml` snake_case tags, e.g. `TurnFiles` → `turn_files`):
  `type GitFacts struct { Branch, Target string; OnTarget, Detached, NoUpstream, TurnKnown bool; TurnFiles, TurnAdded, TurnDeleted, BranchFiles, BranchAdded, BranchDeleted, Commits, Unpushed, Uncommitted int }`;
  `type BriefingPolicy struct { BigDiffFiles, BigDiffLines int; CIWatch bool }` with `func (p BriefingPolicy) normalized() BriefingPolicy` (≤ 0 → 15 / 500);
  `type ghRepo struct { Owner, Name string }`;
  `type gitResult struct { Facts GitFacts; BigDiff bool; Repo ghRepo; Head string }` (`Head` = `git rev-parse HEAD`, kept on the host for `ci_stale`, not sent).
- Produces in `briefing_git.go`:
  `func gitBriefing(ctx context.Context, dir, startTree, windowTarget string, prBase func(ghRepo, string) string, p BriefingPolicy) (gitResult, bool)` — false outside a repo; computes the stop snapshot itself; `TurnKnown` only when `startTree != ""`;
  `func resolveTarget(ctx context.Context, dir, prBase, windowTarget string) string` — order prBase, windowTarget, `origin/HEAD` (short name); compare against `origin/<t>` when that ref exists, else `<t>`;
  `func parseGitHubRemote(url string) (ghRepo, bool)` — `https://github.com/o/r(.git)`, `git@github.com:o/r(.git)`, `ssh://git@github.com/o/r`; other hosts false;
  `func isLockfile(path string) bool` — base name in `package-lock.json`, `go.sum`, `pnpm-lock.yaml`, `yarn.lock`, `Cargo.lock`, `composer.lock`.
- Consumes: Task 3 (`gitx.SnapshotTree`, `gitx.NumStat`, `gitx.CmdContext`).

Remote for `Repo`: the remote of `@{upstream}`, else `origin`. Big diff (spec B3): branch files > `BigDiffFiles` or branch lines without lockfiles > `BigDiffLines`; on the target branch the turn numbers are measured instead.

- [ ] **Step 1: Write failing tests** (temp dirs with a space; a bare repo in the temp dir serves as `origin`):
  - `TestBranchAgainstTarget`: `main` with one commit pushed to origin, `git remote set-head origin main`; branch `feat` with 2 commits (3 files, +40 −0) plus 1 uncommitted change (+2) → `Target "main"`, `Commits 2`, `BranchFiles 4`, `BranchAdded 42`, `Uncommitted 1`, `NoUpstream true`.
  - `TestUnpushedAgainstUpstream`: `feat` pushed with `-u`, then one more commit → `Unpushed 1`, `NoUpstream false`.
  - `TestOnTargetMeasuresTurn`: on `main`, startTree taken, 16 new files → `OnTarget true`, `BranchFiles 0`, `BigDiff true` (16 > 15).
  - `TestLockfileLinesDoNotCount`: branch with `package-lock.json` +900 lines and `a.go` +10 → `BigDiff false`, `BranchAdded 910`.
  - `TestBigDiffByLines`: 2 files, +501 lines → `BigDiff true`.
  - `TestTargetOrder`: prBase `"release"` beats windowTarget `"alpha-main"` beats `origin/HEAD` `"main"`; with neither, `"main"`.
  - `TestNoRepo`: plain temp dir → `ok == false`.
  - `TestDetachedHead`: `git checkout --detach` → `Detached true`, `Branch ""`.
  - `TestParseGitHubRemote`: the three URL forms above → `{patrick-goecommerce Multiterminal-UI}`; `https://gitlab.com/o/r.git` → false.
- [ ] **Step 2:** `go test ./internal/hub/ -run 'Branch|Unpushed|OnTarget|Lockfile|BigDiff|TargetOrder|NoRepo|Detached|ParseGitHub'` — FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Same command — PASS.
- [ ] **Step 5:** Commit `feat(hub): git numbers for a briefing, turn and branch`.

### Task 5: Briefing record and turn book

**Files:**
- Modify: `internal/hub/briefing_types.go` (record types)
- Create: `internal/hub/turn.go`
- Test: `internal/hub/turn_test.go`, `internal/hub/briefing_types_test.go`

**Interfaces:**
- Produces:
  `type PRFacts struct { Number int; URL, State, Base, HeadOID, Review, Checks, FailedCheck string; Draft, Conflicting bool; ChecksTotal, ChecksPassed int }` (tags `number, url, state, base, head_oid, review, checks, failed_check, draft, conflicting, checks_total, checks_passed`);
  `type Briefing struct { PromptID string; BuiltAt int64; Prompt, Closing string; Checks []CheckResult; Git *GitFacts; PR *PRFacts; Warnings, Notes []string }` (tags `prompt_id, built_at, prompt, closing, checks, git,omitempty, pr,omitempty, warnings, notes`; `BuiltAt` Unix seconds);
  warning constants `WarnCIRed="ci_red"`, `WarnPRBlocked="pr_blocked"`, `WarnCheckFailed="check_failed"`, `WarnEndedError="ended_error"`, `WarnDirty="dirty"`, `WarnBigDiff="big_diff"`; note constants `NoteDiffSlow="diff_slow"`, `NoteCIPaused="ci_paused"`, `NoteCIStale="ci_stale"`;
  `func warningsFor(b Briefing, endedError, bigDiff bool) []string` — in that order; `ci_red` from `b.PR.Checks == "red"`, `pr_blocked` from `Conflicting || Review == "CHANGES_REQUESTED"`, `check_failed` when any check's result is `fail`;
  `func closingWords(msg string) string` — last of `paragraphs(msg)` (hook_detail.go), whitespace collapsed, ≤ 600 runes;
  `type turn struct { promptID, prompt, cwd, startTree string; keepAlive, publish, closed bool; checks []CheckResult }`;
  `type turnBook struct` with `func newTurnBook() *turnBook`, `func (b *turnBook) expectKeepAlive(id int)`, `func (b *turnBook) begin(id int, promptID, prompt, cwd string) (keepAlive bool)`, `func (b *turnBook) setStartTree(id int, promptID, tree string)`, `func (b *turnBook) observe(id int, ev hooks.Event)`, `func (b *turnBook) close(id int, promptID string) (turn, bool)`, `func (b *turnBook) forget(id int)`.
- Consumes: Task 2 (`classifyCheck`, `publishCommand`), Task 1 fields on `hooks.Event`.

`observe`: ignored when the turn is closed, when both prompt IDs are set and differ, and when `Interrupt`; updates `cwd` from any event with one; for `PostToolUse`/`PostToolUseFailure` with `Tool == "Bash"` classifies `Command` (failed = event is the failure one) and replaces the result of an existing label; sets `publish`. `close` returns a copy and marks the turn closed; a second `close` returns false. `begin` consumes a pending `expectKeepAlive`.

- [ ] **Step 1: Write failing tests**: `TestRedThenGreenIsOK` (two `go test` events, fail then ok → one check `ok`); `TestKeepAliveTurnIsMarked` (`expectKeepAlive(1)`, `begin` → true; next `begin` → false); `TestEventsAfterCloseIgnored`; `TestForeignPromptIgnored`; `TestInterruptIgnored`; `TestPublishNoted` (`git push` → `publish`); `TestCloseTwice`; `TestPromptClipped` (300-rune prompt → 200 runes); `TestWarningsOrder` (all six conditions → exact order above); `TestClosingWords` (`"Erster.\n\nZweiter Absatz\nmit Umbruch."` → `"Zweiter Absatz mit Umbruch."`).
- [ ] **Step 2:** `go test ./internal/hub/ -run 'RedThenGreen|KeepAliveTurn|AfterClose|ForeignPrompt|InterruptIgnored|PublishNoted|CloseTwice|PromptClipped|WarningsOrder|ClosingWords'` — FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Same command — PASS.
- [ ] **Step 5:** Commit `feat(hub): turn book and briefing record`.

### Task 6: The briefing desk on `Embedded`

**Files:**
- Create: `internal/hub/briefing_desk.go`, `internal/hub/embedded_turn.go`
- Modify: `internal/hub/embedded.go` (one field `desk *briefingDesk`, construct it, forget on close next to `h.attention.forget(id)` at :195; stay ≤ 300 lines — move a small helper out if needed), `internal/hub/embedded_hooks.go` (`applyHookEvent`: call `h.onTurnHook(ev)` right after the `lookup`, before the `SessionEnd` branch), `internal/hub/embedded_stream.go` (`watchExit`: `h.finishTurn(id, exitCode != 0, "")`), `internal/hub/embedded_scan.go` (where the hang's `noteFact` is made: `h.finishTurn(r.ID, true, "")`), `internal/hub/keepalive.go` (`nudge`: `h.desk.turns.expectKeepAlive(id)` **before** writing the message), `internal/hub/options.go` (`Briefing func() BriefingPolicy`), `internal/hub/events.go` (`EventSessionBriefing = "session.briefing"`, `type BriefingUpdate struct { ID int \`json:"id" yaml:"id"\`; Briefing Briefing \`json:"briefing" yaml:"briefing"\` }`)
- Test: `internal/hub/embedded_turn_test.go`

**Interfaces:**
- Produces: `type briefingDesk struct` with at least `turns *turnBook`, the stored entries `{briefing Briefing; endedError, bigDiff bool; res gitResult}` per session, the window targets, a semaphore of 4, `limit time.Duration` (5 s), a `snapshot func(ctx context.Context, dir string) (string, error)` seam (default `gitx.SnapshotTree`) and the policy func; `func (h *Embedded) onTurnHook(ev hooks.Event)`; `func (h *Embedded) finishTurn(id int, endedError bool, closing string)`; `func (h *Embedded) Briefing(id int) (Briefing, bool, error)` (`ErrNoSession` for an unknown id); `func (h *Embedded) SetTarget(id int, branch string) error`; desk hook for Task 13: `briefingDesk.afterBuild func(id int, res gitResult, publish bool)` (nil in stage a) and `func (d *briefingDesk) updatePR(id int, pr *PRFacts, notes []string) (Briefing, bool)` (recomputes warnings from stored `endedError`/`bigDiff`).
- Consumes: Tasks 4 and 5.

Flow: `UserPromptSubmit` → `begin`; unless keep-alive, snapshot in a goroutine (5 s limit) → `setStartTree`. `Post*` → `observe`. `Stop` → `finishTurn(id, false, closingWords(ev.Message))`. `finishTurn`: `close`; keep-alive turn → nothing; else build in a goroutine: semaphore of 4, cancel a running build of the same session, ctx limit 5 s (desk field `limit`, tests shorten it), `gitBriefing`, on ctx expiry `Notes += diff_slow` and only the numbers gathered; store `{briefing, endedError, bigDiff, key}`; emit `EventSessionBriefing` only if the session still exists and the build was not superseded. Policy: `Options.Briefing` nil → `BriefingPolicy{}.normalized()` (CI off).

- [ ] **Step 1: Write failing tests** (sink records events; drive with `h.applyHookEvent(nil, hooks.Event{…})` as `embedded_attention_test.go` does; temp git repo as `Cwd`; wait for the event with a 5 s timeout):
  - `TestStopBuildsBriefing`: `UserPromptSubmit` (prompt "kürz die Details", prompt_id p1) → write a file in the repo → `PostToolUseFailure` Bash `go vet ./...` → `Stop` (p1, message "Erledigt.\n\nvet meldet noch was.") ⇒ one `session.briefing` with `Prompt "kürz die Details"`, `Checks [{go vet fail}]`, `Closing "vet meldet noch was."`, `Git.TurnKnown true`, `Git.TurnFiles 1`, `Warnings [check_failed dirty]`; `h.Briefing(id)` returns the same.
  - `TestKeepAliveTurnKeepsLastBriefing`: a briefing exists; `nudge`-style `expectKeepAlive`, then `UserPromptSubmit`/`Stop` ⇒ no new event, `Briefing` unchanged.
  - `TestExitBuildsWithEndedError`: `finishTurn(id, true, "")` after a turn ⇒ `ended_error` in `Warnings`.
  - `TestStopBeforeSnapshot` (Review Focus 1): snapshot blocked (test seam: desk `snapshot func(ctx, dir) (string, error)` replaced by one that waits on a channel) ⇒ briefing with `Git.TurnKnown false`, `TurnFiles 0`.
  - `TestCloseDuringBuild` (Review Focus 2): build blocked on the same seam, `Close(id)`, release ⇒ no event, `Briefing` → `ErrNoSession`.
  - `TestBuildTimeLimit`: desk `limit` 50 ms with a blocking seam ⇒ `Notes [diff_slow]`.
  - `TestNoRepoNoGit`: `Cwd` a plain temp dir ⇒ `Git == nil`.
  - `TestSetTargetUsed`: `SetTarget(id, "alpha-main")` before `Stop` ⇒ `Git.Target "alpha-main"`.
- [ ] **Step 2:** `go test ./internal/hub/ -run 'StopBuildsBriefing|KeepAliveTurnKeeps|ExitBuilds|StopBeforeSnapshot|CloseDuringBuild|BuildTimeLimit|NoRepoNoGit|SetTargetUsed'` — FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** `go test ./internal/hub/` — PASS (whole package; Part 1 attention tests must stay green). `wc -l internal/hub/embedded.go` ≤ 300.
- [ ] **Step 5:** Commit `feat(hub): build a briefing when a turn ends`.

### Task 7: Config and host policy

**Files:**
- Modify: `internal/config/config.go` (field on `Config` next to `KeepAlive`: `Briefing BriefingSettings \`yaml:"briefing" json:"briefing"\``; struct `BriefingSettings { BigDiffFiles int \`yaml:"big_diff_files" json:"big_diff_files"\`; BigDiffLines int \`yaml:"big_diff_lines" json:"big_diff_lines"\`; CIWatch *bool \`yaml:"ci_watch" json:"ci_watch"\` }`; defaults 15 / 500 / `boolPtr(true)` in both default blocks like `KeepAlive`; `func (c Config) ShouldWatchCI() bool` (nil → true)). This file is on the debt list: add only these lines.
- Modify: `internal/backend/launch_delegate.go` (`func (a *AppService) briefingPolicy() hub.BriefingPolicy`), `internal/backend/app_host_daemon.go` (`Briefing: a.briefingPolicy,` next to `KeepAlive:`), `cmd/mtuid/launcher.go` (`func briefingPolicy() hub.BriefingPolicy` with `config.Load()`), `cmd/mtuid/main.go` (`Briefing: briefingPolicy,`)
- Modify: `frontend/wailsjs/go/models.ts` (`config.BriefingSettings` class + field on `config.Config`), `frontend/src/stores/config.ts` (interface and default object)
- Test: `internal/config/config_test.go`, `internal/backend/launch_delegate_test.go` (or the file where `keepAlivePolicy` is tested)

- [ ] **Step 1: Write failing tests**: `TestBriefingDefaults` (fresh config → 15, 500, `ShouldWatchCI()` true); `TestBriefingYAMLRoundTrip` (`ci_watch: false` survives save/load); `TestBriefingPolicyFromConfig` (backend: 20/800/false in config → `hub.BriefingPolicy{20, 800, false}`).
- [ ] **Step 2:** `go test ./internal/config/ ./internal/backend/ -run Briefing` and `go test ./internal/tsmodels/` — FAIL (tsmodels names the missing fields).
- [ ] **Step 3:** Implement, including the mirrors the tsmodels test prints.
- [ ] **Step 4:** `go test ./internal/config/ ./internal/backend/ ./internal/tsmodels/ ./cmd/mtuid/` — PASS.
- [ ] **Step 5:** Commit `feat(config): briefing thresholds and CI watch switch`.

### Task 8: Wire and `Host`

**Files:**
- Modify: `internal/hub/host.go` (add `Briefing(id int) (Briefing, bool, error)` and `SetTarget(id int, branch string) error`)
- Create: `internal/hub/server_briefing.go`, `internal/hub/client_briefing.go`
- Modify: `internal/hub/server.go` (`handleSession` switch: add `case action == "briefing" && r.Method == http.MethodGet: s.handleBriefing(w, id)` and `case action == "target" && r.Method == http.MethodPost: s.handleTarget(w, r, id)`; move the inline `suspend` and `wake` bodies into `handleSuspend(w, id)` / `handleWake(w, id)` in `server_activity.go` so the file shrinks)
- Test: `internal/hub/server_briefing_test.go` (follow the existing Remote↔Server round-trip tests, e.g. the seen/keepalive ones from Part 1)

**Interfaces:**
- Wire: `GET /v1/sessions/{id}/briefing` → 200 `{"briefing": <Briefing>|null}` (ruling 2), unknown session → `writeHostError`; `POST /v1/sessions/{id}/target` body `{"branch": "…"}` → 204.
- Produces: `func (r *Remote) Briefing(id int) (Briefing, bool, error)`, `func (r *Remote) SetTarget(id int, branch string) error`. Events need no work: `Server.Sink` forwards every event name.

- [ ] **Step 1: Write failing tests**: `TestBriefingOverTheWire` (Embedded with a stored briefing behind a Server; `Remote.Briefing` returns it field for field, `ok true`); `TestNoBriefingIsNull` (`ok false`, `err nil`); `TestUnknownSessionBriefing` (`ErrNoSession`); `TestSetTargetOverTheWire` (`Remote.SetTarget(id, "alpha-main")` → Embedded's next build uses it); `TestBriefingEventReachesRemote` (the `session.briefing` event arrives at the Remote's sink and decodes with `DecodePayload[BriefingUpdate]`).
- [ ] **Step 2:** `go test ./internal/hub/ -run 'OverTheWire|NoBriefingIsNull|UnknownSessionBriefing|BriefingEventReaches'` — FAIL.
- [ ] **Step 3:** Implement; fix any Host fake the compiler names (backend fakes embed `hub.Host` and need nothing).
- [ ] **Step 4:** `go test ./internal/hub/ ./internal/backend/ ./cmd/mt/ ./cmd/mtuid/` — PASS; `wc -l internal/hub/server.go` ≤ 452.
- [ ] **Step 5:** Commit `feat(hub): briefing and target over the wire`.

### Task 9: Backend event and bindings

**Files:**
- Create: `internal/backend/app_briefing.go`, `internal/backend/app_session_refs_briefing.go`
- Modify: `internal/backend/app_host_stream.go` (`onHostEvent` switch: `case hub.EventSessionBriefing:` decode `hub.BriefingUpdate`, `a.emitBriefing(ev.ID, ev.Briefing)`)
- Modify: `frontend/wailsjs/go/backend/App.js`, `App.d.ts` (add `import {hub} from '../models';`), `frontend/wailsjs/go/models.ts` (new `export namespace hub` with classes `Briefing`, `CheckResult`, `GitFacts`, `PRFacts` in the file's existing class/constructor style), `internal/tsmodels/drift_test.go` (`mirrored` gets `"hub": "internal/hub"`)
- Test: `internal/backend/app_session_refs_briefing_test.go`

**Interfaces:**
- Produces: `type BriefingInfo struct { ID hub.Ref \`json:"id"\`; Briefing hub.Briefing \`json:"briefing"\` }` (event payload, comment like `AttentionInfo`: no models.ts mirror); `func (a *AppService) emitBriefing(id int, b hub.Briefing)` → `"terminal:briefing"`; bindings `func (a *AppService) GetSessionBriefing(r hub.Ref) *hub.Briefing` (nil for a foreign ref, an error or none) and `func (a *AppService) SetSessionTarget(r hub.Ref, branch string)` (logs `[briefing] SetTarget: <err>` and returns).
- App.d.ts: `export function GetSessionBriefing(arg1:string):Promise<hub.Briefing|null>;` and `export function SetSessionTarget(arg1:string,arg2:string):Promise<void>;`; App.js with `$Call.ByID(3334881243, arg1)` and `$Call.ByID(2176172594, arg1, arg2)`.

- [ ] **Step 1: Write failing tests** with a fake host embedding `hub.Host`: `TestGetSessionBriefingTranslatesRef` (local ref → host called with its int id, value returned); `TestForeignRefNeverReachesHost` (both bindings, host not called, nil returned); `TestSetSessionTargetLogsError` (host returns an error → no panic).
- [ ] **Step 2:** `go test ./internal/backend/ -run 'SessionBriefing|ForeignRefNever|SetSessionTarget'` and `go test ./internal/tsmodels/` — FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** `go test ./internal/backend/ ./internal/tsmodels/` — PASS; the backend import check is empty.
- [ ] **Step 5:** Commit `feat(backend): briefing event and bindings`.

### Task 10: Frontend data: store, formatting, target sync

**Files:**
- Create: `frontend/src/stores/briefings.ts`, `frontend/src/lib/briefing.ts`, `frontend/src/lib/targetSync.ts`
- Modify: `frontend/src/App.svelte` (`EventsOn('terminal:briefing', e => applyBriefing(e.data.id, e.data.briefing))` next to `terminal:attention`; `forgetBriefing` where a pane's session closes; `loadBriefings(<all pane refs>)` once after the restore; subscribe the tabs store to the target sync)
- Test: `frontend/src/stores/briefings.test.ts`, `frontend/src/lib/briefing.test.ts`, `frontend/src/lib/targetSync.test.ts`

**Interfaces:**
- `stores/briefings.ts`: `export const briefings: Writable<Record<string, hub.Briefing>>`; `applyBriefing(ref: SessionRef, b: hub.Briefing): void`; `forgetBriefing(ref: SessionRef): void`; `loadBriefings(refs: SessionRef[], fetch?: (ref: SessionRef) => Promise<hub.Briefing | null>): Promise<void>` (default `App.GetSessionBriefing`; null and rejections skipped).
- `lib/briefing.ts`:
  `hasWarnings(b?: hub.Briefing): boolean`;
  `warningShort(code: string, b: hub.Briefing): string` → `ci_red` "CI rot", `pr_blocked` "PR blockiert", `check_failed` "`<label of first failed check>` ✗", `ended_error` "Fehlerende", `dirty` "ungesichert", `big_diff` "großer Diff";
  `type Verdict = { kind: 'warn' | 'pending' | 'clean' | 'detail'; text: string }`;
  `verdict(b: hub.Briefing | undefined, att: PaneAttention | undefined): Verdict` (spec E1);
  `prLine(b: hub.Briefing): string`;
  `cardLines(b: hub.Briefing, head: { project: string; pane: string }): { key: string; text: string }[]` (spec E3 order, empty rows dropped);
  `cardPosition(entry: Rect, panel: Rect, card: Size, viewport: Size): { left: number; top: number }` with `Rect = {left, top, width, height}`, `Size = {width, height}`: left = `panel.left - card.width - 8`, at least 8; top = `entry.top`, clamped to `[8, viewport.height - card.height - 8]`.
- `lib/targetSync.ts`: `createTargetSync(send: (ref: SessionRef, branch: string) => Promise<void>): (tabs: Tab[]) => void` — sends once per (ref, branch) change, skips empty refs and empty branches.

- [ ] **Step 1: Write failing tests**:
  - `verdict`: warnings `[check_failed, dirty, big_diff, ci_red]` with failed check `go vet` → `{warn, "⚠ CI rot · go vet ✗ · ungesichert"}` (order by host, max three); none + `pr.checks "pending"` + closing "Fertig." → `{pending, "◷ CI läuft · „Fertig.“"}`; none → `{clean, "✓ sauber · „Fertig.“"}`; empty closing → `"✓ sauber"`; `att.source "ci"`, detail "CI rot: backend" → `{detail, "CI rot: backend"}`; no briefing → `{detail, att.detail}`.
  - `prLine`: `{number 12, checks "green", checks_total 2, checks_passed 2, review "REVIEW_REQUIRED", state "OPEN"}` → `"PR #12 · CI 2/2 ✓ · Review offen"`; pending 1/3 → `"PR #12 · CI läuft 1/3"`; `state "MERGED"` → `"PR #12 gemergt"`; red, failed "backend" → `"PR #12 · CI rot: backend"`; conflicting → `… · Konflikte`; `review "CHANGES_REQUESTED"` → `… · Änderungen angefordert`; `review "APPROVED"` → `… · freigegeben`; `draft` → `… · Entwurf`; note `ci_stale` → suffix `" · für älteren Stand"`; no PR with git off target → `"kein PR"`; note `ci_paused` → `"CI-Abfrage pausiert"`; no git and no PR → `""`.
  - `cardLines`: with all data the keys come in the order head, prompt, turn, branch, checks, pr, closing, warnings; `turn` text `"Turn:   3 Dateien +40 −5"`; branch `"Branch: 7 Dateien +212 −48 · 3 Commits · 2 ungepusht"`, with `no_upstream` `"… · nie gepusht"`; head `"goki · hook-refactor · gegen alpha-main"`, on target `"… · auf alpha-main"`, detached `"… · abgekoppelt"`; checks `"go test ✓   go vet ✗   test ?"`; without git the turn and branch rows are absent; `diff_slow` adds `"Diff zu langsam"` to the warnings row.
  - `cardPosition`: entry at top 900 in a 1000-high viewport, card height 300 → top 692; panel left 200, card width 340 → left 8.
  - `loadBriefings` (Review Focus 5): fetch stub returns a briefing for `h:1`, null for `h:2`, rejects for `h:3` → store has only `h:1`.
  - `targetSync`: same tabs twice → one send; branch change → second send; empty branch → none.
- [ ] **Step 2:** `cd frontend && npx vitest run src/lib/briefing.test.ts src/stores/briefings.test.ts src/lib/targetSync.test.ts` — FAIL.
- [ ] **Step 3:** Implement and wire into `App.svelte`.
- [ ] **Step 4:** Same command — PASS; `npm run build` ok.
- [ ] **Step 5:** Commit `feat(ui): briefing store, formatting and target sync`.

### Task 11: Verdict line, sort, card

**Files:**
- Modify: `frontend/src/lib/attention.ts` (`attentionList(tabs, currentTabId, warned: (p: Pane) => boolean = () => false)`: within one rank, warned entries before others, then age as today)
- Modify: `frontend/src/components/WaitingPanel.svelte` (line 2 from `verdict`, classes `warn`/`pending`/`clean`; hover 300 ms or keyboard focus on a rank 2–3 entry with a briefing shows the card; leaving hides it)
- Create: `frontend/src/components/BriefingCard.svelte` (renders `cardLines`, width 340 px, positioned with `cardPosition`; the PR number is a button that calls `BrowserOpenURL(pr.url)` from `wailsjs/runtime/runtime`)
- Modify: `frontend/src/components/PaneGrid.svelte:442`, `frontend/src/components/Toolbar.svelte:89` (pass `warned` from `$briefings`)
- Test: `frontend/src/lib/attention.test.ts`

- [ ] **Step 1: Write failing tests**: `attentionList` with two rank-3 entries, the older one clean and the younger one warned → warned first; two warned entries → older first; ranks still dominate (a clean rank-2 before a warned rank-3); `panelRows` on that list hides the clean ones first.
- [ ] **Step 2:** `cd frontend && npx vitest run src/lib/attention.test.ts` — FAIL.
- [ ] **Step 3:** Implement the components; follow Part 1's markup and theme tokens (`--status-done` etc.); German texts exactly as in spec E1/E3.
- [ ] **Step 4:** `npx vitest run` (all) and `npm run build` — PASS.
- [ ] **Step 5:** Commit `feat(ui): verdict on line 2 and the briefing card next to the box`.

## Stage b — CI and PR watch

### Task 12: Parse `gh` and pick the cadence

**Files:**
- Create: `internal/hub/ciwatch_parse.go`, `internal/hub/ciwatch_schedule.go`, `internal/hub/testdata/gh_pr_list_214.json`
- Test: `internal/hub/ciwatch_parse_test.go`, `internal/hub/ciwatch_schedule_test.go`

**Interfaces:**
- Produces: `func pickPR(raw []byte, owner string) (PRFacts, bool, error)` (owner vs `headRepositoryOwner.login`, case-insensitive; open before others; else highest number; fills checks per spec C3 — `CheckRun` by `status`/`conclusion`, `StatusContext` by `state`, name from `name` resp. `context`); `func ciSignals(pr PRFacts) (red, blocked bool)`; `func nextPoll(pr PRFacts, found bool, pendingSince, now time.Time) (time.Duration, bool)` — false means stop.
- The fixture is the measured response, verbatim:

```json
[{"baseRefName":"alpha-main","headRefOid":"437bedd24c5d77058f04e9b7d126a2e75ca852ac","headRepositoryOwner":{"id":"MDQ6VXNlcjQ2ODg3NDQ1","name":"Patrick Hennig","login":"patrick-goecommerce"},"isDraft":false,"mergeable":"UNKNOWN","number":214,"reviewDecision":"","state":"MERGED","statusCheckRollup":[{"__typename":"CheckRun","completedAt":"2026-10-14T09:48:03Z","conclusion":"SUCCESS","detailsUrl":"https://github.com/patrick-goecommerce/Multiterminal-UI/actions/runs/38544415862/job/115786808676","name":"backend","startedAt":"2026-10-14T09:43:18Z","status":"COMPLETED","workflowName":"CI"},{"__typename":"CheckRun","completedAt":"2026-10-14T09:43:55Z","conclusion":"SUCCESS","detailsUrl":"https://github.com/patrick-goecommerce/Multiterminal-UI/actions/runs/38544415862/job/115786808809","name":"frontend","startedAt":"2026-10-14T09:43:18Z","status":"COMPLETED","workflowName":"CI"}],"url":"https://github.com/patrick-goecommerce/Multiterminal-UI/pull/214"}]
```

  Other cases are derived from it in the test by changing single fields after unmarshalling into `[]map[string]any` and marshalling again (`conclusion`, `status`, `state`, `mergeable`, `reviewDecision`, `headRepositoryOwner.login`, an added `StatusContext` `{"__typename":"StatusContext","context":"ci/legacy","state":"FAILURE"}` — that shape is GitHub's GraphQL schema, not measured).

- [ ] **Step 1: Write failing tests**: fixture → `Number 214, State "MERGED", Base "alpha-main", HeadOID "437bedd…", Checks "green", ChecksTotal 2, ChecksPassed 2`; `backend` `conclusion "FAILURE"` → `Checks "red"`, `FailedCheck "backend"`, `red true`; `status "IN_PROGRESS"`, `conclusion ""` → `"pending"`; each of `TIMED_OUT`, `CANCELLED`, `ACTION_REQUIRED`, `STARTUP_FAILURE` → red; `StatusContext FAILURE` → red, `FailedCheck "ci/legacy"`; `mergeable "CONFLICTING"` → `blocked`; `reviewDecision "CHANGES_REQUESTED"` → `blocked`; owner `"someone-else"` → `found false`; two entries (closed #10, open #11) → #11; empty rollup → `"none"`. Schedule: pending since 10 min → 1 min; since 40 min → 5 min; since 3 h 1 min and open → 15 min (ruling 5); open green → 15 min; merged → stop; not found → stop.
- [ ] **Step 2:** `go test ./internal/hub/ -run 'PickPR|CISignals|NextPoll'` — FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Same command — PASS.
- [ ] **Step 5:** Commit `feat(hub): read PR and check state from gh`.

### Task 13: The watcher, raising attention, wiring `gh`

**Files:**
- Create: `internal/hub/ciwatch.go`, `internal/hub/attention_raise.go`, `internal/hub/ghexec.go`
- Modify: `internal/hub/options.go` (`GH GHRunner`, `type GHRunner func(ctx context.Context, dir string, args ...string) ([]byte, error)`), `internal/hub/briefing_desk.go` (set `afterBuild` to register the branch; give `gitBriefing` the watcher's known PR base), `internal/hub/embedded.go` (start the loop when `opts.GH != nil`; keep ≤ 300), `internal/backend/app_host_daemon.go` and `cmd/mtuid/main.go` (`GH: hub.ExecGH,`)
- Test: `internal/hub/ciwatch_test.go`, `internal/hub/attention_raise_test.go`

**Interfaces:**
- Produces: `func ExecGH(ctx context.Context, dir string, args ...string) ([]byte, error)` (`exec.CommandContext("gh", …)`, `procs.HideConsole`, env + `GH_PROMPT_DISABLED=1`, 15 s limit); `func (b *attentionBook) raise(id int, source, detail string, at time.Time) (Attention, bool)` — false when the current state is `active`, `waitingPermission` or `waitingAnswer`; otherwise `State error`, `Seen false`, `Since at`, `Source`, `Detail`, `Rank` recomputed; `type ciKey struct { Owner, Name, Branch string }` (from `gitResult.Repo` and `GitFacts.Branch`); `func newCIWatch(gh GHRunner, now func() time.Time) *ciWatch`; `func (c *ciWatch) touch(id int, key ciKey, dir string)` (due now); `func (c *ciWatch) drop(id int)`; `func (c *ciWatch) base(key ciKey) string`; `func (h *Embedded) ciLoop()` (tick 15 s; also woken by `touch`).
- Consumes: Tasks 6 and 12.

Rules (spec C1–C4): policy `CIWatch` false → no polling; `gh auth status` once per host start, remembered; a failing `gh pr list` sets auth to unknown, pauses the key 15 min and adds note `ci_paused` to each session's briefing; a found PR updates every session on the key through `desk.updatePR` and emits `session.briefing`; `ci_stale` when the session's stored `gitResult.Head` differs from `HeadOID`; a transition to red or blocked for `(HeadOID, signal)` not seen before raises each idle session on the key with source `"ci"` and detail `"CI rot: <FailedCheck>"`, `"PR #<n>: Konflikte"` or `"PR #<n>: Änderungen angefordert"`, and emits `EventSessionAttention`; merged/closed or no PR → stop watching the key; `drop` on session close. A `Post*` with `publish` and the `Stop` that follows trigger `touch` immediately.

- [ ] **Step 1: Write failing tests** (fake `GHRunner` serving the Task 12 fixture variants, fake clock, sessions created with `stubLauncher`):
  - `TestPendingThenRedRaisesOnce`: poll 1 pending → no raise; poll 2 red → one `session.attention` with `State error`, `Rank 2`, `Source "ci"`, `Detail "CI rot: backend"`; poll 3 still red, same head → no new raise.
  - `TestNewHeadRedRaisesAgain`: after a raise, a fixture with another `headRefOid` and red → raise again.
  - `TestActiveSessionOnlyGetsBriefing` + two sessions on one key (Review Focus 3): one `active`, one `done`+seen → only the idle one raised, both get a `session.briefing` with `PR.Checks "red"` and `ci_red` in `Warnings`.
  - `TestBlockedByConflict`: `mergeable CONFLICTING` → detail `"PR #214: Konflikte"`.
  - `TestMergedStopsWatching`: merged fixture → no further `gh` call after the clock advances 1 h.
  - `TestGHErrorPauses`: runner returns an error → note `ci_paused`; no call before 15 min; a call after.
  - `TestCIWatchOffNoCalls`: policy `CIWatch false` → runner never called.
  - `TestAuthCheckedOnce`: two keys → `gh auth status` called once.
  - `TestRaiseRefusesWaiting` (attention_raise_test): `waitingAnswer` → false, record unchanged; `done` seen → error, unseen, rank 2.
- [ ] **Step 2:** `go test ./internal/hub/ -run 'PendingThenRed|NewHeadRed|ActiveSessionOnly|BlockedByConflict|MergedStops|GHErrorPauses|CIWatchOff|AuthChecked|RaiseRefuses'` — FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** `go test ./internal/hub/ ./internal/backend/ ./cmd/mtuid/ ./cmd/mt/` — PASS; `GOOS=windows CGO_ENABLED=0 go vet ./internal/... ./cmd/...` clean.
- [ ] **Step 5:** Commit `feat(hub): watch CI and PRs and bring a pane back when they turn red`.

## Stage c — settings

### Task 14: SettingsDialog section „Leitstand“

**Files:**
- Modify: `frontend/src/components/SettingsDialog.svelte` (locals `briefingBigFiles`, `briefingBigLines`, `briefingCIWatch` set in `initDialog()` from `$config.briefing` with fallbacks 15/500/true; saved as `briefing: { big_diff_files, big_diff_lines, ci_watch }`; reset-to-defaults sets 15/500/true; a `setting-group` with the toggle "CI und PRs beobachten" (Aktiv/Inaktiv) and two number inputs "Großer Diff ab Dateien" / "Großer Diff ab geänderten Zeilen", with the description "Lockfiles zählen bei den Zeilen nicht mit.")
- Test: create `frontend/src/components/SettingsDialog.briefing.test.ts`, modelled on `SettingsDialog.sessionhost.test.ts` (same render and save-capture setup).

- [ ] **Step 1: Write the failing test**: toggling CI off and entering 20/800 produces `briefing: { big_diff_files: 20, big_diff_lines: 800, ci_watch: false }` in the saved object; opening the dialog again shows those values; no `$:` block assigns any of the three.
- [ ] **Step 2:** `cd frontend && npx vitest run <test file>` — FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** `npx vitest run` and `npm run build` — PASS.
- [ ] **Step 5:** Commit `feat(settings): Leitstand section for the briefing thresholds and the CI watch`.

### Task 15: Documentation

**Files:**
- Modify: `CLAUDE.md` (active specs list: add `docs/superpowers/specs/2026-10-19-leitstand-review-briefing-design.md — Leitstand Teil 2: Review-Briefing`; Architecture data flow: one bullet "Briefings live on the host (`turnBook`, `briefingDesk`, `ciWatch`); the frontend only formats them; CI/PR red re-raises attention as an error with source `ci`"; config example: the `briefing:` block)
- Modify: `docs/superpowers/specs/2026-10-19-leitstand-review-briefing-design.md` (status "Umgesetzt auf `worktree-leitstand-review-briefing`, E2E-Abnahme offen"; Betrifft adds `internal/gitx`; Teil D: `Review` instead of `ChangesRequested`, `BuiltAt` as Unix seconds, `GET` answers `{"briefing": null}`; A1: the `command` field; the plan's rulings 5–7)
- Modify: `docs/superpowers/specs/2026-10-07-leitstand-aufmerksamkeit-design.md` (table row 2 links the new spec)

- [ ] **Step 1:** Make the edits; every changed spec sentence matches what Tasks 1–14 built.
- [ ] **Step 2:** Full verification: `go test ./internal/... ./cmd/mt/ ./cmd/mtuid/ ./cmd/mtui-hook/` (only the known `internal/launch` worktree test red), `go vet ./...`, `gofmt -l .` empty, backend import check empty, `cd frontend && npx vitest run && npm run build`.
- [ ] **Step 3:** Commit `docs: Leitstand part 2 in CLAUDE.md and the specs`.

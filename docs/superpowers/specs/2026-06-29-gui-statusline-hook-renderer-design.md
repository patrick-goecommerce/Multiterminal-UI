# GUI Statusline Renderer + GUI Hook Handler — design

- **Date:** 2026-06-29
- **Status:** Approved design, ready for implementation plan
- **Branch target:** `feat/claudelog-jsonl-reader` (off `alpha-main`; not merged anywhere — free to rewrite the committed Task 1–5 statusline mechanics)
- **One-line goal:** Stop the console-window flash that Claude triggers on every statusline render and every hook event on Windows, by replacing the two PowerShell scripts MTUI registers (`mtui-statusline.ps1`, `hook_handler.ps1`) with GUI-subsystem Go binaries — and fold the existing statusline cost-capture feature into the renderer.

---

## Background — the bug (diagnosed with evidence)

A process-creation monitor (`Get-CimInstance Win32_Process` polling while submitting input to a Claude pane) established:

> **Every console-subsystem subprocess that Claude — or a PowerShell it spawns — starts per render/event allocates a `conhost.exe` window** (observed as `\??\C:\Windows\system32\conhost.exe 0x4`).

Three observed sources of the flash:
1. **Hooks** — `powershell -NonInteractive -File hook_handler.ps1 <Event>`, fired per `PreToolUse`/`PostToolUse`/etc. (many times per turn). Each spawns powershell → conhost.
2. **Statusline** — `powershell … mtui-statusline.ps1`, fired per render. Each spawns powershell → conhost.
3. **`git branch --show-current`** inside the statusline ps1 — even if the powershell parent is launched with `CREATE_NO_WINDOW`, the inner `git.exe` (console subsystem, no inherited console) allocates its own conhost.

A GUI-subsystem binary (`-ldflags -H windowsgui`) does **not** get a console allocated. This was verified live: `mtui-hook.exe` (built GUI subsystem, Subsystem=2) ran with **no accompanying conhost**, while the powershell statusline in the same window kept flooding conhost.

`-WindowStyle Hidden` on the hook command did **not** help (the conhost is allocated by the OS before powershell applies the style).

### Why this supersedes the committed Task 1–5 mechanics

Task 1–5 (committed on this branch) captured cost by **wrapping** the existing powershell statusline with a console-subsystem `statusline-forward` shim that tees stdin and POSTs. That shim is itself console-subsystem and still leaves the inner powershell + `git` flashing. Replacing the powershell statusline with a single GUI renderer that *also* POSTs the cost unifies the flash fix with the capture feature and removes the wrap/embed-of-wrapper machinery.

---

## Architecture

Two GUI-subsystem Go binaries replace the two flashing PowerShell scripts:

| New binary (GUI subsystem) | Replaces | Responsibility |
|---|---|---|
| `cmd/mtui-statusline` | `mtui-statusline.ps1` **and** the `cmd/statusline-forward` wrapper shim | Render the status line **and** POST cost/context/model to MTUI |
| `cmd/mtui-hook` | `hook_handler.ps1` | Append the hook event as a JSONL line (already written & verified) |

Both are built with `-ldflags -H windowsgui` on Windows so Claude spawning them never allocates a console window. On non-Windows they build normally (no flash concern there).

### `mtui-statusline` (renderer + capture)

- Reads Claude's statusline JSON from **stdin**; writes the rendered status line to **stdout** (this is what Claude displays).
- **Rendering configuration is passed as CLI flags** baked into the registered command string (e.g. `--template standard --model --context --git`). `applyStatusLine` generates the command per `config.StatusLineSettings`. The binary stays static and is not coupled to `~/.multiterminal.yaml`.
- **Git branch without a subprocess:** start from the JSON's `workspace.current_dir`, walk up to the first `.git`, and read the branch directly:
  - `.git` is a directory → read `.git/HEAD`; if it is `ref: refs/heads/<branch>` use `<branch>`, else (detached) show the short SHA or nothing.
  - `.git` is a file (worktree / submodule) → `gitdir: <path>` → read `<path>/HEAD` the same way.
  - Any failure → omit the git segment (never error).
- **Cost POST:** fire-and-forget POST of `{sessionId, payload:<raw claude json>}` to `http://127.0.0.1:$MTUI_PORT/api/statusline`, `sessionId` from `$MULTITERMINAL_SESSION_ID`. Short client timeout; silent on any failure (MTUI down / stale port). Identical contract to the current `/api/statusline` handler.
- **Fail-safe ordering:** render and write stdout FIRST, then POST. A failed POST, missing env, or git error must never blank or delay the displayed line.

### `mtui-hook` (event recorder — already implemented)

- Reads the hook event JSON from stdin, event type from `os.Args[1]`.
- Appends one JSONL line `{ts, event, session_id, mt_id, tool, message}` (matching `internal/backend.rawHookEvent`) to `%APPDATA%\Multiterminal\hooks\<session_id>.jsonl`, UTF-8, no BOM.
- `mt_id` from `$MULTITERMINAL_SESSION_ID`; for `UserPromptSubmit`, `message` is the `prompt` field. All failures silent.

---

## Registration & path resolution

- `statusLine.command` = `"<resolved mtui-statusline path>" <render flags>` — registered **directly**, no powershell, no wrapper.
- Hook command = `"<resolved mtui-hook path>" <Event> # multiterminal-hook` (keep the `# multiterminal-hook` marker the installer uses for idempotent detection).
- **Path resolution** generalizes the existing `ensureStatuslineForward`/`extractShim` logic into a reusable `resolveBundledBinary(name)`: prefer a sibling binary next to the running exe (dev / E2E layout, both in `build/bin/`); otherwise extract the embedded copy to a writable dir (`~/.claude/`) and use that (production single-portable-exe). The `go:embed` mechanism stays — it now embeds **both** binaries.
- **Migration of existing installs:** on startup, `applyStatusLine` and the hook installer overwrite the previously-registered powershell commands with the new binary commands. The obsolete `mtui-statusline.ps1` and `hook_handler.ps1` files are removed.

---

## Removals (obsolete after this change)

- `buildStatusLineScript` (ps1 generation) → rendering logic ported to the Go renderer.
- `mtui-statusline.ps1`, `hook_handler.ps1`, and their `go:embed`.
- `wrapStatuslineCommand`, `unwrapStatuslineCommand`, and their tests (no more wrapping).
- The wrapper role of `cmd/statusline-forward` (the binary is repurposed into `cmd/mtui-statusline`; the tee/passthrough of an external command is dropped).

## Kept (still needed)

- `/api/statusline` endpoint, `Session.SetStatuslineData`, the `costSource` gate on `ScanTokens`, and `ContextPct`/`Model` on the `terminal:activity` event — these are the POST target and the UI surface; unchanged.
- `go:embed` + extract mechanics (generalized to both binaries).

---

## Build pipeline

`release-alpha.yml` builds both binaries with `-ldflags -H windowsgui` into the embed location (`internal/backend/`) **before** the main `-tags production` build. Dev builds place both in `build/bin/` (sibling layout). `release.yml` (stable, Wails v2, no `production` tag) is out of scope — it does not embed, and the renderer falls back to the existing powershell only if no binary resolves (fail-safe).

---

## Testing

- **Renderer golden tests:** byte-parity of rendered output vs the current ps1 output for each template (`minimal`, `standard`) × flag combination (`ShowModel`/`ShowContext`/`ShowCost`), including the ANSI color thresholds for the context bar (≥90 red, ≥70 yellow, else green).
- **Git HEAD parsing:** temp `.git` directory for branch, detached HEAD, and worktree `.git`-file cases.
- **Hook handler:** JSONL output shape (port of the existing `app_hooks_script_test.go`).
- **Existing tests** (endpoint, session, scan/done-edge regression) remain green.
- **Subsystem guard:** a check (CI step or test) asserting the shipped binaries are GUI subsystem (PE Subsystem = 2) so a future plain `go build` cannot silently reintroduce the flash.

---

## ⚠️ Critical E2E gate (blocks the branch)

GUI-subsystem processes have no standard handles by default; they work only because the parent (Claude) creates them with inherited, redirected stdio. This must be verified at runtime, not assumed:

1. **Statusline displays:** open a Claude pane (new session, so it reads the re-registered command), confirm the status line renders normally (model + context bar + git branch) — i.e. the GUI renderer's **stdout reaches Claude**. This is the main risk: if GUI stdout does not reach Claude, the line is blank.
2. **No flash:** confirm via the process monitor that neither `mtui-statusline.exe` nor `mtui-hook.exe` is accompanied by a new `conhost.exe`, and visually that no window pops on input or per tool call.
3. **Cost captured:** confirm a non-`$0.00` cost / model appears in MTUI's pane title / footer (the POST round-trip works).
4. **Fail-silent:** stop MTUI's API; confirm the status line still renders and Claude is unaffected.

**Verify-before-commit:** the renderer's E2E (step 1) is a gate, not an afterthought — confirm GUI-subsystem stdout reaches Claude with a throwaway build before porting the full rendering logic. If it proves unreliable, the GUI-renderer approach for the statusline is not viable and we revisit (the hook binary needs no stdout, so it is unaffected and ships regardless). Do not delete the powershell statusline path until step 1 passes.

---

## Open invariant

The pipeline `active→done` edge (`app_scan.go` `DetectActivity`/`HasHookData` → `processQueue`/`notifyOrchestratorDone`) must stay byte-identical; none of these changes touch token/cost state feeding that edge.

# GUI Statusline Renderer + GUI Hook Handler Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the two PowerShell scripts MTUI registers with Claude Code (`mtui-statusline.ps1`, `hook_handler.ps1`) with GUI-subsystem Go binaries, eliminating the per-render / per-hook `conhost` window flash on Windows, and fold the existing statusline cost-capture into the renderer.

**Architecture:** Two new GUI-subsystem Go binaries — `cmd/mtui-statusline` (renders the status line + POSTs cost) and `cmd/mtui-hook` (writes the hook JSONL) — are registered directly as Claude's `statusLine.command` and hook commands. Built with `-ldflags -H windowsgui`, so Claude spawning them never allocates a console window. The renderer reads git branch directly from `.git/HEAD` (no `git` subprocess). Path resolution reuses the existing sibling-or-embed mechanism, generalized to both binaries.

**Tech Stack:** Go 1.26 (stdlib `encoding/json`, `net/http`, `os`), `go:embed`, existing localhost `/api/statusline` server.

## Global Constraints

- **Platform:** Windows (primary). Binaries must be GUI-subsystem on Windows (`-ldflags -H windowsgui`); plain build elsewhere.
- **Max 300 lines per Go file.** Split by responsibility.
- **A hook/statusline command must never block, error visibly, or flash a window.** All failures silent; render/display before any network I/O.
- **The pipeline `active→done` edge stays byte-identical** — driven by `DetectActivity`/`HasHookData` in `app_scan.go`; none of these changes touch token/cost state feeding it.
- **The renderer spawns NO subprocess** (git is read from `.git/HEAD` directly) — a subprocess would reintroduce the flash.
- **Rendered output must be byte-identical to the current `buildStatusLineScript` output** for every template × flag combination.

---

### Task 1: `resolveBundledBinary` — generalize shim path resolution

**Files:**
- Modify: `internal/backend/app_statusline_wrap.go`
- Test: `internal/backend/app_statusline_wrap_test.go` (keep the `extractShim` tests; add one for `resolveBundledBinary`)

**Interfaces:**
- Consumes: `shimBin []byte` (renamed conceptually; see Task 8 for the embed split), `os.Executable()`.
- Produces: `func resolveBundledBinary(name string, embedded []byte) string` — returns a usable absolute path (forward-slashed) to `name(.exe)`: a sibling next to the running exe if present, else the `embedded` bytes extracted to `~/.claude/<name>(.exe)`; `""` if neither resolves.

- [ ] **Step 1: Write the failing test**

```go
// add to internal/backend/app_statusline_wrap_test.go
func TestResolveBundledBinaryPrefersSibling(t *testing.T) {
	// A sibling next to the test binary's own dir is hard to fake; instead
	// assert the embed-extract fallback path is returned when no sibling and
	// embedded bytes are present.
	dst := filepath.Join(t.TempDir(), "fake-home", ".claude")
	t.Setenv("USERPROFILE", filepath.Dir(filepath.Dir(dst))) // ~ -> fake-home
	t.Setenv("HOME", filepath.Dir(filepath.Dir(dst)))
	got := resolveBundledBinary("mtui-probe", []byte("MZ-bytes"))
	if got == "" {
		t.Fatal("resolveBundledBinary returned empty with embedded bytes present")
	}
	if filepath.Base(got) != "mtui-probe.exe" && filepath.Base(got) != "mtui-probe" {
		t.Fatalf("unexpected basename: %q", got)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `go test ./internal/backend/ -run ResolveBundledBinary -v`
Expected: FAIL — `undefined: resolveBundledBinary`.

- [ ] **Step 3: Implement**

Replace the body of `app_statusline_wrap.go` so the sibling/extract logic is parameterized by name+bytes. Keep `extractShim` (unchanged signature). Replace `statuslineForwardSiblingPath`/`ensureStatuslineForward` with:

```go
// siblingBinaryPath returns the path to name(.exe) next to the running exe, or "".
func siblingBinaryPath(name string) string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if filepath.Ext(exe) == ".exe" {
		name += ".exe"
	}
	p := filepath.Join(filepath.Dir(exe), name)
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return filepath.ToSlash(p)
}

// resolveBundledBinary resolves a bundled helper binary: a sibling of the running
// exe (dev / E2E), else the embedded bytes extracted to ~/.claude (production).
// Returns "" if neither is available (caller must fail safe).
func resolveBundledBinary(name string, embedded []byte) string {
	if p := siblingBinaryPath(name); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	ext := ""
	if filepath.Ext(os.Args[0]) == ".exe" || isWindows() {
		ext = ".exe"
	}
	dst := filepath.Join(home, ".claude", name+ext)
	p, err := extractShim(dst, embedded)
	if err != nil {
		return ""
	}
	return filepath.ToSlash(p)
}
```

Add `isWindows()` helper in `app_statusline_wrap.go`:

```go
func isWindows() bool { return runtime.GOOS == "windows" }
```

(import `runtime`). Drop `wrapStatuslineCommand`/`unwrapStatuslineCommand` and their tests in Task 9 (still referenced until then — leave for now).

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/backend/ -run 'ResolveBundledBinary|ExtractShim' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/backend/app_statusline_wrap.go internal/backend/app_statusline_wrap_test.go
git commit -m "refactor(statusline): generalize shim resolution to resolveBundledBinary"
```

---

### Task 2: `mtui-hook` test + registration + build wiring

**Files:**
- Exists: `cmd/mtui-hook/main.go`
- Test: `cmd/mtui-hook/main_test.go`
- Modify: `internal/backend/app_hooks_setup.go` (command builder, ~line 43)
- Modify: `.github/workflows/release-alpha.yml` (build step)

**Interfaces:**
- Consumes: `resolveBundledBinary` (Task 1).
- Produces: hook command string `"<mtui-hook path>" <Event> # multiterminal-hook`.

- [ ] **Step 1: Write the failing test for the hook handler**

```go
// cmd/mtui-hook/main_test.go
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRunWritesJSONLLine(t *testing.T) {
	appData := t.TempDir()
	t.Setenv("APPDATA", appData)
	t.Setenv("MULTITERMINAL_SESSION_ID", "7")
	os.Args = []string{"mtui-hook", "PreToolUse"}

	// stdin: a tool-use event
	r, w, _ := os.Pipe()
	w.Write([]byte(`{"session_id":"abc","tool_name":"Bash"}`))
	w.Close()
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()

	run()

	data, err := os.ReadFile(filepath.Join(appData, "Multiterminal", "hooks", "abc.jsonl"))
	if err != nil {
		t.Fatalf("jsonl not written: %v", err)
	}
	var line struct {
		Event     string `json:"event"`
		SessionID string `json:"session_id"`
		MtID      int    `json:"mt_id"`
		Tool      string `json:"tool"`
	}
	if err := json.Unmarshal(data[:len(data)-1], &line); err != nil {
		t.Fatalf("bad jsonl: %v (%q)", err, data)
	}
	if line.Event != "PreToolUse" || line.SessionID != "abc" || line.MtID != 7 || line.Tool != "Bash" {
		t.Fatalf("unexpected line: %+v", line)
	}
}
```

- [ ] **Step 2: Run to verify it passes (logic already implemented)**

Run: `go test ./cmd/mtui-hook/ -v`
Expected: PASS (the implementation from the working tree is correct). If it fails, fix `cmd/mtui-hook/main.go` until green — do NOT change the test.

- [ ] **Step 3: Register mtui-hook instead of powershell**

In `internal/backend/app_hooks_setup.go`, replace the command builder (~line 43):

```go
	command := fmt.Sprintf(`powershell -NonInteractive -File "%s"`, scriptPath)
```

with:

```go
	// Register the GUI-subsystem hook binary directly (no powershell → no console
	// window flash). Fall back to the powershell script only if the binary cannot
	// be resolved (keeps hooks working in a misconfigured/partial build).
	hookExe := resolveBundledBinary("mtui-hook", hookBin)
	var command string
	if hookExe != "" {
		command = fmt.Sprintf(`"%s"`, hookExe)
	} else {
		command = fmt.Sprintf(`powershell -NonInteractive -File "%s"`, scriptPath)
	}
```

(`hookBin` is the embedded bytes; defined in Task 8. Until Task 8 lands, temporarily declare `var hookBin []byte` at the top of `app_hooks_setup.go` so it compiles and uses the sibling path.)

The installer already appends ` <Event> # multiterminal-hook` per event (see `app_hooks_installer.go`); no change needed there.

- [ ] **Step 4: Add the build step**

In `.github/workflows/release-alpha.yml`, before the "Build Go binary" step, extend the shim build step to also build the hook binary as GUI-subsystem:

```yaml
      - name: Build embedded helper binaries (GUI subsystem, no console flash)
        shell: bash
        run: |
          go build -ldflags "-H windowsgui" -o internal/backend/mtui-hook.exe ./cmd/mtui-hook
          go build -ldflags "-H windowsgui" -o internal/backend/mtui-statusline.exe ./cmd/mtui-statusline
```

(`mtui-statusline` exists from Task 7; if running tasks in order, this step will fail to build it until Task 7 — acceptable, the CI workflow is not exercised until release.)

Add to `.gitignore`:

```
internal/backend/mtui-hook.exe
internal/backend/mtui-statusline.exe
```

- [ ] **Step 5: Build + commit**

Run: `go build ./... && go test ./cmd/mtui-hook/ ./internal/backend/`
Expected: PASS.

```bash
git add cmd/mtui-hook/ internal/backend/app_hooks_setup.go .github/workflows/release-alpha.yml .gitignore
git commit -m "feat(hooks): GUI-subsystem mtui-hook binary replaces powershell handler"
```

---

### Task 3: GUI-stdout E2E gate (manual — BLOCKS the renderer)

**Files:** none (verification only). This proves the core assumption before any rendering work.

**Why:** A GUI-subsystem process has no std handles by default; it works only because Claude creates it with redirected stdio. If GUI-subsystem **stdout** does not reach Claude, the status line renders blank and the whole approach is void. Verify with a throwaway echo before building Tasks 4–7.

- [ ] **Step 1: Build a throwaway GUI echo**

Create a temporary `cmd/mtui-statusline/main.go` that ignores stdin and writes a fixed marker:

```go
package main

import "fmt"

func main() { fmt.Println("[GUI-PROBE] ctx 0% main") }
```

Run: `go build -ldflags "-H windowsgui" -o build/bin/mtui-statusline.exe ./cmd/mtui-statusline`

- [ ] **Step 2: Register it and verify display**

Point `~/.claude/settings.json` `statusLine.command` at `"<abs path>\build\bin\mtui-statusline.exe"`, open a NEW Claude pane, and confirm the status line shows `[GUI-PROBE] ctx 0% main` — i.e. the GUI binary's stdout reached Claude.

Also run the process monitor (`Get-CimInstance Win32_Process` polling) and confirm NO `conhost.exe` accompanies `mtui-statusline.exe`.

- [ ] **Step 3: Decide**

- Display shows the marker AND no conhost → **gate passed**, proceed to Task 4.
- Display blank → **STOP.** GUI-subsystem stdout does not reach Claude; the renderer approach is not viable. Do not delete the powershell statusline. Escalate to the human partner to revisit the design (the hook binary from Task 2 still ships).

- [ ] **Step 4: Record the result**

Append "GUI-stdout gate: PASS/FAIL on <date>" to this plan file and commit it. Then delete the throwaway `main.go` (Task 7 writes the real one).

---

### Task 4: Renderer core — `render.go` with golden parity tests

**Files:**
- Create: `cmd/mtui-statusline/render.go`
- Test: `cmd/mtui-statusline/render_test.go`

**Interfaces:**
- Produces:
  - `type RenderConfig struct { Template string; ShowModel, ShowContext, ShowCost, ShowGitBranch, ShowDuration bool }`
  - `type Status struct { Model string; ContextPct int; CostUSD float64; DurationMs int; CurrentDir string }`
  - `func Render(cfg RenderConfig, s Status, gitBranch string) string` — returns the full rendered output **including the trailing newline** that PowerShell's `Write-Host` emits. `gitBranch` is pre-resolved ("" = omit). For `extended`, returns up to two `\n`-terminated lines.

**Parity rules (from `buildStatusLineScript`):**
- Segments joined with `" | "`. `Write-Host` appends `\n`.
- model: `[<model>]`; model text = `display_name` or `?`. In `extended` line 1 it is `\x1b[36m[<model>]\x1b[0m` (cyan).
- context (minimal): `<pct>%`. context (standard/extended line 2): `<color><10-char bar><reset> <pct>%` where bar = `█`×floor(pct/10) + `░`×(10−floor(pct/10)); color = `\x1b[31m` if pct≥90, `\x1b[33m` if pct≥70, else `\x1b[32m`; reset = `\x1b[0m`.
- cost: `$` + `CostUSD` rounded to 3 decimals, fixed 3 decimals (e.g. `$0.420`). Omit if cost data absent (represent absent as a `*float64`/`hasCost` flag — see Status note).
- git: `git:<branch>` (omit if empty).
- duration: `<m>m <s>s` where m=floor(ms/60000), s=floor((ms%60000)/1000). Omit if absent.
- `minimal`: one line, segments [model?, pct%?, cost?, git?, duration?].
- `standard`: one line, segments [model?, bar+pct?, cost?, git?, duration?].
- `extended`: line 1 = [cyan model?, dir-basename(CurrentDir) if non-empty, git?]; line 2 = [bar+pct?, cost?, duration?], printed only if non-empty.

> **Status note:** absence of cost/duration must be distinguishable from zero (the ps1 checks `$null -ne`). Make `Status` carry `CostUSD *float64` and `DurationMs *int`; `ContextPct *int` too (absent → treated as 0 only where the ps1 does). Reflect this in the struct and `Render`.

- [ ] **Step 1: Write failing golden tests**

```go
// cmd/mtui-statusline/render_test.go
package main

import "testing"

func p[T any](v T) *T { return &v }

func TestRenderStandardFull(t *testing.T) {
	cfg := RenderConfig{Template: "standard", ShowModel: true, ShowContext: true, ShowCost: true, ShowGitBranch: true}
	s := Status{Model: "Opus 4.8", ContextPct: p(45), CostUSD: p(0.42), CurrentDir: "/x"}
	got := Render(cfg, s, "main")
	want := "[Opus 4.8] | \x1b[32m████░░░░░░\x1b[0m 45% | $0.420 | git:main\n"
	if got != want {
		t.Fatalf("\n got=%q\nwant=%q", got, want)
	}
}

func TestRenderMinimalNoColorNoBar(t *testing.T) {
	cfg := RenderConfig{Template: "minimal", ShowModel: true, ShowContext: true}
	s := Status{Model: "Sonnet", ContextPct: p(72)}
	got := Render(cfg, s, "")
	want := "[Sonnet] | 72%\n"
	if got != want {
		t.Fatalf("\n got=%q\nwant=%q", got, want)
	}
}

func TestRenderContextColorThresholds(t *testing.T) {
	cfg := RenderConfig{Template: "standard", ShowContext: true}
	for pct, color := range map[int]string{50: "\x1b[32m", 75: "\x1b[33m", 95: "\x1b[31m"} {
		got := Render(cfg, Status{ContextPct: p(pct)}, "")
		if got[:len(color)] != color {
			t.Fatalf("pct=%d got prefix %q want %q", pct, got[:len(color)], color)
		}
	}
}

func TestRenderModelFallbackQuestionMark(t *testing.T) {
	got := Render(RenderConfig{Template: "minimal", ShowModel: true}, Status{Model: ""}, "")
	if got != "[?]\n" {
		t.Fatalf("got %q want %q", got, "[?]\n")
	}
}

func TestRenderExtendedTwoLines(t *testing.T) {
	cfg := RenderConfig{Template: "extended", ShowModel: true, ShowContext: true, ShowGitBranch: true}
	s := Status{Model: "Opus", ContextPct: p(20), CurrentDir: "/home/u/proj"}
	got := Render(cfg, s, "dev")
	want := "\x1b[36m[Opus]\x1b[0m | proj | git:dev\n\x1b[32m██░░░░░░░░\x1b[0m 20%\n"
	if got != want {
		t.Fatalf("\n got=%q\nwant=%q", got, want)
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./cmd/mtui-statusline/ -v`
Expected: FAIL — `undefined: Render` / `RenderConfig` / `Status`.

- [ ] **Step 3: Implement `render.go`**

Implement `RenderConfig`, `Status` (`ContextPct *int`, `CostUSD *float64`, `DurationMs *int`, `Model`, `CurrentDir string`), and `Render`. Use `filepath.Base` for the extended dir segment, `strings.Repeat` for the bar, `fmt.Sprintf("$%.3f", *s.CostUSD)` for cost (Go `%.3f` round-half-to-even matches PowerShell `[Math]::Round`). Keep the file under 300 lines; if needed split bar/segment helpers into `render_segments.go`.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./cmd/mtui-statusline/ -v`
Expected: PASS (all 5).

- [ ] **Step 5: Commit**

```bash
git add cmd/mtui-statusline/render.go cmd/mtui-statusline/render_test.go
git commit -m "feat(statusline): Go renderer with byte-parity to the powershell statusline"
```

---

### Task 5: Git branch from `.git/HEAD` — `gitbranch.go`

**Files:**
- Create: `cmd/mtui-statusline/gitbranch.go`
- Test: `cmd/mtui-statusline/gitbranch_test.go`

**Interfaces:**
- Produces: `func gitBranch(startDir string) string` — walks up from `startDir` to the first `.git`, returns the branch name, or `""` (detached / not a repo / any error). Reads `.git/HEAD` directly; resolves the `gitdir:` indirection for worktrees/submodules. No subprocess.

- [ ] **Step 1: Write failing tests**

```go
// cmd/mtui-statusline/gitbranch_test.go
package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitBranchReadsHEAD(t *testing.T) {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, ".git"), 0755)
	os.WriteFile(filepath.Join(d, ".git", "HEAD"), []byte("ref: refs/heads/feature/x\n"), 0644)
	sub := filepath.Join(d, "a", "b")
	os.MkdirAll(sub, 0755)
	if got := gitBranch(sub); got != "feature/x" {
		t.Fatalf("got %q want feature/x", got)
	}
}

func TestGitBranchDetachedReturnsEmpty(t *testing.T) {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, ".git"), 0755)
	os.WriteFile(filepath.Join(d, ".git", "HEAD"), []byte("a1b2c3d4e5\n"), 0644)
	if got := gitBranch(d); got != "" {
		t.Fatalf("got %q want empty (detached)", got)
	}
}

func TestGitBranchWorktreeFile(t *testing.T) {
	d := t.TempDir()
	real := filepath.Join(d, "realgit")
	os.MkdirAll(real, 0755)
	os.WriteFile(filepath.Join(real, "HEAD"), []byte("ref: refs/heads/wt\n"), 0644)
	wt := filepath.Join(d, "wt")
	os.MkdirAll(wt, 0755)
	os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: "+real+"\n"), 0644)
	if got := gitBranch(wt); got != "wt" {
		t.Fatalf("got %q want wt", got)
	}
}

func TestGitBranchNoRepo(t *testing.T) {
	if got := gitBranch(t.TempDir()); got != "" {
		t.Fatalf("got %q want empty", got)
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./cmd/mtui-statusline/ -run GitBranch -v`
Expected: FAIL — `undefined: gitBranch`.

- [ ] **Step 3: Implement `gitbranch.go`**

Walk up from `startDir` checking for `.git`. If `.git` is a dir, read `.git/HEAD`. If a file, parse `gitdir: <path>` and read `<path>/HEAD` (resolve relative to the `.git` file's dir). Parse `ref: refs/heads/<branch>` → `<branch>`; anything else → `""`. All errors → `""`.

- [ ] **Step 4: Run to verify pass**

Run: `go test ./cmd/mtui-statusline/ -run GitBranch -v`
Expected: PASS (4).

- [ ] **Step 5: Commit**

```bash
git add cmd/mtui-statusline/gitbranch.go cmd/mtui-statusline/gitbranch_test.go
git commit -m "feat(statusline): read git branch from .git/HEAD (no subprocess)"
```

---

### Task 6: Cost POST — `post.go`

**Files:**
- Create: `cmd/mtui-statusline/post.go`
- Test: `cmd/mtui-statusline/post_test.go`

**Interfaces:**
- Produces: `func postCapture(rawJSON []byte, port, sid string)` — fire-and-forget POST of `{"sessionId":<int>,"payload":<rawJSON>}` to `http://127.0.0.1:<port>/api/statusline`; no-op if port/sid empty or sid non-numeric; silent on any error; short timeout. (Ported verbatim from `cmd/statusline-forward/main.go` `forward`.)

- [ ] **Step 1: Write failing test** (reuse the proven `statusline-forward` tests):

```go
// cmd/mtui-statusline/post_test.go
package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostCaptureSendsSessionAndPayload(t *testing.T) {
	type got struct {
		SessionID int             `json:"sessionId"`
		Payload   json.RawMessage `json:"payload"`
	}
	ch := make(chan got, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var g got
		_ = json.NewDecoder(r.Body).Decode(&g)
		ch <- g
	}))
	defer srv.Close()
	port := strings.TrimPrefix(srv.URL, "http://127.0.0.1:")
	postCapture([]byte(`{"cost":{"total_cost_usd":0.42}}`), port, "7")
	g := <-ch
	if g.SessionID != 7 || !strings.Contains(string(g.Payload), "0.42") {
		t.Fatalf("bad post: %+v", g)
	}
}

func TestPostCaptureNoEnvIsNoop(t *testing.T) { postCapture([]byte(`{}`), "", "") }
func TestPostCaptureDeadPortIsSilent(t *testing.T) { postCapture([]byte(`{}`), "1", "7") }
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./cmd/mtui-statusline/ -run PostCapture -v`
Expected: FAIL — `undefined: postCapture`.

- [ ] **Step 3: Implement `post.go`** (copy `forward` from `cmd/statusline-forward/main.go`, rename to `postCapture`).

- [ ] **Step 4: Run to verify pass**

Run: `go test ./cmd/mtui-statusline/ -run PostCapture -v`
Expected: PASS (3).

- [ ] **Step 5: Commit**

```bash
git add cmd/mtui-statusline/post.go cmd/mtui-statusline/post_test.go
git commit -m "feat(statusline): cost-capture POST in the renderer binary"
```

---

### Task 7: Renderer `main.go` — wire stdin → render → stdout → POST

**Files:**
- Create/replace: `cmd/mtui-statusline/main.go` (replaces the Task 3 throwaway)
- Test: `cmd/mtui-statusline/main_test.go`

**Interfaces:**
- Consumes: `Render`, `gitBranch`, `postCapture`, flag parsing.
- Produces: a binary that reads Claude's JSON on stdin, parses flags (`--template`, `--model`, `--context`, `--cost`, `--git`, `--duration`), renders to stdout, then POSTs. `func parseStatus(raw []byte) Status` (testable).

- [ ] **Step 1: Write failing test for parse + flag mapping**

```go
// cmd/mtui-statusline/main_test.go
package main

import "testing"

func TestParseStatusExtractsFields(t *testing.T) {
	raw := []byte(`{"model":{"display_name":"Opus 4.8"},"context_window":{"used_percentage":45.7},"cost":{"total_cost_usd":0.42,"total_duration_ms":65000},"workspace":{"current_dir":"/p"}}`)
	s := parseStatus(raw)
	if s.Model != "Opus 4.8" {
		t.Fatalf("model %q", s.Model)
	}
	if s.ContextPct == nil || *s.ContextPct != 46 { // [int] rounds 45.7 -> 46
		t.Fatalf("ctx %v want 46", s.ContextPct)
	}
	if s.CostUSD == nil || *s.CostUSD != 0.42 {
		t.Fatalf("cost %v", s.CostUSD)
	}
	if s.DurationMs == nil || *s.DurationMs != 65000 {
		t.Fatalf("dur %v", s.DurationMs)
	}
	if s.CurrentDir != "/p" {
		t.Fatalf("dir %q", s.CurrentDir)
	}
}

func TestParseStatusMissingFieldsAreNil(t *testing.T) {
	s := parseStatus([]byte(`{}`))
	if s.ContextPct != nil || s.CostUSD != nil || s.DurationMs != nil {
		t.Fatalf("expected nil absent fields: %+v", s)
	}
}
```

> **Rounding note:** PowerShell `[int]` is round-half-to-even. Go: round used_percentage with `math.RoundToEven`. Implement `parseStatus` accordingly so 45.7→46 and 44.5→44.

- [ ] **Step 2: Run to verify fail**

Run: `go test ./cmd/mtui-statusline/ -run ParseStatus -v`
Expected: FAIL — `undefined: parseStatus`.

- [ ] **Step 3: Implement `main.go`**

```go
package main

import (
	"flag"
	"encoding/json"
	"io"
	"math"
	"os"
)

func main() {
	defer func() { _ = recover() }()
	cfg := RenderConfig{}
	flag.StringVar(&cfg.Template, "template", "standard", "")
	flag.BoolVar(&cfg.ShowModel, "model", false, "")
	flag.BoolVar(&cfg.ShowContext, "context", false, "")
	flag.BoolVar(&cfg.ShowCost, "cost", false, "")
	flag.BoolVar(&cfg.ShowGitBranch, "git", false, "")
	flag.BoolVar(&cfg.ShowDuration, "duration", false, "")
	flag.Parse()

	raw, _ := io.ReadAll(os.Stdin)
	s := parseStatus(raw)
	branch := ""
	if cfg.ShowGitBranch {
		branch = gitBranch(s.CurrentDir)
	}
	// Display FIRST (never blocked by the POST).
	os.Stdout.WriteString(Render(cfg, s, branch))
	postCapture(raw, os.Getenv("MTUI_PORT"), os.Getenv("MULTITERMINAL_SESSION_ID"))
}

func parseStatus(raw []byte) Status {
	var d struct {
		Model struct{ DisplayName string `json:"display_name"` } `json:"model"`
		Context struct{ UsedPercentage *float64 `json:"used_percentage"` } `json:"context_window"`
		Cost struct {
			TotalCostUSD  *float64 `json:"total_cost_usd"`
			TotalDuration *int     `json:"total_duration_ms"`
		} `json:"cost"`
		Workspace struct{ CurrentDir string `json:"current_dir"` } `json:"workspace"`
	}
	_ = json.Unmarshal(raw, &d)
	s := Status{Model: d.Model.DisplayName, CostUSD: d.Cost.TotalCostUSD, DurationMs: d.Cost.TotalDuration, CurrentDir: d.Workspace.CurrentDir}
	if d.Context.UsedPercentage != nil {
		v := int(math.RoundToEven(*d.Context.UsedPercentage))
		s.ContextPct = &v
	}
	return s
}
```

- [ ] **Step 4: Run all renderer tests + build GUI**

Run: `go test ./cmd/mtui-statusline/... && go build -ldflags "-H windowsgui" -o build/bin/mtui-statusline.exe ./cmd/mtui-statusline`
Expected: PASS; build succeeds.

- [ ] **Step 5: Commit**

```bash
git add cmd/mtui-statusline/main.go cmd/mtui-statusline/main_test.go
git commit -m "feat(statusline): renderer main — stdin→render→stdout→capture POST"
```

---

### Task 8: Register the renderer + embed both binaries

**Files:**
- Modify: `internal/backend/app_statusline.go` (`applyStatusLine` — build flags, register binary; drop ps1 write)
- Modify: `internal/backend/statusline_shim_embed.go` / `statusline_shim_noembed.go` (embed both binaries)
- Modify: `internal/backend/app_hooks_setup.go` (use real `hookBin` from Task 8 embed)
- Test: `internal/backend/app_statusline_test.go` (flag-string builder)

**Interfaces:**
- Consumes: `resolveBundledBinary` (Task 1), embedded `statuslineBin`, `hookBin`.
- Produces: `func statuslineRenderFlags(cfg config.StatusLineSettings) string` — `--template <t>` plus `--model`/`--context`/`--cost`/`--git`/`--duration` for each enabled flag.

- [ ] **Step 1: Write failing test for the flag builder**

```go
// internal/backend/app_statusline_test.go (add)
func TestStatuslineRenderFlags(t *testing.T) {
	cfg := config.StatusLineSettings{Template: "standard", ShowModel: true, ShowContext: true, ShowGitBranch: true}
	got := statuslineRenderFlags(cfg)
	want := "--template standard --model --context --git"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
```

- [ ] **Step 2: Run to verify fail**

Run: `go test ./internal/backend/ -run StatuslineRenderFlags -v`
Expected: FAIL — `undefined: statuslineRenderFlags`.

- [ ] **Step 3: Implement the flag builder + rewrite `applyStatusLine`**

Add `statuslineRenderFlags`. Rewrite `applyStatusLine` to: resolve `resolveBundledBinary("mtui-statusline", statuslineBin)`; if found, register `"<exe>" <flags>`; else (fail-safe) keep writing + registering the existing powershell script. Remove the unconditional ps1 write when the binary resolves; remove the wrap call.

- [ ] **Step 4: Embed both binaries**

Rename `shimBin` → split into two embeds. In `statusline_shim_embed.go` (`//go:build production`):

```go
//go:embed mtui-statusline.exe
var statuslineBin []byte

//go:embed mtui-hook.exe
var hookBin []byte
```

In `statusline_shim_noembed.go` (`//go:build !production`):

```go
var statuslineBin []byte
var hookBin []byte
```

Remove the temporary `var hookBin []byte` added in Task 2. Delete the old `//go:embed statusline-forward.exe` line.

- [ ] **Step 5: Build (both tag paths) + test**

Run:
```bash
go build ./... && go vet ./internal/backend/
go build -ldflags "-H windowsgui" -o internal/backend/mtui-statusline.exe ./cmd/mtui-statusline
go build -ldflags "-H windowsgui" -o internal/backend/mtui-hook.exe ./cmd/mtui-hook
go build -tags production ./... && rm -f internal/backend/mtui-statusline.exe internal/backend/mtui-hook.exe
go test ./internal/backend/...
```
Expected: all PASS; production build embeds both.

- [ ] **Step 6: Commit**

```bash
git add internal/backend/app_statusline.go internal/backend/app_statusline_test.go internal/backend/statusline_shim_embed.go internal/backend/statusline_shim_noembed.go internal/backend/app_hooks_setup.go
git commit -m "feat(statusline): register GUI renderer directly; embed both helper binaries"
```

---

### Task 9: Remove obsolete powershell-generation and wrap machinery

**Files:**
- Modify: `internal/backend/app_statusline.go` (delete `buildStatusLineScript` + the `build*Script` helpers, ps1 path write, ps1 removal-on-cleanup adjustments)
- Modify: `internal/backend/app_statusline_wrap.go` (delete `wrapStatuslineCommand`, `unwrapStatuslineCommand`)
- Delete: `internal/backend/app_statusline_wrap_test.go` wrap/unwrap tests (keep `extractShim`/`resolveBundledBinary`)
- Delete: `cmd/statusline-forward/` (superseded by `cmd/mtui-statusline`)
- Delete: `internal/backend/hooks/hook_handler.ps1` + adjust `internal/backend/hooks/embed.go`; `cmd/mtui-hook` is the handler now
- Modify: migration — `applyStatusLine`/hook setup remove stale `mtui-statusline.ps1` / `hook_handler.ps1` files on startup

**Interfaces:** removes symbols; confirm no remaining references.

- [ ] **Step 1: Find references before deleting**

Run: `grep -rn "buildStatusLineScript\|wrapStatuslineCommand\|unwrapStatuslineCommand\|statusline-forward\|HookHandlerScript\|buildMinimalScript\|buildStandardScript\|buildExtendedScript" --include='*.go' .`
Expected: references only in the files this task edits/deletes (and their tests).

- [ ] **Step 2: Delete + adjust**

Remove the listed functions/files. For `hooks/embed.go`: if `HookHandlerScript` is now unused, delete the embed and the `.ps1`. Add startup cleanup: `os.Remove(statusLineScriptPath())` and the old hook script path after successful binary registration (best-effort, ignore errors).

- [ ] **Step 3: Build + full test + vet**

Run: `go build ./... && go test ./... && go vet ./...`
Expected: PASS, no undefined references, no dead-code vet errors.

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor(statusline): drop powershell statusline/hook generation + wrap machinery"
```

---

### Task 10: Final E2E verification gate (manual)

**Files:** none.

- [ ] **Step 1: Build dev binaries**

Run:
```bash
cd frontend && npm run build && cd ..
go build -tags desktop -o build/bin/multiterminal.exe .
go build -ldflags "-H windowsgui" -o build/bin/mtui-statusline.exe ./cmd/mtui-statusline
go build -ldflags "-H windowsgui" -o build/bin/mtui-hook.exe ./cmd/mtui-hook
```
Run `build/bin/multiterminal.exe` (close other MTUI instances first to avoid the shared-`settings.json` race).

- [ ] **Step 2: Verify (all four)**

1. **No flash:** open a Claude pane, submit input that triggers tools — no console window pops (statusline render nor any hook). Confirm via the process monitor that neither `mtui-statusline.exe` nor `mtui-hook.exe` has an accompanying `conhost.exe`.
2. **Statusline displays** the model + colored context bar + git branch, matching the previous look.
3. **Cost captured:** pane title / footer shows a non-`$0.00` cost and model (the `/api/statusline` round-trip works; check the `[statusline] session N cost=...` log line).
4. **Fail-silent:** stop MTUI's API; the status line still renders, Claude unaffected.

- [ ] **Step 3: Record + commit the note**

Append "E2E verified on <date>: no flash, statusline renders, cost captured, fail-silent" to this plan. Commit.

---

## Self-Review

**Spec coverage:**
- GUI renderer replaces ps1 + forwarder, POSTs cost → Tasks 4–8. ✓
- Git from `.git/HEAD`, no subprocess → Task 5. ✓
- `mtui-hook` GUI binary replaces hook ps1 → Task 2. ✓
- Direct registration, no powershell/wrapper → Task 8. ✓
- Generalized path resolution (sibling/embed) → Task 1. ✓
- Embed both binaries; build pipeline → Tasks 2, 8. ✓
- Removals (ps1 gen, wrap/unwrap, statusline-forward, hook ps1) → Task 9. ✓
- Migration (overwrite old commands, delete stale ps1) → Tasks 2, 8, 9. ✓
- Byte-parity rendering (3 templates, flags, colors) → Task 4. ✓
- **Critical GUI-stdout gate before porting** → Task 3. ✓
- Final runtime E2E (display/flash/cost/fail-silent) → Task 10. ✓
- `done`-edge invariant untouched → Global Constraints; no task alters scan/queue state. ✓

**Placeholder scan:** every code step has complete code; every command has expected output; no TBD/TODO. ✓

**Type consistency:** `RenderConfig`/`Status`/`Render(cfg,s,gitBranch)` used identically across Tasks 4 and 7; `resolveBundledBinary(name, embedded)` defined Task 1, used Tasks 2 & 8; `statuslineBin`/`hookBin` embeds defined Task 8, the temporary `hookBin` stub in Task 2 is removed in Task 8 Step 4; `postCapture`/`gitBranch`/`parseStatus` consistent. ✓

# Leitstand, Teil 1: Aufmerksamkeit — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Kein Pane, das auf Patrick wartet, fehlt mehr: ein Aufmerksamkeitsmodell auf dem Host, der Kasten „Wartet auf dich“ im Fokus-Layout als Leitstand über alle Projekte, Pane-Zeichen in der Tab-Leiste, Zähler, Toast mit Sprung und Taskleisten-Zahl.

**Architecture:** Der Host (`internal/hub`) führt je Session einen Attention-Datensatz (Zustand, seit, gesehen, Kurztext, Quelle, Rang) neben dem Debounce und meldet jede Änderung als `EventSessionAttention`. Das Backend reicht ihn als `terminal:attention` weiter und nimmt „gesehen“ über `MarkSessionSeen` zurück. Das Frontend rechnet nichts nach: Es sortiert nach dem Rang des Hosts und nimmt je Tab den kleinsten.

**Tech Stack:** Go 1.25 (hub, backend, mtui-hook), Wails v3 beta.25, Svelte 5 im Legacy-Syntax, Vitest, Windows-COM per `syscall` für die Taskleiste.

**Spec:** `docs/superpowers/specs/2026-10-07-leitstand-aufmerksamkeit-design.md` (freigegeben am 2026-10-07). Prototyp: `docs/superpowers/specs/assets/2026-10-07-leitstand-prototyp.html`.

**Status:** Entwurf zur Durchsicht
**Worktree:** C:\Users\Patrick.Hennig\repos\Multiterminal-UI\.claude\worktrees\leitstand-aufmerksamkeit

## Global Constraints

- Höchstens 300 Zeilen je Go-Produktionsdatei. `hub/server.go` (484) und `backend/app.go` stehen auf der Schuldenliste und dürfen **nicht wachsen**.
- Go-Structs, die das Frontend sieht, tragen `json`- **und** `yaml`-Tags.
- UI-Texte Deutsch, Code und Kommentare Englisch.
- Keine Produktionsdatei in `internal/backend` importiert `internal/terminal` (Prüfung: `grep -rl internal/terminal internal/backend --include='*.go' | grep -v _test.go` muss leer sein).
- Ein neues Binding nimmt einen `hub.Ref` und übersetzt mit `a.local(ref)`. Jedes neue Binding steht von Hand in `frontend/wailsjs/go/backend/App.js` **und** `App.d.ts`; `go test ./internal/tsmodels/` prüft die FNV-1a-IDs.
- Svelte: In keinem `$:`-Block direkt Variablen zuweisen, die er auch liest; stattdessen eine Funktion aufrufen. `$: x = f(a, b)` als reine Ableitung ist erlaubt.
- Statusfarbe „fertig“: `--status-done: #4f9cf0`, `--status-done-tint: rgba(79,156,240,.16)`, in der theme-übergreifenden `SEMANTIC`-Tabelle.
- Rangfolge (einzige Regel, `hub.AttentionRank`): 0 Freigabe, 1 Frage, 2 Fehler ungesehen, 3 fertig ungesehen, 4 läuft, 5 Rest. Gesehen räumt nur 2 und 3 ab.
- Kurztext höchstens 200 Zeichen (Runen), Whitespace zusammengefasst.
- „Gesehen“: Fenster fokussiert und Pane 1000 ms mit Tastaturfokus, oder Eingabe ins Pane.
- „Hängt“: nur Hook-Sessions, Zustand `active`, 10 Minuten ohne Ausgabe **und** ohne Hook.
- Commits: Conventional Commits auf Englisch, letzte Zeile `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Kein Push, kein PR ohne Rückfrage.

## Review Focus

1. **Ein Pane, das Patrick selbst schließt, darf keinen Fehler-Toast auslösen.** Close tötet den Prozess, und dessen Exit-Code ist ungleich 0. Erwartet: kein `error`-Datensatz für diese Session. Test in Task 5 (`TestEmbedded_CloseIsNotAnError`).
2. **Ein älterer Daemon schickt keinen Attention-Datensatz.** Ein Null-`Attention` hat `Rank` 0, und das hieße „Freigabe“. Erwartet: Jeder Leser rechnet den Rang mit `AttentionRank(State, Seen)` neu, und der leere Zustand ist ruhig. Tests in Task 1 (`AttentionRank("")`) und Task 7 (`attentionInfo` rechnet neu). Im Frontend gilt bei fehlendem Datensatz „fertig“ als gesehen (Task 10, `paneRank`).
3. **Viele Panes in einem Tab** (6 und mehr) dürfen die Tab-Leiste nicht sprengen. Erwartet: höchstens sechs Zeichen plus „+N“. Test in Task 10 (`tabGlyphs`).
4. **Das Pane, das Patrick gerade ansieht, wird fertig.** Erwartet: Nach einer Sekunde gilt es als gesehen, ohne neuen Fokuswechsel. Test in Task 11 (`look` mit derselben Ref und neuem Rang).
5. **Ctrl+Shift+J landet auf einem Eintrag des eigenen Tabs („hier“).** Eine Vorschau über dem eigenen Tab ergibt keinen Sinn. Erwartet: Das Pane bekommt den Fokus in seinem Slot. Test in Task 12 (`preview` mit `here`).

---

## Dateiübersicht

**Neu**
- `internal/hub/attention.go`: Attention-Typ, `AttentionRank`, `attentionBook`, `clipDetail`
- `internal/hub/embedded_attention.go`: `MarkSeen`/`MarkUnseen`, `recordConfirmed`
- `internal/hub/hook_detail.go`: `paragraphs`, `stopDetail`, `questionSentence`, `hookDetail`
- `internal/hub/permission_dialog.go`: Erkennung des Freigabedialogs am Bildschirm
- `internal/hub/server_activity.go`: Routen `activity` (verschoben) und `seen`
- `cmd/mtui-hook/detail.go`: Kurztext aus `tool_input`
- `internal/backend/app_attention.go`: `terminal:attention`, `MarkSessionSeen`/`MarkSessionUnseen`
- `internal/backend/taskbar_badge.go`, `taskbar_badge_windows.go`, `taskbar_badge_other.go`
- `frontend/src/lib/attention.ts`, `seenTracker.ts`, `attentionNav.ts`, `projectTabs.ts`
- `frontend/src/components/StatusGlyph.svelte`, `WaitingPanel.svelte`, `AttentionMenu.svelte`

**Geändert:** `internal/hub/{types,events,host,embedded,embedded_scan,embedded_hooks,embedded_stream,hook_activity,client_screen,server}.go`, `internal/hooks/watcher.go`, `cmd/mtui-hook/main.go`, `cmd/mt/cmd_list.go`, `main.go`, `internal/backend/{app_host_stream,app_activity_resend,app_dashboard,app_agent_panes,app_notify,notify_toast_windows,notify_toast_other}.go`, `frontend/wailsjs/go/backend/{App.js,App.d.ts}`, `frontend/src/stores/{tabs,theme}.ts`, `frontend/src/lib/{focusLayout,shortcuts}.ts`, `frontend/src/App.svelte`, `frontend/src/components/{PaneGrid,TabBar,Toolbar,TerminalPane,PaneTitlebar}.svelte`, `CLAUDE.md`, Spec.

---

### Task 1: Das Aufmerksamkeitsmodell im Hub

**Files:**
- Create: `internal/hub/attention.go`
- Test: `internal/hub/attention_test.go`

**Interfaces:**
- Produces: `type Attention struct{State Activity; Since time.Time; Seen bool; Detail, Source string; Rank int}`, Konstanten `RankPermission … RankQuiet`, `func AttentionRank(state Activity, seen bool) int`, `func clipDetail(string) string`, `type attentionBook` mit `newAttentionBook()`, `noteHook(id int, state Activity, event, detail string)`, `noteFact(id int, state Activity, source, detail string)`, `confirmed(id int, state Activity, since time.Time) (Attention, bool)`, `markSeen(id int, seen bool) (Attention, bool)`, `get(id int) Attention`, `forget(id int)`.

- [ ] **Step 1: Write the failing tests**

```go
package hub

import (
	"strings"
	"testing"
	"time"
)

func TestAttentionRank_Order(t *testing.T) {
	cases := []struct {
		state Activity
		seen  bool
		want  int
	}{
		{ActivityWaitingPermission, false, RankPermission},
		{ActivityWaitingPermission, true, RankPermission},
		{ActivityWaitingAnswer, true, RankQuestion},
		{ActivityError, false, RankError},
		{ActivityError, true, RankQuiet},
		{ActivityDone, false, RankDone},
		{ActivityDone, true, RankQuiet},
		{ActivityActive, false, RankWorking},
		{ActivityIdle, false, RankQuiet},
		{ActivitySleeping, false, RankQuiet},
		// A daemon from before attention sends a zero record: it must be quiet,
		// not rank 0, which would read as a permission request.
		{"", false, RankQuiet},
	}
	for _, c := range cases {
		if got := AttentionRank(c.state, c.seen); got != c.want {
			t.Errorf("AttentionRank(%q, %v) = %d, want %d", c.state, c.seen, got, c.want)
		}
	}
}

func TestAttentionBook_NewStateCarriesTheHookDetail(t *testing.T) {
	b := newAttentionBook()
	b.noteHook(1, ActivityWaitingPermission, "PermissionRequest", "Bash: npm install")
	at := time.Unix(1000, 0)
	a, ok := b.confirmed(1, ActivityWaitingPermission, at)
	if !ok || a.Seen || a.Detail != "Bash: npm install" || a.Source != "Hook: PermissionRequest" ||
		a.Rank != RankPermission || !a.Since.Equal(at) {
		t.Fatalf("got %+v, %v", a, ok)
	}
}

func TestAttentionBook_SeenClearsDoneButNotAPermission(t *testing.T) {
	b := newAttentionBook()
	b.confirmed(1, ActivityDone, time.Now())
	if a, changed := b.markSeen(1, true); !changed || a.Rank != RankQuiet {
		t.Errorf("done after seen: %+v changed=%v, want quiet", a, changed)
	}
	b.confirmed(2, ActivityWaitingPermission, time.Now())
	if a, _ := b.markSeen(2, true); a.Rank != RankPermission {
		t.Errorf("permission after seen: rank %d, want %d", a.Rank, RankPermission)
	}
}

func TestAttentionBook_ANewStateIsUnseenAgain(t *testing.T) {
	b := newAttentionBook()
	b.confirmed(1, ActivityDone, time.Now())
	b.markSeen(1, true)
	b.confirmed(1, ActivityActive, time.Now())
	a, _ := b.confirmed(1, ActivityDone, time.Now())
	if a.Seen || a.Rank != RankDone {
		t.Errorf("second done: %+v, want unseen rank %d", a, RankDone)
	}
}

func TestAttentionBook_SleepKeepsTheRecord(t *testing.T) {
	b := newAttentionBook()
	b.confirmed(1, ActivityDone, time.Now())
	if _, changed := b.confirmed(1, ActivitySleeping, time.Now()); changed {
		t.Error("sleeping replaced the record")
	}
	if a := b.get(1); a.Rank != RankDone {
		t.Errorf("after sleep: rank %d, want %d", a.Rank, RankDone)
	}
}

func TestAttentionBook_NotificationKeepsThePermissionDetail(t *testing.T) {
	b := newAttentionBook()
	b.noteHook(1, ActivityWaitingPermission, "PermissionRequest", "Bash: rm -rf build")
	b.noteHook(1, ActivityWaitingPermission, "Notification", "Claude needs your permission to use Bash")
	a, _ := b.confirmed(1, ActivityWaitingPermission, time.Now())
	if a.Detail != "Bash: rm -rf build" {
		t.Errorf("detail = %q, want the command", a.Detail)
	}
}

func TestAttentionBook_ScreenQuestionAfterStopKeepsTheText(t *testing.T) {
	b := newAttentionBook()
	b.noteHook(1, ActivityDone, "Stop", "Alles grün. Was liegt an?")
	a, _ := b.confirmed(1, ActivityWaitingAnswer, time.Now())
	if a.Detail != "Alles grün. Was liegt an?" || !strings.HasPrefix(a.Source, "Bildschirm-Scan nach Hook: Stop") {
		t.Errorf("got %+v", a)
	}
}

func TestAttentionBook_UnknownSessionIsQuiet(t *testing.T) {
	if a := newAttentionBook().get(42); a.Rank != RankQuiet || a.State != "" {
		t.Errorf("got %+v", a)
	}
}

func TestClipDetail_CollapsesAndCuts(t *testing.T) {
	if got := clipDetail("  a \n\n b\tc  "); got != "a b c" {
		t.Errorf("collapse: %q", got)
	}
	long := strings.Repeat("ä", 250)
	got := clipDetail(long)
	if n := len([]rune(got)); n != detailMax || !strings.HasSuffix(got, "…") {
		t.Errorf("cut: %d runes, suffix %q", n, got[len(got)-3:])
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/hub/ -run 'TestAttention|TestClipDetail' -count=1`
Expected: FAIL, `undefined: AttentionRank` u. a.

- [ ] **Step 3: Write the implementation**

```go
package hub

import (
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// What a session wants from the person at the screen.
//
// The confirmed state says what the agent is doing. Attention adds whether the
// person has seen it, what the agent wants (the detail) and where the state
// came from. It lives on the host next to the debounce, for the same reason
// the debounce does: the daemon, `mt ls` and the MCP server need the same
// answer as the window, and one rank function is what keeps the badge, the
// list and the counter from disagreeing (spec 2026-10-07, Teil A).

// Attention is one session's attention record.
type Attention struct {
	// State is the last confirmed state that is not a lifecycle label: a
	// suspend does not turn "done, unseen" into nothing.
	State Activity  `json:"state" yaml:"state"`
	Since time.Time `json:"since" yaml:"since"`
	// Seen is set when the person looked at the pane (MarkSeen) and cleared
	// by every newly confirmed state.
	Seen   bool   `json:"seen" yaml:"seen"`
	Detail string `json:"detail" yaml:"detail"`
	Source string `json:"source" yaml:"source"`
	// Rank is AttentionRank(State, Seen), carried along for convenience. A
	// reader that may face an older peer recomputes it instead.
	Rank int `json:"rank" yaml:"rank"`
}

// Ranks, most urgent first. Everything up to RankDone is listed as waiting.
const (
	RankPermission = 0
	RankQuestion   = 1
	RankError      = 2
	RankDone       = 3
	RankWorking    = 4
	RankQuiet      = 5
)

// AttentionRank is the only place the urgency order is decided. Seeing a pane
// clears "done" and "error" but not a permission or a question: those block
// the agent until it moves on, and looking does not answer them.
func AttentionRank(state Activity, seen bool) int {
	switch state {
	case ActivityWaitingPermission:
		return RankPermission
	case ActivityWaitingAnswer:
		return RankQuestion
	case ActivityError:
		if seen {
			return RankQuiet
		}
		return RankError
	case ActivityDone:
		if seen {
			return RankQuiet
		}
		return RankDone
	case ActivityActive:
		return RankWorking
	default:
		return RankQuiet
	}
}

// detailMax bounds a detail, in runes. The list shows one line of it.
const detailMax = 200

// clipDetail collapses whitespace and cuts the text to detailMax runes.
func clipDetail(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= detailMax {
		return s
	}
	r := []rune(s)
	return string(r[:detailMax-1]) + "…"
}

// attentionNote is what the latest report said about the state it set.
type attentionNote struct {
	state  Activity
	source string
	detail string
}

// attentionBook holds the per-session records. It has its own mutex, like the
// debouncer: the scan, the hook reader and client calls all write to it, and
// none of them may wait on the host's session map.
type attentionBook struct {
	mu      sync.Mutex
	entries map[int]Attention
	notes   map[int]attentionNote
}

func newAttentionBook() *attentionBook {
	return &attentionBook{entries: map[int]Attention{}, notes: map[int]attentionNote{}}
}

// noteHook records what a lifecycle hook said. A Notification for the state
// already noted does not replace a detail: PermissionRequest carries the
// command, the Notification six seconds later only "Claude needs your
// permission".
func (b *attentionBook) noteHook(id int, state Activity, event, detail string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if prev, ok := b.notes[id]; ok && event == "Notification" && prev.state == state && prev.detail != "" {
		return
	}
	source := "Hook: " + event
	if event == "Stop" && state == ActivityWaitingAnswer {
		source = "Hook: Stop, Frage im letzten Absatz"
	}
	b.notes[id] = attentionNote{state: state, source: source, detail: clipDetail(detail)}
}

// noteFact records a state the host worked out itself: an exit, a hang.
func (b *attentionBook) noteFact(id int, state Activity, source, detail string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.notes[id] = attentionNote{state: state, source: source, detail: clipDetail(detail)}
}

// confirmed records a newly confirmed state and returns the new record. It
// reports false for the lifecycle labels, which leave the record alone: a
// pane put to sleep while "done, unseen" is still done and unseen.
func (b *attentionBook) confirmed(id int, state Activity, since time.Time) (Attention, bool) {
	if state == "" || state == ActivitySleeping || state == ActivityResuming {
		return Attention{}, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	a := Attention{State: state, Since: since, Source: "Bildschirm-Scan"}
	if n, ok := b.notes[id]; ok {
		switch {
		case n.state == state:
			a.Source, a.Detail = n.source, n.detail
		case state == ActivityWaitingAnswer && n.state == ActivityDone:
			// The screen saw a question the Stop text did not end with.
			a.Source, a.Detail = "Bildschirm-Scan nach "+n.source, n.detail
		}
	}
	a.Rank = AttentionRank(a.State, false)
	b.entries[id] = a
	return a, true
}

// markSeen sets or clears the seen flag and reports whether it changed.
func (b *attentionBook) markSeen(id int, seen bool) (Attention, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	a, ok := b.entries[id]
	if !ok || a.Seen == seen {
		return a, false
	}
	a.Seen = seen
	a.Rank = AttentionRank(a.State, seen)
	b.entries[id] = a
	return a, true
}

// get returns the record, or a quiet one for a session that has none yet.
func (b *attentionBook) get(id int) Attention {
	b.mu.Lock()
	defer b.mu.Unlock()
	if a, ok := b.entries[id]; ok {
		return a
	}
	return Attention{Rank: RankQuiet}
}

// forget drops a closed session's record.
func (b *attentionBook) forget(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.entries, id)
	delete(b.notes, id)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/hub/ -run 'TestAttention|TestClipDetail' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/hub/attention.go internal/hub/attention_test.go
git commit -m "feat(hub): attention record and the one rank rule" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Kurztexte aus Hook-Ereignissen

**Files:**
- Create: `internal/hub/hook_detail.go`
- Modify: `internal/hub/hook_activity.go` (die Absatzlogik in `endsWithQuestion` wird zu `paragraphs`)
- Modify: `internal/hooks/watcher.go:24-39` (zwei Felder in `Event`)
- Test: `internal/hub/hook_detail_test.go`

**Interfaces:**
- Consumes: `clipDetail` (Task 1)
- Produces: `func paragraphs(msg string) []string`, `func stopDetail(msg string) string`, `func questionSentence(para string) string`, `func hookDetail(ev hooks.Event) string`; neue Felder `hooks.Event.Detail string` (`json:"detail"`), `hooks.Event.AgentID string` (`json:"agent_id"`).

- [ ] **Step 1: Add the two event fields**

In `internal/hooks/watcher.go`, im Struct `Event` nach `NotificationType`:

```go
	// Detail is what the agent is about to do, as mtui-hook read it from
	// tool_input: "Bash: npm install", "Edit: src/app.ts", the question of an
	// AskUserQuestion. Empty for events without a tool.
	Detail string `json:"detail"`
	// AgentID is set when the event comes from a subagent, not from the main
	// conversation (Claude Code's agent_id).
	AgentID string `json:"agent_id"`
```

- [ ] **Step 2: Write the failing tests**

```go
package hub

import (
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hooks"
)

func TestStopDetail(t *testing.T) {
	cases := []struct{ name, msg, want string }{
		{"question mid-paragraph (screenshot case)",
			"Zur Einordnung: Der Spesen-Test ist zweimal gescheitert.\n\nSoll ich den Stack als zwei PRs submitten? Danach starte ich den Review je Layer.",
			"Soll ich den Stack als zwei PRs submitten?"},
		{"question after a statement", "Tests sind grün.\n\nAlles erledigt. Soll ich pushen?", "Soll ich pushen?"},
		{"no question", "Erledigt.\n\nAlle 41 Tests grün.", "Alle 41 Tests grün."},
		{"bold question", "**Soll ich mergen?**", "Soll ich mergen?"},
		{"question introduces a list", "Wie weiter?\n\n1. Mergen\n2. Warten", "Wie weiter?"},
		{"url is not a question", "Siehe https://x.example/?tab=1 dort.", "Siehe https://x.example/?tab=1 dort."},
		{"empty", "", ""},
	}
	for _, c := range cases {
		if got := stopDetail(c.msg); got != c.want {
			t.Errorf("%s: stopDetail = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestHookDetail(t *testing.T) {
	cases := []struct {
		ev   hooks.Event
		want string
	}{
		{hooks.Event{Event: "PermissionRequest", Tool: "Bash", Detail: "Bash: npm install"}, "Bash: npm install"},
		{hooks.Event{Event: "Notification", Message: "Claude needs your permission"}, "Claude needs your permission"},
		{hooks.Event{Event: "Stop", Message: "Fertig.\n\nSoll ich pushen?"}, "Soll ich pushen?"},
		{hooks.Event{Event: "PostToolUse", Tool: "Bash"}, ""},
	}
	for _, c := range cases {
		if got := hookDetail(c.ev); got != c.want {
			t.Errorf("%s: hookDetail = %q, want %q", c.ev.Event, got, c.want)
		}
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/hub/ -run 'TestStopDetail|TestHookDetail' -count=1`
Expected: FAIL, `undefined: stopDetail`

- [ ] **Step 4: Extract `paragraphs` in `hook_activity.go`**

Ersetze den Körper von `endsWithQuestion` (die Zeilen vom Fence-Check bis vor `last := ...`) durch den Aufruf von `paragraphs`, sodass die Funktion so lautet:

```go
func endsWithQuestion(msg string) bool {
	paras := paragraphs(msg)
	if len(paras) == 0 {
		return false
	}
	last := paras[len(paras)-1]
	if sentenceQuestion.MatchString(last) {
		return true
	}
	if isList(last) && len(paras) > 1 {
		return sentenceQuestion.MatchString(paras[len(paras)-2])
	}
	return false
}
```

- [ ] **Step 5: Write `hook_detail.go`**

```go
package hub

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hooks"
)

// What a hook event says the agent wants, as one line for the waiting list.

// sentenceEnd is the end of a sentence before the question: a full stop,
// exclamation or question mark followed by space.
var sentenceEnd = regexp.MustCompile(`[.!?…]["'»“”)\]*_~]*\s+`)

// paragraphs splits a message into its non-empty paragraphs, code blocks
// removed. mtui-hook sends only the tail of a long message, which can start
// inside a code block: an odd number of fences means the first one closes a
// block whose start was cut off.
func paragraphs(msg string) []string {
	if strings.Count(msg, "```")%2 == 1 {
		msg = msg[strings.Index(msg, "```")+3:]
	}
	msg = codeFence.ReplaceAllString(msg, "")
	var paras []string
	for _, p := range strings.Split(strings.ReplaceAll(msg, "\r\n", "\n"), "\n\n") {
		if strings.TrimSpace(p) != "" {
			paras = append(paras, p)
		}
	}
	return paras
}

// questionSentence returns the last question in a paragraph, from the start
// of its sentence to its question mark, or "" when there is none.
func questionSentence(para string) string {
	locs := sentenceQuestion.FindAllStringIndex(para, -1)
	if len(locs) == 0 {
		return ""
	}
	qStart := locs[len(locs)-1][0]
	_, size := utf8.DecodeRuneInString(para[qStart:])
	start := 0
	for _, m := range sentenceEnd.FindAllStringIndex(para[:qStart], -1) {
		start = m[1]
	}
	return strings.TrimLeft(strings.TrimSpace(para[start:qStart+size]), "*_ ")
}

// stopDetail is what a Stop message shows: the question when the message
// ends by asking one, otherwise its last paragraph.
func stopDetail(msg string) string {
	paras := paragraphs(msg)
	if len(paras) == 0 {
		return ""
	}
	last := paras[len(paras)-1]
	if isList(last) && len(paras) > 1 && sentenceQuestion.MatchString(paras[len(paras)-2]) {
		last = paras[len(paras)-2]
	}
	if q := questionSentence(last); q != "" {
		return clipDetail(q)
	}
	return clipDetail(strings.TrimLeft(last, "*_ "))
}

// hookDetail is what one hook event says the agent wants.
func hookDetail(ev hooks.Event) string {
	switch {
	case ev.Event == "Stop":
		return stopDetail(ev.Message)
	case ev.Detail != "":
		return clipDetail(ev.Detail)
	case ev.Event == "Notification":
		return clipDetail(ev.Message)
	}
	return ""
}
```

Hinweis: Der Fall „bold question“ erwartet `Soll ich mergen?`. `questionSentence` liefert `**Soll ich mergen?`, das `TrimLeft` entfernt die Sterne.

- [ ] **Step 6: Run the hub tests**

Run: `go test ./internal/hub/ -count=1`
Expected: PASS, auch die bestehenden `endsWithQuestion`-Tests in `embedded_hooks_test.go`

- [ ] **Step 7: Commit**

```bash
git add internal/hub/hook_detail.go internal/hub/hook_detail_test.go internal/hub/hook_activity.go internal/hooks/watcher.go
git commit -m "feat(hub): one-line details from hook events" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Der Embedded-Host führt den Datensatz

**Files:**
- Create: `internal/hub/embedded_attention.go`
- Modify: `internal/hub/embedded.go` (Feld, Konstruktor, `Close`, `List`, `Get`)
- Modify: `internal/hub/embedded_scan.go` (`ScanActivity`, `ForceActivity`)
- Modify: `internal/hub/embedded_hooks.go` (`applyHookEvent`, Subagent-Sperre)
- Modify: `internal/hub/types.go` (`SessionSummary.Attention`)
- Modify: `internal/hub/events.go` (`EventSessionAttention`, `AttentionUpdate`)
- Test: `internal/hub/embedded_attention_test.go`

**Interfaces:**
- Consumes: `attentionBook`, `hookDetail` (Task 1, 2)
- Produces: `const EventSessionAttention = "session.attention"`, `type AttentionUpdate struct{ID int; Attention Attention}`, `SessionSummary.Attention`, `func (h *Embedded) MarkSeen(id int) error`, `func (h *Embedded) MarkUnseen(id int) error`, `func (h *Embedded) recordConfirmed(id int, state Activity, since time.Time)`, `func subagentWorkAfterTurn(ev hooks.Event, next Activity, current terminal.ActivityState) bool`.

- [ ] **Step 1: Write the failing tests**

```go
package hub

import (
	"sync"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hooks"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// attentionSink keeps the attention payloads and every event name.
type attentionSink struct {
	mu      sync.Mutex
	updates []AttentionUpdate
	names   chan string
}

func newAttentionSink() *attentionSink { return &attentionSink{names: make(chan string, 256)} }

func (s *attentionSink) Emit(name string, payload any) {
	if name == EventSessionAttention {
		if u, ok := DecodePayload[AttentionUpdate](payload); ok {
			s.mu.Lock()
			s.updates = append(s.updates, u)
			s.mu.Unlock()
		}
	}
	select {
	case s.names <- name:
	default:
	}
}

func (s *attentionSink) last(t *testing.T) AttentionUpdate {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.updates) == 0 {
		t.Fatal("no attention update was emitted")
	}
	return s.updates[len(s.updates)-1]
}

func (s *attentionSink) all() []AttentionUpdate {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]AttentionUpdate(nil), s.updates...)
}

func (s *attentionSink) await(t *testing.T, name string) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case got := <-s.names:
			if got == name {
				return
			}
		case <-deadline:
			t.Fatalf("event %q never arrived", name)
		}
	}
}

// confirmNow drives one state through the debounce without sleeping.
func confirmNow(t *testing.T, h *Embedded, id int) {
	t.Helper()
	h.ScanActivity()
	if !h.BackdateActivityForTest(id) {
		t.Fatal("no candidate armed")
	}
	h.ScanActivity()
}

func TestEmbedded_FinishedPaneIsUnseenUntilMarkSeen(t *testing.T) {
	sink := newAttentionSink()
	h := newTestHost(t, sink)
	h.AdoptForTest(1, terminal.NewSession(1, 24, 80))
	h.applyHookEvent(nil, hooks.Event{Event: "Stop", MtID: 1, Message: "Fertig.\n\nAlle Tests grün."})
	confirmNow(t, h, 1)

	a := sink.last(t).Attention
	if a.State != ActivityDone || a.Seen || a.Rank != RankDone || a.Detail != "Alle Tests grün." || a.Source != "Hook: Stop" {
		t.Fatalf("after Stop: %+v", a)
	}
	if err := h.MarkSeen(1); err != nil {
		t.Fatal(err)
	}
	if a := sink.last(t).Attention; !a.Seen || a.Rank != RankQuiet {
		t.Errorf("after MarkSeen: %+v", a)
	}
	if got, _ := h.Get(1); got.Attention.Rank != RankQuiet {
		t.Errorf("summary rank %d, want %d", got.Attention.Rank, RankQuiet)
	}
	if err := h.MarkUnseen(1); err != nil {
		t.Fatal(err)
	}
	if a := sink.last(t).Attention; a.Seen || a.Rank != RankDone {
		t.Errorf("after MarkUnseen: %+v", a)
	}
}

func TestEmbedded_MarkSeenOnAnUnknownSessionFails(t *testing.T) {
	h := newTestHost(t, nil)
	if err := h.MarkSeen(99); err == nil {
		t.Error("want an error for an unknown session")
	}
}

func TestEmbedded_SleepKeepsTheFinishedPaneOnTheList(t *testing.T) {
	h := newTestHost(t, newAttentionSink())
	h.AdoptForTest(1, terminal.NewSession(1, 24, 80))
	_ = h.ForceActivity(1, ActivityDone, time.Now())
	_ = h.ForceActivity(1, ActivitySleeping, time.Now())
	if got, _ := h.Get(1); got.Attention.Rank != RankDone {
		t.Errorf("rank %d after sleep, want %d", got.Attention.Rank, RankDone)
	}
}

// A background subagent at work after the turn ended must not repaint the
// pane as running; its question would vanish (spec Teil C4).
func TestEmbedded_SubagentWorkDoesNotEndATurn(t *testing.T) {
	h := newTestHost(t, newAttentionSink())
	sess := terminal.NewSession(1, 24, 80)
	h.AdoptForTest(1, sess)
	h.applyHookEvent(nil, hooks.Event{Event: "Stop", MtID: 1, Message: "Soll ich pushen?"})
	h.applyHookEvent(nil, hooks.Event{Event: "PreToolUse", MtID: 1, Tool: "Bash", AgentID: "a-1"})
	if got := sess.GetActivity(); got != terminal.ActivityWaitingAnswer {
		t.Errorf("after subagent tool use: %v, want waitingAnswer", got)
	}
	// A subagent that needs a permission blocks the user and still counts.
	h.applyHookEvent(nil, hooks.Event{Event: "PermissionRequest", MtID: 1, Tool: "Bash", AgentID: "a-1"})
	if got := sess.GetActivity(); got != terminal.ActivityWaitingPermission {
		t.Errorf("after subagent permission: %v, want waitingPermission", got)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/hub/ -run 'TestEmbedded_(Finished|MarkSeen|Sleep|Subagent)' -count=1`
Expected: FAIL, `undefined: EventSessionAttention`

- [ ] **Step 3: Types and event**

In `internal/hub/types.go`, `SessionSummary`, nach `Offset`:

```go
	// Attention is what the session wants from the person (attention.go). A
	// daemon from before it sends a zero record; read the rank with
	// AttentionRank(State, Seen), which is quiet for the empty state.
	Attention Attention `json:"attention" yaml:"attention"`
```

In `internal/hub/events.go`, im Konstantenblock nach `EventSessionResumed`:

```go
	// EventSessionAttention carries a session's attention record after any
	// change: a newly confirmed state, a person looking at the pane.
	EventSessionAttention = "session.attention"
```

und am Dateiende:

```go
// AttentionUpdate is the payload of EventSessionAttention.
type AttentionUpdate struct {
	ID        int       `json:"id" yaml:"id"`
	Attention Attention `json:"attention" yaml:"attention"`
}
```

- [ ] **Step 4: `embedded_attention.go`**

```go
package hub

import (
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hooks"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// MarkSeen records that the person looked at the session. It clears "done"
// and "error" from the waiting list; a permission or a question stays,
// because only the agent moving on answers those.
func (h *Embedded) MarkSeen(id int) error { return h.setSeen(id, true) }

// MarkUnseen puts a finished or failed session back on the waiting list.
func (h *Embedded) MarkUnseen(id int) error { return h.setSeen(id, false) }

func (h *Embedded) setSeen(id int, seen bool) error {
	if _, err := h.lookup(id); err != nil {
		return err
	}
	if a, changed := h.attention.markSeen(id, seen); changed {
		h.emit(EventSessionAttention, AttentionUpdate{ID: id, Attention: a})
	}
	return nil
}

// recordConfirmed turns a confirmed state into a new attention record and
// tells the clients. Lifecycle labels (sleeping, resuming) leave it alone.
func (h *Embedded) recordConfirmed(id int, state Activity, since time.Time) {
	if a, changed := h.attention.confirmed(id, state, since); changed {
		h.emit(EventSessionAttention, AttentionUpdate{ID: id, Attention: a})
	}
}

// subagentWorkAfterTurn reports a subagent's tool event arriving after the
// main turn ended. Claude Code marks those with agent_id. A background
// subagent at work must not repaint a finished pane as running: that hid the
// closing question of the turn (spec Teil C4). A subagent that needs a
// permission or asks something still counts, because it blocks.
func subagentWorkAfterTurn(ev hooks.Event, next Activity, current terminal.ActivityState) bool {
	return ev.AgentID != "" && next == ActivityActive &&
		(current == terminal.ActivityDone || current == terminal.ActivityWaitingAnswer)
}
```

- [ ] **Step 5: Wire it into the host**

`internal/hub/embedded.go`:
- Struct `Embedded`, nach `activity *activityDebouncer`: `attention *attentionBook`
- `NewEmbedded`, nach `activity: newActivityDebouncer(),`: `attention: newAttentionBook(),`
- `Close`, nach `h.activity.forget(id)`: `h.attention.forget(id)`
- `List`, in der Schleife statt `out = append(out, summarize(id, m))`:

```go
		s := summarize(id, m)
		s.Attention = h.attention.get(id)
		out = append(out, s)
```

- `Get`, statt `return summarize(id, m), nil`:

```go
	s := summarize(id, m)
	s.Attention = h.attention.get(id)
	return s, nil
```

`internal/hub/embedded_scan.go`:
- `ScanActivity`, direkt nach `r.Since = began`:

```go
		if changed {
			h.recordConfirmed(r.ID, confirmed, began)
		}
```

- `ForceActivity`, nach `h.activity.force(id, state, at)`: `h.recordConfirmed(id, state, at)`

`internal/hub/embedded_hooks.go`, in `applyHookEvent` den Block `if activity, ok := hookActivity(ev); ok { ... }` ersetzen durch:

```go
	if activity, ok := hookActivity(ev); ok {
		if subagentWorkAfterTurn(ev, activity, m.sess.GetActivity()) {
			h.emit(EventSessionHook, report)
			return
		}
		h.attention.noteHook(ev.MtID, activity, ev.Event, hookDetail(ev))
		m.sess.SetHookActivity(terminalActivity(activity))
		report.Activity = activity
	}
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/hub/ -count=1`
Expected: PASS
Run: `go vet ./internal/hub/`
Expected: keine Meldungen. `wc -l internal/hub/embedded.go` bleibt unter 300.

- [ ] **Step 7: Commit**

```bash
git add internal/hub/
git commit -m "feat(hub): the embedded host keeps attention and reports every change" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: „Gesehen“ über das Wire-Protokoll

**Files:**
- Create: `internal/hub/server_activity.go`
- Modify: `internal/hub/host.go` (Interface), `internal/hub/client_screen.go` (Remote), `internal/hub/server.go` (Routen ausgelagert, Typen verschoben)
- Test: `internal/hub/roundtrip_test.go` (neuer Test anhängen)

**Interfaces:**
- Consumes: `Embedded.MarkSeen/MarkUnseen` (Task 3)
- Produces: `Host.MarkSeen(id int) error`, `Host.MarkUnseen(id int) error`; Route `POST /v1/sessions/{id}/seen` mit Body `{"seen": bool}`.

- [ ] **Step 1: Write the failing test** (an `roundtrip_test.go` anhängen; dort ist `terminal` bereits importiert oder in den Import aufnehmen)

```go
func TestRemote_MarkSeenCrossesTheWire(t *testing.T) {
	h := newTestHost(t, nil)
	h.AdoptForTest(1, terminal.NewSession(1, 24, 80))
	_ = h.ForceActivity(1, ActivityDone, time.Now())
	addr, _ := serveHost(t, h)
	r := dialTest(t, addr, newSink())

	if err := r.MarkSeen(1); err != nil {
		t.Fatalf("MarkSeen: %v", err)
	}
	got, err := r.Get(1)
	if err != nil || !got.Attention.Seen || AttentionRank(got.Attention.State, got.Attention.Seen) != RankQuiet {
		t.Fatalf("after MarkSeen: %+v, %v", got.Attention, err)
	}
	if err := r.MarkUnseen(1); err != nil {
		t.Fatalf("MarkUnseen: %v", err)
	}
	if got, _ := r.Get(1); got.Attention.Seen {
		t.Errorf("after MarkUnseen still seen")
	}
	if err := r.MarkSeen(99); err == nil {
		t.Error("MarkSeen on an unknown session: want an error")
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/hub/ -run TestRemote_MarkSeen -count=1`
Expected: FAIL, `r.MarkSeen undefined`

- [ ] **Step 3: Extend the interface** (`host.go`, nach `SeedActivity`)

```go
	// MarkSeen records that the person looked at the session; MarkUnseen puts
	// it back. See Attention: seeing clears "done" and "error", not a
	// permission or a question.
	MarkSeen(id int) error
	MarkUnseen(id int) error
```

- [ ] **Step 4: Remote** (`client_screen.go`, nach `SeedActivity`)

```go
// MarkSeen implements Host.
func (r *Remote) MarkSeen(id int) error {
	return r.call(http.MethodPost, fmt.Sprintf("/v1/sessions/%d/seen", id), seenWrite{Seen: true}, nil)
}

// MarkUnseen implements Host.
func (r *Remote) MarkUnseen(id int) error {
	return r.call(http.MethodPost, fmt.Sprintf("/v1/sessions/%d/seen", id), seenWrite{Seen: false}, nil)
}
```

- [ ] **Step 5: `server_activity.go`, and shrink `server.go`**

Neue Datei:

```go
package hub

import (
	"encoding/json"
	"net/http"
	"time"
)

// The agent-state routes of /v1/sessions/{id}: the confirmed activity and the
// seen flag. Out of server.go, which is over the file-size rule and must not
// grow (CLAUDE.md).

// confirmedActivity is the body of GET /v1/sessions/{id}/activity.
type confirmedActivity struct {
	Activity Activity  `json:"activity"`
	Since    time.Time `json:"since"`
}

// activityWrite carries both ways of setting a confirmed state. Seed picks
// which: a seed is refused once the session has confirmed something, a force
// always applies.
type activityWrite struct {
	Activity Activity  `json:"activity"`
	At       time.Time `json:"at"`
	Seed     bool      `json:"seed,omitempty"`
}

// seenWrite is the body of POST /v1/sessions/{id}/seen.
type seenWrite struct {
	Seen bool `json:"seen"`
}

// handleActivity serves GET and POST /v1/sessions/{id}/activity.
func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request, id int) {
	switch r.Method {
	case http.MethodGet:
		state, began := s.host.ConfirmedActivity(id)
		writeJSON(w, http.StatusOK, confirmedActivity{Activity: state, Since: began})
	case http.MethodPost:
		var body activityWrite
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
			return
		}
		fn := s.host.ForceActivity
		if body.Seed {
			fn = s.host.SeedActivity
		}
		if err := fn(id, body.Activity, body.At); err != nil {
			writeHostError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSeen serves POST /v1/sessions/{id}/seen.
func (s *Server) handleSeen(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body seenWrite
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}
	fn := s.host.MarkUnseen
	if body.Seen {
		fn = s.host.MarkSeen
	}
	if err := fn(id); err != nil {
		writeHostError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

In `server.go`:
- Die beiden Fälle `case action == "activity" && r.Method == http.MethodGet:` und `case action == "activity" && r.Method == http.MethodPost:` samt Körper ersetzen durch:

```go
	case action == "activity":
		s.handleActivity(w, r, id)
	case action == "seen":
		s.handleSeen(w, r, id)
```

- Die Deklarationen `type confirmedActivity struct` und `type activityWrite struct` samt Kommentar löschen (jetzt in `server_activity.go`).
- Ungenutzte Importe entfernen, falls `go vet` sie meldet.

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/hub/ -count=1 && go vet ./internal/hub/`
Expected: PASS, keine Vet-Meldung. `wc -l internal/hub/server.go` ist kleiner als 484.

- [ ] **Step 7: Commit**

```bash
git add internal/hub/
git commit -m "feat(hub): MarkSeen and MarkUnseen over the wire" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4b: `mt ls` zeigt, was wartet

**Files:**
- Modify: `cmd/mt/cmd_list.go`
- Test: `cmd/mt/waiting_test.go`

**Interfaces:**
- Consumes: `SessionSummary.Attention`, `hub.AttentionRank` (Task 3)
- Produces: `func waitingCell(s hub.SessionSummary, now time.Time) string`; neue Spalte `WARTET` in `mt ls`.

- [ ] **Step 1: Write the failing test**

```go
package main

import (
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

func TestWaitingCell(t *testing.T) {
	now := time.Unix(10_000, 0)
	at := func(state hub.Activity, seen bool, ago time.Duration) hub.SessionSummary {
		return hub.SessionSummary{Attention: hub.Attention{State: state, Seen: seen, Since: now.Add(-ago)}}
	}
	cases := []struct {
		s    hub.SessionSummary
		want string
	}{
		{at(hub.ActivityWaitingPermission, false, 2*time.Minute), "Freigabe 2m"},
		{at(hub.ActivityWaitingAnswer, true, 30*time.Second), "Frage <1m"},
		{at(hub.ActivityDone, false, 90*time.Minute), "fertig 1h30m"},
		{at(hub.ActivityDone, true, time.Minute), "-"},
		{at(hub.ActivityActive, false, time.Minute), "-"},
		{hub.SessionSummary{}, "-"}, // an older daemon: no record
	}
	for _, c := range cases {
		if got := waitingCell(c.s, now); got != c.want {
			t.Errorf("%+v: got %q, want %q", c.s.Attention, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./cmd/mt/ -run TestWaitingCell -count=1`
Expected: FAIL, `undefined: waitingCell`

- [ ] **Step 3: Implement** (in `cmd_list.go`, Import `time` ergänzen)

```go
// waitingCell is the WARTET column: what the session waits for and since
// when, "-" when it waits for nobody. The rank is recomputed, because an older
// daemon sends a zero record whose Rank would read as a permission.
func waitingCell(s hub.SessionSummary, now time.Time) string {
	label := map[int]string{hub.RankPermission: "Freigabe", hub.RankQuestion: "Frage", hub.RankError: "Fehler", hub.RankDone: "fertig"}
	text, ok := label[hub.AttentionRank(s.Attention.State, s.Attention.Seen)]
	if !ok {
		return "-"
	}
	if s.Attention.Since.IsZero() {
		return text
	}
	d := now.Sub(s.Attention.Since)
	switch {
	case d < time.Minute:
		return text + " <1m"
	case d < time.Hour:
		return fmt.Sprintf("%s %dm", text, int(d.Minutes()))
	default:
		return fmt.Sprintf("%s %dh%dm", text, int(d.Hours()), int(d.Minutes())%60)
	}
}
```

Die Tabelle in `cmdList` bekommt die Spalte nach `ZUSTAND`:

```go
	now := time.Now()
	fmt.Fprintln(tw, "ID\tMODUS\tZUSTAND\tWARTET\tVERZEICHNIS\tTITEL\tKOSTEN")
	for _, s := range sessions {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			s.ID, orDash(s.Mode), displayState(s), waitingCell(s, now), shortDir(s.Dir),
			orDash(truncate(s.Title, 40)), cost(s.Cost))
	}
```

- [ ] **Step 4: Run the CLI tests**

Run: `go test ./cmd/mt/ -count=1`
Expected: PASS. Falls ein bestehender Test die Kopfzeile wörtlich prüft (`grep -n "ZUSTAND" cmd/mt/*_test.go`), die Erwartung um `WARTET` ergänzen.

- [ ] **Step 5: Commit**

```bash
git add cmd/mt/
git commit -m "feat(cli): mt ls shows what each session waits for" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Zustandskorrekturen

**Files:**
- Create: `internal/hub/permission_dialog.go`
- Modify: `internal/hub/hook_activity.go` (`PostToolUseFailure`)
- Modify: `internal/hub/embedded_scan.go` (`classifyForScan`, `scanOne`, `ScanActivity`, `hung`, `exited`)
- Modify: `internal/hub/embedded_stream.go` (`watchExit`, `isClosing`)
- Modify: `internal/hub/embedded.go` (`managed.closing`, gesetzt in `Close`)
- Test: `internal/hub/hook_permission_test.go` (anhängen), `internal/hub/embedded_attention_test.go` (anhängen), bestehende Erwartung zu `PostToolUseFailure` anpassen

**Interfaces:**
- Consumes: `attentionBook.noteFact`, `recordConfirmed`, `attentionSink` (Task 3)
- Produces: `func permissionDialogVisible(sess *terminal.Session) bool`, `const hangAfter = 10 * time.Minute`, `func hung(sess *terminal.Session, activity terminal.ActivityState, now time.Time) bool`, `func exited(s *terminal.Session) bool`, `scanOne` liefert jetzt `(ScanResult, bool)`.

- [ ] **Step 1: Write the failing tests**

An `hook_permission_test.go` anhängen:

```go
// Declined with Esc: Claude Code prints "Interrupted" and its prompt, and no
// hook follows. Once the pane is quiet the screen's reading wins.
func TestClassifyForScan_DeclinedPermissionLetsGo(t *testing.T) {
	sess := permissionSession(t,
		"● Bash(rm -rf build)\r\n  ⎿  Interrupted · What should Claude do instead?\r\n\x1b[1;35m❯\x1b[0m ",
		30*time.Second, 10*time.Second)
	if got := classifyForScan(sess); got != terminal.ActivityWaitingAnswer {
		t.Errorf("declined and quiet: got %v, want waitingAnswer", got)
	}
}

func TestClassifyForScan_QuietDialogStillWaits(t *testing.T) {
	sess := permissionSession(t,
		"Bash command\r\n  rm -rf build\r\nDo you want to proceed?\r\n❯ 1. Yes\r\n  2. No, and tell Claude what to do differently (esc)\r\n",
		30*time.Second, 20*time.Second)
	if got := classifyForScan(sess); got != terminal.ActivityWaitingPermission {
		t.Errorf("dialog still shown: got %v, want waitingPermission", got)
	}
}

func TestScanOne_HungAgentIsAnError(t *testing.T) {
	sess := terminal.NewSession(1, 24, 80)
	sess.SetHookActivity(terminal.ActivityActive)
	sess.SetHookActivityAtForTest(time.Now().Add(-11 * time.Minute))
	sess.SetLastOutputAtForTest(time.Now().Add(-11 * time.Minute))
	r, isHung := scanOne(1, sess)
	if !isHung || r.Activity != ActivityError {
		t.Errorf("got %v hung=%v, want error and hung", r.Activity, isHung)
	}
}

func TestScanOne_ShellIsNeverHung(t *testing.T) {
	sess := terminal.NewSession(1, 24, 80) // no hook data: a shell or an agent without hooks
	sess.SetLastOutputAtForTest(time.Now().Add(-time.Hour))
	if _, isHung := scanOne(1, sess); isHung {
		t.Error("a session without hooks counted as hung")
	}
}
```

An `embedded_attention_test.go` anhängen (dort zusätzlich `"fmt"` importieren):

```go
func exitArgv(code int) []string {
	if isWindows() {
		return []string{comspec(), "/c", fmt.Sprintf("exit %d", code)}
	}
	return []string{"/bin/sh", "-c", fmt.Sprintf("exit %d", code)}
}

func TestEmbedded_FailedExitIsAnError(t *testing.T) {
	sink := newAttentionSink()
	h := newTestHost(t, sink)
	id, err := h.Create(CreateSpec{Argv: exitArgv(3), Dir: sessionDir(t)})
	if err != nil {
		t.Fatal(err)
	}
	sink.await(t, EventSessionExited)
	got, _ := h.Get(id)
	if got.Attention.State != ActivityError || got.Attention.Source != "Prozess beendet (Exit 3)" {
		t.Errorf("after exit 3: %+v", got.Attention)
	}
}

func TestEmbedded_CloseIsNotAnError(t *testing.T) {
	sink := newAttentionSink()
	h := newTestHost(t, sink)
	id, err := h.Create(CreateSpec{Argv: sleepArgv(), Dir: sessionDir(t)})
	if err != nil {
		t.Fatal(err)
	}
	sink.await(t, EventSessionCreated)
	if err := h.Close(id); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond) // watchExit runs after Close
	for _, u := range sink.all() {
		if u.ID == id && u.Attention.State == ActivityError {
			t.Fatalf("closing the pane reported an error: %+v", u.Attention)
		}
	}
}
```

Bestehende Erwartung anpassen: `grep -rn PostToolUseFailure internal/hub/*_test.go`. Jede Tabellenzeile, die dafür `ActivityError` erwartet, erwartet jetzt `ActivityActive`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/hub/ -run 'Declined|QuietDialog|ScanOne|FailedExit|CloseIsNot|PostToolUseFailure|hook' -count=1`
Expected: FAIL (u. a. `scanOne` liefert nur einen Wert)

- [ ] **Step 3: `PostToolUseFailure`** in `hook_activity.go`:

```go
	case "PostToolUseFailure":
		// A failed tool call is part of the agent's work: a red test, a typo
		// in a command. The agent goes on by itself, so it is still running.
		return ActivityActive, true
```

- [ ] **Step 4: `permission_dialog.go`**

```go
package hub

import (
	"regexp"
	"strings"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// permissionDialog matches what Claude Code draws for a tool permission: the
// question, or the option that sends the user's own instructions instead.
// Checked against Claude Code 2.1. If a later version words it differently,
// the Esc fallback in classifyForScan stops firing and a declined permission
// waits for the next hook again, which is how it behaved before.
var permissionDialog = regexp.MustCompile(`(?i)(do you want to (proceed|make this edit|create|allow)|no, and tell claude what to do differently)`)

// dialogRows is how many bottom rows the check reads.
const dialogRows = 14

// permissionDialogVisible reports whether a permission dialog is on screen.
func permissionDialogVisible(sess *terminal.Session) bool {
	lines := strings.Split(sess.Screen.PlainText(), "\n")
	if len(lines) > dialogRows {
		lines = lines[len(lines)-dialogRows:]
	}
	return permissionDialog.MatchString(strings.Join(lines, "\n"))
}
```

- [ ] **Step 5: `classifyForScan`, `scanOne`, `ScanActivity`** in `embedded_scan.go`

Den Block `if activity == terminal.ActivityWaitingPermission { ... }` ersetzen durch:

```go
	if activity == terminal.ActivityWaitingPermission {
		lastOutput := sess.GetLastOutputAt()
		settled := time.Since(sess.HookActivityAt()) > permissionSettle
		if settled && !lastOutput.IsZero() && time.Since(lastOutput) < terminal.ActivityStaleThreshold &&
			sess.ClassifyScreenState() != terminal.ActivityWaitingAnswer {
			return terminal.ActivityActive
		}
		// Declined with Esc: Claude Code prints "Interrupted" and its prompt,
		// and no hook follows until the user types. Once the dialog is gone
		// and the pane is quiet, the screen's reading wins, so the pane stops
		// asking for a permission nobody can give any more.
		if settled && time.Since(lastOutput) >= permissionSettle && !permissionDialogVisible(sess) {
			if screen := sess.ClassifyScreenState(); screen == terminal.ActivityDone || screen == terminal.ActivityWaitingAnswer {
				return screen
			}
		}
		return activity
	}
```

`scanOne` gibt zusätzlich zurück, ob die Session hängt:

```go
func scanOne(id int, sess *terminal.Session) (ScanResult, bool) {
	if sess.IsSuspendedOrSuspending() {
		return ScanResult{ID: id, Asleep: true}, false
	}

	sess.ScanTokens() // always scan for token/cost data

	activity := classifyForScan(sess)
	isHung := hung(sess, activity, time.Now())
	if isHung {
		activity = terminal.ActivityError
	}
	ctxPct, model, _ := sess.StatuslineInfo()
	return ScanResult{
		ID:         id,
		Activity:   activityOf(activity),
		Cost:       sess.GetTokens().TotalCost,
		Title:      cleanTitle(sess.GetTitle()),
		ContextPct: ctxPct,
		Model:      model,
		Name:       sess.AgentName(),
	}, isHung
}

// hangAfter is how long a hook-driven agent may stay "active" with neither
// output nor a hook event before it counts as hung. Claude Code redraws its
// spinner the whole time it works, so ten quiet minutes are a real stall.
const hangAfter = 10 * time.Minute

// hung reports a stalled agent. Only sessions with hook data qualify: a shell
// running a quiet build is not stuck.
func hung(sess *terminal.Session, activity terminal.ActivityState, now time.Time) bool {
	if activity != terminal.ActivityActive || !sess.HasHookData() {
		return false
	}
	last := sess.GetLastOutputAt()
	return !last.IsZero() && now.Sub(last) > hangAfter && now.Sub(sess.HookActivityAt()) > hangAfter
}

// exited reports a session whose process ended for good. Its screen is the
// last frame; classifying it would replace the exit's "error" with whatever
// that frame looks like.
func exited(s *terminal.Session) bool {
	st := s.GetStatus()
	return st == terminal.StatusExited || st == terminal.StatusError
}
```

In `ScanActivity` die Schleife beginnen mit:

```go
	for i, sess := range sessions {
		if exited(sess) {
			continue
		}
		r, isHung := scanOne(ids[i], sess)
		if isHung {
			h.attention.noteFact(r.ID, ActivityError, "keine Ausgabe und kein Hook seit 10 min", "keine Ausgabe seit 10 min")
		}
```

(die alte Zeile `r := scanOne(ids[i], sess)` entfällt).

- [ ] **Step 6: Exit ≠ 0** in `embedded.go` und `embedded_stream.go`

`managed` bekommt `closing atomic.Bool` (Import `sync/atomic`). In `Close` direkt nach dem erfolgreichen `h.lookup(id)`: `m.closing.Store(true)`.

In `embedded_stream.go` (Importe `fmt`, `time` ergänzen), `watchExit` vor dem `h.emit(EventSessionExited, ...)`:

```go
	if code := sess.GetExitCode(); code != 0 && !h.isClosing(id) {
		reason := fmt.Sprintf("Prozess beendet (Exit %d)", code)
		h.attention.noteFact(id, ActivityError, reason, reason)
		_ = h.ForceActivity(id, ActivityError, time.Now())
	}
```

und darunter:

```go
// isClosing reports a session that is being closed or already gone. Its exit
// code comes from the kill, not from a failure.
func (h *Embedded) isClosing(id int) bool {
	m, err := h.lookup(id)
	return err != nil || m.closing.Load()
}
```

- [ ] **Step 7: Run all hub tests**

Run: `go test ./internal/hub/ -count=1 && go vet ./internal/hub/`
Expected: PASS. Die Zeilenzahlen von `embedded.go`, `embedded_scan.go` und `embedded_stream.go` bleiben unter 300.

- [ ] **Step 8: Commit**

```bash
git add internal/hub/
git commit -m "fix(hub): declined permissions, hung agents and failed exits reach the right state" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: `mtui-hook` liefert Kurztext und `agent_id`

**Files:**
- Create: `cmd/mtui-hook/detail.go`
- Modify: `cmd/mtui-hook/main.go` (`claudeEvent`, `hookLine`, `run`)
- Test: `cmd/mtui-hook/detail_test.go`

**Interfaces:**
- Produces: `func toolDetail(tool string, input json.RawMessage) string`; JSON-Zeile mit `detail` und `agent_id` (Felder aus Task 2 in `hooks.Event` lesen sie).

- [ ] **Step 1: Write the failing test**

```go
package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestToolDetail(t *testing.T) {
	cases := []struct {
		tool, input, want string
	}{
		{"Bash", `{"command":"npm install @shipcloud/sdk --save"}`, "Bash: npm install @shipcloud/sdk --save"},
		{"Edit", `{"file_path":"src/app.ts","old_string":"a","new_string":"b"}`, "Edit: src/app.ts"},
		{"WebFetch", `{"url":"https://example.org/x","prompt":"p"}`, "WebFetch: https://example.org/x"},
		{"AskUserQuestion", `{"questions":[{"question":"Auf v3 heben?","header":"API"}]}`, "Auf v3 heben?"},
		{"TodoWrite", `{"todos":[]}`, "TodoWrite"},
		{"Bash", `not json`, ""},
		{"Bash", ``, ""},
	}
	for _, c := range cases {
		if got := toolDetail(c.tool, json.RawMessage(c.input)); got != c.want {
			t.Errorf("%s %s: got %q, want %q", c.tool, c.input, got, c.want)
		}
	}
	long := toolDetail("Bash", json.RawMessage(`{"command":"`+strings.Repeat("x", 500)+`"}`))
	if n := len([]rune(long)); n > 200 {
		t.Errorf("not clipped: %d runes", n)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./cmd/mtui-hook/ -run TestToolDetail -count=1`
Expected: FAIL, `undefined: toolDetail`

- [ ] **Step 3: `detail.go`**

```go
package main

import (
	"encoding/json"
	"strings"
)

// detailMax matches the hub's limit; the hub clips again, this keeps the hook
// file small.
const detailMax = 200

// toolDetail is one line on what a tool call is about to do, for MTUI's
// waiting list: "Bash: npm install", "Edit: src/app.ts", or the question of an
// AskUserQuestion. Empty when the input cannot be read.
func toolDetail(tool string, input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	var in struct {
		Command      string `json:"command"`
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
		URL          string `json:"url"`
		Pattern      string `json:"pattern"`
		Questions    []struct {
			Question string `json:"question"`
		} `json:"questions"`
	}
	if json.Unmarshal(input, &in) != nil {
		return ""
	}
	if tool == "AskUserQuestion" {
		if len(in.Questions) > 0 {
			return clip(in.Questions[0].Question)
		}
		return ""
	}
	for _, what := range []string{in.Command, in.FilePath, in.NotebookPath, in.URL, in.Pattern} {
		if what != "" {
			return clip(tool + ": " + what)
		}
	}
	return tool
}

func clip(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > detailMax {
		return string(r[:detailMax-1]) + "…"
	}
	return s
}
```

- [ ] **Step 4: Wire into `main.go`**

`claudeEvent`, nach `ToolResponse`:

```go
	// AgentID is set when the hook fires inside a subagent.
	AgentID string `json:"agent_id"`
```

`hookLine`, nach `NotificationType`:

```go
	Detail  string `json:"detail,omitempty"`
	AgentID string `json:"agent_id,omitempty"`
```

In `run()` vor `line, err := json.Marshal(hookLine{`:

```go
	detail := ""
	if eventType == "PreToolUse" || eventType == "PermissionRequest" {
		detail = toolDetail(ev.ToolName, ev.ToolInput)
	}
```

und im `hookLine{...}`-Literal ergänzen: `Detail: detail,` und `AgentID: ev.AgentID,`.

- [ ] **Step 5: Run the tests**

Run: `go test ./cmd/mtui-hook/ -count=1 && go test ./internal/hooks/ -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/mtui-hook/
git commit -m "feat(mtui-hook): pass a one-line detail and the subagent id" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Backend: `terminal:attention`, Bindings, Dashboard

**Files:**
- Create: `internal/backend/app_attention.go`
- Modify: `internal/backend/app_host_stream.go` (Fall `EventSessionAttention`)
- Modify: `internal/backend/app_activity_resend.go`
- Modify: `internal/backend/app_dashboard.go:157`
- Modify: `frontend/wailsjs/go/backend/App.js`, `App.d.ts`
- Test: `internal/backend/app_attention_test.go`

**Interfaces:**
- Consumes: `hub.Attention`, `hub.AttentionRank`, `hub.AttentionUpdate`, `Host.MarkSeen/MarkUnseen`
- Produces: Wails-Event `terminal:attention` mit `{id, state, rank, seen, detail, source, since}`; Bindings `MarkSessionSeen(ref)`, `MarkSessionUnseen(ref)`; `func attentionInfo(ref hub.Ref, a hub.Attention) AttentionInfo`; `func dashboardActivity(s hub.SessionSummary) string`.

- [ ] **Step 1: Write the failing tests**

```go
package backend

import (
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// An older daemon sends Rank 0 with an empty state; that must not read as a
// permission request (Review Focus 2).
func TestAttentionInfo_RecomputesTheRank(t *testing.T) {
	got := attentionInfo("h:1", hub.Attention{State: hub.ActivityDone, Rank: 0})
	if got.Rank != hub.RankDone {
		t.Errorf("rank %d, want %d", got.Rank, hub.RankDone)
	}
	since := time.Unix(1700000000, 0)
	got = attentionInfo("h:1", hub.Attention{State: hub.ActivityWaitingAnswer, Since: since, Detail: "Pushen?"})
	if got.Since != 1700000000 || got.Detail != "Pushen?" || got.State != "waitingAnswer" {
		t.Errorf("got %+v", got)
	}
}

func TestDashboardActivity_PrefersTheConfirmedState(t *testing.T) {
	s := hub.SessionSummary{Activity: hub.ActivityActive, Attention: hub.Attention{State: hub.ActivityDone}}
	if got := dashboardActivity(s); got != "done" {
		t.Errorf("got %q, want done", got)
	}
	s.Attention = hub.Attention{}
	if got := dashboardActivity(s); got != "active" {
		t.Errorf("without a record: got %q, want the raw active", got)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/backend/ -run 'TestAttentionInfo|TestDashboardActivity' -count=1`
Expected: FAIL, `undefined: attentionInfo`

- [ ] **Step 3: `app_attention.go`**

```go
package backend

import "github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"

// AttentionInfo is what the frontend learns about a session's attention
// record (terminal:attention). An event payload: it needs no models.ts mirror.
type AttentionInfo struct {
	ID     hub.Ref `json:"id" yaml:"id"`
	State  string  `json:"state" yaml:"state"`
	Rank   int     `json:"rank" yaml:"rank"`
	Seen   bool    `json:"seen" yaml:"seen"`
	Detail string  `json:"detail" yaml:"detail"`
	Source string  `json:"source" yaml:"source"`
	// Since is unix seconds, 0 when unknown.
	Since int64 `json:"since" yaml:"since"`
}

// attentionInfo recomputes the rank instead of trusting the record's: a
// daemon from before attention existed sends a zero Rank, which is
// "permission".
func attentionInfo(ref hub.Ref, a hub.Attention) AttentionInfo {
	return AttentionInfo{
		ID:     ref,
		State:  string(a.State),
		Rank:   hub.AttentionRank(a.State, a.Seen),
		Seen:   a.Seen,
		Detail: a.Detail,
		Source: a.Source,
		Since:  unixOrZero(a.Since),
	}
}

// emitAttention forwards one record to the frontend.
func (a *AppService) emitAttention(id int, att hub.Attention) {
	if a.app == nil || att.State == "" {
		return
	}
	a.app.Event.Emit("terminal:attention", attentionInfo(a.ref(id), att))
}

// MarkSessionSeen tells the host the person looked at a pane: a finished or
// failed pane leaves the waiting list.
func (a *AppService) MarkSessionSeen(r hub.Ref) {
	if id := a.local(r); id != 0 {
		_ = a.host.MarkSeen(id)
	}
}

// MarkSessionUnseen puts a finished or failed pane back on the waiting list.
func (a *AppService) MarkSessionUnseen(r hub.Ref) {
	if id := a.local(r); id != 0 {
		_ = a.host.MarkUnseen(id)
	}
}

// dashboardActivity is the confirmed state, the one the pane badge shows. The
// raw reading flickers and disagreed with the badge (spec Befund 9).
func dashboardActivity(s hub.SessionSummary) string {
	if s.Attention.State != "" && !s.Asleep() {
		return string(s.Attention.State)
	}
	return string(s.Activity)
}
```

- [ ] **Step 4: Wire it**

`app_host_stream.go`, in `onHostEvent` vor `case hub.EventSessionSuspended:`:

```go
	case hub.EventSessionAttention:
		ev, ok := hub.DecodePayload[hub.AttentionUpdate](payload)
		if !ok {
			return
		}
		a.emitAttention(ev.ID, ev.Attention)
```

`app_activity_resend.go`, am Ende von `resendActivity` (nach dem bestehenden `Emit`):

```go
	a.emitAttention(id, summary.Attention)
```

`app_dashboard.go:157`: `Activity:  string(s.Activity),` wird zu `Activity:  dashboardActivity(s),`.

- [ ] **Step 5: Binding stubs**

`frontend/wailsjs/go/backend/App.js`, alphabetisch einsortiert:

```js
export function MarkSessionSeen(arg1) {
  return $Call.ByID(2450841005, arg1);
}

export function MarkSessionUnseen(arg1) {
  return $Call.ByID(1453842906, arg1);
}
```

`App.d.ts`:

```ts
export function MarkSessionSeen(arg1:string):Promise<void>;

export function MarkSessionUnseen(arg1:string):Promise<void>;
```

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/backend/ -count=1 && go test ./internal/tsmodels/ -count=1`
Expected: PASS
Run: `grep -rl internal/terminal internal/backend --include='*.go' | grep -v _test.go`
Expected: keine Ausgabe

- [ ] **Step 7: Commit**

```bash
git add internal/backend/ frontend/wailsjs/go/backend/
git commit -m "feat(backend): attention reaches the frontend, panes can be marked seen" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Agent-Panes landen im Tab ihres Projekts

**Files:**
- Modify: `internal/backend/app_agent_panes.go` (`SessionSpawnedEvent.ProjectRoot`, `ProjectRoot`-Binding, `projectRootOf`)
- Modify: `frontend/wailsjs/go/backend/App.js`, `App.d.ts`
- Create: `frontend/src/lib/projectTabs.ts`
- Modify: `frontend/src/App.svelte` (Handler `mtui:session-spawned`)
- Test: `frontend/src/lib/projectTabs.test.ts`, `internal/backend/app_agent_panes_test.go` (anhängen oder neu)

**Interfaces:**
- Produces: Binding `ProjectRoot(dir string) string` (ID 861199428); Event-Feld `projectRoot`; `normPath(p: string): string`; `tabForProject(tabs: Tab[], rootsByDir: Record<string, string>, projectRoot: string): Tab | null`.

- [ ] **Step 1: Write the failing tests**

`frontend/src/lib/projectTabs.test.ts`:

```ts
import { describe, it, expect } from 'vitest';
import { normPath, tabForProject } from './projectTabs';

const tab = (id: string, dir: string) => ({ id, name: id, dir, panes: [], focusedPaneId: '' }) as any;

describe('tabForProject', () => {
  const tabs = [tab('a', 'C:\\repos\\polaris'), tab('b', 'C:\\repos\\goki\\src')];
  const roots = { 'C:\\repos\\polaris': 'C:\\repos\\polaris', 'C:\\repos\\goki\\src': 'C:\\repos\\goki' };

  it('finds the tab whose project root matches, ignoring case and slashes', () => {
    expect(tabForProject(tabs, roots, 'c:/Repos/goki/')?.id).toBe('b');
  });
  it('returns null when no tab belongs to the project', () => {
    expect(tabForProject(tabs, roots, 'C:\\repos\\other')).toBeNull();
  });
  it('falls back to the tab dir while its root is unknown', () => {
    expect(tabForProject([tab('c', 'D:\\x')], {}, 'd:\\x')?.id).toBe('c');
  });
  it('normalises paths', () => {
    expect(normPath('C:\\A\\b\\')).toBe('c:/a/b');
  });
});
```

Backend (`internal/backend/app_agent_panes_test.go`, neu oder anhängen):

```go
func TestProjectRootOf_OutsideARepoIsTheDirItself(t *testing.T) {
	a := newTestApp()
	dir := t.TempDir()
	if got := a.projectRootOf(dir); got != dir {
		t.Errorf("got %q, want %q", got, dir)
	}
	if got := a.projectRootOf(""); got != "" {
		t.Errorf("empty dir: got %q", got)
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd frontend && npx vitest run src/lib/projectTabs.test.ts` und `go test ./internal/backend/ -run TestProjectRootOf -count=1`
Expected: FAIL

- [ ] **Step 3: Backend**

In `SessionSpawnedEvent` nach `Origin`:

```go
	// ProjectRoot is the main checkout of the repo the session runs in, so
	// the frontend puts the pane into that project's tab even while another
	// tab is active. Outside a repo it is Dir.
	ProjectRoot string `json:"projectRoot" yaml:"projectRoot"`
```

In `onSessionCreated` nach `ev, ok := spawnedEvent(...)`/`if !ok { return }`: `ev.ProjectRoot = a.projectRootOf(s.Dir)`.

Am Dateiende:

```go
// ProjectRoot returns the main checkout of the repo containing dir, or dir
// itself outside a repo. A tab asks once to learn which project it is.
func (a *AppService) ProjectRoot(dir string) string { return a.projectRootOf(dir) }

func (a *AppService) projectRootOf(dir string) string {
	if dir == "" {
		return ""
	}
	if root, err := mainRepoRoot(dir); err == nil {
		return root
	}
	return dir
}
```

`App.js`: `export function ProjectRoot(arg1) {\n  return $Call.ByID(861199428, arg1);\n}`; `App.d.ts`: `export function ProjectRoot(arg1:string):Promise<string>;`

- [ ] **Step 4: `projectTabs.ts`**

```ts
import type { Tab } from '../stores/tabs';

/** For comparing paths: forward slashes, no trailing slash, lower case
 *  (Windows paths are case-insensitive). */
export function normPath(p: string): string {
  return (p || '').replace(/\\/g, '/').replace(/\/+$/, '').toLowerCase();
}

/** The tab that belongs to a project. `rootsByDir` maps a tab dir to its
 *  project root as the backend resolved it; a dir not resolved yet stands
 *  for itself. */
export function tabForProject(tabs: Tab[], rootsByDir: Record<string, string>, projectRoot: string): Tab | null {
  const want = normPath(projectRoot);
  if (!want) return null;
  return tabs.find((t) => normPath(rootsByDir[t.dir] ?? t.dir) === want) ?? null;
}
```

- [ ] **Step 5: `App.svelte`**

Import ergänzen: `import { tabForProject } from './lib/projectTabs';`. Im Skript, auf Modulebene der Komponente:

```ts
  // Tab dir -> project root, as the backend resolved it. Keyed by dir rather
  // than by tab, so a tab whose dir changed is simply looked up again.
  const projectRoots: Record<string, string> = {};
  async function resolveProjectRoots(tabs: typeof $allTabs): Promise<Record<string, string>> {
    await Promise.all(tabs.filter((t) => t.dir && !(t.dir in projectRoots)).map(async (t) => {
      try { projectRoots[t.dir] = await App.ProjectRoot(t.dir); } catch { projectRoots[t.dir] = t.dir; }
    }));
    return projectRoots;
  }
```

Den Handler `EventsOn('mtui:session-spawned', ...)` ersetzen durch:

```ts
    EventsOn('mtui:session-spawned', async (event: any) => {
      const info = event.data; // SessionSpawnedEvent { id, tool, model, dir, name, origin, projectRoot }
      if (!info?.id || !isMainWindow()) return;
      // The pane goes to its own project's tab, never to whichever tab happens
      // to be open: there it would sit in the wrong project and never show up
      // as waiting elsewhere (spec Befund 2). No tab switch either way.
      const roots = await resolveProjectRoots(get(allTabs));
      let tab = tabForProject(get(allTabs), roots, info.projectRoot || info.dir);
      if (!tab) {
        const dirName = String(info.dir || '').replace(/\\/g, '/').split('/').pop() || 'Agent';
        const id = tabStore.addTab(dirName, info.dir, !get(activeTab));
        tab = get(allTabs).find((t) => t.id === id) ?? null;
      }
      if (!tab) return;
      const isActive = tab.id === get(activeTab)?.id;
      const prevFocus = tab.focusedPaneId;
      const paneId = tabStore.addPane(tab.id, info.id, info.name || info.tool, info.tool, info.model || '',
        null, '', '', '', '', '', false, 'terminal', '', '');
      if (layoutMode === 'focus') {
        placePane(tab.id, paneId, 2);
        if (isActive && prevFocus && tab.panes.some((p) => p.id === prevFocus)) tabStore.focusPane(tab.id, prevFocus);
      }
    });
```

- [ ] **Step 6: Run the tests**

Run: `cd frontend && npx vitest run` und `go test ./internal/backend/ ./internal/tsmodels/ -count=1`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/backend/app_agent_panes.go internal/backend/app_agent_panes_test.go frontend/
git commit -m "fix: an agent's pane lands in its own project's tab" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: Toast-Klick springt zum Pane (Backend)

**Files:**
- Modify: `internal/backend/app_notify.go` (`SendPaneNotification`, `readFocusRequest`, `FocusSessionEvent`, `handleFocusSignal`)
- Modify: `internal/backend/notify_toast_windows.go`, `notify_toast_other.go` (Aktivierungs-Argument)
- Modify: `main.go` (`signalFocus(arg)`)
- Modify: `frontend/wailsjs/go/backend/App.js`, `App.d.ts`
- Test: `internal/backend/app_notify_test.go` (neu)

**Interfaces:**
- Produces: Binding `SendPaneNotification(title, body string, r hub.Ref)` (ID 3894238824); Wails-Event `mtui:focus-session` mit `{id}`; `func readFocusRequest(conn net.Conn, want string) (bool, hub.Ref)`; Aktivierung `multiterminal:focus/<ref>`.

- [ ] **Step 1: Write the failing test**

```go
package backend

import (
	"net"
	"testing"
)

func focusRequest(t *testing.T, send string) (bool, string) {
	t.Helper()
	server, client := net.Pipe()
	go func() {
		_, _ = client.Write([]byte(send))
		_ = client.Close()
	}()
	ok, ref := readFocusRequest(server, "tok")
	_ = server.Close()
	return ok, string(ref)
}

func TestReadFocusRequest(t *testing.T) {
	cases := []struct {
		send    string
		wantOK  bool
		wantRef string
	}{
		{"tok\nab12:5\n", true, "ab12:5"},
		{"tok\n", true, ""},           // a plain focus click, as before
		{"tok\nnot a ref\n", true, ""}, // garbage names no pane
		{"wrong\nab12:5\n", false, ""},
	}
	for _, c := range cases {
		ok, ref := focusRequest(t, c.send)
		if ok != c.wantOK || ref != c.wantRef {
			t.Errorf("%q: got (%v, %q), want (%v, %q)", c.send, ok, ref, c.wantOK, c.wantRef)
		}
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/backend/ -run TestReadFocusRequest -count=1`
Expected: FAIL, `undefined: readFocusRequest`

- [ ] **Step 3: Toast with an activation argument**

`notify_toast_windows.go`: Signatur `func sendDesktopNotification(title, body, activation string)` und `ActivationArguments: activation,`. `notify_toast_other.go`: dieselbe Signatur, Körper unverändert. Danach `grep -rn "sendDesktopNotification(" internal/ cmd/` und jeden weiteren Aufrufer auf `sendDesktopNotification(title, body, focusActivation)` umstellen.

`app_notify.go`: `SendNotification` ruft `sendDesktopNotification(title, body, focusActivation)` auf. Dazu:

```go
// focusActivation is what a toast click runs: mtui.exe with this argument,
// which signals the running instance (main.go, signalFocus).
const focusActivation = "multiterminal:focus"

// SendPaneNotification is SendNotification for a toast about one pane: a
// click raises the window and jumps to that pane.
func (a *AppService) SendPaneNotification(title string, body string, r hub.Ref) {
	sendDesktopNotification(title, body, focusActivation+"/"+string(r))
}

// FocusSessionEvent asks the frontend to show one pane (a toast was clicked).
type FocusSessionEvent struct {
	ID hub.Ref `json:"id" yaml:"id"`
}

// focusRefPattern is what a focus request may name: a hub.Ref, "hubid:number".
var focusRefPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}:[0-9]{1,9}$`)

// readFocusRequest reads the token line and an optional pane line. ok is false
// when the token does not match; ref is empty when no valid pane was named.
func readFocusRequest(conn net.Conn, want string) (ok bool, ref hub.Ref) {
	_ = conn.SetReadDeadline(time.Now().Add(focusHandshakeTimeout))
	rd := bufio.NewReaderSize(io.LimitReader(conn, 2*focusTokenMaxLen), focusTokenMaxLen)
	line, err := rd.ReadString('\n')
	if err != nil && line == "" {
		return false, ""
	}
	got := strings.TrimSpace(line)
	if got == "" || want == "" || subtle.ConstantTimeCompare([]byte(got), []byte(want)) != 1 {
		return false, ""
	}
	next, _ := rd.ReadString('\n')
	if r := strings.TrimSpace(next); focusRefPattern.MatchString(r) {
		return true, hub.Ref(r)
	}
	return true, ""
}
```

`handleFocusSignal` wird zu:

```go
func (a *AppService) handleFocusSignal(conn net.Conn, token string) {
	defer conn.Close()
	ok, ref := readFocusRequest(conn, token)
	if !ok {
		log.Printf("[focusListener] rejected focus request from %s: bad token", conn.RemoteAddr())
		return
	}
	a.focusMainWindow()
	if ref != "" && a.app != nil {
		a.app.Event.Emit("mtui:focus-session", FocusSessionEvent{ID: ref})
	}
}
```

Die alte Funktion `readFocusToken` löschen (`grep -rn readFocusToken internal/` muss danach leer sein). Importe `io`, `regexp` und `internal/hub` ergänzen.

- [ ] **Step 4: `main.go`**

Aufruf: `signalFocus(arg)`. Die Funktion:

```go
func signalFocus(arg string) {
	rec, err := discovery.Resolve(discovery.ServiceFocus)
	if err != nil {
		return // no running instance of this user, or its listener never started
	}
	conn, err := net.DialTimeout("tcp", rec.Addr(), 2*time.Second)
	if err != nil {
		return
	}
	defer conn.Close()
	payload := rec.Token + "\n"
	// A toast about one pane activates "multiterminal:focus/<ref>"; the
	// running instance jumps to that pane after raising the window.
	if ref, found := strings.CutPrefix(arg, "multiterminal:focus/"); found {
		if ref, err := url.PathUnescape(strings.TrimSuffix(ref, "/")); err == nil && ref != "" {
			payload += ref + "\n"
		}
	}
	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, _ = conn.Write([]byte(payload))
}
```

Import `net/url` ergänzen.

- [ ] **Step 5: Binding stubs**

`App.js`: `export function SendPaneNotification(arg1, arg2, arg3) {\n  return $Call.ByID(3894238824, arg1, arg2, arg3);\n}`; `App.d.ts`: `export function SendPaneNotification(arg1:string,arg2:string,arg3:string):Promise<void>;`

- [ ] **Step 6: Run the tests**

Run: `go test ./internal/backend/ ./internal/tsmodels/ -count=1 && go build ./... && GOOS=windows CGO_ENABLED=0 go vet ./internal/... ./cmd/...`
Expected: PASS, keine Vet-Meldung

- [ ] **Step 7: Commit**

```bash
git add internal/backend/ main.go frontend/wailsjs/go/backend/
git commit -m "feat(notify): a toast about a pane jumps to that pane" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: Frontend-Modell: `attention.ts`, Store, Tab-Rang

**Files:**
- Create: `frontend/src/lib/attention.ts`, `frontend/src/lib/attention.test.ts`
- Modify: `frontend/src/stores/tabs.ts` (`PaneAttention`, `Pane.attention`, `applyAttention`; `unreadActivity` und `computeTabActivity` entfernen)
- Modify: `frontend/src/components/TabBar.svelte:180-182` (Punkt nach Rang, Übergang bis Task 13)
- Modify: `frontend/src/App.svelte` (Listener `terminal:attention`)
- Modify: Tests mit `unreadActivity`/`computeTabActivity`: `stores/tabs.test.ts`, `stores/tabs.sleeping.test.ts`, `lib/focusLayout.test.ts`, `lib/dashboard.test.ts`

**Interfaces:**
- Consumes: Event `terminal:attention` (Task 7)
- Produces: `interface PaneAttention {state; rank; seen; detail; source; since}`, `Pane.attention?: PaneAttention`, `tabStore.applyAttention(ref, att)`, aus `attention.ts`: `RANK_*`, `paneRank`, `GlyphKind`, `glyphKind`, `AttentionEntry`, `attentionList`, `panelRows`, `AttentionCursor`, `cursorOf`, `nextEntry`, `tabRank`, `tabGlyphs`, `tabTooltip`, `ENTRY_LABEL`, `GLYPH_LABEL`, `shouldToast`.

- [ ] **Step 1: Write the failing tests** (`attention.test.ts`)

```ts
import { describe, it, expect } from 'vitest';
import {
  paneRank, glyphKind, attentionList, panelRows, nextEntry, cursorOf, tabRank, tabGlyphs, shouldToast,
} from './attention';

const pane = (id: string, activity: string, att?: Partial<{ rank: number; seen: boolean; since: number; state: string }>) =>
  ({ id, sessionId: 'h:' + id, name: id, activity, activitySince: 0,
     attention: att ? { state: activity, rank: 5, seen: false, detail: '', source: '', since: 0, ...att } : undefined }) as any;
const tab = (id: string, panes: any[]) => ({ id, name: id, dir: '', panes, focusedPaneId: '' }) as any;

describe('paneRank', () => {
  it('uses the host rank when there is one', () => {
    expect(paneRank(pane('a', 'done', { rank: 3 }))).toBe(3);
  });
  it('without a record, a finished pane counts as seen (older daemon)', () => {
    expect(paneRank(pane('a', 'done'))).toBe(5);
    expect(paneRank(pane('b', 'waitingPermission'))).toBe(0);
  });
});

describe('attentionList', () => {
  const tabs = [
    tab('t1', [pane('p1', 'done', { rank: 3, since: 100 }), pane('p2', 'active', { rank: 4 })]),
    tab('t2', [pane('p3', 'waitingAnswer', { rank: 1, since: 300 }), pane('p4', 'waitingPermission', { rank: 0, since: 500 })]),
  ];
  it('lists ranks 0-3 from every tab, most urgent first, the current tab marked', () => {
    const list = attentionList(tabs, 't1');
    expect(list.map((e) => e.pane.id)).toEqual(['p4', 'p3', 'p1']);
    expect(list.find((e) => e.pane.id === 'p1')?.here).toBe(true);
  });
  it('within a rank, the longest waiting first', () => {
    const list = attentionList([tab('t', [pane('x', 'done', { rank: 3, since: 900 }), pane('y', 'done', { rank: 3, since: 100 })])], '');
    expect(list.map((e) => e.pane.id)).toEqual(['y', 'x']);
  });
});

describe('panelRows', () => {
  it('keeps every blocking entry and collapses finished ones beyond three', () => {
    const list = attentionList([tab('t', [
      pane('q', 'waitingAnswer', { rank: 1, since: 1 }),
      ...[1, 2, 3, 4, 5].map((n) => pane('d' + n, 'done', { rank: 3, since: n })),
    ])], '');
    const r = panelRows(list, false);
    expect(r.rows).toHaveLength(4);
    expect(r.hiddenDone).toBe(2);
    expect(panelRows(list, true).hiddenDone).toBe(0);
  });
});

describe('nextEntry', () => {
  const list = attentionList([tab('t', [
    pane('a', 'waitingPermission', { rank: 0, since: 1 }),
    pane('b', 'waitingAnswer', { rank: 1, since: 1 }),
    pane('c', 'done', { rank: 3, since: 1 }),
  ])], '');
  it('walks in order and wraps', () => {
    let e = nextEntry(list, null);
    expect(e?.pane.id).toBe('a');
    e = nextEntry(list, cursorOf(e!));
    expect(e?.pane.id).toBe('b');
    expect(nextEntry(list, cursorOf(list[2]))?.pane.id).toBe('a');
  });
  it('an entry that left the list does not send the walk back to the top', () => {
    const cursor = cursorOf(list[1]); // b, then b gets answered
    const without = list.filter((e) => e.pane.id !== 'b');
    expect(nextEntry(without, cursor)?.pane.id).toBe('c');
  });
});

describe('tabRank and tabGlyphs', () => {
  it('a question beats a working pane in either order', () => {
    expect(tabRank([pane('a', 'waitingAnswer', { rank: 1 }), pane('b', 'active', { rank: 4 })])).toBe(1);
    expect(tabRank([pane('b', 'active', { rank: 4 }), pane('a', 'waitingAnswer', { rank: 1 })])).toBe(1);
  });
  it('shows at most six glyphs and counts the rest', () => {
    const many = Array.from({ length: 8 }, (_, i) => pane('p' + i, 'active', { rank: 4 }));
    const g = tabGlyphs(many);
    expect(g.kinds).toHaveLength(6);
    expect(g.more).toBe(2);
  });
  it('glyphKind tells seen from unseen', () => {
    expect(glyphKind(pane('a', 'done', { rank: 3, seen: false }))).toBe('done');
    expect(glyphKind(pane('a', 'done', { rank: 5, seen: true }))).toBe('doneSeen');
  });
});

describe('shouldToast', () => {
  const att = (rank: number, since = 1, seen = false) => ({ state: '', rank, seen, detail: '', source: '', since });
  it('toasts a new permission, question or error', () => {
    expect(shouldToast(undefined, att(0), false)).toBe(true);
    expect(shouldToast(att(4), att(2), false)).toBe(true);
  });
  it('never for done, never for the pane being looked at, not twice', () => {
    expect(shouldToast(undefined, att(3), false)).toBe(false);
    expect(shouldToast(undefined, att(0), true)).toBe(false);
    expect(shouldToast(att(0, 5), att(0, 5, true), false)).toBe(false);
  });
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd frontend && npx vitest run src/lib/attention.test.ts`
Expected: FAIL, Modul nicht gefunden

- [ ] **Step 3: `stores/tabs.ts`**

Vor `export interface Pane`:

```ts
/** What the host says a pane wants from the user (terminal:attention).
 *  Absent until the host sent one; a daemon from before never does. */
export interface PaneAttention {
  state: string;
  /** 0 permission, 1 question, 2 error, 3 done unseen, 4 working, 5 quiet. */
  rank: number;
  seen: boolean;
  detail: string;
  source: string;
  /** Unix seconds; 0 = unknown. */
  since: number;
}
```

In `Pane`, nach `activitySince`: `attention?: PaneAttention;`

Entfernen: das Feld `unreadActivity` im `Tab`, die Funktion `computeTabActivity` samt Kommentar, die Zeilen `if (tab.id !== state.activeTabId) { tab.unreadActivity = computeTabActivity(tab.panes); }` in `applyActivity`, `unreadActivity: null,` in `addTab`, `if (tab) tab.unreadActivity = null;` in `setActiveTab`, die Zuweisung in `closePane` (Zeile 317) und `unreadActivity: null` in `importTab`. Danach muss `grep -rn "unreadActivity\|computeTabActivity" frontend/src` nur noch Testdateien zeigen.

Neue Store-Methode (nach `updateActivity`):

```ts
    applyAttention(sessionId: SessionRef, att: PaneAttention) {
      update((state) => {
        for (const tab of state.tabs) {
          const pane = tab.panes.find((p) => p.sessionId === sessionId);
          if (pane) {
            pane.attention = att;
            return state;
          }
        }
        return state;
      });
    },
```

- [ ] **Step 4: `lib/attention.ts`**

```ts
import { paneDisplayName, type Pane, type PaneAttention, type Tab } from '../stores/tabs';

// The frontend half of the attention model (spec 2026-10-07). The host decides
// the rank; this file only sorts, groups and walks what the host said.

export const RANK_PERMISSION = 0;
export const RANK_QUESTION = 1;
export const RANK_ERROR = 2;
export const RANK_DONE = 3;
export const RANK_WORKING = 4;
export const RANK_QUIET = 5;

export const ENTRY_LABEL: Record<number, string> = { 0: 'Freigabe nötig', 1: 'Frage', 2: 'Fehler', 3: 'fertig' };

/** A pane's rank: the host's, or, without a record (a daemon from before),
 *  read from the activity with "done" and "error" taken as seen. That is how
 *  the app behaved before, so an old daemon does not flood the list. */
export function paneRank(pane: Pane): number {
  if (pane.attention) return pane.attention.rank;
  switch (pane.activity) {
    case 'waitingPermission': return RANK_PERMISSION;
    case 'waitingAnswer': return RANK_QUESTION;
    case 'active': return RANK_WORKING;
    default: return RANK_QUIET;
  }
}

export type GlyphKind = 'permission' | 'question' | 'error' | 'errorSeen' | 'done' | 'doneSeen' | 'working' | 'quiet';

export const GLYPH_LABEL: Record<GlyphKind, string> = {
  permission: 'braucht Freigabe', question: 'hat eine Frage', error: 'Fehler', errorSeen: 'Fehler, gesehen',
  done: 'fertig, ungesehen', doneSeen: 'fertig', working: 'läuft', quiet: 'ruht',
};

/** Which status glyph a pane shows. The shape carries the meaning. */
export function glyphKind(pane: Pane): GlyphKind {
  const state = pane.attention?.state || pane.activity;
  const seen = pane.attention ? pane.attention.seen : true;
  switch (state) {
    case 'waitingPermission': return 'permission';
    case 'waitingAnswer': return 'question';
    case 'error': return seen ? 'errorSeen' : 'error';
    case 'done': return seen ? 'doneSeen' : 'done';
    case 'active': return 'working';
    default: return 'quiet';
  }
}

export interface AttentionEntry {
  tabId: string;
  tabName: string;
  /** The entry belongs to the tab currently shown. */
  here: boolean;
  pane: Pane;
  rank: number;
  /** Unix seconds; 0 = unknown. */
  since: number;
}

const sinceKey = (since: number) => since || Number.MAX_SAFE_INTEGER;

/** Everything that waits, from every tab, most urgent first; within a rank the
 *  longest waiting first. An unknown start sorts after the known ones. */
export function attentionList(tabs: Tab[], currentTabId: string): AttentionEntry[] {
  const out: AttentionEntry[] = [];
  for (const tab of tabs) {
    for (const pane of tab.panes) {
      const rank = paneRank(pane);
      if (rank > RANK_DONE) continue;
      out.push({ tabId: tab.id, tabName: tab.name, here: tab.id === currentTabId, pane, rank,
        since: pane.attention?.since || pane.activitySince || 0 });
    }
  }
  return out.sort((a, b) => a.rank - b.rank || sinceKey(a.since) - sinceKey(b.since) || a.pane.id.localeCompare(b.pane.id));
}

export interface PanelRows { rows: AttentionEntry[]; hiddenDone: number }

/** What the panel shows: everything blocking or failed, and at most `keep`
 *  finished entries unless expanded. */
export function panelRows(list: AttentionEntry[], expanded: boolean, keep = 3): PanelRows {
  const top = list.filter((e) => e.rank < RANK_DONE);
  const done = list.filter((e) => e.rank === RANK_DONE);
  const shown = expanded ? done : done.slice(0, keep);
  return { rows: [...top, ...shown], hiddenDone: done.length - shown.length };
}

export interface AttentionCursor { rank: number; since: number; paneId: string }

export const cursorOf = (e: AttentionEntry): AttentionCursor => ({ rank: e.rank, since: sinceKey(e.since), paneId: e.pane.id });

function isAfter(e: AttentionEntry, c: AttentionCursor): boolean {
  if (e.rank !== c.rank) return e.rank > c.rank;
  const s = sinceKey(e.since);
  if (s !== c.since) return s > c.since;
  return e.pane.id.localeCompare(c.paneId) > 0;
}

/** The entry after the cursor, wrapping to the first. An entry that left the
 *  list (seen, answered) does not send the walk back to the top. */
export function nextEntry(list: AttentionEntry[], cursor: AttentionCursor | null): AttentionEntry | null {
  if (list.length === 0) return null;
  if (!cursor) return list[0];
  return list.find((e) => isAfter(e, cursor)) ?? list[0];
}

/** A tab's rank: its most urgent pane, whatever the pane order. */
export function tabRank(panes: Pane[]): number {
  return panes.reduce((min, p) => Math.min(min, paneRank(p)), RANK_QUIET);
}

/** One glyph per pane, at most `max`; `more` counts the rest. */
export function tabGlyphs(panes: Pane[], max = 6): { kinds: GlyphKind[]; more: number } {
  return { kinds: panes.slice(0, max).map(glyphKind), more: Math.max(0, panes.length - max) };
}

/** The tab tooltip: full name, then every pane with its state. */
export function tabTooltip(tab: Tab): string {
  return [tab.name, ...tab.panes.map((p) => `${paneDisplayName(p)}: ${GLYPH_LABEL[glyphKind(p)]}`)].join('\n');
}

/** A toast only for something new that blocks or failed, never for "done"
 *  and never for the pane the user is looking at in a focused window. */
export function shouldToast(before: PaneAttention | undefined, next: PaneAttention, lookedAt: boolean): boolean {
  if (next.rank > RANK_ERROR || next.seen || lookedAt) return false;
  return !before || before.rank !== next.rank || before.since !== next.since;
}
```

- [ ] **Step 5: Übergang in `TabBar.svelte`** (bis Task 13 die Zeichen bringt)

Import: `import { tabRank } from '../lib/attention';`. Den Block `{#if tab.unreadActivity} ... {/if}` ersetzen durch:

```svelte
        {#if tab.id !== activeTabId && tabRank(tab.panes) <= 3}
          {@const r = tabRank(tab.panes)}
          <span class="tab-activity-dot {r <= 1 ? 'tab-dot-waitingAnswer' : r === 2 ? 'tab-dot-error' : 'tab-dot-done'}"></span>
        {/if}
```

- [ ] **Step 6: Listener in `App.svelte`** (neben `terminal:activity`)

```ts
    EventsOn('terminal:attention', (event: any) => {
      const info = event.data; // AttentionInfo { id, state, rank, seen, detail, source, since }
      tabStore.applyAttention(info.id, {
        state: info.state, rank: info.rank, seen: info.seen,
        detail: info.detail ?? '', source: info.source ?? '', since: info.since ?? 0,
      });
    });
```

- [ ] **Step 7: Alte Tests anpassen**

In `stores/tabs.test.ts` die `describe`-Blöcke zu `computeTabActivity` und alle Erwartungen auf `unreadActivity` löschen. Ersatz:

```ts
  it('applyAttention puts the record on the pane with that session', () => {
    const tabId = tabStore.addTab('att', 'C:\\att');
    tabStore.addPane(tabId, 'h:77', 'P', 'claude', '');
    tabStore.applyAttention('h:77', { state: 'done', rank: 3, seen: false, detail: 'x', source: 'Hook: Stop', since: 5 });
    const pane = get(allTabs).flatMap((t) => t.panes).find((p) => p.sessionId === 'h:77');
    expect(pane?.attention?.rank).toBe(3);
  });
```

(`get` aus `svelte/store` sowie `tabStore` und `allTabs` aus `./tabs` importieren, falls die Datei sie noch nicht importiert.) In den übrigen Testdateien `unreadActivity: null` aus Fixture-Objekten entfernen.

- [ ] **Step 8: Run the frontend tests**

Run: `cd frontend && npx vitest run`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add frontend/src/
git commit -m "feat(frontend): attention model, one rank per tab from the host" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 11: „Gesehen“ melden

**Files:**
- Create: `frontend/src/lib/seenTracker.ts`, `frontend/src/lib/seenTracker.test.ts`
- Modify: `frontend/src/App.svelte` (Tracker, Fensterfokus)
- Modify: `frontend/src/components/TerminalPane.svelte` (`onData`)

**Interfaces:**
- Consumes: `MarkSessionSeen` (Task 7), `Pane.attention` (Task 10)
- Produces: `interface SeenTarget {ref; rank; seen}`, `needsSeen(t)`, `seenTargetOf(pane)`, `createSeenTracker(markSeen, delayMs?, timers?)` mit `look(t)`, `windowFocus(b)`, `typed(t)`; in `App.svelte` die Variable `lookedAtRef: string` (für Task 14).

- [ ] **Step 1: Write the failing tests**

```ts
import { describe, it, expect } from 'vitest';
import { createSeenTracker } from './seenTracker';

function fakeTimers() {
  let fn: (() => void) | null = null;
  return {
    timers: { set: (f: () => void) => { fn = f; return 1; }, clear: () => { fn = null; } },
    fire: () => { const f = fn; fn = null; f?.(); },
    armed: () => fn !== null,
  };
}
const done = (ref = 'h:1', seen = false) => ({ ref, rank: 3, seen });

describe('seenTracker', () => {
  it('marks a finished pane after it was looked at for the delay', () => {
    const marked: string[] = [];
    const ft = fakeTimers();
    const t = createSeenTracker((r) => marked.push(r), 1000, ft.timers);
    t.look(done());
    expect(marked).toEqual([]);
    ft.fire();
    expect(marked).toEqual(['h:1']);
  });
  it('does nothing for a permission request, which only the agent clears', () => {
    const ft = fakeTimers();
    const t = createSeenTracker(() => {}, 1000, ft.timers);
    t.look({ ref: 'h:1', rank: 0, seen: false });
    expect(ft.armed()).toBe(false);
  });
  it('a pane that finishes while being looked at is marked too', () => {
    const marked: string[] = [];
    const ft = fakeTimers();
    const t = createSeenTracker((r) => marked.push(r), 1000, ft.timers);
    t.look({ ref: 'h:1', rank: 4, seen: false });
    expect(ft.armed()).toBe(false);
    t.look(done());
    ft.fire();
    expect(marked).toEqual(['h:1']);
  });
  it('looking away or losing window focus cancels', () => {
    const marked: string[] = [];
    const ft = fakeTimers();
    const t = createSeenTracker((r) => marked.push(r), 1000, ft.timers);
    t.look(done());
    t.look(done('h:2', true));
    expect(ft.armed()).toBe(false);
    t.look(done());
    t.windowFocus(false);
    expect(ft.armed()).toBe(false);
    expect(marked).toEqual([]);
  });
  it('typing marks at once', () => {
    const marked: string[] = [];
    const t = createSeenTracker((r) => marked.push(r), 1000, fakeTimers().timers);
    t.typed(done());
    t.typed(done('h:2', true));
    expect(marked).toEqual(['h:1']);
  });
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd frontend && npx vitest run src/lib/seenTracker.test.ts`
Expected: FAIL

- [ ] **Step 3: `seenTracker.ts`**

```ts
import type { SessionRef } from './sessionRef';

export interface SeenTarget { ref: SessionRef; rank: number; seen: boolean }

/** Only "error" and "done" are cleared by looking; a permission or a question
 *  stays until the agent moves on (spec Teil A). */
export function needsSeen(t: SeenTarget | null | undefined): t is SeenTarget {
  return !!t && !!t.ref && !t.seen && (t.rank === 2 || t.rank === 3);
}

/** The seen target for a pane, null without an attention record. */
export function seenTargetOf(pane: { sessionId: SessionRef; attention?: { rank: number; seen: boolean } } | null | undefined): SeenTarget | null {
  if (!pane || !pane.sessionId || !pane.attention) return null;
  return { ref: pane.sessionId, rank: pane.attention.rank, seen: pane.attention.seen };
}

export interface Timers { set: (fn: () => void, ms: number) => unknown; clear: (h: unknown) => void }

const realTimers: Timers = { set: (fn, ms) => setTimeout(fn, ms), clear: (h) => clearTimeout(h as ReturnType<typeof setTimeout>) };

/** Marks a finished or failed pane as seen once the user looked at it for
 *  `delayMs` with the window focused, or typed into it. */
export function createSeenTracker(markSeen: (ref: SessionRef) => void, delayMs = 1000, timers: Timers = realTimers) {
  let target: SeenTarget | null = null;
  let windowFocused = true;
  let timer: unknown = null;

  const disarm = () => {
    if (timer !== null) { timers.clear(timer); timer = null; }
  };
  const arm = () => {
    if (timer !== null || !windowFocused || !needsSeen(target)) return;
    const ref = target.ref;
    timer = timers.set(() => {
      timer = null;
      if (windowFocused && target?.ref === ref && needsSeen(target)) markSeen(ref);
    }, delayMs);
  };

  return {
    /** The pane the user is looking at, or its state, changed. */
    look(next: SeenTarget | null) {
      if (next?.ref !== target?.ref) disarm();
      target = next;
      if (needsSeen(target)) arm(); else disarm();
    },
    windowFocus(focused: boolean) {
      windowFocused = focused;
      if (focused) arm(); else disarm();
    },
    typed(t: SeenTarget | null) {
      if (needsSeen(t)) markSeen(t.ref);
    },
  };
}
```

- [ ] **Step 4: Wire it**

`App.svelte`, Importe: `import { createSeenTracker, seenTargetOf } from './lib/seenTracker';`. Im Skript, nach der Deklaration von `floatHasKeyboard`:

```ts
  const seenTracker = createSeenTracker((ref) => { App.MarkSessionSeen(ref).catch(() => {}); });
  // The ref of the pane the user is looking at; '' for none. Read by the
  // toast rule, which never fires for this pane.
  let lookedAtRef = '';
  // The pane being looked at: the floating one while it has the keyboard,
  // otherwise the active tab's focused pane. A function call, so only the
  // arguments are dependencies (CLAUDE.md).
  $: trackLookedAt($allTabs, $activeTab, $floatingPane, floatHasKeyboard);
  function trackLookedAt(tabs: typeof $allTabs, tab: typeof $activeTab, float: typeof $floatingPane, floatKeys: boolean) {
    let pane = null;
    if (float && floatKeys) pane = tabs.find((t) => t.id === float.tabId)?.panes.find((p) => p.id === float.paneId) ?? null;
    else if (tab) pane = tab.panes.find((p) => p.id === tab.focusedPaneId) ?? null;
    lookedAtRef = pane?.sessionId ?? '';
    seenTracker.look(seenTargetOf(pane));
  }
  onMount(() => {
    const onFocus = () => seenTracker.windowFocus(true);
    const onBlur = () => seenTracker.windowFocus(false);
    window.addEventListener('focus', onFocus);
    window.addEventListener('blur', onBlur);
    seenTracker.windowFocus(document.hasFocus());
    return () => { window.removeEventListener('focus', onFocus); window.removeEventListener('blur', onBlur); };
  });
```

(`onMount` ist in `App.svelte` bereits importiert; sonst ergänzen.)

`TerminalPane.svelte`, Import `import { needsSeen, seenTargetOf } from '../lib/seenTracker';`, im `onData`-Handler vor `App.WriteToSession(...)`:

```ts
        // Typing into a finished or failed pane means it has been seen.
        const target = seenTargetOf(pane);
        if (needsSeen(target)) App.MarkSessionSeen(pane.sessionId).catch(() => {});
```

- [ ] **Step 5: Run the tests**

Run: `cd frontend && npx vitest run`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add frontend/src/
git commit -m "feat(frontend): a pane looked at for a second counts as seen" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 12: Der Kasten „Wartet auf dich“

**Files:**
- Create: `frontend/src/components/StatusGlyph.svelte`, `WaitingPanel.svelte`, `AttentionMenu.svelte`
- Create: `frontend/src/lib/attentionNav.ts`, `frontend/src/lib/attentionNav.test.ts`
- Modify: `frontend/src/stores/theme.ts` (`SEMANTIC`)
- Modify: `frontend/src/lib/focusLayout.ts` (`focusGeometry` Listenhöhe; `waitingElsewhere`, `isWaiting`, `nextWaiting`, `WaitingEntry` entfernen)
- Modify: `frontend/src/lib/focusLayout.test.ts`
- Modify: `frontend/src/components/PaneGrid.svelte`, `PaneTitlebar.svelte`, `TerminalPane.svelte` (Ctrl+Shift+K durchlassen)
- Modify: `frontend/src/lib/shortcuts.ts`, `frontend/src/App.svelte`

**Interfaces:**
- Consumes: `attentionList`, `panelRows`, `nextEntry`, `cursorOf`, `glyphKind`, `ENTRY_LABEL` (Task 10)
- Produces: `<StatusGlyph kind size mono>`, `<WaitingPanel rows total hiddenDone expanded selectedPaneId on:jump on:preview on:toggleMore on:menu>`, `<AttentionMenu entry pane x y layoutMode on:close>`, aus `attentionNav.ts`: `jumpTo(tabId, paneId)`, `preview(entry, mode)`, `next(mode)`, `back()`, `walkPaneId` (Store); `focusGeometry(w, h, paneCount, listRows)`; Shortcut-Callback `onBackFromWaiting`.

- [ ] **Step 1: Write the failing tests**

`attentionNav.test.ts`:

```ts
import { describe, it, expect, beforeEach } from 'vitest';
import { get } from 'svelte/store';
import { tabStore, allTabs, activeTab } from '../stores/tabs';
import { floatingPane } from '../stores/focusLayout';
import { attentionList } from './attention';
import { jumpTo, preview, next, back } from './attentionNav';

function setup() {
  for (const t of get(allTabs)) tabStore.forceCloseTab(t.id);
  const a = tabStore.addTab('alpha', 'C:\\a');
  const b = tabStore.addTab('beta', 'C:\\b', false);
  const pa = tabStore.addPane(a, 'h:1', 'A', 'claude', '');
  const pb = tabStore.addPane(b, 'h:2', 'B', 'claude', '');
  tabStore.applyAttention('h:2', { state: 'waitingPermission', rank: 0, seen: false, detail: '', source: '', since: 1 });
  tabStore.applyAttention('h:1', { state: 'done', rank: 3, seen: false, detail: '', source: '', since: 1 });
  tabStore.setActiveTab(a);
  return { a, b, pa, pb };
}

describe('attentionNav', () => {
  beforeEach(() => { floatingPane.set(null); back(); });

  it('next opens the most urgent entry as a preview in the focus layout', () => {
    const { a, b, pb } = setup();
    next('focus');
    expect(get(floatingPane)).toEqual({ tabId: b, paneId: pb });
    expect(get(activeTab)?.id).toBe(a);
  });

  it('back returns to where the walk started', () => {
    const { a, pa } = setup();
    next('grid');
    expect(get(activeTab)?.id).not.toBe(a);
    back();
    expect(get(activeTab)?.id).toBe(a);
    expect(get(activeTab)?.focusedPaneId).toBe(pa);
  });

  it('a preview of an entry in the current tab focuses it instead (Review Focus 5)', () => {
    const { a, pa } = setup();
    const here = attentionList(get(allTabs), a).find((e) => e.pane.id === pa)!;
    preview(here, 'focus');
    expect(get(floatingPane)).toBeNull();
    expect(get(activeTab)?.focusedPaneId).toBe(pa);
  });

  it('jumpTo switches tab and focuses the pane', () => {
    const { b, pb } = setup();
    jumpTo(b, pb);
    expect(get(activeTab)?.id).toBe(b);
    expect(get(activeTab)?.focusedPaneId).toBe(pb);
  });
});
```

(Falls `forceCloseTab` den letzten Tab nicht schließt oder `addPane` eine andere Signatur hat, an `tabs.test.ts` angleichen: dort stehen die gültigen Aufrufe.)

`focusLayout.test.ts`: Die Tests zu `waitingElsewhere` und `nextWaiting` löschen. Im Block `describe('focusGeometry')` ergänzen:

```ts
  it('shrinks the list to one line when nothing waits', () => {
    const g = focusGeometry(2000, 1000, 4, 0);
    expect(g.list.h).toBe(36);
  });

  it('caps the list at 40 % of the column', () => {
    const g = focusGeometry(2000, 1000, 4, 50);
    expect(g.list.h).toBeLessThanOrEqual(Math.round((1000 - 8) * 0.4));
  });
```

- [ ] **Step 2: Run them to verify they fail**

Run: `cd frontend && npx vitest run src/lib/attentionNav.test.ts src/lib/focusLayout.test.ts`
Expected: FAIL

- [ ] **Step 3: Theme** (`stores/theme.ts`)

Im Kommentar der Farbsprache eine Zeile ergänzen: `//   blue   = finished, not yet seen`. In `SEMANTIC` nach `'status-idle'`:

```ts
  'status-done': '#4f9cf0',
  'status-done-tint': 'rgba(79,156,240,.16)',
```

- [ ] **Step 4: `focusGeometry`** (`lib/focusLayout.ts`)

Konstanten: `const LIST_HEAD = 34;`, `const LIST_ROW = 46;`, neu `const LIST_QUIET = 36;`. Signatur `focusGeometry(w: number, h: number, paneCount: number, listRows: number)`. Die Zeile `const listH = ...` wird zu:

```ts
  // An empty list is one quiet line; the small panes get the room back. A
  // busy one grows with its rows up to 40 % of the column and then scrolls.
  const listH = listRows === 0
    ? LIST_QUIET
    : Math.round(Math.min(innerH * 0.4, LIST_HEAD + listRows * LIST_ROW + 8));
```

`WaitingEntry`, `isWaiting`, `waitingElsewhere` und `nextWaiting` samt Kommentaren löschen (`floatingStillValid` bleibt).

- [ ] **Step 5: `StatusGlyph.svelte`**

```svelte
<script lang="ts">
  import { GLYPH_LABEL, type GlyphKind } from '../lib/attention';

  export let kind: GlyphKind;
  export let size = 12;
  /** Draw in the muted text colour, where colour would be noise. */
  export let mono = false;

  const COLOR: Record<GlyphKind, string> = {
    permission: 'var(--status-waiting)', question: 'var(--status-waiting)',
    error: 'var(--status-danger)', errorSeen: 'var(--status-idle)',
    done: 'var(--status-done)', doneSeen: 'var(--status-idle)',
    working: 'var(--status-running)', quiet: 'var(--status-idle)',
  };
</script>

<svg class="status-glyph" class:spin={kind === 'working'} width={size} height={size} viewBox="0 0 12 12"
  style="color:{mono ? 'var(--fg-muted)' : COLOR[kind]}" role="img" aria-label={GLYPH_LABEL[kind]}>
  {#if kind === 'permission'}
    <path d="M6 1.2 11.3 10.6H.7Z" fill="currentColor" />
  {:else if kind === 'question'}
    <path d="M6 1.2a4.8 4.8 0 1 0 2.5 8.9l2.6.8-.8-2.5A4.8 4.8 0 0 0 6 1.2Z" fill="currentColor" />
  {:else if kind === 'error' || kind === 'errorSeen'}
    <path d="m2.6 2.6 6.8 6.8m0-6.8-6.8 6.8" stroke="currentColor" stroke-width={kind === 'error' ? 2.4 : 1.4} stroke-linecap="round" fill="none" />
  {:else if kind === 'done' || kind === 'doneSeen'}
    <path d="m1.8 6.4 2.8 2.8 5.6-6" stroke="currentColor" stroke-width={kind === 'done' ? 2.4 : 1.3} stroke-linecap="round" stroke-linejoin="round" fill="none" />
  {:else if kind === 'working'}
    <path d="M6 1.4a4.6 4.6 0 1 1-4.6 4.6" stroke="currentColor" stroke-width="2" stroke-linecap="round" fill="none" />
  {:else}
    <circle cx="6" cy="6" r="2.1" fill="currentColor" />
  {/if}
</svg>

<style>
  .status-glyph { flex: none; display: inline-block; vertical-align: middle; }
  .spin { animation: status-glyph-spin 1.1s linear infinite; }
  @keyframes status-glyph-spin { to { transform: rotate(360deg); } }
  @media (prefers-reduced-motion: reduce) { .spin { animation: none; } }
</style>
```

- [ ] **Step 6: `attentionNav.ts`**

```ts
import { get, writable } from 'svelte/store';
import { tabStore, allTabs, activeTab } from '../stores/tabs';
import { floatingPane, focusOrders, setFocusOrder } from '../stores/focusLayout';
import { promoteInOrder, reconcileOrder, type LayoutMode } from './focusLayout';
import { attentionList, nextEntry, cursorOf, type AttentionEntry, type AttentionCursor } from './attention';

// Moving through what waits: a click in the panel, Ctrl+Shift+J and
// Ctrl+Shift+K, a toast click. Module state on purpose: there is one walk per
// window, and the panel, the toolbar counter and the shortcuts share it.

let returnTo: { tabId: string; paneId: string } | null = null;
let cursor: AttentionCursor | null = null;

/** The pane the keyboard walk is on, for the panel's highlight; '' = none. */
export const walkPaneId = writable('');

function remember() {
  if (returnTo) return;
  const tab = get(activeTab);
  if (tab) returnTo = { tabId: tab.id, paneId: tab.focusedPaneId };
}

// In the focus layout a focused pane also moves to a big slot.
function focusIn(tabId: string, paneId: string) {
  const tab = get(allTabs).find((t) => t.id === tabId);
  if (tab) setFocusOrder(tabId, promoteInOrder(reconcileOrder(get(focusOrders)[tabId], tab.panes.map((p) => p.id)), paneId, true));
  tabStore.focusPane(tabId, paneId);
}

/** Switch to the pane's tab and give it the keyboard. */
export function jumpTo(tabId: string, paneId: string) {
  remember();
  floatingPane.set(null);
  if (get(activeTab)?.id !== tabId) tabStore.setActiveTab(tabId);
  focusIn(tabId, paneId);
}

/** Show the entry floating over the current tab. An entry of the current tab
 *  has its own slot, and the grid has no floating window: both are focused
 *  instead. */
export function preview(entry: AttentionEntry, mode: LayoutMode) {
  if (mode !== 'focus' || entry.here) {
    jumpTo(entry.tabId, entry.pane.id);
    return;
  }
  remember();
  floatingPane.set({ tabId: entry.tabId, paneId: entry.pane.id });
  tabStore.focusPane(entry.tabId, entry.pane.id);
}

/** Ctrl+Shift+J: the next entry in rank order. */
export function next(mode: LayoutMode): AttentionEntry | null {
  const e = nextEntry(attentionList(get(allTabs), get(activeTab)?.id ?? ''), cursor);
  if (!e) return null;
  cursor = cursorOf(e);
  walkPaneId.set(e.pane.id);
  preview(e, mode);
  return e;
}

/** Ctrl+Shift+K: back to the pane the user was in before the walk. */
export function back() {
  const t = returnTo;
  returnTo = null;
  cursor = null;
  walkPaneId.set('');
  floatingPane.set(null);
  if (!t) return;
  if (get(activeTab)?.id !== t.tabId) tabStore.setActiveTab(t.tabId);
  if (t.paneId) focusIn(t.tabId, t.paneId);
}

/** Jump to the pane with this session, from a toast click. */
export function jumpToSession(ref: string) {
  const tab = get(allTabs).find((t) => t.panes.some((p) => p.sessionId === ref));
  const pane = tab?.panes.find((p) => p.sessionId === ref);
  if (tab && pane) jumpTo(tab.id, pane.id);
}
```

- [ ] **Step 7: `WaitingPanel.svelte`**

```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import StatusGlyph from './StatusGlyph.svelte';
  import { paneDisplayName } from '../stores/tabs';
  import { glyphKind, ENTRY_LABEL, type AttentionEntry } from '../lib/attention';
  import { formatDuration } from '../lib/duration';
  import { now } from '../stores/clock';

  export let rows: AttentionEntry[] = [];
  export let total = 0;
  export let hiddenDone = 0;
  export let expanded = false;
  export let selectedPaneId = '';

  const dispatch = createEventDispatcher<{
    jump: AttentionEntry; preview: AttentionEntry; toggleMore: void;
    menu: { entry: AttentionEntry; x: number; y: number };
  }>();

  // A new permission or question lights up once; nothing pulses for long.
  let known = new Set<string>();
  let flashing = new Set<string>();
  let primed = false;
  $: noteNew(rows);
  function noteNew(list: AttentionEntry[]) {
    const keys = list.map((e) => `${e.pane.id}:${e.since}:${e.rank}`);
    if (primed) {
      const fresh = list.filter((e, i) => e.rank <= 1 && !known.has(keys[i])).map((e) => e.pane.id);
      if (fresh.length) {
        flashing = new Set(fresh);
        setTimeout(() => { flashing = new Set(); }, 1300);
      }
    }
    known = new Set(keys);
    primed = true;
  }

  function activate(e: MouseEvent | KeyboardEvent, entry: AttentionEntry) {
    dispatch(e.altKey ? 'preview' : 'jump', entry);
  }
  function onKey(e: KeyboardEvent, entry: AttentionEntry) {
    if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); activate(e, entry); }
  }
  function onMenu(e: MouseEvent, entry: AttentionEntry) {
    e.preventDefault();
    dispatch('menu', { entry, x: e.clientX, y: e.clientY });
  }
  function tip(entry: AttentionEntry): string {
    const src = entry.pane.attention?.source;
    const at = entry.since ? new Date(entry.since * 1000).toLocaleTimeString('de-DE', { hour: '2-digit', minute: '2-digit' }) : '';
    return [src ? `Quelle: ${src}` : '', at ? `Seit: ${at} Uhr` : '', 'Alt+Klick: Vorschau · Rechtsklick: mehr'].filter(Boolean).join('\n');
  }
</script>

{#if total === 0}
  <div class="wp quiet"><span class="wp-title">Wartet auf dich</span><span class="wp-q">Nichts wartet</span></div>
{:else}
  <div class="wp">
    <div class="wp-head">
      <span class="wp-title">Wartet auf dich</span><span class="wp-n">{total}</span>
      <kbd class="wp-key" title="Nächsten Eintrag öffnen">Strg+Umschalt+J</kbd>
    </div>
    <div class="wp-rows">
      {#each rows as entry (entry.pane.id)}
        <div class="wp-entry r{entry.rank}" class:sel={entry.pane.id === selectedPaneId} class:flash={flashing.has(entry.pane.id)}
          role="button" tabindex="0" title={tip(entry)}
          aria-label="{ENTRY_LABEL[entry.rank]}: {entry.here ? 'dieses Projekt' : entry.tabName}, {paneDisplayName(entry.pane)}"
          on:click={(e) => activate(e, entry)} on:keydown={(e) => onKey(e, entry)} on:contextmenu={(e) => onMenu(e, entry)}>
          <div class="l1">
            <StatusGlyph kind={glyphKind(entry.pane)} />
            <span class="proj" class:here={entry.here}>{entry.here ? 'hier' : entry.tabName}</span>
            <span class="sep">·</span>
            <span class="pane">{paneDisplayName(entry.pane)}</span>
            <span class="age">{entry.since ? formatDuration(entry.since, $now) : ''}</span>
          </div>
          <div class="l2" class:mono={entry.rank === 0 || entry.rank === 2}>{entry.pane.attention?.detail || ENTRY_LABEL[entry.rank]}</div>
        </div>
      {/each}
      {#if hiddenDone > 0}
        <button class="more" on:click={() => dispatch('toggleMore')}>+{hiddenDone} weitere fertig</button>
      {:else if expanded}
        <button class="more" on:click={() => dispatch('toggleMore')}>weniger anzeigen</button>
      {/if}
    </div>
  </div>
{/if}

<style>
  .wp { flex: 1; min-width: 0; min-height: 0; display: flex; flex-direction: column; background: var(--pane-bg); border: 1px solid var(--pane-border); border-radius: 6px; overflow: hidden; font-family: var(--font-sans); }
  .wp.quiet { flex-direction: row; align-items: center; gap: 10px; padding: 0 12px; }
  .wp-q { color: var(--fg-muted); font-size: 12px; }
  .wp-head { flex: none; display: flex; align-items: center; gap: 8px; padding: 7px 10px; border-bottom: 1px solid var(--pane-border); font-size: 12px; }
  .wp-title { font-weight: 600; font-size: 12px; }
  .wp-n { font: 500 11px var(--font-mono); padding: 0 6px; border-radius: 9px; background: var(--bg-tertiary); font-variant-numeric: tabular-nums; }
  .wp-key { margin-left: auto; font: 10.5px var(--font-mono); color: var(--fg-muted); border: 1px solid var(--border); border-radius: 4px; padding: 1px 5px; }
  .wp-rows { overflow-y: auto; min-height: 0; padding: 4px; display: flex; flex-direction: column; gap: 2px; }
  .wp-entry { padding: 5px 7px; border-radius: 5px; cursor: pointer; display: flex; flex-direction: column; gap: 2px; border: 1px solid transparent; }
  .wp-entry:hover { background: var(--bg-tertiary); }
  .wp-entry.r0 { background: var(--status-waiting-tint); }
  .wp-entry.sel { border-color: var(--border-focused); }
  .wp-entry:focus-visible { outline: 2px solid var(--border-focused); outline-offset: -1px; }
  .wp-entry.flash { animation: wp-flash 1.2s ease-out 1; }
  .l1 { display: flex; align-items: center; gap: 6px; font-size: 12px; min-width: 0; }
  .proj { font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; flex: 0 1 auto; }
  .proj.here { color: var(--accent); }
  .sep { color: var(--fg-muted); flex: none; }
  .pane { color: var(--fg-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; flex: 0 1 auto; }
  .age { margin-left: auto; padding-left: 6px; flex: none; font: 10.5px var(--font-mono); color: var(--fg-muted); font-variant-numeric: tabular-nums; }
  .l2 { padding-left: 18px; font-size: 11.5px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .l2.mono { font: 11px var(--font-mono); }
  .more { align-self: flex-start; margin: 2px 0 4px 18px; background: none; border: 0; padding: 3px 0; color: var(--fg-muted); font: 500 11.5px var(--font-sans); cursor: pointer; }
  .more:hover { color: var(--fg); }
  @keyframes wp-flash {
    0% { box-shadow: 0 0 0 0 color-mix(in srgb, var(--status-waiting) 70%, transparent); background: color-mix(in srgb, var(--status-waiting) 38%, transparent); }
    100% { box-shadow: 0 0 0 10px transparent; }
  }
  @media (prefers-reduced-motion: reduce) { .wp-entry.flash { animation: none; } }
</style>
```

- [ ] **Step 8: `AttentionMenu.svelte`**

```svelte
<script lang="ts">
  import { createEventDispatcher } from 'svelte';
  import { get } from 'svelte/store';
  import * as App from '../../wailsjs/go/backend/App';
  import { allTabs, type Pane } from '../stores/tabs';
  import { paneRank, RANK_DONE, RANK_ERROR, type AttentionEntry } from '../lib/attention';
  import { jumpTo, preview } from '../lib/attentionNav';
  import type { LayoutMode } from '../lib/focusLayout';

  /** Either an entry of the waiting list, or a pane (its title bar). */
  export let entry: AttentionEntry | null = null;
  export let pane: Pane | null = null;
  export let x = 0;
  export let y = 0;
  export let layoutMode: LayoutMode = 'grid';

  const dispatch = createEventDispatcher<{ close: void }>();
  const target = entry ? entry.pane : pane;
  const rank = target ? paneRank(target) : 99;
  const att = target?.attention;
  const canSee = rank === RANK_ERROR || rank === RANK_DONE;
  const canUnsee = !!att && att.seen && (att.state === 'done' || att.state === 'error');

  function run(fn: () => void) { fn(); dispatch('close'); }
  function seenAllDone() {
    for (const p of get(allTabs).flatMap((t) => t.panes)) {
      if (paneRank(p) === RANK_DONE) App.MarkSessionSeen(p.sessionId).catch(() => {});
    }
  }
</script>

<svelte:window on:keydown={(e) => e.key === 'Escape' && dispatch('close')} />
<!-- svelte-ignore a11y-click-events-have-key-events a11y-no-static-element-interactions -->
<div class="am-overlay" on:click={() => dispatch('close')} on:contextmenu|preventDefault={() => dispatch('close')}>
  <div class="am" role="menu" style="left:{x}px; top:{y}px" on:click|stopPropagation>
    {#if entry}
      <button role="menuitem" on:click={() => run(() => jumpTo(entry.tabId, entry.pane.id))}>Hingehen</button>
      {#if !entry.here && layoutMode === 'focus'}
        <button role="menuitem" on:click={() => run(() => preview(entry, layoutMode))}>Vorschau öffnen</button>
      {/if}
    {/if}
    {#if canSee && target}
      <button role="menuitem" on:click={() => run(() => App.MarkSessionSeen(target.sessionId).catch(() => {}))}>Als gesehen markieren</button>
    {/if}
    {#if canUnsee && target}
      <button role="menuitem" on:click={() => run(() => App.MarkSessionUnseen(target.sessionId).catch(() => {}))}>Als ungelesen markieren</button>
    {/if}
    {#if entry}
      <div class="am-sep"></div>
      <button role="menuitem" on:click={() => run(seenAllDone)}>Alle fertigen als gesehen markieren</button>
    {/if}
    {#if !entry && !canSee && !canUnsee}
      <button role="menuitem" on:click={() => dispatch('close')}>Nichts zu markieren</button>
    {/if}
  </div>
</div>

<style>
  .am-overlay { position: fixed; inset: 0; z-index: 1000; }
  .am { position: fixed; min-width: 250px; display: flex; flex-direction: column; padding: 4px; background: var(--bg-secondary); border: 1px solid var(--border); border-radius: 6px; box-shadow: 0 10px 30px rgba(0,0,0,.4); }
  .am button { text-align: left; background: none; border: 0; color: var(--fg); font: 12.5px var(--font-sans); padding: 6px 10px; border-radius: 4px; cursor: pointer; }
  .am button:hover { background: var(--bg-tertiary); }
  .am-sep { border-top: 1px solid var(--border); margin: 4px 0; }
</style>
```

- [ ] **Step 9: `PaneGrid.svelte`**

Importe: aus `../lib/focusLayout` `waitingElsewhere` und `type WaitingEntry` entfernen; neu:

```ts
  import WaitingPanel from './WaitingPanel.svelte';
  import AttentionMenu from './AttentionMenu.svelte';
  import { attentionList, panelRows, type AttentionEntry } from '../lib/attention';
  import { jumpTo, preview, walkPaneId } from '../lib/attentionNav';
```

Die Zeilen `$: waiting = ...` und `$: geo = focusGeometry(...)` ersetzen durch:

```ts
  // Only the visible grid needs the list; hidden tabs would re-sort on every
  // update for nothing. It now holds every tab, the current one included.
  $: waitingAll = focus && active ? attentionList($allTabs, tabId) : [];
  let expanded = false;
  $: panel = panelRows(waitingAll, expanded);
  $: geo = focusGeometry(areaW, areaH, panes.length, panel.rows.length + (panel.hiddenDone > 0 || expanded ? 1 : 0));
  let menuFor: { entry: AttentionEntry; x: number; y: number } | null = null;
```

Die Funktion `openFloating` löschen. Den Block `{#if focus && active && !maximizedPane} <div class="waiting-list" ...> ... {/if}` ersetzen durch:

```svelte
  {#if focus && active && !maximizedPane}
    <div class="waiting-slot" style="left:{geo.list.x}px;top:{geo.list.y}px;width:{geo.list.w}px;height:{geo.list.h}px;">
      <WaitingPanel rows={panel.rows} total={waitingAll.length} hiddenDone={panel.hiddenDone} {expanded}
        selectedPaneId={$walkPaneId}
        on:jump={(e) => { releaseButtonFocus(); jumpTo(e.detail.tabId, e.detail.pane.id); }}
        on:preview={(e) => { releaseButtonFocus(); preview(e.detail, layoutMode); }}
        on:toggleMore={() => (expanded = !expanded)}
        on:menu={(e) => (menuFor = e.detail)} />
    </div>
  {/if}
  {#if menuFor}
    <AttentionMenu entry={menuFor.entry} x={menuFor.x} y={menuFor.y} {layoutMode} on:close={() => (menuFor = null)} />
  {/if}
```

CSS: alle Regeln `.waiting-list` und `.wl-*` löschen, neu: `.waiting-slot { position: absolute; display: flex; }`. Falls `dotClass` danach unbenutzt ist, ebenfalls löschen. Zum Schluss `grep -n "waiting\b\|WaitingEntry\|openFloating" frontend/src/components/PaneGrid.svelte`: Jeder verbliebene Leser von `waiting` liest jetzt `waitingAll`, und `WaitingEntry`/`openFloating` kommen nicht mehr vor.

- [ ] **Step 10: Pane-Titelleiste** (`PaneTitlebar.svelte`)

Importe: `import StatusGlyph from './StatusGlyph.svelte';`, `import AttentionMenu from './AttentionMenu.svelte';`, `import { glyphKind } from '../lib/attention';`. Im Skript: `let titleMenu: { x: number; y: number } | null = null;`. Am Wurzel-Element `<div class="pane-titlebar" ...>` ergänzen: `on:contextmenu|preventDefault={(e) => (titleMenu = { x: e.clientX, y: e.clientY })}`. Den Statuspunkt `<span class="status-dot {getActivityDot(pane.activity)}"></span>` ersetzen durch `<StatusGlyph kind={glyphKind(pane)} size={11} />`. Am Ende des Markups:

```svelte
{#if titleMenu}
  <AttentionMenu {pane} x={titleMenu.x} y={titleMenu.y} on:close={() => (titleMenu = null)} />
{/if}
```

`getActivityDot` löschen, falls danach unbenutzt.

- [ ] **Step 11: Tasten** (`shortcuts.ts`, `TerminalPane.svelte`, `App.svelte`)

`shortcuts.ts`: Interface um `onBackFromWaiting?: () => void;` ergänzen; nach dem Block für `J`:

```ts
    if (e.shiftKey && (e.key === 'K' || e.key === 'k')) {
      e.preventDefault();
      cb.onBackFromWaiting?.();
      return;
    }
```

`TerminalPane.svelte`, im Key-Handler nach der Zeile für Ctrl+Shift+J:

```ts
        // Ctrl+Shift+K goes back to where the walk through waiting panes started.
        if (e.ctrlKey && e.shiftKey && (e.key === 'K' || e.key === 'k')) return false;
```

`App.svelte`: Import `waitingElsewhere, nextWaiting` aus `./lib/focusLayout` entfernen; neu `import { next as nextAttention, back as backFromAttention } from './lib/attentionNav';`. Die Funktion `jumpToNextWaiting` ersetzen:

```ts
  /** Ctrl+Shift+J: the next pane that waits, from any tab. */
  function jumpToNextWaiting() {
    nextAttention(layoutMode);
  }
```

Im Shortcut-Objekt neben `onNextWaiting` ergänzen: `onBackFromWaiting: () => backFromAttention(),`.

- [ ] **Step 12: Run the tests**

Run: `cd frontend && npx vitest run`
Expected: PASS
Run: `cd frontend && npm run build`
Expected: Build ohne Fehler

- [ ] **Step 13: Sichtprüfung**

`wails dev` starten, Fokus-Layout einschalten (Einstellungen → Pane-Anordnung). Prüfen: Ein leerer Kasten ist eine Zeile hoch. Ein `claude`-Pane in einem zweiten Tab, das eine Frage stellt, erscheint mit Projektname und Fragetext. Klick springt hin, Alt+Klick öffnet die Vorschau, Rechtsklick zeigt das Menü. Ctrl+Shift+J/K funktionieren. Bild mit `C:\Users\Patrick.Hennig\repos\screenshotter\Take-Screenshot.ps1 -Screen All` aufnehmen und auf den MTUI-Monitor zuschneiden, Gesamtaufnahme danach löschen.

- [ ] **Step 14: Commit**

```bash
git add frontend/src/
git commit -m "feat(ui): the waiting box lists everything that waits, from every project" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 13: Tab-Leiste mit Pane-Zeichen, Zähler in der Werkzeugzeile

**Files:**
- Modify: `frontend/src/components/TabBar.svelte`
- Modify: `frontend/src/components/Toolbar.svelte`
- Modify: `frontend/src/App.svelte` (Props an die Toolbar)

**Interfaces:**
- Consumes: `tabRank`, `tabGlyphs`, `tabTooltip`, `attentionList`, `panelRows`, `glyphKind` (Task 10), `StatusGlyph`, `WaitingPanel`, `AttentionMenu`, `jumpTo`, `preview`, `walkPaneId` (Task 12)
- Produces: Toolbar-Props `waiting: AttentionEntry[]`, `showCounter: boolean`, `layoutMode: LayoutMode`.

- [ ] **Step 1: `TabBar.svelte`**

Importe: `import StatusGlyph from './StatusGlyph.svelte';` und `import { tabRank, tabGlyphs, tabTooltip } from '../lib/attention';`. Den Tab-Button so ändern:

```svelte
      <button
        class="tab rank-{tabRank(tab.panes)}"
        class:active={tab.id === activeTabId && !isDashboard}
        class:highlight={tab._highlight}
        title={tabTooltip(tab)}
        draggable="true"
        on:click={(e) => handleTabClick(e, tab.id)}
        on:dblclick={() => handleTabDblClick(tab.id)}
        on:dragstart={(e) => handleDragStart(e, tab.id)}
        on:dragend={(e) => handleDragEnd(e, tab.id)}
        on:contextmenu={(e) => handleContextMenu(e, tab.id)}
      >
        <span class="tab-name">{tab.name}</span>
        {#if tab.panes.length > 0}
          {@const g = tabGlyphs(tab.panes)}
          <span class="tab-glyphs">
            {#each g.kinds as kind}<StatusGlyph {kind} size={10} />{/each}
            {#if g.more > 0}<span class="tab-more">+{g.more}</span>{/if}
          </span>
        {/if}
        <!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
        <span class="tab-close" role="button" tabindex="-1" on:click={(e) => handleCloseTab(e, tab.id)}>
          &times;
        </span>
      </button>
```

CSS: Die Regeln `.tab-count`, `.tab-activity-dot`, `.tab-dot-*` und deren Keyframes löschen. Neu:

```css
  .tab-glyphs { display: inline-flex; align-items: center; gap: 2px; flex: none; }
  .tab-more { font-size: 10px; color: var(--fg-muted); margin-left: 2px; }
  .tab-close { opacity: 0; }
  .tab:hover .tab-close, .tab.active .tab-close { opacity: 1; }
  .tab.rank-0:not(.active), .tab.rank-1:not(.active) { background: var(--status-waiting-tint); color: var(--fg); }
  .tab.rank-2:not(.active) { background: var(--status-danger-tint); color: var(--fg); }
  .tab.rank-3:not(.active) { background: var(--status-done-tint); color: var(--fg); }
```

Prüfen, dass `.tab-name` `min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;` hat; sonst ergänzen.

- [ ] **Step 2: `Toolbar.svelte`**

Skript ergänzen:

```ts
  import StatusGlyph from './StatusGlyph.svelte';
  import WaitingPanel from './WaitingPanel.svelte';
  import AttentionMenu from './AttentionMenu.svelte';
  import { glyphKind, panelRows, type AttentionEntry } from '../lib/attention';
  import { jumpTo, preview, walkPaneId } from '../lib/attentionNav';
  import type { LayoutMode } from '../lib/focusLayout';

  /** Everything that waits, from every tab (attentionList). */
  export let waiting: AttentionEntry[] = [];
  /** Shown where the focus layout's box is not: the grid, or a maximized pane. */
  export let showCounter = false;
  export let layoutMode: LayoutMode = 'grid';

  let open = false;
  let expanded = false;
  let menuFor: { entry: AttentionEntry; x: number; y: number } | null = null;
  $: rows = panelRows(waiting, expanded);
```

Im Markup vor dem Button „Neues Terminal“:

```svelte
  {#if showCounter && waiting.length > 0}
    <div class="wait-wrap">
      <button class="wait-counter rank-{waiting[0].rank}" aria-expanded={open} on:click={() => (open = !open)}>
        <StatusGlyph kind={glyphKind(waiting[0].pane)} /> {waiting.length} warten
      </button>
      {#if open}
        <div class="wait-pop">
          <WaitingPanel rows={rows.rows} total={waiting.length} hiddenDone={rows.hiddenDone} {expanded}
            selectedPaneId={$walkPaneId}
            on:jump={(e) => { open = false; jumpTo(e.detail.tabId, e.detail.pane.id); }}
            on:preview={(e) => { open = false; preview(e.detail, layoutMode); }}
            on:toggleMore={() => (expanded = !expanded)}
            on:menu={(e) => (menuFor = e.detail)} />
        </div>
      {/if}
    </div>
  {/if}
  {#if menuFor}
    <AttentionMenu entry={menuFor.entry} x={menuFor.x} y={menuFor.y} {layoutMode} on:close={() => (menuFor = null)} />
  {/if}
```

und `<svelte:window on:keydown={(e) => e.key === 'Escape' && (open = false)} />` am Komponentenanfang. CSS:

```css
  .wait-wrap { position: relative; }
  .wait-counter { display: inline-flex; align-items: center; gap: 7px; padding: 5px 10px; border-radius: 6px; font: 500 12.5px var(--font-sans); color: var(--fg); cursor: pointer; background: var(--status-done-tint); border: 1px solid color-mix(in srgb, var(--status-done) 45%, transparent); }
  .wait-counter.rank-0, .wait-counter.rank-1 { background: var(--status-waiting-tint); border-color: color-mix(in srgb, var(--status-waiting) 50%, transparent); }
  .wait-counter.rank-2 { background: var(--status-danger-tint); border-color: color-mix(in srgb, var(--status-danger) 50%, transparent); }
  .wait-pop { position: absolute; top: calc(100% + 6px); right: 0; width: 400px; max-height: 60vh; display: flex; z-index: 50; box-shadow: 0 14px 40px rgba(0,0,0,.4); border-radius: 6px; }
```

- [ ] **Step 3: `App.svelte`**

Import `attentionList` aus `./lib/attention`. Reaktive Ableitungen:

```ts
  $: waitingAll = attentionList($allTabs, $activeTab?.id ?? '');
  $: showWaitCounter = layoutMode !== 'focus' || !!$activeTab?.panes.some((p) => p.maximized);
```

Die `<Toolbar ...>`-Props ergänzen: `waiting={waitingAll}`, `showCounter={showWaitCounter}`, `{layoutMode}`.

- [ ] **Step 4: Run tests and build**

Run: `cd frontend && npx vitest run && npm run build`
Expected: PASS, Build ohne Fehler

- [ ] **Step 5: Sichtprüfung**

`wails dev`: zehn Tabs anlegen. Jeder Tab zeigt seine Pane-Zeichen, wartende Tabs sind getönt, das × erscheint erst beim Überfahren, der Tooltip listet die Panes. Ein Pane maximieren (Ctrl+Z) → „N warten“ erscheint, Klick öffnet die Liste. Theme auf `light` stellen und prüfen, dass Blau („fertig“) sich von Grün („läuft“) abhebt.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/
git commit -m "feat(ui): one glyph per pane in the tab bar, a waiting counter where the box is hidden" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 14: Benachrichtigungen und Fenstertitel

**Files:**
- Modify: `frontend/src/App.svelte` (Toast aus `terminal:attention`, `mtui:focus-session`, Titel)
- Modify: `frontend/src/components/TerminalPane.svelte` (Toasts raus, Töne bleiben)
- Modify: `frontend/src/stores/tabs.ts` (`windowTitle`)
- Test: `frontend/src/stores/tabs.test.ts` (Titel)

**Interfaces:**
- Consumes: `shouldToast`, `ENTRY_LABEL` (Task 10), `lookedAtRef` (Task 11), `jumpToSession` (Task 12), Binding `SendPaneNotification` (Task 9)
- Produces: `windowTitle(tab, version, blocking = 0)`; `blockingCount` in `App.svelte` (für Task 15).

- [ ] **Step 1: Write the failing test** (`tabs.test.ts`)

```ts
  it('windowTitle puts the number of blocked panes in front', () => {
    expect(windowTitle(null, '2.0', 3)).toBe('(3) Multiterminal v2.0');
    expect(windowTitle(null, '2.0', 0)).toBe('Multiterminal v2.0');
  });
```

- [ ] **Step 2: Run it to verify it fails**

Run: `cd frontend && npx vitest run src/stores/tabs.test.ts`
Expected: FAIL

- [ ] **Step 3: `windowTitle`**

```ts
export function windowTitle(tab: Tab | null | undefined, version: string = '', blocking: number = 0): string {
  const base = (blocking > 0 ? `(${blocking}) ` : '') + (version ? `Multiterminal v${version}` : 'Multiterminal');
  if (!tab) return base;
  const focused = tab.panes.find((p) => p.id === tab.focusedPaneId);
  const ctx = focused ? paneDisplayName(focused) : tab.name;
  return ctx ? `${ctx} — ${base}` : base;
}
```

Hinweis: Mit Tab steht die Zahl hinter dem Pane-Namen („Name — (3) Multiterminal“). In der Taskleiste ist das Overlay aus Task 15 der eigentliche Zähler, der Titel ist der Rückfall.

- [ ] **Step 4: `App.svelte`**

Importe: `import { shouldToast, ENTRY_LABEL } from './lib/attention';`, `import { jumpToSession } from './lib/attentionNav';`. Den Listener `terminal:attention` aus Task 10 so erweitern:

```ts
    EventsOn('terminal:attention', (event: any) => {
      const info = event.data; // AttentionInfo { id, state, rank, seen, detail, source, since }
      const tab = $allTabs.find((t) => t.panes.some((p) => p.sessionId === info.id));
      const pane = tab?.panes.find((p) => p.sessionId === info.id);
      const before = pane?.attention;
      const next = { state: info.state, rank: info.rank, seen: info.seen,
        detail: info.detail ?? '', source: info.source ?? '', since: info.since ?? 0 };
      tabStore.applyAttention(info.id, next);
      const lookedAt = document.hasFocus() && lookedAtRef === info.id;
      if (tab && pane && shouldToast(before, next, lookedAt)) {
        App.SendPaneNotification(`${ENTRY_LABEL[next.rank]} · ${tab.name}`,
          `${paneDisplayName(pane)}${next.detail ? ': ' + next.detail : ''}`, info.id).catch(() => {});
      }
    });
    // A toast about one pane was clicked: the backend raised the window.
    EventsOn('mtui:focus-session', (event: any) => {
      if (event.data?.id) jumpToSession(event.data.id);
    });
```

Titel und Zähler:

```ts
  $: blockingCount = waitingAll.filter((e) => e.rank <= 1).length;
```

`syncWindowTitle` bekommt einen dritten Parameter `blocking: number` und ruft `windowTitle(tab, version, blocking)` auf; der Aufruf wird `$: syncWindowTitle($activeTab, appVersion, blockingCount);`.

- [ ] **Step 5: `TerminalPane.svelte`**

Im `$:`-Block für Benachrichtigungen die beiden `sendNotification(...)`-Aufrufe samt ihrer `if (!document.hasFocus())`-Klammern löschen; die `playBell`-Aufrufe bleiben. Kommentar darüber: `// Toasts come from the attention model (App.svelte); the pane only rings.` Den Import `sendNotification` entfernen, falls ungenutzt. Der Issue-Toast in `App.svelte` (`app.agentDone`) bleibt unverändert; die Spec vermerkt ihn in Task 16.

- [ ] **Step 6: Run tests and build**

Run: `cd frontend && npx vitest run && npm run build`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add frontend/src/
git commit -m "feat(ui): toasts only for what blocks or failed, a click jumps to the pane" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 15: Zahl am Taskleisten-Symbol (Windows)

**Files:**
- Create: `internal/backend/taskbar_badge.go` (plattformneutral: Text und Pixel)
- Create: `internal/backend/taskbar_badge_windows.go` (COM und Icon)
- Create: `internal/backend/taskbar_badge_other.go` (No-op)
- Create: `internal/backend/taskbar_badge_test.go`
- Modify: `frontend/wailsjs/go/backend/App.js`, `App.d.ts`
- Modify: `frontend/src/App.svelte`

**Interfaces:**
- Consumes: `blockingCount` (Task 14), `a.mainWindow *application.WebviewWindow`
- Produces: Binding `SetAttentionBadge(n int)` (ID 2789518836); `func badgeText(n int) string`, `func badgePixels(n int) []byte` (16×16 BGRA, von oben nach unten), `func setTaskbarBadge(w *application.WebviewWindow, n int)`.

- [ ] **Step 1: Write the failing test**

```go
package backend

import "testing"

func TestBadgeText(t *testing.T) {
	for n, want := range map[int]string{0: "", -1: "", 1: "1", 9: "9", 10: "9+", 42: "9+"} {
		if got := badgeText(n); got != want {
			t.Errorf("badgeText(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestBadgePixels(t *testing.T) {
	pix := badgePixels(3)
	if len(pix) != 16*16*4 {
		t.Fatalf("len %d", len(pix))
	}
	alpha := func(x, y int) byte { return pix[(y*16+x)*4+3] }
	if alpha(0, 0) != 0 {
		t.Error("corner must be transparent")
	}
	if alpha(1, 1) != 0xff {
		t.Error("background must be opaque")
	}
	ink := 0
	for i := 0; i < len(pix); i += 4 {
		if pix[i+2] == 0x1a { // red channel of the digit colour
			ink++
		}
	}
	if ink == 0 {
		t.Error("no digit drawn")
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/backend/ -run 'TestBadge' -count=1`
Expected: FAIL

- [ ] **Step 3: `taskbar_badge.go`**

```go
package backend

import "strconv"

// The number on the taskbar button: how many panes are blocked on the user
// (a permission or a question). Drawn here, without an image library, so the
// Windows half only has to hand pixels to the OS.

// badgeText is the overlay's label and its accessible description.
func badgeText(n int) string {
	switch {
	case n <= 0:
		return ""
	case n > 9:
		return "9+"
	default:
		return strconv.Itoa(n)
	}
}

// digitFont is a 3x5 bitmap font: one string per row, '#' is ink.
var digitFont = map[rune][5]string{
	'0': {"###", "#.#", "#.#", "#.#", "###"},
	'1': {".#.", "##.", ".#.", ".#.", "###"},
	'2': {"###", "..#", "###", "#..", "###"},
	'3': {"###", "..#", "###", "..#", "###"},
	'4': {"#.#", "#.#", "###", "..#", "..#"},
	'5': {"###", "#..", "###", "..#", "###"},
	'6': {"###", "#..", "###", "#.#", "###"},
	'7': {"###", "..#", "..#", "..#", "..#"},
	'8': {"###", "#.#", "###", "#.#", "###"},
	'9': {"###", "#.#", "###", "..#", "###"},
	'+': {"...", ".#.", "###", ".#.", "..."},
}

// badgePixels draws n on an amber square with cut corners, 16x16, as top-down
// BGRA rows with straight alpha, the layout a 32-bit icon bitmap takes.
func badgePixels(n int) []byte {
	const size, scale = 16, 2
	pix := make([]byte, size*size*4)
	set := func(x, y int, r, g, b byte) {
		i := (y*size + x) * 4
		pix[i], pix[i+1], pix[i+2], pix[i+3] = b, g, r, 0xff
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if (x == 0 || x == size-1) && (y == 0 || y == size-1) {
				continue // cut corners stay transparent
			}
			set(x, y, 0xd6, 0xa8, 0x5c) // --status-waiting
		}
	}
	text := badgeText(n)
	w := (len(text)*4 - 1) * scale
	ox, oy := (size-w)/2, (size-5*scale)/2
	for ci, ch := range text {
		for gy, row := range digitFont[ch] {
			for gx, c := range row {
				if c != '#' {
					continue
				}
				for sy := 0; sy < scale; sy++ {
					for sx := 0; sx < scale; sx++ {
						set(ox+(ci*4+gx)*scale+sx, oy+gy*scale+sy, 0x1a, 0x14, 0x05)
					}
				}
			}
		}
	}
	return pix
}

// SetAttentionBadge shows n on the taskbar button; 0 removes it. Windows only.
func (a *AppService) SetAttentionBadge(n int) {
	if a.mainWindow == nil {
		return
	}
	setTaskbarBadge(a.mainWindow, n)
}
```

- [ ] **Step 4: `taskbar_badge_other.go`**

```go
//go:build !windows

package backend

import "github.com/wailsapp/wails/v3/pkg/application"

// setTaskbarBadge has no counterpart outside Windows.
func setTaskbarBadge(_ *application.WebviewWindow, _ int) {}
```

- [ ] **Step 5: `taskbar_badge_windows.go`**

```go
//go:build windows

package backend

import (
	"sync"
	"syscall"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

// The overlay goes through ITaskbarList3::SetOverlayIcon. COM is called on the
// UI thread (InvokeSync), which WebView2 has already initialised as STA.

var (
	modOle32             = windows.NewLazySystemDLL("ole32.dll")
	procCoCreateInstance = modOle32.NewProc("CoCreateInstance")
	modUser32            = windows.NewLazySystemDLL("user32.dll")
	procCreateIconIndir  = modUser32.NewProc("CreateIconIndirect")
	procDestroyIcon      = modUser32.NewProc("DestroyIcon")
	modGdi32             = windows.NewLazySystemDLL("gdi32.dll")
	procCreateBitmap     = modGdi32.NewProc("CreateBitmap")
	procDeleteObject     = modGdi32.NewProc("DeleteObject")

	clsidTaskbarList = windows.GUID{Data1: 0x56FDF344, Data2: 0xFD6D, Data3: 0x11d0, Data4: [8]byte{0x95, 0x8A, 0x00, 0x60, 0x97, 0xC9, 0xA0, 0x90}}
	iidTaskbarList3  = windows.GUID{Data1: 0xea1afb91, Data2: 0x9e28, Data3: 0x4b86, Data4: [8]byte{0x90, 0xe9, 0x9e, 0x9f, 0x8a, 0x5e, 0xef, 0xaf}}
)

const (
	clsctxInprocServer = 0x1
	// vtable slots: IUnknown 0-2, ITaskbarList 3-7, ITaskbarList2 8,
	// ITaskbarList3 9-20.
	slotHrInit         = 3
	slotSetOverlayIcon = 18
)

type iconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  uintptr
	hbmColor uintptr
}

var badge struct {
	once sync.Once
	list uintptr // ITaskbarList3*, 0 when unavailable
	icon uintptr // the HICON currently shown
	last int
}

func comCall(obj uintptr, slot int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(unsafe.Pointer(obj))
	fn := *(*uintptr)(unsafe.Pointer(vtbl + uintptr(slot)*unsafe.Sizeof(uintptr(0))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{obj}, args...)...)
	return r
}

func badgeIcon(n int) uintptr {
	pix := badgePixels(n)
	mask := make([]byte, 16*2) // 1 bpp, 16 rows of 2 bytes, all zero: alpha decides
	color, _, _ := procCreateBitmap.Call(16, 16, 1, 32, uintptr(unsafe.Pointer(&pix[0])))
	maskBm, _, _ := procCreateBitmap.Call(16, 16, 1, 1, uintptr(unsafe.Pointer(&mask[0])))
	defer procDeleteObject.Call(color)
	defer procDeleteObject.Call(maskBm)
	if color == 0 || maskBm == 0 {
		return 0
	}
	info := iconInfo{fIcon: 1, hbmMask: maskBm, hbmColor: color}
	icon, _, _ := procCreateIconIndir.Call(uintptr(unsafe.Pointer(&info)))
	return icon
}

func setTaskbarBadge(w *application.WebviewWindow, n int) {
	application.InvokeSync(func() {
		badge.once.Do(func() {
			var p uintptr
			hr, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidTaskbarList)), 0, clsctxInprocServer,
				uintptr(unsafe.Pointer(&iidTaskbarList3)), uintptr(unsafe.Pointer(&p)))
			if hr == 0 && p != 0 && comCall(p, slotHrInit) == 0 {
				badge.list = p
			}
		})
		hwnd := uintptr(w.NativeWindow())
		if badge.list == 0 || hwnd == 0 || (n == badge.last && badge.icon != 0) {
			return
		}
		var icon uintptr
		if n > 0 {
			icon = badgeIcon(n)
		}
		desc, _ := windows.UTF16PtrFromString(badgeText(n))
		comCall(badge.list, slotSetOverlayIcon, hwnd, icon, uintptr(unsafe.Pointer(desc)))
		if badge.icon != 0 {
			procDestroyIcon.Call(badge.icon)
		}
		badge.icon, badge.last = icon, n
	})
}
```

- [ ] **Step 6: Binding und Frontend**

`App.js`: `export function SetAttentionBadge(arg1) {\n  return $Call.ByID(2789518836, arg1);\n}`; `App.d.ts`: `export function SetAttentionBadge(arg1:number):Promise<void>;`

`App.svelte`:

```ts
  // The taskbar number follows the blocked count; the call only goes out on a
  // change. A function, so only the argument is a dependency (CLAUDE.md).
  let _lastBadge = -1;
  function syncBadge(n: number) {
    if (n === _lastBadge) return;
    _lastBadge = n;
    App.SetAttentionBadge(n).catch(() => {});
  }
  $: syncBadge(blockingCount);
```

- [ ] **Step 7: Run tests, vet, build**

Steht `golang.org/x/sys` in `go.mod` nur als `// indirect`, zuerst `go mod tidy` ausführen (die Zeile wird dann direkt); andere Änderungen an `go.mod`/`go.sum` darf das nicht bringen, sonst anhalten und nachfragen.
Run: `go test ./internal/backend/ ./internal/tsmodels/ -count=1 && GOOS=windows CGO_ENABLED=0 go vet ./internal/backend/ && go vet ./internal/backend/`
Expected: PASS, keine Vet-Meldung (die `unsafe.Pointer`-Arithmetik in `comCall` ist das übliche COM-Muster; meldet `go vet` „possible misuse of unsafe.Pointer“, die Zeile so lassen und die Meldung im Commit nennen).

- [ ] **Step 8: Manuelle Prüfung**

`wails build -debug`, App starten, in einem Pane eine Freigabe auslösen (etwa `claude` eine Datei löschen lassen). Am Taskleisten-Symbol erscheint eine „1“ auf bernsteinfarbenem Grund. Nach der Antwort verschwindet sie. Aufnahme mit `Take-Screenshot.ps1 -Screen All`, auf die Taskleiste zuschneiden, Gesamtaufnahme löschen. **Erscheint kein Overlay** (etwa weil `CoCreateInstance` scheitert), bleibt der Titel-Rückfall aus Task 14; dann diesen Task mit einem Log-Eintrag in `setTaskbarBadge` (`hr`-Wert) zurückstellen und das Ergebnis in der Spec unter offenen Punkten festhalten.

- [ ] **Step 9: Commit**

```bash
git add internal/backend/ frontend/
git commit -m "feat(windows): the number of blocked panes on the taskbar button" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 16: Doku, Gesamtprüfung, Abnahme

**Files:**
- Modify: `CLAUDE.md`
- Modify: `docs/superpowers/specs/2026-10-07-leitstand-aufmerksamkeit-design.md`

- [ ] **Step 1: `CLAUDE.md`**

Unter „Spec vs. Implementation“ die Liste der aktiven Specs um eine Zeile ergänzen:

```markdown
  - `docs/superpowers/specs/2026-10-07-leitstand-aufmerksamkeit-design.md` — Leitstand Teil 1: Aufmerksamkeit über alle Projekte
```

Im Abschnitt „Architecture“, nach dem Punkt über den Scan, einen Punkt ergänzen:

```markdown
- **Attention lives on the host** (`internal/hub/attention.go`): state, since, seen, detail,
  source. `hub.AttentionRank` is the only urgency rule; the frontend sorts by the host's rank
  and takes the smallest per tab, it never re-derives one. `MarkSeen` clears "done" and
  "error", never a permission or a question. A reader that may face an older daemon
  recomputes the rank from State and Seen, because a zero record has Rank 0.
```

- [ ] **Step 2: Spec nachziehen**

- C2 an die Umsetzung angleichen: Der Datensatz reist nicht im Scan-Ereignis mit, sondern als eigenes `EventSessionAttention` → `terminal:attention`. Der Grund: „gesehen“ ändert sich ohne Zustandswechsel und braucht ohnehin ein eigenes Ereignis. `since` ist im Datensatz immer gesetzt; der Hook-Pfad von `terminal:activity` schickt weiter `since=0`, die Sortierung liest aber den Datensatz.
- Offener Punkt 1 als erledigt markieren: „Subagent-Ereignisse tragen `agent_id` (Claude-Code-Hook-Doku); umgesetzt als `subagentWorkAfterTurn`.“
- Offener Punkt 2 je nach Ergebnis von Task 15 Step 8 als erledigt oder zurückgestellt markieren.
- Abnahme 1: „binnen 2 s“ wird zu „binnen 3 s“, mit dem Satz: „Debounce 1,2 s plus bis zu einem Scan-Takt (0,75 s) plus Ereignisweg.“
- Unter „Nicht im Umfang“: „Der Toast ‚Issue fertig‘ (`app.agentDone`) bleibt unverändert; er gehört zur Issue-Verknüpfung, nicht zum Leitstand.“
- Status: „Umgesetzt auf `worktree-leitstand-aufmerksamkeit`, E2E-Abnahme offen“.

- [ ] **Step 3: Volle Testläufe**

```bash
go test ./internal/... ./cmd/mt/ ./cmd/mtuid/ ./cmd/mtui-hook/ -count=1
go vet ./...
GOOS=windows CGO_ENABLED=0 go vet ./internal/... ./cmd/...
cd frontend && npx vitest run && npm run build
grep -rl internal/terminal internal/backend --include='*.go' | grep -v _test.go
```

Expected: alles grün, der letzte Befehl ohne Ausgabe. Zeilenzahl prüfen: `wc -l internal/hub/*.go internal/backend/*.go | awk '$1 > 300'` darf nur die Dateien der Schuldenliste zeigen, und `server.go` ist kürzer als vorher.

- [ ] **Step 4: E2E-Abnahme nach Spec**

Die neun Punkte der Spec-Abnahme mit echtem Claude CLI in drei Projekten durchgehen. Je Punkt notieren: bestanden / nicht bestanden mit Beobachtung. Screenshots nur vom MTUI-Monitor.

- [ ] **Step 5: Commit**

```bash
git add CLAUDE.md docs/superpowers/specs/2026-10-07-leitstand-aufmerksamkeit-design.md
git commit -m "docs: Leitstand part 1 in CLAUDE.md, spec follows the implementation" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

- [ ] **Step 6: Übergabe**

Patrick das Ergebnis der Abnahme zeigen und fragen, ob ein GitHub-Issue (mit `needs-e2e-testing`, falls Punkte offen sind) und ein PR gegen `alpha-main` angelegt werden sollen. Ohne Zustimmung weder pushen noch einen PR öffnen.

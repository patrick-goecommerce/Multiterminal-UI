package hub

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/gitx"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hooks"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// briefingSink keeps every briefing the host published.
type briefingSink struct {
	mu   sync.Mutex
	list []BriefingUpdate
	ch   chan BriefingUpdate
}

func newBriefingSink() *briefingSink { return &briefingSink{ch: make(chan BriefingUpdate, 16)} }

func (s *briefingSink) Emit(name string, payload any) {
	if name != EventSessionBriefing {
		return
	}
	u, ok := DecodePayload[BriefingUpdate](payload)
	if !ok {
		return
	}
	s.mu.Lock()
	s.list = append(s.list, u)
	s.mu.Unlock()
	s.ch <- u
}

func (s *briefingSink) await(t *testing.T) BriefingUpdate {
	t.Helper()
	select {
	case u := <-s.ch:
		return u
	case <-time.After(5 * time.Second):
		t.Fatal("no session.briefing within 5 s")
	}
	return BriefingUpdate{}
}

// quiet fails if a briefing arrives within a short window.
func (s *briefingSink) quiet(t *testing.T) {
	t.Helper()
	select {
	case u := <-s.ch:
		t.Fatalf("unexpected session.briefing: %+v", u)
	case <-time.After(300 * time.Millisecond):
	}
}

// briefingHost is a host with one adopted session (no process) and a sink.
func briefingHost(t *testing.T, id int) (*Embedded, *briefingSink) {
	t.Helper()
	sink := newBriefingSink()
	h := NewEmbedded(Options{Version: "test", Sink: sink})
	t.Cleanup(h.Release)
	h.AdoptForTest(id, terminal.NewSession(id, 24, 80))
	return h, sink
}

func submit(h *Embedded, id int, promptID, prompt, cwd string) {
	h.applyHookEvent(nil, hooks.Event{Event: "UserPromptSubmit", MtID: id, PromptID: promptID, Message: prompt, Cwd: cwd})
}

func bash(h *Embedded, id int, promptID, command string, failed bool) {
	ev := "PostToolUse"
	if failed {
		ev = "PostToolUseFailure"
	}
	h.applyHookEvent(nil, hooks.Event{Event: ev, MtID: id, PromptID: promptID, Tool: "Bash", Command: command})
}

func stop(h *Embedded, id int, promptID, message string) {
	h.applyHookEvent(nil, hooks.Event{Event: "Stop", MtID: id, PromptID: promptID, Message: message})
}

// waitStartTree waits until the turn-start snapshot has landed (ruling R2):
// a Stop sent before it would race the snapshot goroutine.
func waitStartTree(t *testing.T, h *Embedded, id int) {
	t.Helper()
	waitFor(t, func() bool {
		b := h.desk.turns
		b.mu.Lock()
		defer b.mu.Unlock()
		tr := b.turns[id]
		return tr != nil && tr.startTree != ""
	}, "the turn-start snapshot never landed")
}

// blockingSnapshot replaces the desk's snapshot seam with one that waits for
// release (or its context) on the calls block selects, and reports each call
// on entered.
func blockingSnapshot(t *testing.T, h *Embedded, block func(call int32) bool) (release func(), entered <-chan struct{}) {
	t.Helper()
	ch := make(chan struct{})
	in := make(chan struct{}, 16)
	var calls atomic.Int32
	h.desk.snapshot = func(ctx context.Context, dir string) (string, error) {
		n := calls.Add(1)
		in <- struct{}{}
		if block(n) {
			select {
			case <-ch:
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}
		return gitx.SnapshotTree(ctx, dir)
	}
	var once sync.Once
	rel := func() { once.Do(func() { close(ch) }) }
	t.Cleanup(rel)
	return rel, in
}

func waitEntered(t *testing.T, entered <-chan struct{}, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatalf("snapshot seam entered %d times, want %d", i, n)
		}
	}
}

func TestStopBuildsBriefing(t *testing.T) {
	h, sink := briefingHost(t, 1)
	repo := newRepoWithOrigin(t)

	submit(h, 1, "p1", "kürz die Details", repo)
	waitStartTree(t, h, 1)
	twrite(t, repo, "new.txt", 3)
	bash(h, 1, "p1", "go vet ./...", true)
	stop(h, 1, "p1", "Erledigt.\n\nvet meldet noch was.")

	u := sink.await(t)
	b := u.Briefing
	if u.ID != 1 || b.PromptID != "p1" {
		t.Errorf("id/prompt id = %d/%q", u.ID, b.PromptID)
	}
	if b.Prompt != "kürz die Details" {
		t.Errorf("Prompt = %q", b.Prompt)
	}
	if want := []CheckResult{{Label: "go vet", Result: CheckFail}}; !reflect.DeepEqual(b.Checks, want) {
		t.Errorf("Checks = %+v, want %+v", b.Checks, want)
	}
	if b.Closing != "vet meldet noch was." {
		t.Errorf("Closing = %q", b.Closing)
	}
	if b.Git == nil || !b.Git.TurnKnown || b.Git.TurnFiles != 1 {
		t.Fatalf("Git = %+v, want turn known with 1 file", b.Git)
	}
	if want := []string{WarnCheckFailed, WarnDirty}; !reflect.DeepEqual(b.Warnings, want) {
		t.Errorf("Warnings = %v, want %v", b.Warnings, want)
	}
	got, ok, err := h.Briefing(1)
	if err != nil || !ok {
		t.Fatalf("Briefing = %v, %v", ok, err)
	}
	if !reflect.DeepEqual(got, b) {
		t.Errorf("Briefing() = %+v\nevent      = %+v", got, b)
	}
}

func TestKeepAliveTurnKeepsLastBriefing(t *testing.T) {
	h, sink := briefingHost(t, 1)
	dir := t.TempDir()
	submit(h, 1, "p1", "mach was", dir)
	stop(h, 1, "p1", "Fertig.")
	first := sink.await(t).Briefing

	h.desk.turns.expectKeepAlive(1)
	submit(h, 1, "p2", "hallo", dir)
	stop(h, 1, "p2", "Hallo zurück.")
	sink.quiet(t)

	got, ok, err := h.Briefing(1)
	if err != nil || !ok || !reflect.DeepEqual(got, first) {
		t.Errorf("Briefing = %+v (%v, %v), want the first one %+v", got, ok, err, first)
	}
}

// A keep-alive that could not be typed starts no turn, so the next real turn
// must not be taken for the keep-alive's own.
func TestKeepAliveWriteFailureClearsExpectation(t *testing.T) {
	h, sink := briefingHost(t, 1) // adopted: no PTY, every write fails
	if err := h.nudge(1, "hallo"); err == nil {
		t.Fatal("nudge into a session without a PTY: want an error")
	}
	submit(h, 1, "p1", "echte Arbeit", t.TempDir())
	stop(h, 1, "p1", "Fertig.")
	if b := sink.await(t).Briefing; b.PromptID != "p1" {
		t.Errorf("PromptID = %q, want p1", b.PromptID)
	}
}

func TestExitBuildsWithEndedError(t *testing.T) {
	h, sink := briefingHost(t, 1)
	submit(h, 1, "p1", "mach was", t.TempDir())
	h.finishTurn(1, true, "")
	b := sink.await(t).Briefing
	if want := []string{WarnEndedError}; !reflect.DeepEqual(b.Warnings, want) {
		t.Errorf("Warnings = %v, want %v", b.Warnings, want)
	}
}

// Review Focus 1: a Stop that overtakes the turn-start snapshot measures no
// turn rather than diffing against an older tree.
func TestStopBeforeSnapshot(t *testing.T) {
	h, sink := briefingHost(t, 1)
	repo := newRepoWithOrigin(t)
	release, entered := blockingSnapshot(t, h, func(n int32) bool { return n == 1 })

	submit(h, 1, "p1", "mach was", repo)
	waitEntered(t, entered, 1)
	twrite(t, repo, "new.txt", 3)
	stop(h, 1, "p1", "Fertig.")
	release()

	b := sink.await(t).Briefing
	if b.Git == nil {
		t.Fatal("Git = nil in a repository")
	}
	if b.Git.TurnKnown || b.Git.TurnFiles != 0 {
		t.Errorf("TurnKnown/TurnFiles = %v/%d, want false/0", b.Git.TurnKnown, b.Git.TurnFiles)
	}
	if len(b.Notes) != 0 {
		t.Errorf("Notes = %v, want none", b.Notes)
	}
}

// Review Focus 2: a session closed while its briefing is being built gets no
// event and keeps nothing.
func TestCloseDuringBuild(t *testing.T) {
	h, sink := briefingHost(t, 1)
	repo := newRepoWithOrigin(t)
	release, entered := blockingSnapshot(t, h, func(int32) bool { return true })

	submit(h, 1, "p1", "mach was", repo)
	stop(h, 1, "p1", "Fertig.")
	waitEntered(t, entered, 2) // the start snapshot and the build's own
	if err := h.Close(1); err != nil {
		t.Fatalf("Close: %v", err)
	}
	release()
	sink.quiet(t)

	if _, _, err := h.Briefing(1); !errors.Is(err, ErrNoSession) {
		t.Errorf("Briefing after Close: err = %v, want ErrNoSession", err)
	}
	waitFor(t, func() bool { return len(h.desk.sem) == 0 }, "the build never gave its slot back")
	h.desk.mu.Lock()
	defer h.desk.mu.Unlock()
	if len(h.desk.entries) != 0 || len(h.desk.builds) != 0 {
		t.Errorf("desk kept entries %v / builds %v", h.desk.entries, h.desk.builds)
	}
}

func TestBuildTimeLimit(t *testing.T) {
	h, sink := briefingHost(t, 1)
	repo := newRepoWithOrigin(t)
	h.desk.limit = 50 * time.Millisecond
	blockingSnapshot(t, h, func(int32) bool { return true })

	submit(h, 1, "p1", "mach was", repo)
	stop(h, 1, "p1", "Fertig.")
	b := sink.await(t).Briefing
	if want := []string{NoteDiffSlow}; !reflect.DeepEqual(b.Notes, want) {
		t.Errorf("Notes = %v, want %v", b.Notes, want)
	}
	if b.Git != nil && b.Git.TurnKnown {
		t.Error("a build that ran out of time claims to know the turn")
	}
}

func TestNoRepoNoGit(t *testing.T) {
	h, sink := briefingHost(t, 1)
	submit(h, 1, "p1", "mach was", t.TempDir())
	stop(h, 1, "p1", "Fertig.")
	b := sink.await(t).Briefing
	if b.Git != nil {
		t.Errorf("Git = %+v outside a repository, want nil", b.Git)
	}
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{`"checks":[]`, `"warnings":[]`, `"notes":[]`} {
		if !strings.Contains(string(raw), field) {
			t.Errorf("JSON %s lacks %s", raw, field)
		}
	}
}

func TestSetTargetUsed(t *testing.T) {
	h, sink := briefingHost(t, 1)
	repo := newRepoWithOrigin(t)
	tgit(t, repo, "checkout", "-q", "-b", "feat")
	if err := h.SetTarget(1, "alpha-main"); err != nil {
		t.Fatalf("SetTarget: %v", err)
	}
	if err := h.SetTarget(99, "alpha-main"); !errors.Is(err, ErrNoSession) {
		t.Errorf("SetTarget on an unknown id: err = %v, want ErrNoSession", err)
	}
	submit(h, 1, "p1", "mach was", repo)
	stop(h, 1, "p1", "Fertig.")
	b := sink.await(t).Briefing
	if b.Git == nil || b.Git.Target != "alpha-main" {
		t.Errorf("Git = %+v, want Target alpha-main", b.Git)
	}
}

// The command as mtui-hook now delivers it: a newline became "; ". The check
// before it has a masked exit code, so a success proves nothing.
func TestChainedCheckIsUnsure(t *testing.T) {
	h, sink := briefingHost(t, 1)
	submit(h, 1, "p1", "teste", t.TempDir())
	bash(h, 1, "p1", "go test ./...; echo done", false)
	stop(h, 1, "p1", "Fertig.")
	b := sink.await(t).Briefing
	if want := []CheckResult{{Label: "go test", Result: CheckUnsure}}; !reflect.DeepEqual(b.Checks, want) {
		t.Errorf("Checks = %+v, want %+v", b.Checks, want)
	}
}

// A newer Stop of the same session cancels the build still running for the
// older one; only the newer briefing is published.
func TestNewerStopSupersedes(t *testing.T) {
	h, sink := briefingHost(t, 1)
	repo := newRepoWithOrigin(t)
	release, _ := blockingSnapshot(t, h, func(int32) bool { return true })

	submit(h, 1, "p1", "eins", repo)
	stop(h, 1, "p1", "Eins fertig.")
	submit(h, 1, "p2", "zwei", repo)
	stop(h, 1, "p2", "Zwei fertig.")
	release()

	if b := sink.await(t).Briefing; b.PromptID != "p2" {
		t.Errorf("PromptID = %q, want p2", b.PromptID)
	}
	sink.quiet(t)
}

func TestUpdatePRRecomputesWarnings(t *testing.T) {
	h, sink := briefingHost(t, 1)
	submit(h, 1, "p1", "mach was", t.TempDir())
	h.finishTurn(1, true, "")
	sink.await(t)

	b, ok := h.desk.updatePR(1, &PRFacts{Number: 7, State: "OPEN", Checks: "red"}, []string{NoteCIStale})
	if !ok {
		t.Fatal("updatePR on a session with a briefing: want true")
	}
	if want := []string{WarnCIRed, WarnEndedError}; !reflect.DeepEqual(b.Warnings, want) {
		t.Errorf("Warnings = %v, want %v", b.Warnings, want)
	}
	if want := []string{NoteCIStale}; !reflect.DeepEqual(b.Notes, want) {
		t.Errorf("Notes = %v, want %v", b.Notes, want)
	}
	if got, _, _ := h.Briefing(1); !reflect.DeepEqual(got, b) {
		t.Errorf("stored = %+v, want %+v", got, b)
	}
	if _, ok := h.desk.updatePR(2, nil, nil); ok {
		t.Error("updatePR on a session without a briefing: want false")
	}
}

// I1: a turn that starts in one checkout and stops in another (EnterWorktree)
// shares the object store, so the trees diff fine but mean nothing. Without
// the same toplevel at both ends the turn stays unknown.
func TestTurnAcrossCheckoutsIsUnknown(t *testing.T) {
	h, sink := briefingHost(t, 1)
	repo := newRepoWithOrigin(t)
	wt := filepath.Join(filepath.Dir(repo), "linked wt")
	tgit(t, repo, "worktree", "add", "-q", "-b", "wt", wt)
	twrite(t, repo, "dirty.txt", 5) // the main checkout is dirty

	submit(h, 1, "p1", "mach was", repo)
	waitStartTree(t, h, 1)
	h.applyHookEvent(nil, hooks.Event{Event: "PostToolUse", MtID: 1, PromptID: "p1", Tool: "Bash", Command: "ls", Cwd: wt})
	stop(h, 1, "p1", "Fertig.")

	b := sink.await(t).Briefing
	if b.Git == nil {
		t.Fatal("Git = nil in a repository")
	}
	if b.Git.TurnKnown || b.Git.TurnFiles != 0 || b.Git.TurnAdded != 0 || b.Git.TurnDeleted != 0 {
		t.Errorf("turn = known %v, %d files +%d -%d; want unknown and no numbers",
			b.Git.TurnKnown, b.Git.TurnFiles, b.Git.TurnAdded, b.Git.TurnDeleted)
	}
	if b.Git.Branch != "wt" {
		t.Errorf("Branch = %q, want the worktree's wt", b.Git.Branch)
	}
}

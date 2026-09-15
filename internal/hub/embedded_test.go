package hub

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func isWindows() bool { return runtime.GOOS == "windows" }

// comspec is the Windows shell, resolved the way the rest of MTUI resolves it:
// never a bare cmd.exe, which Go would look for next to the binary.
func comspec() string {
	if c := os.Getenv("COMSPEC"); c != "" {
		return c
	}
	return "cmd.exe"
}

// printArgv returns a command that writes text to the terminal and exits.
func printArgv(text string) []string {
	if isWindows() {
		return []string{comspec(), "/c", "echo " + text}
	}
	return []string{"/bin/sh", "-c", "printf '%s' " + text}
}

// sleepArgv returns a command that stays alive without producing output.
func sleepArgv() []string {
	if isWindows() {
		return []string{comspec(), "/c", "ping -n 60 127.0.0.1 > nul"}
	}
	return []string{"/bin/sh", "-c", "sleep 60"}
}

// recordingSink collects events so a test can wait for one.
type recordingSink struct {
	mu     sync.Mutex
	events []string
	got    chan string
}

func newSink() *recordingSink {
	return &recordingSink{got: make(chan string, 16)}
}

func (s *recordingSink) Emit(name string, _ any) {
	s.mu.Lock()
	s.events = append(s.events, name)
	s.mu.Unlock()
	select {
	case s.got <- name:
	default:
	}
}

func (s *recordingSink) await(t *testing.T, name string) {
	t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		select {
		case got := <-s.got:
			if got == name {
				return
			}
		case <-deadline:
			t.Fatalf("event %q never arrived; saw %v", name, s.events)
		}
	}
}

// collect reads a subscription until it sees want or the deadline passes.
func collect(t *testing.T, sub *Subscription, want string) (text string, truncated bool) {
	t.Helper()
	var b strings.Builder
	deadline := time.After(10 * time.Second)
	for {
		select {
		case c, ok := <-sub.C:
			if !ok {
				return b.String(), truncated
			}
			truncated = truncated || c.Truncated
			b.Write(c.Data)
			if strings.Contains(b.String(), want) {
				return b.String(), truncated
			}
		case <-deadline:
			t.Fatalf("waiting for %q, got %q", want, b.String())
		}
	}
}

func newTestHost(t *testing.T, sink EventSink) *Embedded {
	t.Helper()
	h := NewEmbedded(Options{Version: "test", Sink: sink})
	t.Cleanup(h.Release)
	return h
}

func TestEmbedded_InfoCarriesProtocolAndIdentity(t *testing.T) {
	h := newTestHost(t, nil)
	info := h.Info()
	if info.Protocol != Protocol {
		t.Errorf("Protocol = %d, want %d", info.Protocol, Protocol)
	}
	if info.HubID == "" {
		t.Error("HubID must not be empty")
	}
	if info.PID != os.Getpid() {
		t.Errorf("PID = %d, want %d", info.PID, os.Getpid())
	}
}

func TestEmbedded_HubIDIsStableAcrossRestartsWhenPassedIn(t *testing.T) {
	h := NewEmbedded(Options{HubID: "persisted-id"})
	defer h.Release()
	if h.HubID() != "persisted-id" {
		t.Errorf("HubID = %q, want %q", h.HubID(), "persisted-id")
	}
}

func TestEmbedded_UnknownSessionIsReported(t *testing.T) {
	h := newTestHost(t, nil)
	if err := h.Write(99, []byte("x")); !errors.Is(err, ErrNoSession) {
		t.Errorf("Write on unknown id = %v, want ErrNoSession", err)
	}
	if _, err := h.Get(99); !errors.Is(err, ErrNoSession) {
		t.Errorf("Get on unknown id = %v, want ErrNoSession", err)
	}
	if _, err := h.Attach(99, ReplayAll); !errors.Is(err, ErrNoSession) {
		t.Errorf("Attach on unknown id = %v, want ErrNoSession", err)
	}
}

func TestEmbedded_CreateAfterReleaseIsRefused(t *testing.T) {
	h := NewEmbedded(Options{})
	h.Release()
	if _, err := h.Create(CreateSpec{Argv: sleepArgv()}); !errors.Is(err, ErrClosed) {
		t.Errorf("Create after Release = %v, want ErrClosed", err)
	}
}

func TestEmbedded_OutputReachesAnAttachedClient(t *testing.T) {
	h := newTestHost(t, nil)
	id, err := h.Create(CreateSpec{Argv: printArgv("mtui-hub-marker"), Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sub, err := h.Attach(id, ReplayAll)
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	defer sub.Close()

	text, truncated := collect(t, sub, "mtui-hub-marker")
	if truncated {
		t.Error("a fresh attach to a short-lived session must not be truncated")
	}
	if !strings.Contains(text, "mtui-hub-marker") {
		t.Errorf("output = %q, want it to contain the marker", text)
	}
}

// Attaching after the fact is the whole point: the client was not there when
// the bytes were produced and must still get them.
func TestEmbedded_LateAttachReplaysHistory(t *testing.T) {
	h := newTestHost(t, nil)
	id, err := h.Create(CreateSpec{Argv: printArgv("written-before-attach"), Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Wait until the session has actually produced its output.
	deadline := time.Now().Add(10 * time.Second)
	for {
		s, err := h.Get(id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		if s.Offset > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("session produced no output")
		}
		time.Sleep(10 * time.Millisecond)
	}

	sub, err := h.Attach(id, ReplayAll)
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	defer sub.Close()

	text, _ := collect(t, sub, "written-before-attach")
	if !strings.Contains(text, "written-before-attach") {
		t.Errorf("replay = %q, want it to contain the marker", text)
	}
}

// Two clients on one session is the multi-window and CLI case: both get the
// same bytes, and neither starves the other.
func TestEmbedded_TwoClientsSeeTheSameOutput(t *testing.T) {
	h := newTestHost(t, nil)
	id, err := h.Create(CreateSpec{Argv: printArgv("shared-output"), Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	subA, err := h.Attach(id, ReplayAll)
	if err != nil {
		t.Fatalf("Attach A: %v", err)
	}
	defer subA.Close()
	subB, err := h.Attach(id, ReplayAll)
	if err != nil {
		t.Fatalf("Attach B: %v", err)
	}
	defer subB.Close()

	textA, _ := collect(t, subA, "shared-output")
	textB, _ := collect(t, subB, "shared-output")
	if !strings.Contains(textA, "shared-output") || !strings.Contains(textB, "shared-output") {
		t.Errorf("A = %q, B = %q, both want the marker", textA, textB)
	}
}

func TestEmbedded_ExitIsReported(t *testing.T) {
	sink := newSink()
	h := newTestHost(t, sink)
	if _, err := h.Create(CreateSpec{Argv: printArgv("bye"), Dir: t.TempDir()}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	sink.await(t, EventSessionCreated)
	sink.await(t, EventSessionExited)
}

func TestEmbedded_CloseForgetsTheSession(t *testing.T) {
	h := newTestHost(t, nil)
	id, err := h.Create(CreateSpec{Argv: sleepArgv(), Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(h.List()) != 1 {
		t.Fatalf("List() = %d sessions, want 1", len(h.List()))
	}
	if err := h.Close(id); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if len(h.List()) != 0 {
		t.Errorf("List() = %d sessions after Close, want 0", len(h.List()))
	}
	if _, err := h.Get(id); !errors.Is(err, ErrNoSession) {
		t.Errorf("Get after Close = %v, want ErrNoSession", err)
	}
}

// Closing a session must end its subscriptions rather than leave a reader
// hanging on a channel nobody will ever write to again.
func TestEmbedded_CloseEndsSubscriptions(t *testing.T) {
	h := newTestHost(t, nil)
	id, err := h.Create(CreateSpec{Argv: sleepArgv(), Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sub, err := h.Attach(id, ReplayAll)
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	defer sub.Close()

	go func() { _ = h.Close(id) }()
	deadline := time.After(20 * time.Second)
	for {
		select {
		case _, ok := <-sub.C:
			if !ok {
				return // channel closed, which is what we want
			}
		case <-deadline:
			t.Fatal("subscription stayed open after the session was closed")
		}
	}
}

// A subscription that is closed by its reader must not keep a goroutine alive
// waiting to hand over bytes nobody collects.
func TestEmbedded_SubscriptionCloseStopsTheFeed(t *testing.T) {
	h := newTestHost(t, nil)
	id, err := h.Create(CreateSpec{Argv: sleepArgv(), Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sub, err := h.Attach(id, ReplayAll)
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	sub.Close()
	sub.Close() // idempotent

	deadline := time.After(10 * time.Second)
	for {
		select {
		case _, ok := <-sub.C:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("feed goroutine did not stop after Subscription.Close")
		}
	}
}

func TestEmbedded_ListIsOrderedByID(t *testing.T) {
	h := newTestHost(t, nil)
	for i := 0; i < 3; i++ {
		if _, err := h.Create(CreateSpec{Argv: sleepArgv(), Dir: t.TempDir()}); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}
	list := h.List()
	if len(list) != 3 {
		t.Fatalf("List() = %d sessions, want 3", len(list))
	}
	for i := 1; i < len(list); i++ {
		if list[i-1].ID >= list[i].ID {
			t.Errorf("List() is not ordered by ID: %v", list)
			break
		}
	}
}

func TestEmbedded_RepaintRendersTheMirror(t *testing.T) {
	h := newTestHost(t, nil)
	id, err := h.Create(CreateSpec{Argv: sleepArgv(), Dir: t.TempDir(), Rows: 24, Cols: 80})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	painted, err := h.Repaint(id)
	if err != nil {
		t.Fatalf("Repaint: %v", err)
	}
	if !strings.HasPrefix(string(painted), "\x1b[0m\x1b[2J") {
		t.Errorf("repaint does not start with a reset and clear: %q", first(painted, 16))
	}
}

func first(b []byte, n int) string {
	if len(b) < n {
		n = len(b)
	}
	return string(b[:n])
}

// Reserve exists so the caller can build an environment that names the session
// before the session is there to name.
func TestEmbedded_ReserveThenCreateKeepsTheID(t *testing.T) {
	h := newTestHost(t, nil)

	reserved, err := h.Reserve()
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	if reserved == 0 {
		t.Fatal("Reserve returned 0, which means \"allocate one\" to Create")
	}

	id, err := h.Create(CreateSpec{ID: reserved, Argv: sleepArgv(), Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id != reserved {
		t.Errorf("Create used id %d, want the reserved %d", id, reserved)
	}
}

// A reservation must not be handed out again by the next allocation.
func TestEmbedded_ReserveIsNotReused(t *testing.T) {
	h := newTestHost(t, nil)

	reserved, err := h.Reserve()
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	id, err := h.Create(CreateSpec{Argv: sleepArgv(), Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == reserved {
		t.Errorf("Create allocated %d, which was already reserved", id)
	}
}

func TestEmbedded_CreateRefusesAnIDThatIsInUse(t *testing.T) {
	h := newTestHost(t, nil)

	id, err := h.Create(CreateSpec{Argv: sleepArgv(), Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := h.Create(CreateSpec{ID: id, Argv: sleepArgv(), Dir: t.TempDir()}); err == nil {
		t.Error("creating a second session with a live ID succeeded")
	}
}

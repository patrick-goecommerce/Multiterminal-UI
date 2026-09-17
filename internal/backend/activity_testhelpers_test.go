package backend

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// The debounce lives on the host now (internal/hub/activity_debounce.go), so a
// test that wants a session to be in a known confirmed state asks the host
// rather than writing a package-level map.
//
// That is also why there is no reset helper any more: the state belongs to an
// Embedded, and every test builds its own, so two tests can no longer leak
// into each other through a global.

// setConfirmed puts a session in a confirmed state, starting now.
func setConfirmed(t *testing.T, a *AppService, id int, state string) {
	t.Helper()
	setConfirmedSince(t, a, id, state, time.Now())
}

// setConfirmedSince puts a session in a confirmed state that began at since.
func setConfirmedSince(t *testing.T, a *AppService, id int, state string, since time.Time) {
	t.Helper()
	if err := a.host.ForceActivity(id, hub.Activity(state), since); err != nil {
		t.Fatalf("ForceActivity(%d, %q): %v", id, state, err)
	}
}

// confirmedOf reads back what the host has confirmed for a session.
func confirmedOf(a *AppService, id int) (string, time.Time) {
	state, since := a.host.ConfirmedActivity(id)
	return string(state), since
}

// currentStateForTest is the confirmed state, for a rewrite that only wants to
// move the timestamp and leave the state where it is.
func currentStateForTest(a *AppService, id int) string {
	state, _ := a.host.ConfirmedActivity(id)
	return string(state)
}

// backdateCandidate pushes an armed debounce candidate past the window so the
// next scan confirms it, instead of sleeping the test. It fails when nothing
// was armed, because a test that back-dates nothing and then asserts a
// confirmation would pass without having exercised the debounce at all.
func backdateCandidate(t *testing.T, a *AppService, id int) {
	t.Helper()
	emb, ok := a.host.(*hub.Embedded)
	if !ok {
		t.Fatalf("host is %T, not an *hub.Embedded", a.host)
	}
	if !emb.BackdateActivityForTest(id) {
		t.Fatal("no debounce candidate was armed; the scan never observed the new state")
	}
}

// newTestApp builds an AppService with every map initialised and a host of
// its own.
//
// Initialise new maps here as they are added. A nil map that production code
// writes to panics *while holding a.mu*, so the deferred cleanup then blocks on
// that mutex forever and the test hangs until the package times out instead of
// failing. That cost ten minutes per run until it was found (#186).
//
// The queues map is gone from this list on purpose: the prompt queue belongs
// to the host now, so there is nothing here to forget to initialise.
func newTestApp() *AppService {
	a := &AppService{
		launches:      make(map[int]launchSpec),
		sessionMode:   make(map[int]string),
		sessionIssues: make(map[int]*sessionIssue),
		chatSessions:  make(map[string]*ChatSession),
		chatBuffers:   make(map[string]*strings.Builder),
		finishStates:  make(map[int]*finishState),
		worktreeState: make(map[int]worktreeState),
		lastProbedCwd: make(map[int]string),
	}
	// The host's events have to reach the app, exactly as they do in
	// production. Without this a test sees a queue advance on the host and no
	// reaction in the window, which is the bug this wiring exists to prevent.
	a.host = hub.NewEmbedded(hub.Options{Sink: hub.SinkFunc(a.onHostEvent)})
	return a
}

// sessionDir returns a working directory for a test session.
//
// Not t.TempDir(), for the reason spelled out in internal/hub's copy: on
// Windows a directory that is a live process's working directory cannot be
// removed, and t.TempDir's cleanup runs before the host has released the
// session using it. That turned three tests here red on CI with a cleanup
// error and no assertion failure.
func sessionDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "mtui-session-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() {
		for i := 0; i < 40; i++ {
			if os.RemoveAll(dir) == nil {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	})
	return dir
}

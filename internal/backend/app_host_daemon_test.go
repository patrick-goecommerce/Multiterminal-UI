package backend

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/discovery"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// The default is the embedded host: the sessions belong to this process, and
// nothing about the daemon happens unless it was asked for.
func TestNewSessionHost_DefaultsToEmbedded(t *testing.T) {
	a := &AppService{cfg: config.Config{}}
	host := a.newSessionHost()
	t.Cleanup(host.Release)

	if _, ok := host.(*hub.Embedded); !ok {
		t.Errorf("host = %T, want an embedded host", host)
	}
	if len(a.bindWarnings) != 0 {
		t.Errorf("the default produced warnings: %v", a.bindWarnings)
	}
}

// A daemon that cannot be reached must not stop the window from opening. The
// app falls back to local sessions and says so, because a pane that runs is
// worth more than a pane that does not.
func TestNewSessionHost_FallsBackWhenTheDaemonIsUnreachable(t *testing.T) {
	// An empty runtime dir: no record, and no mtuid next to the test binary.
	t.Setenv(discovery.EnvDirOverride, filepath.Join(t.TempDir(), "runtime"))

	a := &AppService{cfg: config.Config{SessionHost: "daemon"}}
	host := a.newSessionHost()
	t.Cleanup(host.Release)

	if _, ok := host.(*hub.Embedded); !ok {
		t.Fatalf("host = %T, want the embedded fallback", host)
	}
	if len(a.bindWarnings) != 1 {
		t.Fatalf("got %d warnings, want exactly one about the daemon", len(a.bindWarnings))
	}
	if w := a.bindWarnings[0]; w.Service != "daemon" || !strings.Contains(w.Detail, "mtuid") {
		t.Errorf("warning = %+v, want it to name the daemon and mtuid", w)
	}
}

// UsesSessionDaemon is what the frontend asks before deciding whether a
// restore should re-attach or launch.
func TestUsesSessionDaemon_FalseForTheEmbeddedHost(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)
	if a.UsesSessionDaemon() {
		t.Error("an embedded host reported that sessions outlive the window")
	}
}

// A window that starts while the daemon is already holding sessions has to
// find them, or it would launch duplicates next to the running agents.
func TestListLiveSessions_ReportsWhatTheHostHolds(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)

	if got := a.ListLiveSessions(); len(got) != 0 {
		t.Fatalf("a fresh host reported %d sessions, want none", len(got))
	}

	id := a.CreateSession(sleepArgvForTest(), t.TempDir(), 24, 80, "claude")
	if id <= 0 {
		t.Fatalf("CreateSession returned %d", id)
	}

	live := a.ListLiveSessions()
	if len(live) != 1 {
		t.Fatalf("got %d live sessions, want 1", len(live))
	}
	if live[0].ID != id || live[0].Mode != "claude" || !live[0].Running {
		t.Errorf("live session = %+v, want id %d, mode claude, running", live[0], id)
	}
}

func TestAttachSession_UnknownSessionIsReported(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)

	if a.AttachSession(404, 24, 80) {
		t.Error("attaching to a session the host does not have succeeded")
	}
}

// Re-attaching has to bring the pane back with what is already on its screen,
// not with a blank one: that replay is the whole reason the host keeps a ring.
func TestAttachSession_ReplaysWhatTheSessionAlreadyProduced(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)

	id := a.CreateSession(printArgv("attach-marker"), t.TempDir(), 24, 80, "shell")
	if id <= 0 {
		t.Fatalf("CreateSession returned %d", id)
	}
	// Let the session produce its output and drain it, the way a running
	// window would have.
	if got := drainBatcher(t, a, id, "attach-marker"); !strings.Contains(got, "attach-marker") {
		t.Fatalf("first pass never saw the marker: %q", got)
	}

	// A new window attaches to the same session.
	if !a.AttachSession(id, 24, 80) {
		t.Fatal("AttachSession refused a live session")
	}
	if got := drainBatcher(t, a, id, "attach-marker"); !strings.Contains(got, "attach-marker") {
		t.Errorf("re-attached pane got %q, want the replayed marker", got)
	}
}

// The mode is what tells the UI a pane is a Claude pane rather than a shell.
// It lives in this process, so a window that attaches to a session it did not
// start has to take it from the host.
func TestAttachSession_RecoversTheModeFromTheHost(t *testing.T) {
	a := newTestApp()
	t.Cleanup(a.host.Release)

	id := a.CreateSession(sleepArgvForTest(), t.TempDir(), 24, 80, "claude")
	if id <= 0 {
		t.Fatalf("CreateSession returned %d", id)
	}

	// Forget it, as a restarted window would have.
	a.mu.Lock()
	delete(a.sessionMode, id)
	a.mu.Unlock()

	if !a.AttachSession(id, 24, 80) {
		t.Fatal("AttachSession refused a live session")
	}
	a.mu.Lock()
	mode := a.sessionMode[id]
	a.mu.Unlock()
	if mode != "claude" {
		t.Errorf("mode after attach = %q, want %q", mode, "claude")
	}
}

// The whole chain, end to end: AppService creates a session on a host in
// another process, the bytes come back over the socket, and closing the
// client leaves the session running for the next window.
//
// Every layer in between is real here (the HTTP server, the WebSocket, the
// ring, the PTY); only the process boundary is simulated, which is the one
// part cmd/mtuid's own tests cover.
func TestAppService_AgainstARemoteHost(t *testing.T) {
	daemon := hub.NewEmbedded(hub.Options{Version: "test"})
	t.Cleanup(daemon.Release)
	server := hub.NewServer(daemon, "e2e-token")
	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)

	remote, err := hub.Dial(strings.TrimPrefix(ts.URL, "http://"), "e2e-token", hub.DialOptions{})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if !remote.WaitReady(10 * time.Second) {
		t.Fatal("stream socket never came up")
	}

	a := newTestApp()
	a.host = remote

	id := a.CreateSession(printArgv("remote-marker"), t.TempDir(), 24, 80, "shell")
	if id <= 0 {
		t.Fatalf("CreateSession returned %d", id)
	}
	if got := drainBatcher(t, a, id, "remote-marker"); !strings.Contains(got, "remote-marker") {
		t.Errorf("batched output = %q, want the marker", got)
	}

	// Closing the window: the client lets go, the daemon keeps the session.
	a.host.Release()
	if _, err := daemon.Get(id); err != nil {
		t.Fatalf("the session died with the client: %v", err)
	}

	// The next window finds it and attaches.
	next, err := hub.Dial(strings.TrimPrefix(ts.URL, "http://"), "e2e-token", hub.DialOptions{})
	if err != nil {
		t.Fatalf("second Dial: %v", err)
	}
	t.Cleanup(next.Release)
	if !next.WaitReady(10 * time.Second) {
		t.Fatal("second stream socket never came up")
	}

	b := newTestApp()
	b.host = next
	if !b.UsesSessionDaemon() {
		t.Error("a remote host did not report that sessions outlive the window")
	}
	live := b.ListLiveSessions()
	if len(live) != 1 || live[0].ID != id {
		t.Fatalf("second window sees %+v, want session %d", live, id)
	}
	if !b.AttachSession(id, 24, 80) {
		t.Fatal("AttachSession refused the surviving session")
	}
	if got := drainBatcher(t, b, id, "remote-marker"); !strings.Contains(got, "remote-marker") {
		t.Errorf("re-attached output = %q, want the replayed marker", got)
	}
}

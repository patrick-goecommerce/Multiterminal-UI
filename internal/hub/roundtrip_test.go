package hub

import (
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const testToken = "test-token"

// serveHost puts a Host behind an HTTP server and returns its address.
func serveHost(t *testing.T, host Host) (addr string, srv *Server) {
	t.Helper()
	srv = NewServer(host, testToken)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return strings.TrimPrefix(ts.URL, "http://"), srv
}

// dialTest connects a Remote and waits for its socket.
func dialTest(t *testing.T, addr string, sink EventSink) *Remote {
	t.Helper()
	r, err := Dial(addr, testToken, DialOptions{Sink: sink})
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(r.Release)
	if !r.WaitReady(10 * time.Second) {
		t.Fatal("stream socket never came up")
	}
	return r
}

// shellArgv returns an interactive shell, so a test can type into it.
func shellArgv() []string {
	if isWindows() {
		return []string{comspec()}
	}
	return []string{"/bin/sh"}
}

func TestServer_RejectsRequestsWithoutTheToken(t *testing.T) {
	host := newTestHost(t, nil)
	addr, _ := serveHost(t, host)

	resp, err := http.Get("http://" + addr + "/v1/hub")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestDial_RefusesAProtocolItDoesNotSpeak(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, Info{HubID: "future", Protocol: Protocol + 1})
	}))
	defer ts.Close()

	_, err := Dial(strings.TrimPrefix(ts.URL, "http://"), testToken, DialOptions{})
	if !errors.Is(err, ErrProtocol) {
		t.Errorf("Dial against a newer daemon = %v, want ErrProtocol", err)
	}
}

func TestRemote_CreateListAndClose(t *testing.T) {
	host := newTestHost(t, nil)
	addr, _ := serveHost(t, host)
	r := dialTest(t, addr, nil)

	id, err := r.Create(CreateSpec{Argv: sleepArgv(), Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	list := r.List()
	if len(list) != 1 || list[0].ID != id {
		t.Fatalf("List() = %v, want one session with id %d", list, id)
	}
	if _, err := r.Get(id); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if err := r.Close(id); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if len(host.List()) != 0 {
		t.Error("the daemon still has the session after the client closed it")
	}
	if _, err := r.Get(id); !errors.Is(err, ErrNoSession) {
		t.Errorf("Get after Close = %v, want ErrNoSession", err)
	}
}

func TestRemote_OutputArrivesOverTheSocket(t *testing.T) {
	host := newTestHost(t, nil)
	addr, _ := serveHost(t, host)
	r := dialTest(t, addr, nil)

	id, err := r.Create(CreateSpec{Argv: printArgv("over-the-wire"), Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sub, err := r.Attach(id, ReplayAll)
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	defer sub.Close()

	text, truncated := collect(t, sub, "over-the-wire")
	if truncated {
		t.Error("a fresh attach must not report truncation")
	}
	if !strings.Contains(text, "over-the-wire") {
		t.Errorf("output = %q, want the marker", text)
	}
}

// Input has to reach the PTY, and the PTY's echo has to come back, or the
// socket is only half a terminal.
func TestRemote_InputReachesThePTY(t *testing.T) {
	host := newTestHost(t, nil)
	addr, _ := serveHost(t, host)
	r := dialTest(t, addr, nil)

	id, err := r.Create(CreateSpec{Argv: shellArgv(), Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sub, err := r.Attach(id, ReplayAll)
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	defer sub.Close()

	if err := r.Write(id, []byte("echo typed-marker\r\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	text, _ := collect(t, sub, "typed-marker")
	if !strings.Contains(text, "typed-marker") {
		t.Errorf("output = %q, want the typed marker", text)
	}
}

func TestRemote_EventsReachTheSink(t *testing.T) {
	sink := newSink()
	// The daemon's host reports into the server's sink, which fans out to
	// clients; the client hands them to its own sink. The indirection through
	// the variable is what breaks the chicken-and-egg: the server needs the
	// host, and the host needs the server's sink.
	var server *Server
	host := NewEmbedded(Options{Version: "test", Sink: SinkFunc(func(name string, payload any) {
		server.Sink().Emit(name, payload)
	})})
	t.Cleanup(host.Release)
	server = NewServer(host, testToken)
	ts := httptest.NewServer(server.Handler())
	t.Cleanup(ts.Close)

	r := dialTest(t, strings.TrimPrefix(ts.URL, "http://"), sink)
	if _, err := r.Create(CreateSpec{Argv: printArgv("bye"), Dir: t.TempDir()}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	sink.await(t, EventSessionCreated)
	sink.await(t, EventSessionExited)
}

// Release is where the daemon earns its keep: the client goes away and the
// sessions do not.
func TestRemote_ReleaseLeavesSessionsRunning(t *testing.T) {
	host := newTestHost(t, nil)
	addr, _ := serveHost(t, host)
	r := dialTest(t, addr, nil)

	id, err := r.Create(CreateSpec{Argv: sleepArgv(), Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	r.Release()

	summary, err := host.Get(id)
	if err != nil {
		t.Fatalf("session gone after the client released: %v", err)
	}
	if summary.Status != StatusRunning {
		t.Errorf("status = %q, want %q", summary.Status, StatusRunning)
	}

	// A second client picks the same session up again.
	r2 := dialTest(t, addr, nil)
	if list := r2.List(); len(list) != 1 || list[0].ID != id {
		t.Errorf("reconnecting client sees %v, want session %d", list, id)
	}
}

// breakableProxy forwards TCP to an upstream address and can drop every live
// connection on demand, which is how a test makes the socket fail without
// taking the daemon down with it.
type breakableProxy struct {
	ln       net.Listener
	upstream string

	mu    sync.Mutex
	conns []net.Conn
}

func newBreakableProxy(t *testing.T, upstream string) *breakableProxy {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	p := &breakableProxy{ln: ln, upstream: upstream}
	go p.accept()
	t.Cleanup(func() { _ = ln.Close() })
	return p
}

func (p *breakableProxy) addr() string { return p.ln.Addr().String() }

func (p *breakableProxy) accept() {
	for {
		down, err := p.ln.Accept()
		if err != nil {
			return
		}
		up, err := net.Dial("tcp", p.upstream)
		if err != nil {
			_ = down.Close()
			continue
		}
		p.mu.Lock()
		p.conns = append(p.conns, down, up)
		p.mu.Unlock()
		go func() { _, _ = io.Copy(up, down) }()
		go func() { _, _ = io.Copy(down, up) }()
	}
}

func (p *breakableProxy) breakAll() {
	p.mu.Lock()
	conns := p.conns
	p.conns = nil
	p.mu.Unlock()
	for _, c := range conns {
		_ = c.Close()
	}
}

// A dropped socket must cost a round trip, not the pane. The client re-attaches
// at the offset it last delivered and picks up where it left off.
func TestRemote_ReconnectsAndResumesTheStream(t *testing.T) {
	host := newTestHost(t, nil)
	addr, _ := serveHost(t, host)
	proxy := newBreakableProxy(t, addr)
	r := dialTest(t, proxy.addr(), nil)

	id, err := host.Create(CreateSpec{Argv: shellArgv(), Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	sub, err := r.Attach(id, ReplayAll)
	if err != nil {
		t.Fatalf("Attach: %v", err)
	}
	defer sub.Close()

	if err := host.Write(id, []byte("echo before-break\r\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if text, _ := collect(t, sub, "before-break"); !strings.Contains(text, "before-break") {
		t.Fatalf("pre-break output = %q", text)
	}

	proxy.breakAll()

	// Produced while the client is disconnected: it must arrive after the
	// re-attach, not be lost with the socket.
	if err := host.Write(id, []byte("echo after-break\r\n")); err != nil {
		t.Fatalf("Write after break: %v", err)
	}
	text, truncated := collect(t, sub, "after-break")
	if !strings.Contains(text, "after-break") {
		t.Errorf("post-reconnect output = %q, want the marker", text)
	}
	if truncated {
		t.Error("the ring still held those bytes, so the resume must not report a gap")
	}
}

func TestRemote_ReserveComesFromTheDaemon(t *testing.T) {
	host := newTestHost(t, nil)
	addr, _ := serveHost(t, host)
	r := dialTest(t, addr, nil)

	reserved, err := r.Reserve()
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}
	id, err := r.Create(CreateSpec{ID: reserved, Argv: sleepArgv(), Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id != reserved {
		t.Errorf("Create used id %d, want the reserved %d", id, reserved)
	}
	if _, err := host.Get(reserved); err != nil {
		t.Errorf("the daemon does not have session %d: %v", reserved, err)
	}
}

// The screen and agent-state calls have to work over the wire too, or a
// remote client is blind to everything but raw bytes.
func TestRemote_ScreenAndAgentStateOverTheWire(t *testing.T) {
	host := newTestHost(t, nil)
	addr, _ := serveHost(t, host)
	r := dialTest(t, addr, nil)

	id, err := r.Create(CreateSpec{Argv: printArgv("screen-marker"), Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	deadline := time.Now().Add(15 * time.Second)
	for {
		text, err := r.PlainText(id)
		if err != nil {
			t.Fatalf("PlainText: %v", err)
		}
		if strings.Contains(text, "screen-marker") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("PlainText never showed the marker: %q", text)
		}
		time.Sleep(20 * time.Millisecond)
	}

	rows, err := r.PlainTextRows(id, 0, -1)
	if err != nil {
		t.Fatalf("PlainTextRows: %v", err)
	}
	if len(rows) == 0 {
		t.Error("PlainTextRows returned nothing")
	}

	if err := r.SetStatusline(id, 1.25, 42, "opus"); err != nil {
		t.Fatalf("SetStatusline: %v", err)
	}
	if err := r.SetHookActivity(id, ActivityDone); err != nil {
		t.Fatalf("SetHookActivity: %v", err)
	}
	if err := r.SetHookSessionID(id, "agent-uuid"); err != nil {
		t.Fatalf("SetHookSessionID: %v", err)
	}

	summary, err := r.Get(id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if summary.Cost != 1.25 || summary.ContextPct != 42 || summary.Model != "opus" {
		t.Errorf("statusline round trip = cost %v ctx %d model %q", summary.Cost, summary.ContextPct, summary.Model)
	}
	if summary.Activity != ActivityDone {
		t.Errorf("activity = %q, want %q", summary.Activity, ActivityDone)
	}

	if err := r.ClearHookData(id); err != nil {
		t.Fatalf("ClearHookData: %v", err)
	}
}

// The confirmed state has to survive the socket: with session_host: daemon the
// window, the CLI and the queue all read it through a Remote, and a debounce
// that only worked in-process would mean the daemon's agents look like they
// never settle.
func TestRemote_ConfirmedActivityCrossesTheWire(t *testing.T) {
	host := newTestHost(t, nil)
	addr, _ := serveHost(t, host)
	client := dialTest(t, addr, nil)

	id, err := host.Create(CreateSpec{Argv: shellArgv(), Dir: t.TempDir(), Rows: 24, Cols: 80})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	began := time.Now().Add(-90 * time.Minute).Truncate(time.Second)
	if err := client.ForceActivity(id, ActivityDone, began); err != nil {
		t.Fatalf("ForceActivity over the wire: %v", err)
	}

	state, got := client.ConfirmedActivity(id)
	if state != ActivityDone {
		t.Errorf("state = %q, want %q", state, ActivityDone)
	}
	if !got.Equal(began) {
		t.Errorf("began = %s, want %s", got, began)
	}
	// And the host itself agrees, so the client is not just echoing itself.
	if local, _ := host.ConfirmedActivity(id); local != ActivityDone {
		t.Errorf("the host has %q, want %q", local, ActivityDone)
	}
}

// A seed is refused once a state is confirmed, and that rule has to hold over
// the wire too, or a restore racing the scan would mis-stamp a later change.
func TestRemote_SeedActivityKeepsItsRules(t *testing.T) {
	host := newTestHost(t, nil)
	addr, _ := serveHost(t, host)
	client := dialTest(t, addr, nil)

	id, err := host.Create(CreateSpec{Argv: shellArgv(), Dir: t.TempDir(), Rows: 24, Cols: 80})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	seeded := time.Now().Add(-3 * time.Hour).Truncate(time.Second)
	if err := client.SeedActivity(id, ActivityDone, seeded); err != nil {
		t.Fatalf("SeedActivity: %v", err)
	}
	// Nothing is confirmed yet, so the seed is held rather than applied.
	if state, _ := client.ConfirmedActivity(id); state != "" {
		t.Errorf("a seed confirmed a state on its own: %q", state)
	}

	now := time.Now()
	if err := client.ForceActivity(id, ActivityActive, now); err != nil {
		t.Fatalf("ForceActivity: %v", err)
	}
	// Now that a state is confirmed, a second seed must be refused.
	if err := client.SeedActivity(id, ActivityDone, seeded); err != nil {
		t.Fatalf("SeedActivity: %v", err)
	}
	if _, began := client.ConfirmedActivity(id); began.Equal(seeded) {
		t.Error("a seed overwrote a state that was already confirmed")
	}
}

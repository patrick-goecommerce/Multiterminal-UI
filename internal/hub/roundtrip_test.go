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

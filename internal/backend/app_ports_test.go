package backend

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/discovery"
)

// useTempDiscoveryDir redirects the discovery directory so these tests never
// touch the real per-user runtime directory.
func useTempDiscoveryDir(t *testing.T) {
	t.Helper()
	t.Setenv(discovery.EnvDirOverride, filepath.Join(t.TempDir(), "runtime"))
}

// signalFocusLikeMain mirrors what main.signalFocus does, so the handshake is
// exercised end to end from a caller's point of view.
func signalFocusLikeMain(t *testing.T, token string) error {
	t.Helper()
	rec, err := discovery.Resolve(discovery.ServiceFocus)
	if err != nil {
		return err
	}
	conn, err := net.DialTimeout("tcp", rec.Addr(), 2*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write([]byte(token + "\n")); err != nil {
		return err
	}
	return nil
}

func TestStartFocusListenerPublishesEphemeralPort(t *testing.T) {
	useTempDiscoveryDir(t)
	a := newTestApp()

	if err := a.startFocusListener(); err != nil {
		t.Fatalf("startFocusListener: %v", err)
	}
	t.Cleanup(a.releaseDiscoveryRecords)

	rec, err := discovery.Resolve(discovery.ServiceFocus)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if rec.Port == 0 {
		t.Fatal("published port is 0; the OS-assigned port was not resolved")
	}
	if rec.Port == 41987 {
		t.Fatal("still bound to the old machine-wide fixed port 41987")
	}
	if rec.PID != os.Getpid() {
		t.Fatalf("record pid = %d, want %d", rec.PID, os.Getpid())
	}
	if a.focusToken == "" || a.focusToken != rec.Token {
		t.Fatalf("listener token %q does not match published token %q", a.focusToken, rec.Token)
	}
	if !discovery.Probe(rec, 2*time.Second) {
		t.Fatal("nothing is listening on the published focus port")
	}
}

// Two instances must both get a working listener — that is the whole point of
// dropping the fixed port.
func TestTwoFocusListenersBothBind(t *testing.T) {
	useTempDiscoveryDir(t)

	first := newTestApp()
	if err := first.startFocusListener(); err != nil {
		t.Fatalf("first startFocusListener: %v", err)
	}
	firstRec, err := discovery.Resolve(discovery.ServiceFocus)
	if err != nil {
		t.Fatalf("Resolve after first: %v", err)
	}

	second := newTestApp()
	if err := second.startFocusListener(); err != nil {
		t.Fatalf("second startFocusListener: %v", err)
	}
	t.Cleanup(second.releaseDiscoveryRecords)
	secondRec, err := discovery.Resolve(discovery.ServiceFocus)
	if err != nil {
		t.Fatalf("Resolve after second: %v", err)
	}

	if firstRec.Port == secondRec.Port {
		t.Fatalf("both instances bound the same port %d", firstRec.Port)
	}
	if !discovery.Probe(firstRec, 2*time.Second) || !discovery.Probe(secondRec, 2*time.Second) {
		t.Fatal("expected both listeners to be reachable")
	}
	if firstRec.Token == secondRec.Token {
		t.Fatal("both instances published the same token")
	}
}

func TestFocusHandshakeAcceptsMatchingToken(t *testing.T) {
	useTempDiscoveryDir(t)
	a := newTestApp()
	if err := a.startFocusListener(); err != nil {
		t.Fatalf("startFocusListener: %v", err)
	}
	t.Cleanup(a.releaseDiscoveryRecords)

	if err := signalFocusLikeMain(t, a.focusToken); err != nil {
		t.Fatalf("focus signal: %v", err)
	}
	// mainWindow is nil in tests, so success is "the listener stayed healthy
	// and still serves the next request" rather than an observable raise.
	if err := signalFocusLikeMain(t, a.focusToken); err != nil {
		t.Fatalf("second focus signal: %v", err)
	}
}

func TestFocusHandshakeRejectsWrongToken(t *testing.T) {
	useTempDiscoveryDir(t)
	a := newTestApp()
	if err := a.startFocusListener(); err != nil {
		t.Fatalf("startFocusListener: %v", err)
	}
	t.Cleanup(a.releaseDiscoveryRecords)

	rec, err := discovery.Resolve(discovery.ServiceFocus)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	for name, payload := range map[string]string{
		"empty":       "",
		"wrong token": "00000000000000000000000000000000",
		"truncated":   a.focusToken[:len(a.focusToken)-1],
	} {
		conn, err := net.DialTimeout("tcp", rec.Addr(), 2*time.Second)
		if err != nil {
			t.Fatalf("%s: dial: %v", name, err)
		}
		_, _ = conn.Write([]byte(payload + "\n"))
		conn.Close()
	}

	// A rejected handshake must not take the listener down.
	if err := signalFocusLikeMain(t, a.focusToken); err != nil {
		t.Fatalf("listener stopped serving after rejected handshakes: %v", err)
	}
}

// Nothing may be dialled once the owning process is gone — the token in a
// leftover record is exactly the capability that must not be reused.
func TestFocusSignalIgnoresStaleRecord(t *testing.T) {
	useTempDiscoveryDir(t)
	a := newTestApp()
	if err := a.startFocusListener(); err != nil {
		t.Fatalf("startFocusListener: %v", err)
	}
	t.Cleanup(a.releaseDiscoveryRecords)

	a.releaseDiscoveryRecords()
	if err := signalFocusLikeMain(t, a.focusToken); !errors.Is(err, discovery.ErrNotPublished) {
		t.Fatalf("focus signal after cleanup = %v, want ErrNotPublished", err)
	}
}

func TestStartMCPServerUsesEphemeralPortByDefault(t *testing.T) {
	useTempDiscoveryDir(t)
	a := newTestAgentControlService()

	port, err := a.startMCPServer(config.DefaultConfig().MCPServer.Port)
	if err != nil {
		t.Fatalf("startMCPServer: %v", err)
	}
	if port == 0 {
		t.Fatal("startMCPServer returned port 0")
	}
	if port == 51533 {
		t.Fatal("still bound to the old machine-wide fixed port 51533")
	}

	rec, err := discovery.Resolve(discovery.ServiceMCP)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if rec.Port != port {
		t.Fatalf("published port %d, bound port %d", rec.Port, port)
	}
	if !discovery.Probe(rec, 2*time.Second) {
		t.Fatal("nothing is listening on the published MCP port")
	}
	if got, want := mcpURL(rec.Port), fmt.Sprintf("http://127.0.0.1:%d/mcp", port); got != want {
		t.Fatalf("mcpURL = %q, want %q", got, want)
	}
}

// A port the user typed into the config must still be used verbatim; port 0 is
// only the new default.
func TestStartMCPServerHonoursExplicitConfiguredPort(t *testing.T) {
	useTempDiscoveryDir(t)

	// Ask the OS for a free port, release it, then demand exactly that one.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe listen: %v", err)
	}
	wanted := probe.Addr().(*net.TCPAddr).Port
	probe.Close()

	a := newTestAgentControlService()
	port, err := a.startMCPServer(wanted)
	if err != nil {
		t.Skipf("configured port %d was taken again before the server could bind: %v", wanted, err)
	}
	if port != wanted {
		t.Fatalf("bound port %d, want the explicitly configured %d", port, wanted)
	}
	rec, err := discovery.Resolve(discovery.ServiceMCP)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if rec.Port != wanted {
		t.Fatalf("published port %d, want %d", rec.Port, wanted)
	}
}

func TestStartMCPServerReportsBindFailure(t *testing.T) {
	useTempDiscoveryDir(t)

	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("blocker listen: %v", err)
	}
	defer blocker.Close()
	taken := blocker.Addr().(*net.TCPAddr).Port

	a := newTestAgentControlService()
	if _, err := a.startMCPServer(taken); err == nil {
		t.Fatalf("startMCPServer on the occupied port %d succeeded, want an error", taken)
	}
	if _, err := discovery.Read(discovery.ServiceMCP); !errors.Is(err, discovery.ErrNotPublished) {
		t.Fatalf("a failed bind published a record anyway: %v", err)
	}
}

// A swallowed bind error is how issue #183 stayed invisible; CheckHealth must
// carry it to the frontend.
func TestCheckHealthSurfacesBindWarnings(t *testing.T) {
	a := newTestApp()

	if got := a.CheckHealth().BindWarnings; len(got) != 0 {
		t.Fatalf("fresh service reported warnings: %+v", got)
	}

	a.recordBindWarning("focus", errors.New("listen tcp 127.0.0.1:0: permission denied"))
	a.recordBindWarning("mcp", errors.New("mcp server listen on port 51533: address already in use"))
	a.recordBindWarning("ignored", nil)

	warnings := a.CheckHealth().BindWarnings
	if len(warnings) != 2 {
		t.Fatalf("got %d warnings, want 2: %+v", len(warnings), warnings)
	}
	if warnings[0].Service != "focus" || warnings[1].Service != "mcp" {
		t.Fatalf("unexpected services: %+v", warnings)
	}
	if warnings[1].Detail == "" {
		t.Fatal("warning detail is empty; the underlying error was lost")
	}

	// The snapshot must be a copy — the frontend must not be able to mutate
	// the service's state through it.
	warnings[0].Service = "tampered"
	if a.CheckHealth().BindWarnings[0].Service != "focus" {
		t.Fatal("CheckHealth handed out the live slice")
	}
}

// A listener that cannot bind must leave a warning behind, and must not stop
// the other listener from coming up.
func TestStartLocalListenersRecordsMCPBindFailure(t *testing.T) {
	useTempDiscoveryDir(t)

	blocker, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("blocker listen: %v", err)
	}
	defer blocker.Close()

	a := newTestApp()
	a.cfg = config.DefaultConfig()
	a.cfg.MCPServer.Port = blocker.Addr().(*net.TCPAddr).Port
	a.startLocalListeners()
	t.Cleanup(a.releaseDiscoveryRecords)

	if _, err := discovery.Resolve(discovery.ServiceFocus); err != nil {
		t.Fatalf("focus listener did not come up: %v", err)
	}
	warnings := a.CheckHealth().BindWarnings
	if len(warnings) != 1 || warnings[0].Service != "mcp" {
		t.Fatalf("warnings = %+v, want exactly one for mcp", warnings)
	}
	if _, err := discovery.Read(discovery.ServiceMCP); !errors.Is(err, discovery.ErrNotPublished) {
		t.Fatalf("failed MCP bind published a record: %v", err)
	}
}

func TestStartLocalListenersSkipsDisabledMCP(t *testing.T) {
	useTempDiscoveryDir(t)

	a := newTestApp()
	a.cfg = config.DefaultConfig()
	disabled := false
	a.cfg.MCPServer.Enabled = &disabled
	a.startLocalListeners()
	t.Cleanup(a.releaseDiscoveryRecords)

	if _, err := discovery.Resolve(discovery.ServiceFocus); err != nil {
		t.Fatalf("focus listener did not come up: %v", err)
	}
	if _, err := discovery.Read(discovery.ServiceMCP); !errors.Is(err, discovery.ErrNotPublished) {
		t.Fatalf("disabled MCP server published a record: %v", err)
	}
	if warnings := a.CheckHealth().BindWarnings; len(warnings) != 0 {
		t.Fatalf("disabled MCP server produced warnings: %+v", warnings)
	}
}

func TestReleaseDiscoveryRecordsRemovesBoth(t *testing.T) {
	useTempDiscoveryDir(t)

	if _, err := discovery.Publish(discovery.ServiceFocus, 40100); err != nil {
		t.Fatalf("publish focus: %v", err)
	}
	if _, err := discovery.Publish(discovery.ServiceMCP, 40101); err != nil {
		t.Fatalf("publish mcp: %v", err)
	}

	a := newTestApp()
	a.releaseDiscoveryRecords()

	for _, svc := range []discovery.Service{discovery.ServiceFocus, discovery.ServiceMCP} {
		if _, err := discovery.Read(svc); !errors.Is(err, discovery.ErrNotPublished) {
			t.Fatalf("%s record survived shutdown: %v", svc, err)
		}
	}
	// Shutdown runs on paths that may never have published anything.
	a.releaseDiscoveryRecords()
}

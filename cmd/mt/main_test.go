package main

import (
	"bytes"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/discovery"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// These tests drive the real command against a real daemon: a real Embedded
// host behind a real listener, found through a real discovery record. The
// point is that nothing about the path from `mt ls` to a PTY is stubbed,
// because every bug this CLI can have lives in that path.

// testDaemon starts a hub server and publishes it where the CLI will look.
func testDaemon(t *testing.T) hub.Host {
	t.Helper()
	// Per-test runtime directory, so a developer's own daemon is neither
	// found by the test nor disturbed by it.
	t.Setenv(discovery.EnvDirOverride, t.TempDir())

	host := hub.NewEmbedded(hub.Options{Version: "test"})
	t.Cleanup(host.Release)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	rec, err := discovery.Publish(discovery.ServiceHub, ln.Addr().(*net.TCPAddr).Port)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	srv := hub.NewServer(host, rec.Token)
	httpSrv := &http.Server{Handler: srv.Handler()}
	go func() { _ = httpSrv.Serve(ln) }()
	t.Cleanup(func() { _ = httpSrv.Close() })
	return host
}

// cli runs the command and returns its exit code with what it printed.
func cli(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errBuf bytes.Buffer
	code = run(args, &out, &errBuf)
	return code, out.String(), errBuf.String()
}

// startSession puts a shell on the host and waits until it has produced its
// prompt, so that a read has something to find.
func startSession(t *testing.T, host hub.Host) int {
	t.Helper()
	argv := []string{"/bin/sh"}
	if runtime.GOOS == "windows" {
		argv = []string{os.Getenv("COMSPEC")}
	}
	id, err := host.Create(hub.CreateSpec{Argv: argv, Dir: t.TempDir(), Rows: 24, Cols: 80})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = host.Close(id) })
	return id
}

// A CLI whose first act on a machine with no daemon is to start one would
// leave a process behind for every tab-completion mistake. It reports instead,
// with a code a script can branch on.
func TestRun_WithoutADaemonSaysSoAndDoesNotStartOne(t *testing.T) {
	t.Setenv(discovery.EnvDirOverride, t.TempDir())

	code, _, stderr := cli(t, "ls")
	if code != exitNoHub {
		t.Errorf("exit = %d, want %d", code, exitNoHub)
	}
	if !strings.Contains(stderr, "Daemon") {
		t.Errorf("stderr = %q, want it to mention the daemon", stderr)
	}
	if _, err := discovery.Read(discovery.ServiceHub); err == nil {
		t.Error("a daemon record appeared; the CLI started one")
	}
}

func TestRun_UnknownCommandIsAUsageError(t *testing.T) {
	code, _, stderr := cli(t, "frobnicate")
	if code != exitUsage {
		t.Errorf("exit = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "frobnicate") {
		t.Errorf("stderr = %q, want it to name the bad command", stderr)
	}
}

func TestRun_HelpWorksWithoutADaemon(t *testing.T) {
	t.Setenv(discovery.EnvDirOverride, t.TempDir())

	code, stdout, _ := cli(t, "--help")
	if code != exitOK {
		t.Errorf("exit = %d, want %d", code, exitOK)
	}
	for _, want := range []string{"ls", "read", "send", "wait", "kill", "hub"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help does not mention %q", want)
		}
	}
}

func TestLs_EmptyHubIsNotAnError(t *testing.T) {
	testDaemon(t)

	code, stdout, _ := cli(t, "ls")
	if code != exitOK {
		t.Errorf("exit = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stdout, "Keine Sessions") {
		t.Errorf("stdout = %q", stdout)
	}
}

// A script doing `mt ls --json | jq '.[]'` must not break on a quiet hub, so
// an empty list is [] and never null.
func TestLs_EmptyJSONIsAnArray(t *testing.T) {
	testDaemon(t)

	code, stdout, _ := cli(t, "ls", "--json")
	if code != exitOK {
		t.Fatalf("exit = %d", code)
	}
	if strings.TrimSpace(stdout) != "[]" {
		t.Errorf("stdout = %q, want []", stdout)
	}
}

func TestLs_ShowsASession(t *testing.T) {
	host := testDaemon(t)
	id := startSession(t, host)

	code, stdout, stderr := cli(t, "ls")
	if code != exitOK {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	if !strings.Contains(stdout, "ZUSTAND") {
		t.Errorf("stdout has no header:\n%s", stdout)
	}
	if !strings.Contains(stdout, strconv.Itoa(id)) {
		t.Errorf("stdout does not list session %d:\n%s", id, stdout)
	}
}

func TestSendAndRead_GoThroughToThePTY(t *testing.T) {
	host := testDaemon(t)
	id := startSession(t, host)

	if code, _, stderr := cli(t, "send", strconv.Itoa(id), "echo", "mt-cli-marker"); code != exitOK {
		t.Fatalf("send exit = %d, stderr = %s", code, stderr)
	}

	// The shell has to run the command and draw the result; poll rather than
	// sleep a fixed amount, because a loaded CI box is slower than a laptop.
	deadline := time.Now().Add(10 * time.Second)
	for {
		_, stdout, _ := cli(t, "read", strconv.Itoa(id))
		// The echoed command line also contains the marker, so look for the
		// output line: the marker on a line of its own.
		for _, line := range strings.Split(stdout, "\n") {
			if strings.TrimSpace(line) == "mt-cli-marker" {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("the marker never showed up on the screen:\n%s", stdout)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestRead_RejectsAnUnknownSession(t *testing.T) {
	testDaemon(t)

	code, _, stderr := cli(t, "read", "4242")
	if code != exitError {
		t.Errorf("exit = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr, "4242") {
		t.Errorf("stderr = %q, want it to name the session", stderr)
	}
}

func TestRead_RejectsSomethingThatIsNotAnID(t *testing.T) {
	testDaemon(t)

	code, _, stderr := cli(t, "read", "pane-two")
	if code != exitError {
		t.Errorf("exit = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr, "mt ls") {
		t.Errorf("stderr = %q, want it to point at `mt ls`", stderr)
	}
}

func TestHub_ReportsTheDaemon(t *testing.T) {
	testDaemon(t)

	code, stdout, stderr := cli(t, "hub")
	if code != exitOK {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	for _, want := range []string{"Hub", "Protokoll", "PID", "Sessions"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("hub output has no %q:\n%s", want, stdout)
		}
	}
}

// Stopping the daemon ends every agent it holds, so it is not something to do
// by accident while sessions are running.
func TestHub_StopRefusesWhileSessionsAreRunning(t *testing.T) {
	host := testDaemon(t)
	startSession(t, host)

	code, _, stderr := cli(t, "hub", "--stop")
	if code != exitError {
		t.Errorf("exit = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr, "--force") {
		t.Errorf("stderr = %q, want it to name the way through", stderr)
	}
	if len(host.List()) == 0 {
		t.Error("the sessions are gone; the refusal did not hold")
	}
}

func TestKill_EndsASession(t *testing.T) {
	host := testDaemon(t)
	id := startSession(t, host)

	code, stdout, stderr := cli(t, "kill", strconv.Itoa(id))
	if code != exitOK {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	if !strings.Contains(stdout, strconv.Itoa(id)) {
		t.Errorf("stdout = %q, want it to name the session", stdout)
	}
	if _, err := host.Get(id); err == nil {
		t.Error("the session is still there")
	}
}

// A typo in the second ID must not leave the first pane dead and the caller
// wondering which ones went.
func TestKill_ChecksEveryIDBeforeKillingAny(t *testing.T) {
	host := testDaemon(t)
	id := startSession(t, host)

	code, _, stderr := cli(t, "kill", strconv.Itoa(id), "nonsense")
	if code != exitError {
		t.Errorf("exit = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr, "nonsense") {
		t.Errorf("stderr = %q, want it to name the bad argument", stderr)
	}
	if _, err := host.Get(id); err != nil {
		t.Error("the valid session was killed before the bad argument was noticed")
	}
}

// A subcommand's --help must work on a machine with no daemon: somebody
// reading the help is exactly somebody who has not set one up yet.
func TestRun_SubcommandHelpWorksWithoutADaemon(t *testing.T) {
	t.Setenv(discovery.EnvDirOverride, t.TempDir())

	for _, name := range []string{"ls", "read", "send", "keys", "wait", "kill", "hub"} {
		code, _, stderr := cli(t, name, "--help")
		if code != exitOK {
			t.Errorf("%s --help: exit = %d, want %d (stderr: %s)", name, code, exitOK, stderr)
		}
		if !strings.Contains(stderr, "Verwendung") {
			t.Errorf("%s --help printed no usage: %s", name, stderr)
		}
	}
}

// "--" ends the flags, so a "-h" behind it is an argument and not a question.
func TestWantsHelp(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"--help"}, true},
		{[]string{"-h"}, true},
		{[]string{"3", "--help"}, true},
		{[]string{"3", "hallo"}, false},
		{nil, false},
		{[]string{"--", "-h"}, false},
	}
	for _, tc := range cases {
		if got := wantsHelp(tc.args); got != tc.want {
			t.Errorf("wantsHelp(%v) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

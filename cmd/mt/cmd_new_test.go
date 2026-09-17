package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/launch"
	"time"
)

// `mt new` is the point of Phase 2b: before it, only a process with a window
// could start a session, because only that process knew what "claude" meant
// and which variables a pane needs. These tests drive the real command against
// a real host with a real launcher.

// fakeAgent writes a shell script that stands in for an agent CLI, so the test
// exercises the whole path without needing claude on the machine.
func fakeAgent(t *testing.T, name string) string {
	t.Helper()
	if isWindows() {
		t.Skip("the stand-in agent is a shell script")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	script := "#!/bin/sh\necho \"" + name + " gestartet\"\n" +
		"echo \"id=$MULTITERMINAL_SESSION_ID port=$MTUI_PORT\"\n" +
		"echo \"argv: $@\"\nexec /bin/sh\n"
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func isWindows() bool { return os.PathSeparator == '\\' }

// launcherDaemon is testDaemon with a launcher, which is what makes the host
// able to start something by name.
func launcherDaemon(t *testing.T, commands launch.Commands, shimPort int) hub.Host {
	t.Helper()
	return testDaemonWith(t, hub.Options{
		Version:  "test",
		Launcher: launch.Policy{ShimPort: shimPort, Commands: commands},
	})
}

func TestNew_StartsAnAgentTheDaemonResolvesItself(t *testing.T) {
	claude := fakeAgent(t, "claude")
	host := launcherDaemon(t, launch.Commands{"claude": claude}, 45678)
	workdir := t.TempDir()

	code, stdout, stderr := cli(t, "new", "claude", "--dir", workdir)
	if code != exitOK {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	id, err := strconv.Atoi(strings.TrimSpace(stdout))
	if err != nil {
		t.Fatalf("stdout %q is not a session ID: %v", stdout, err)
	}
	t.Cleanup(func() { _ = host.Close(id) })

	summary, err := host.Get(id)
	if err != nil {
		t.Fatalf("the host does not have session %d: %v", id, err)
	}
	if summary.Mode != "claude" {
		t.Errorf("mode = %q, want %q", summary.Mode, "claude")
	}
	if summary.Dir != workdir {
		t.Errorf("dir = %q, want %q", summary.Dir, workdir)
	}
	// An open window draws a pane for a session it did not start, and this is
	// how it knows. Without it, a session started from a terminal stays
	// invisible in the running app until the next restart.
	if summary.Origin != hub.OriginCLI {
		t.Errorf("origin = %q, want %q", summary.Origin, hub.OriginCLI)
	}
}

// The environment is the whole reason this could not be done client-side. A
// pane without MULTITERMINAL_SESSION_ID starts fine and silently has no hook
// wiring, so the test reads it back off the screen the agent printed it to.
func TestNew_TheDaemonBuildsTheEnvironment(t *testing.T) {
	claude := fakeAgent(t, "claude")
	host := launcherDaemon(t, launch.Commands{"claude": claude}, 45678)

	code, stdout, stderr := cli(t, "new", "claude", "--dir", t.TempDir())
	if code != exitOK {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	id, _ := strconv.Atoi(strings.TrimSpace(stdout))
	t.Cleanup(func() { _ = host.Close(id) })

	screen := waitForScreen(t, host, id, "id=")
	want := "id=" + strconv.Itoa(id) + " port=45678"
	if !strings.Contains(screen, want) {
		t.Errorf("the agent did not see %q on its environment:\n%s", want, screen)
	}
}

// A model asked for is a model passed on, or the flag is decoration.
func TestNew_PassesTheModelThrough(t *testing.T) {
	claude := fakeAgent(t, "claude")
	host := launcherDaemon(t, launch.Commands{"claude": claude}, 1)

	code, stdout, stderr := cli(t, "new", "claude", "--model", "opus-5", "--dir", t.TempDir())
	if code != exitOK {
		t.Fatalf("exit = %d, stderr = %s", code, stderr)
	}
	id, _ := strconv.Atoi(strings.TrimSpace(stdout))
	t.Cleanup(func() { _ = host.Close(id) })

	screen := waitForScreen(t, host, id, "argv:")
	if !strings.Contains(screen, "--model opus-5") {
		t.Errorf("the model did not reach the CLI:\n%s", screen)
	}
}

func TestNew_RejectsAnUnknownTool(t *testing.T) {
	launcherDaemon(t, launch.Commands{}, 1)

	code, _, stderr := cli(t, "new", "cursor")
	if code != exitError {
		t.Errorf("exit = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr, "cursor") || !strings.Contains(stderr, "claude") {
		t.Errorf("stderr = %q, want the bad tool and the known ones", stderr)
	}
}

// A --dir that does not exist is caught here rather than in the daemon: the
// daemon's working directory is not the caller's, so a relative path or a typo
// would start an agent somewhere the user never looked.
func TestNew_RejectsADirectoryThatIsNotThere(t *testing.T) {
	launcherDaemon(t, launch.Commands{"claude": "/bin/sh"}, 1)

	code, _, stderr := cli(t, "new", "claude", "--dir", filepath.Join(t.TempDir(), "weg"))
	if code != exitError {
		t.Errorf("exit = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr, "gibt es nicht") {
		t.Errorf("stderr = %q", stderr)
	}
}

// A host without a launcher must refuse rather than start something
// half-configured: a pane launched with no environment looks fine and has no
// hook wiring at all.
func TestNew_AHostWithoutALauncherRefuses(t *testing.T) {
	testDaemon(t)

	code, _, stderr := cli(t, "new", "claude", "--dir", t.TempDir())
	if code != exitError {
		t.Errorf("exit = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr, "launcher") {
		t.Errorf("stderr = %q, want it to say why", stderr)
	}
}

func TestNew_WithoutAToolIsAUsageError(t *testing.T) {
	launcherDaemon(t, launch.Commands{}, 1)

	if code, _, _ := cli(t, "new"); code != exitUsage {
		t.Errorf("exit = %d, want %d", code, exitUsage)
	}
}

// `mt read --follow` has to end when the agent's process ends.
//
// It used to wait for the subscription channel to close, which never happens
// on a process exit: a session outlives its process across a suspend, so the
// channel closes only when the SESSION is closed. Following a finished agent
// hung forever, and the only way out was Ctrl+C.
func TestRead_FollowEndsWhenTheProcessExits(t *testing.T) {
	host := testDaemon(t)
	if isWindows() {
		t.Skip("the short-lived producer is a shell script")
	}
	// A command that prints and exits at once, so the follow has both output
	// to show and an exit to notice.
	id, err := host.Create(hub.CreateSpec{
		Argv: []string{"/bin/sh", "-c", "echo follow-marker"},
		Dir:  t.TempDir(), Rows: 24, Cols: 80,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = host.Close(id) })

	done := make(chan int, 1)
	go func() {
		code, _, _ := cli(t, "read", strconv.Itoa(id), "--follow")
		done <- code
	}()

	select {
	case code := <-done:
		if code != exitOK {
			t.Errorf("exit = %d, want %d", code, exitOK)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("--follow never returned after the process exited")
	}
}

// A session that had already finished before the follow started sends no exit
// event at all, so the status has to decide that case.
func TestRead_FollowEndsOnAnAlreadyFinishedSession(t *testing.T) {
	host := testDaemon(t)
	if isWindows() {
		t.Skip("the short-lived producer is a shell script")
	}
	id, err := host.Create(hub.CreateSpec{
		Argv: []string{"/bin/sh", "-c", "echo schon-fertig"},
		Dir:  t.TempDir(), Rows: 24, Cols: 80,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	t.Cleanup(func() { _ = host.Close(id) })

	// Wait until the host has seen the exit, so the event is long gone.
	deadline := time.Now().Add(20 * time.Second)
	for {
		s, err := host.Get(id)
		if err == nil && s.Status == hub.StatusExited {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the producer never exited")
		}
		time.Sleep(50 * time.Millisecond)
	}

	done := make(chan int, 1)
	go func() {
		code, _, _ := cli(t, "read", strconv.Itoa(id), "--follow")
		done <- code
	}()
	select {
	case code := <-done:
		if code != exitOK {
			t.Errorf("exit = %d, want %d", code, exitOK)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("--follow hung on a session that had already exited")
	}
}

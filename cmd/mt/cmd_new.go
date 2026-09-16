package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/launch"
)

// cmdNew starts an agent in the daemon.
//
// The caller names a tool, not a command line. It has to: the daemon knows
// which binary "claude" is on this machine, which variables the pane needs to
// report its state, and whether the worktree firewall applies to this
// directory. A CLI that built its own argv would get the first part right and
// the rest silently wrong.
func cmdNew(e *env, args []string) int {
	fs := e.newFlags("new", "mt new <tool> [--dir <pfad>] [--model <id>] [--prompt <text>]",
		"Startet einen Agenten im Daemon und gibt seine Session-ID aus.\n"+
			"Tools: "+strings.Join(launch.KnownAgents(), ", ")+".\n"+
			"Die Session gehört dem Daemon, überlebt also jedes Fenster.")
	dir := fs.String("dir", "", "Arbeitsverzeichnis (Vorgabe: das aktuelle)")
	model := fs.String("model", "", "Modell, das der CLI übergeben wird")
	prompt := fs.String("prompt", "", "erster Prompt, sobald der Agent bereit ist")
	rows := fs.Int("rows", 24, "Zeilen")
	cols := fs.Int("cols", 100, "Spalten")
	positional, err := parseArgs(fs, args)
	if err != nil {
		return exitForParse(err)
	}
	if len(positional) != 1 {
		fs.Usage()
		return exitUsage
	}
	tool := strings.ToLower(positional[0])
	if !launch.IsAgentTool(tool) {
		return e.fail("unbekanntes Tool %q, bekannt sind: %s",
			tool, strings.Join(launch.KnownAgents(), ", "))
	}

	workdir, err := resolveDir(*dir)
	if err != nil {
		return e.fail("%v", err)
	}

	id, err := e.hub.Create(hub.CreateSpec{
		Dir:    workdir,
		Rows:   *rows,
		Cols:   *cols,
		Launch: &hub.LaunchRequest{Tool: tool, Model: *model},
	})
	if err != nil {
		return e.fail("%s starten: %v", tool, err)
	}
	fmt.Fprintln(e.stdout, id)

	if *prompt == "" {
		return exitOK
	}
	// The agent needs a moment before its input box exists; typing into the
	// splash screen loses the prompt. Waiting for "idle" is the honest way to
	// find out, and it costs nothing when the CLI is quick.
	if _, err := hub.WaitForAgent(e.ctx(), e.hub, id, []string{"idle", "done", "blocked"}, promptWait); err != nil {
		fmt.Fprintf(e.stderr, "mt: Session %d läuft, aber meldet sich nicht (%v); "+
			"der Prompt wurde nicht geschickt\n", id, err)
		return exitError
	}
	if code := writeTo(e, id, []byte(*prompt)); code != exitOK {
		return code
	}
	time.Sleep(settleDelay)
	return writeTo(e, id, []byte("\r"))
}

// promptWait bounds how long --prompt waits for the agent to be ready. Short,
// because an agent CLI that has not drawn its prompt within this is not going
// to, and the session still exists either way.
const promptWait = 30 * time.Second

// resolveDir turns the --dir value into an absolute path that exists.
//
// It is checked here rather than left to the daemon because the daemon's
// working directory is not the caller's: a relative path would resolve
// somewhere the user never looked, and a typo would start an agent in the
// wrong repository rather than failing.
func resolveDir(dir string) (string, error) {
	if dir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("aktuelles Verzeichnis: %w", err)
		}
		return wd, nil
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("%q auflösen: %w", dir, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("%q gibt es nicht", abs)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%q ist kein Verzeichnis", abs)
	}
	return abs, nil
}

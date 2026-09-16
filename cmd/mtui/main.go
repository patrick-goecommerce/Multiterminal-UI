// Command mtui is the command-line client for MTUI's session daemon.
//
// It is the third face of the same API: the Wails window drives sessions
// through internal/hub, an agent drives them through the MCP server, and this
// drives them from a shell or a script. All three talk to the same Host, so
// none of them can know something the others cannot ask for.
//
// It deliberately does not start a daemon. Sessions come from MTUI; a daemon
// started by `mtui ls` would only ever report an empty list, and would then
// sit there. If nothing is running, that is the answer.
//
// Design: docs/superpowers/specs/2026-09-15-mtuid-daemon-architecture-design.md
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/discovery"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// Version is set at build time by the release workflow.
var Version = "dev"

// Exit codes are part of the interface: a script has to tell "the agent is
// still working" from "the call broke", and "no daemon" from "no session".
const (
	exitOK      = 0
	exitError   = 1
	exitUsage   = 2
	exitNoHub   = 3
	exitTimeout = 4
)

// errNoHub means no daemon is published for this user.
var errNoHub = errors.New("kein laufender Session-Daemon gefunden")

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// command is one subcommand. Every one of them gets an already-connected
// client, except the ones that do not need one.
type command struct {
	name    string
	summary string
	// needsHub is false for help and version, which must work with no daemon.
	needsHub bool
	run      func(env *env, args []string) int
}

func commands() []command {
	return []command{
		{"ls", "Sessions auflisten", true, cmdList},
		{"read", "Bildschirm einer Session als Text", true, cmdRead},
		{"send", "Text plus Enter an eine Session schicken", true, cmdSend},
		{"keys", "Rohe Tasten an eine Session schicken", true, cmdKeys},
		{"wait", "Warten, bis eine Session fertig ist oder nachfragt", true, cmdWait},
		{"kill", "Session beenden", true, cmdKill},
		{"hub", "Daemon anzeigen oder beenden", true, cmdHub},
	}
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return exitUsage
	}
	switch args[0] {
	case "-h", "--help", "help":
		usage(stdout)
		return exitOK
	case "-v", "--version", "version":
		fmt.Fprintln(stdout, Version)
		return exitOK
	}

	var cmd *command
	for i := range commands() {
		if c := commands()[i]; c.name == args[0] {
			cmd = &c
			break
		}
	}
	if cmd == nil {
		fmt.Fprintf(stderr, "mtui: unbekanntes Kommando %q\n\n", args[0])
		usage(stderr)
		return exitUsage
	}

	e := &env{stdout: stdout, stderr: stderr}
	if cmd.needsHub {
		client, err := connect()
		if err != nil {
			if errors.Is(err, errNoHub) {
				fmt.Fprintf(stderr, "mtui: %v\n", err)
				fmt.Fprintln(stderr, "Hinweis: mtui spricht mit dem Daemon. "+
					"Dafür muss in ~/.multiterminal.yaml session_host: daemon stehen "+
					"und MTUI mindestens einmal gestartet worden sein.")
				return exitNoHub
			}
			fmt.Fprintf(stderr, "mtui: %v\n", err)
			return exitError
		}
		defer client.Release()
		e.hub = client
	}
	return cmd.run(e, args[1:])
}

// env is what every subcommand gets: an output pair and, where it asked for
// one, a live client.
type env struct {
	hub    *hub.Remote
	stdout io.Writer
	stderr io.Writer
}

// fail prints an error the way every subcommand should and returns exitError.
func (e *env) fail(format string, args ...any) int {
	fmt.Fprintf(e.stderr, "mtui: "+format+"\n", args...)
	return exitError
}

// dialTimeout bounds a control request. The daemon is on loopback, so a call
// that takes longer than this is not slow, it is stuck.
const dialTimeout = 10 * time.Second

// connect resolves the published daemon and dials it.
func connect() (*hub.Remote, error) {
	rec, err := discovery.Resolve(discovery.ServiceHub)
	if err != nil {
		return nil, errNoHub
	}
	client, err := hub.Dial(rec.Addr(), rec.Token, hub.DialOptions{Timeout: dialTimeout})
	if err != nil {
		if errors.Is(err, hub.ErrProtocol) {
			return nil, fmt.Errorf("%w; mtui und mtuid stammen aus verschiedenen Builds", err)
		}
		return nil, err
	}
	return client, nil
}

func usage(w io.Writer) {
	fmt.Fprintf(w, `mtui %s — Kommandozeile für MTUI-Sessions

Verwendung:
  mtui <kommando> [argumente]

Kommandos:
`, Version)
	for _, c := range commands() {
		fmt.Fprintf(w, "  %-6s %s\n", c.name, c.summary)
	}
	fmt.Fprintf(w, `
Jedes Kommando kennt --help. Die meisten kennen --json für Skripte.

Exit-Codes:
  %d  ok
  %d  Fehler
  %d  falsche Verwendung
  %d  kein Daemon erreichbar
  %d  Timeout beim Warten (die Session arbeitet weiter)
`, exitOK, exitError, exitUsage, exitNoHub, exitTimeout)
}

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// cmdWait blocks until a session reaches a state worth coming back for.
//
// This is the command that makes the CLI more than a remote control. Without
// it a script polls `mtui read` in a loop and guesses; with it, chaining two
// agents is three lines of shell:
//
//	mtui send 1 "refactor the parser"
//	mtui wait 1
//	mtui send 2 "review what pane 1 just did"
func cmdWait(e *env, args []string) int {
	fs := e.newFlags("wait", "mtui wait <id> [--until <liste>] [--timeout <dauer>] [--json]",
		"Wartet, bis die Session einen der genannten Zustände erreicht, und gibt ihn aus.\n"+
			"Ohne --until: done oder blocked, also fertig oder wartet auf einen Menschen.\n"+
			"Zustände: "+hub.KnownAgentStates()+".")
	until := fs.String("until", "", "Zustände, kommagetrennt (Vorgabe: done,blocked)")
	timeout := fs.Duration("timeout", hub.AgentWaitDefault,
		fmt.Sprintf("Obergrenze fürs Warten (max %s)", hub.AgentWaitMax))
	asJSON := fs.Bool("json", false, "Ausgabe als JSON")
	positional, err := parseArgs(fs, args)
	if err != nil {
		return exitUsage
	}
	id, err := sessionID(fs, positional)
	if err != nil {
		if errors.Is(err, errUsage) {
			return exitUsage
		}
		return e.fail("%v", err)
	}

	// Ctrl+C stops the wait, not the session. Somebody who gives up on
	// watching an agent has not asked for the agent to stop.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	defer signal.Stop(interrupt)
	go func() {
		select {
		case <-interrupt:
			cancel()
		case <-ctx.Done():
		}
	}()

	started := time.Now()
	state, err := hub.WaitForAgent(ctx, e.hub, id, splitList(*until), *timeout)
	waited := time.Since(started)

	switch {
	case err == nil:
	case errors.Is(err, context.Canceled):
		fmt.Fprintln(e.stderr, "\nmtui: Warten abgebrochen; die Session läuft weiter.")
		return exitError
	case errors.Is(err, hub.ErrWaitTimeout):
		// A distinct code, because "still working after ten minutes" is a
		// different thing for a script than "the call broke".
		if *asJSON {
			e.writeJSON(waitResult{ID: id, State: hub.AgentStateOf(e.hub, id),
				TimedOut: true, WaitedSeconds: waited.Seconds()})
		} else {
			fmt.Fprintf(e.stderr, "mtui: %v\n", err)
		}
		return exitTimeout
	default:
		return e.fail("%v", err)
	}

	if *asJSON {
		return e.writeJSON(waitResult{ID: id, State: state, WaitedSeconds: waited.Seconds()})
	}
	fmt.Fprintln(e.stdout, state)
	return exitOK
}

// waitResult is what --json prints. TimedOut is a field rather than an absent
// state because a script should not have to tell them apart by exit code
// alone when it is already parsing the output.
type waitResult struct {
	ID            int     `json:"id"`
	State         string  `json:"state"`
	TimedOut      bool    `json:"timed_out,omitempty"`
	WaitedSeconds float64 `json:"waited_seconds"`
}

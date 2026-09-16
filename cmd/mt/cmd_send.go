package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// settleDelay is how long send waits between the text and the Enter that
// submits it.
//
// Agent CLIs redraw their input box as they receive characters, and a Return
// arriving in the same read as the text is sometimes swallowed by that redraw.
// The pause is below human perception and removes the whole class of "the
// prompt was typed but never sent".
const settleDelay = 30 * time.Millisecond

// cmdSend types a prompt into a session and submits it. This is the call an
// orchestrating script makes most.
func cmdSend(e *env, args []string) int {
	fs := e.newFlags("send", "mt send [--no-enter] <id> <text...>",
		"Schickt Text an eine Session und drückt Enter.\n"+
			"Mehrere Argumente werden mit Leerzeichen verbunden; \"-\" liest von stdin.\n"+
			"Alles hinter der ID ist Text, Optionen stehen deshalb davor.")
	noEnter := fs.Bool("no-enter", false, "nur tippen, nicht abschicken")
	// Plain parsing, not parseArgs: everything after the ID is the prompt, and
	// a prompt that starts with a dash is still a prompt.
	if err := fs.Parse(args); err != nil {
		return exitForParse(err)
	}
	id, err := sessionID(fs, fs.Args())
	if err != nil {
		if errors.Is(err, errUsage) {
			return exitUsage
		}
		return e.fail("%v", err)
	}
	rest := fs.Args()[1:]
	if len(rest) == 0 {
		fs.Usage()
		return exitUsage
	}

	text, err := gatherText(rest)
	if err != nil {
		return e.fail("%v", err)
	}
	if code := writeTo(e, id, []byte(text)); code != exitOK {
		return code
	}
	if *noEnter {
		return exitOK
	}
	time.Sleep(settleDelay)
	return writeTo(e, id, []byte("\r"))
}

// cmdKeys sends key names rather than text: the way to answer a permission
// prompt, interrupt a run, or drive a menu.
func cmdKeys(e *env, args []string) int {
	fs := e.newFlags("keys", "mt keys <id> <taste...>",
		"Schickt Tasten an eine Session, etwa `mt keys 3 ctrl-c` oder `mt keys 3 down enter`.\n"+
			"Bekannt: "+knownKeyNames())
	if err := fs.Parse(args); err != nil {
		return exitForParse(err)
	}
	id, err := sessionID(fs, fs.Args())
	if err != nil {
		if errors.Is(err, errUsage) {
			return exitUsage
		}
		return e.fail("%v", err)
	}
	names := fs.Args()[1:]
	if len(names) == 0 {
		fs.Usage()
		return exitUsage
	}

	// Resolve every name first. Half a key sequence is worse than none: the
	// pane would be left in whatever state the first few keys put it in.
	var seq []byte
	for _, name := range names {
		bytes, ok := keyBytes(name)
		if !ok {
			return e.fail("unbekannte Taste %q; bekannt sind: %s", name, knownKeyNames())
		}
		seq = append(seq, bytes...)
	}
	return writeTo(e, id, seq)
}

// writeTo sends bytes to a session, waking it first when it is asleep.
//
// A sleeping pane has no process to write to. Refusing would be correct and
// useless: somebody sending a prompt to a pane that was put to sleep while
// idle means the prompt, not a lecture about the pane's state. The daemon
// knows how the session was launched, so it can wake it on its own.
func writeTo(e *env, id int, data []byte) int {
	if summary, err := e.hub.Get(id); err == nil && summary.Asleep() {
		if code := wakeAndWait(e, id); code != exitOK {
			return code
		}
	}
	if err := e.hub.Write(id, data); err != nil {
		return e.fail("an Session %d schreiben: %v", id, err)
	}
	return exitOK
}

// wakeAndWait resumes a sleeping pane and waits for its agent to be ready.
//
// Waking relaunches the CLI and replays the conversation, which takes long
// enough that typing into it straight away lands on a splash screen. Waiting
// for a state the agent reports is the honest way to know it is listening.
func wakeAndWait(e *env, id int) int {
	fmt.Fprintf(e.stderr, "mt: Session %d schläft, wird geweckt …\n", id)
	if err := e.hub.Wake(id); err != nil {
		if errors.Is(err, hub.ErrNoResumeID) {
			return e.fail("Session %d kann nicht geweckt werden: "+
				"es ist keine Agent-Session-ID bekannt, mit der sich das Gespräch fortsetzen ließe", id)
		}
		return e.fail("Session %d wecken: %v", id, err)
	}
	if _, err := hub.WaitForAgent(e.ctx(), e.hub, id, []string{"idle", "done", "blocked"}, wakeWait); err != nil {
		return e.fail("Session %d wacht nicht auf: %v", id, err)
	}
	return exitOK
}

// wakeWait bounds the wait for a woken agent. Replaying a long conversation is
// the slow part and takes tens of seconds; past this something is wrong.
const wakeWait = 2 * time.Minute

// gatherText joins the arguments, or reads stdin when the caller passed "-".
//
// stdin matters because a prompt worth sending to an agent is often longer
// than a shell argument wants to be, and quoting it is how people lose
// newlines.
func gatherText(args []string) (string, error) {
	if len(args) == 1 && args[0] == "-" {
		data, err := readAllStdin()
		if err != nil {
			return "", fmt.Errorf("stdin lesen: %w", err)
		}
		// A trailing newline from a heredoc would submit the prompt before the
		// Enter this command sends, so it goes.
		return strings.TrimRight(string(data), "\r\n"), nil
	}
	return strings.Join(args, " "), nil
}

package main

import (
	"errors"
	"fmt"
	"strings"
	"time"
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
	fs := e.newFlags("send", "mtui send [--no-enter] <id> <text...>",
		"Schickt Text an eine Session und drückt Enter.\n"+
			"Mehrere Argumente werden mit Leerzeichen verbunden; \"-\" liest von stdin.\n"+
			"Alles hinter der ID ist Text, Optionen stehen deshalb davor.")
	noEnter := fs.Bool("no-enter", false, "nur tippen, nicht abschicken")
	// Plain parsing, not parseArgs: everything after the ID is the prompt, and
	// a prompt that starts with a dash is still a prompt.
	if err := fs.Parse(args); err != nil {
		return exitUsage
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
	fs := e.newFlags("keys", "mtui keys <id> <taste...>",
		"Schickt Tasten an eine Session, etwa `mtui keys 3 ctrl-c` oder `mtui keys 3 down enter`.\n"+
			"Bekannt: "+knownKeyNames())
	if err := fs.Parse(args); err != nil {
		return exitUsage
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

// writeTo sends bytes to a session and turns the host's errors into messages a
// user can act on.
func writeTo(e *env, id int, data []byte) int {
	if err := e.hub.Write(id, data); err != nil {
		// A sleeping pane has no process to write to, and that is the one
		// failure here with an obvious next step.
		if summary, getErr := e.hub.Get(id); getErr == nil && summary.Asleep() {
			return e.fail("Session %d schläft; sie muss erst geweckt werden", id)
		}
		return e.fail("an Session %d schreiben: %v", id, err)
	}
	return exitOK
}

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

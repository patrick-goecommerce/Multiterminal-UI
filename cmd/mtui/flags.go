package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// errUsage marks a parse failure the caller turns into exitUsage. The flag
// set has already printed what went wrong, so this carries no message.
var errUsage = errors.New("usage")

// newFlags builds a flag set that prints its help to the command's own stderr
// rather than to the process-wide default, so tests can read it.
func (e *env) newFlags(name, usageLine, notes string) *flag.FlagSet {
	fs := flag.NewFlagSet("mtui "+name, flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	fs.Usage = func() {
		fmt.Fprintf(e.stderr, "Verwendung: %s\n", usageLine)
		if notes != "" {
			fmt.Fprintf(e.stderr, "\n%s\n", strings.TrimRight(notes, "\n"))
		}
		fmt.Fprintln(e.stderr, "\nOptionen:")
		fs.PrintDefaults()
	}
	return fs
}

// parseArgs parses a command's flags and returns its positional arguments.
//
// It exists because flag.Parse stops at the first non-flag argument, which
// would make `mtui wait 3 --timeout 30s` silently ignore the timeout: the flag
// sits behind the ID, so it never gets read, and the caller waits five minutes
// having asked for thirty seconds. Silently is the problem. Parsing in rounds,
// peeling one positional off each time, lets flags stand on either side while
// the flag package still decides for itself which ones take a value.
//
// Everything after a "--" ends up positional, which is how a caller passes an
// argument that looks like a flag.
func parseArgs(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return positional, nil
		}
		positional = append(positional, rest[0])
		args = rest[1:]
	}
}

// sessionID reads the session ID from a command's first positional argument.
//
// IDs are the only handle the protocol has; a name would be ambiguous the
// moment two panes sit in the same directory.
func sessionID(fs *flag.FlagSet, positional []string) (int, error) {
	if len(positional) < 1 {
		fs.Usage()
		return 0, errUsage
	}
	return parseSessionID(positional[0])
}

// parseSessionID turns one argument into an ID, with an error a user can act
// on rather than strconv's.
func parseSessionID(raw string) (int, error) {
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%q ist keine Session-ID; `mtui ls` zeigt die vorhandenen", raw)
	}
	return id, nil
}

// writeJSON prints a value as indented JSON with a trailing newline, which is
// what a shell pipeline and a human reading the output both want.
func (e *env) writeJSON(v any) int {
	enc := json.NewEncoder(e.stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return e.fail("JSON schreiben: %v", err)
	}
	return exitOK
}

// displayState is how a listing names a session's state.
//
// It is hub.AgentState with one addition: a suspended pane reads as "asleep"
// rather than "done". Waiting collapses the two on purpose (finished work with
// its process released is finished work), but somebody looking at a list wants
// to see that the process is gone.
func displayState(s hub.SessionSummary) string {
	if s.Asleep() {
		return "asleep"
	}
	return hub.AgentState(s)
}

// splitList turns a comma-separated flag value into its parts, dropping empty
// ones so that "done," and "done" mean the same thing.
func splitList(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

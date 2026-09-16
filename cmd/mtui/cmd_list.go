package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// cmdList prints what the daemon is holding. It is the command everything else
// starts from, because every other command needs an ID.
func cmdList(e *env, args []string) int {
	fs := e.newFlags("ls", "mtui ls [--json] [--state <liste>]",
		"Zeigt die Sessions, die der Daemon hält, mit ihrem Agent-Zustand.")
	asJSON := fs.Bool("json", false, "Ausgabe als JSON")
	state := fs.String("state", "", "nur diese Zustände, kommagetrennt ("+hub.KnownAgentStates()+", asleep)")
	if _, err := parseArgs(fs, args); err != nil {
		return exitUsage
	}

	sessions := e.hub.List()
	if *state != "" {
		wanted := make(map[string]bool)
		for _, s := range splitList(*state) {
			wanted[strings.ToLower(s)] = true
		}
		var kept []hub.SessionSummary
		for _, s := range sessions {
			if wanted[displayState(s)] {
				kept = append(kept, s)
			}
		}
		sessions = kept
	}
	sort.Slice(sessions, func(i, j int) bool { return sessions[i].ID < sessions[j].ID })

	if *asJSON {
		// Never null: a script doing `mtui ls --json | jq '.[]'` should get an
		// empty list rather than an error when nothing is running.
		if sessions == nil {
			sessions = []hub.SessionSummary{}
		}
		return e.writeJSON(sessions)
	}

	if len(sessions) == 0 {
		fmt.Fprintln(e.stdout, "Keine Sessions.")
		return exitOK
	}
	tw := tabwriter.NewWriter(e.stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tMODUS\tZUSTAND\tVERZEICHNIS\tTITEL\tKOSTEN")
	for _, s := range sessions {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\n",
			s.ID, orDash(s.Mode), displayState(s), shortDir(s.Dir),
			orDash(truncate(s.Title, 40)), cost(s.Cost))
	}
	_ = tw.Flush()
	return exitOK
}

// cmdKill ends sessions. It takes several IDs because killing panes one call
// at a time is the usual reason somebody writes a loop around a CLI.
func cmdKill(e *env, args []string) int {
	fs := e.newFlags("kill", "mtui kill <id> [<id>...]",
		"Beendet die genannten Sessions. Das killt den Prozessbaum; laufende Arbeit ist weg.")
	positional, err := parseArgs(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(positional) == 0 {
		fs.Usage()
		return exitUsage
	}

	// Every ID is parsed before anything is killed: a typo in the third
	// argument must not leave the first two dead and the caller guessing.
	ids := make([]int, 0, len(positional))
	for _, raw := range positional {
		id, err := parseSessionID(raw)
		if err != nil {
			return e.fail("%v", err)
		}
		ids = append(ids, id)
	}

	code := exitOK
	for _, id := range ids {
		if err := e.hub.Close(id); err != nil {
			if errors.Is(err, hub.ErrNoSession) {
				e.fail("Session %d gibt es nicht", id)
			} else {
				e.fail("Session %d beenden: %v", id, err)
			}
			code = exitError
			continue
		}
		fmt.Fprintf(e.stdout, "Session %d beendet.\n", id)
	}
	return code
}

// shortDir keeps a listing readable when the panes sit deep in a tree: the
// last two segments are what tells two worktrees apart.
//
// Both separators are folded, not filepath.ToSlash: that one is a no-op for
// backslashes off Windows, and a Windows path can reach a listing printed
// anywhere once a hub is remote.
func shortDir(dir string) string {
	if dir == "" {
		return "-"
	}
	dir = strings.ReplaceAll(dir, "\\", "/")
	parts := strings.Split(strings.TrimRight(dir, "/"), "/")
	if len(parts) <= 2 {
		return dir
	}
	return ".../" + strings.Join(parts[len(parts)-2:], "/")
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func cost(c float64) string {
	if c <= 0 {
		return "-"
	}
	return fmt.Sprintf("$%.2f", c)
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

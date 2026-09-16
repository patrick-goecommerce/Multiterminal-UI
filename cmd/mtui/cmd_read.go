package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"
)

// followReadyTimeout bounds the wait for the output socket before a --follow
// starts. Without it the first chunks would queue until the socket connects,
// which looks like a hang.
const followReadyTimeout = 5 * time.Second

// cmdRead prints a session's screen.
//
// Plain text by default, because the caller is usually a human or a script
// grepping for a line. --follow switches to the raw stream, escape sequences
// included, because at that point the caller is a terminal.
func cmdRead(e *env, args []string) int {
	fs := e.newFlags("read", "mtui read <id> [--lines N] [--follow] [--json]",
		"Gibt den Bildschirm einer Session aus.\n"+
			"Ohne --follow den aktuellen Stand als Text, mit --follow den laufenden Strom.")
	lines := fs.Int("lines", 0, "nur die letzten N nicht-leeren Zeilen (0 = ganzer Bildschirm)")
	follow := fs.Bool("follow", false, "am Strom bleiben, bis Ctrl+C oder die Session endet")
	asJSON := fs.Bool("json", false, "Ausgabe als JSON (nicht mit --follow)")
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
	if *follow && *asJSON {
		return e.fail("--follow und --json gehen nicht zusammen: ein Strom ist kein Dokument")
	}
	if *follow {
		return followSession(e, id)
	}

	text, err := e.hub.PlainText(id)
	if err != nil {
		return e.fail("Session %d lesen: %v", id, err)
	}
	if *lines > 0 {
		text = lastLines(text, *lines)
	}

	if *asJSON {
		summary, err := e.hub.Get(id)
		if err != nil {
			return e.fail("Session %d lesen: %v", id, err)
		}
		return e.writeJSON(struct {
			ID     int    `json:"id"`
			State  string `json:"state"`
			Offset int64  `json:"offset"`
			Screen string `json:"screen"`
		}{id, displayState(summary), summary.Offset, text})
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	fmt.Fprint(e.stdout, text)
	return exitOK
}

// followSession streams a session to stdout until the session ends or the user
// interrupts.
//
// It starts with a repaint rather than with the ring's history: the ring holds
// raw bytes from an arbitrary point, and a VT100 stream entered mid-sequence
// stays garbled (#157). The repaint puts a known screen on the terminal, and
// everything after it appends cleanly.
func followSession(e *env, id int) int {
	summary, err := e.hub.Get(id)
	if err != nil {
		return e.fail("Session %d lesen: %v", id, err)
	}
	if !e.hub.WaitReady(followReadyTimeout) {
		return e.fail("der Daemon öffnet den Ausgabe-Socket nicht")
	}

	painted, err := e.hub.Repaint(id)
	if err != nil {
		return e.fail("Session %d zeichnen: %v", id, err)
	}
	// Attach from where the repaint stands, so nothing is printed twice.
	sub, err := e.hub.Attach(id, summary.Offset)
	if err != nil {
		return e.fail("Session %d verfolgen: %v", id, err)
	}
	defer sub.Close()
	_, _ = e.stdout.Write(painted)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	defer signal.Stop(interrupt)

	for {
		select {
		case chunk, ok := <-sub.C:
			if !ok {
				fmt.Fprintf(e.stderr, "\nmtui: Session %d liefert nichts mehr.\n", id)
				return exitOK
			}
			if chunk.Truncated {
				// The stream has a hole, so appending would garble the screen
				// for good. Repaint and carry on from there.
				if again, err := e.hub.Repaint(id); err == nil {
					_, _ = e.stdout.Write(again)
				}
			}
			if _, err := e.stdout.Write(chunk.Data); err != nil {
				return e.fail("schreiben: %v", err)
			}
		case <-interrupt:
			// A newline so the shell prompt does not land mid-line on
			// whatever the pane was drawing.
			fmt.Fprintln(e.stdout)
			return exitOK
		}
	}
}

// lastLines keeps the final n lines that have something on them.
//
// A terminal screen is padded to its full height with blanks, so "the last 5
// lines" of a mostly empty pane would otherwise be five empty strings.
func lastLines(text string, n int) string {
	all := strings.Split(strings.TrimRight(text, "\n"), "\n")
	kept := make([]string, 0, n)
	for i := len(all) - 1; i >= 0 && len(kept) < n; i-- {
		if strings.TrimSpace(all[i]) == "" {
			continue
		}
		kept = append(kept, all[i])
	}
	// Collected back to front, so put them back in reading order.
	for i, j := 0, len(kept)-1; i < j; i, j = i+1, j-1 {
		kept[i], kept[j] = kept[j], kept[i]
	}
	return strings.Join(kept, "\n")
}

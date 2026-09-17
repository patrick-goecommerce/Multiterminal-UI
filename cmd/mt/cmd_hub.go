package main

import (
	"fmt"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// cmdHub reports on the daemon, and can stop it.
//
// Stopping is here rather than as its own command because it is the one thing
// about the daemon a user ever has to do by hand, and because putting it
// behind a flag on an informational command makes it harder to do by accident.
func cmdHub(e *env, args []string) int {
	fs := e.newFlags("hub", "mt hub [--json] [--stop]",
		"Zeigt den laufenden Session-Daemon.\n"+
			"--stop beendet ihn und damit jede Session, die er hält.")
	asJSON := fs.Bool("json", false, "Ausgabe als JSON")
	stop := fs.Bool("stop", false, "den Daemon beenden (beendet alle Sessions)")
	force := fs.Bool("force", false, "auch mit laufenden Sessions beenden")
	if _, err := parseArgs(fs, args); err != nil {
		return exitForParse(err)
	}

	info := e.hub.Info()
	sessions := e.hub.List()

	if *stop {
		// Refusing by default is the point: the daemon exists so that agents
		// survive a closed window, and stopping it is the one call that
		// undoes that for all of them at once.
		if live := countLive(sessions); live > 0 && !*force {
			return e.fail("der Daemon hält noch %d Session(s); "+
				"mit --force trotzdem beenden, oder erst `mt ls` ansehen", live)
		}
		if err := e.hub.Shutdown(); err != nil {
			return e.fail("Daemon beenden: %v", err)
		}
		fmt.Fprintln(e.stdout, "Daemon beendet.")
		return exitOK
	}

	if *asJSON {
		return e.writeJSON(struct {
			hub.Info
			Sessions int `json:"sessions"`
			Live     int `json:"live_sessions"`
		}{info, len(sessions), countLive(sessions)})
	}

	fmt.Fprintf(e.stdout, "Hub       %s\n", info.HubID)
	fmt.Fprintf(e.stdout, "Version   %s (Protokoll %d)\n", info.Version, info.Protocol)
	fmt.Fprintf(e.stdout, "PID       %d\n", info.PID)
	fmt.Fprintf(e.stdout, "Läuft     seit %s (%s)\n",
		info.StartedAt.Local().Format("2006-01-02 15:04:05"),
		uptime(info.StartedAt))
	fmt.Fprintf(e.stdout, "Sessions  %d, davon %d laufend\n", len(sessions), countLive(sessions))
	if info.ShimPort > 0 {
		fmt.Fprintf(e.stdout, "Shim-Port %d\n", info.ShimPort)
	}
	return exitOK
}

// countLive counts the sessions with a process behind them. A suspended pane
// is a session but not a running one, and the difference is what decides
// whether stopping the daemon throws work away.
func countLive(sessions []hub.SessionSummary) int {
	n := 0
	for _, s := range sessions {
		if s.Status == hub.StatusRunning {
			n++
		}
	}
	return n
}

// uptime renders a duration the way somebody glancing at it reads it, which
// is not what time.Duration.String does past an hour.
func uptime(since time.Time) string {
	if since.IsZero() {
		return "unbekannt"
	}
	d := time.Since(since).Round(time.Minute)
	if d < time.Minute {
		return "weniger als eine Minute"
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	if hours == 0 {
		return fmt.Sprintf("%d min", minutes)
	}
	return fmt.Sprintf("%d h %d min", hours, minutes)
}

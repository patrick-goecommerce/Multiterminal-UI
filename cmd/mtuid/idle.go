package main

import (
	"context"
	"log"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// idleCheckInterval is how often the daemon asks whether anyone still needs
// it. A variable so a test does not have to wait half a minute for a tick.
var idleCheckInterval = 30 * time.Second

// sessionCounter and clientCounter are the two questions the idle watcher asks.
// They are interfaces so a test can answer them without a PTY or a socket.
type sessionCounter interface{ List() []hub.SessionSummary }

type clientCounter interface{ Clients() int }

// watchIdle stops the daemon once it has had no sessions and no clients for
// the whole timeout.
//
// A daemon that outlives its clients is the point of this program, so the bar
// for exiting is deliberately high: one attached client is enough to stay, and
// so is one session, even an exited one that nobody has cleared yet. What this
// catches is the daemon nobody ever used, or the one whose last pane was
// closed an hour ago: leaving those running forever would turn "start it when
// you need it" into a process that accumulates.
func watchIdle(ctx context.Context, sessions sessionCounter, clients clientCounter,
	timeout time.Duration, stop func()) {
	if timeout <= 0 {
		return // idle shutdown disabled
	}

	ticker := time.NewTicker(idleCheckInterval)
	defer ticker.Stop()

	idleSince := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if len(sessions.List()) > 0 || clients.Clients() > 0 {
				idleSince = now
				continue
			}
			if now.Sub(idleSince) >= timeout {
				log.Printf("[mtuid] idle for %s with no sessions and no clients, exiting", timeout)
				stop()
				return
			}
		}
	}
}

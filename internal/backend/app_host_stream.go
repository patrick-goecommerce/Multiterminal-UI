package backend

import (
	"log"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// This file is the join between the session host and the frontend: output goes
// one way, host events the other.

// streamSession forwards a session's output into the output batcher, which is
// what the frontend reads.
//
// It attaches with ReplayAll so a pane that already produced output before the
// attach (a restore, a second window, and later a reconnect to the daemon)
// starts with what is there rather than with an empty screen.
func (a *AppService) streamSession(id int) {
	sub, err := a.host.Attach(id, hub.ReplayAll)
	if err != nil {
		log.Printf("[stream] session %d: attach failed: %v", id, err)
		return
	}
	go a.pumpToBatcher(id, sub)
}

// pumpToBatcher copies one subscription into the batcher until it ends.
//
// A chunk marked truncated means bytes before it are gone. Appending across
// that gap would leave the pane garbled for good (#157), so the batcher's
// backlog is replaced with a repaint of the mirror first and the chunk lands
// on top of it. The chunk is then applied twice, which is the same trade
// ResyncSession makes: a doubled write is a repaint, a hole is a broken pane.
func (a *AppService) pumpToBatcher(id int, sub *hub.Subscription) {
	defer sub.Close()
	for chunk := range sub.C {
		if chunk.Truncated {
			log.Printf("[stream] session %d: output before offset %d was dropped, repainting",
				id, chunk.Offset)
			a.outputBatch().replaceWith(id, func() []byte {
				painted, err := a.host.Repaint(id)
				if err != nil {
					return nil
				}
				return painted
			})
		}
		if len(chunk.Data) > 0 {
			a.outputBatch().add(id, chunk.Data)
		}
	}
}

// onHostEvent turns the host's session events into the Wails events the
// frontend already listens for. Keeping the translation here is what lets the
// frontend stay unchanged when the host moves into another process.
func (a *AppService) onHostEvent(name string, payload any) {
	// The payload arrives as a struct from an in-process host and as JSON from
	// the daemon, so every case decodes rather than asserts. Asserting worked
	// against the embedded host only, which meant an exit reported by the
	// daemon reached nobody.
	switch name {
	case hub.EventSessionScan:
		report, ok := hub.DecodePayload[hub.ScanReport](payload)
		if !ok {
			return
		}
		a.applyScanResults(report.Results)

	case hub.EventQueueUpdate:
		ev, ok := hub.DecodePayload[hub.QueueUpdate](payload)
		if !ok {
			return
		}
		a.onQueueUpdate(ev.SessionID)

	case hub.EventQueueItemDone:
		// The host says a queued prompt finished. What that means is this
		// window's business: the worktree-finish flow enqueues a prep prompt
		// and moves on when it completes.
		ev, ok := hub.DecodePayload[hub.QueueItemDone](payload)
		if !ok {
			return
		}
		a.onQueueItemDone(ev.SessionID, ev.ItemID)

	case hub.EventTmuxCommand:
		entry, ok := hub.DecodePayload[hub.TmuxCommand](payload)
		if !ok || a.app == nil {
			return
		}
		a.app.Event.Emit("tmux:command", entry)

	case hub.EventSessionHook:
		report, ok := hub.DecodePayload[hub.HookReport](payload)
		if !ok {
			return
		}
		a.onHookReport(report)

	case hub.EventSessionExited:
		ev, ok := hub.DecodePayload[hub.SessionExited](payload)
		if !ok || a.app == nil {
			return // no frontend to notify (before ServiceStartup, and in tests)
		}
		a.app.Event.Emit("terminal:exit", TerminalExitEvent{ID: ev.ID, ExitCode: ev.ExitCode})

	case hub.EventSessionSuspended:
		ev, ok := hub.DecodePayload[hub.SessionSuspended](payload)
		if !ok {
			return
		}
		a.emitLifecycleActivity(ev.ID, "sleeping")

	case hub.EventSessionResumed:
		ev, ok := hub.DecodePayload[hub.SessionResumed](payload)
		if !ok {
			return
		}
		// 12 to 15 seconds pass before the agent has replayed its transcript;
		// the pane shows "wacht auf" until the scan reports a real state.
		a.emitLifecycleActivity(ev.ID, "resuming")
	}
}

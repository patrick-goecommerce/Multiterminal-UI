package backend

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
)

// Waiting for another agent.
//
// An agent that delegates work can send a prompt and read output, but it had
// no way to wait: it either polled read_output in a loop, burning turns on a
// screen that had not changed, or guessed at a sleep. Waiting is what turns
// several panes into a pipeline, and it is one call because the host already
// knows every session's state.
//
// The vocabulary is deliberately small and about the agent, not the process:
// "done" is finished, "blocked" is waiting for a human (a permission prompt or
// a question), "idle" is neither, and "exited" is the process being gone.

// agentWaitStates are the states WaitForAgent accepts.
var agentWaitStates = map[string]bool{
	"done":    true,
	"blocked": true,
	"idle":    true,
	"exited":  true,
}

const (
	// agentWaitPoll is how often the session's state is checked. Short enough
	// that a caller does not sit on a finished agent, long enough that a wait
	// of several minutes costs nothing measurable.
	agentWaitPoll = 250 * time.Millisecond
	// agentWaitDefault applies when the caller names no timeout.
	agentWaitDefault = 5 * time.Minute
	// agentWaitMax bounds it. A wait is a call somebody is holding open, and
	// an agent that would wait forever should be told to come back instead.
	agentWaitMax = 30 * time.Minute
)

// WaitForAgent blocks until a session reaches one of the given states and
// returns the state it reached.
//
// An empty list means "done or blocked", which is the question worth asking
// about another agent: it has either finished or it needs a human. A session
// that is already in one of the states returns at once.
func (a *AppService) WaitForAgent(ctx context.Context, sessionID int, until []string, timeout time.Duration) (string, error) {
	wanted, err := normalizeWaitStates(until)
	if err != nil {
		return "", err
	}
	switch {
	case timeout <= 0:
		timeout = agentWaitDefault
	case timeout > agentWaitMax:
		timeout = agentWaitMax
	}

	if _, err := a.host.Get(sessionID); err != nil {
		return "", fmt.Errorf("session %d not found", sessionID)
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(agentWaitPoll)
	defer ticker.Stop()
	for {
		state := a.agentWaitState(sessionID)
		if wanted[state] {
			return state, nil
		}
		// "exited" always ends the wait, asked for or not: nothing about a
		// gone process is going to change, and a caller waiting for "done" on
		// a session that died would otherwise sit out the whole timeout.
		if state == "exited" {
			return state, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("session %d was still %q after %s", sessionID, state, timeout)
		}
	}
}

// agentWaitState collapses a session's summary into the wait vocabulary.
func (a *AppService) agentWaitState(sessionID int) string {
	summary, err := a.host.Get(sessionID)
	if err != nil {
		return "exited" // gone from the host entirely
	}
	switch summary.Status {
	case hub.StatusExited, hub.StatusError:
		return "exited"
	case hub.StatusSuspended, hub.StatusSuspending:
		// A sleeping pane is finished work with its process released; for a
		// caller waiting on the outcome that is the same as done.
		return "done"
	}
	switch summary.Activity {
	case hub.ActivityDone:
		return "done"
	case hub.ActivityWaitingPermission, hub.ActivityWaitingAnswer:
		return "blocked"
	case hub.ActivityError:
		return "blocked"
	default:
		return "idle"
	}
}

// normalizeWaitStates validates the requested states and returns them as a set.
func normalizeWaitStates(until []string) (map[string]bool, error) {
	if len(until) == 0 {
		return map[string]bool{"done": true, "blocked": true}, nil
	}
	wanted := make(map[string]bool, len(until))
	for _, s := range until {
		s = strings.ToLower(strings.TrimSpace(s))
		if !agentWaitStates[s] {
			return nil, fmt.Errorf("unknown state %q, want one of %s", s, knownWaitStates())
		}
		wanted[s] = true
	}
	return wanted, nil
}

func knownWaitStates() string {
	names := make([]string, 0, len(agentWaitStates))
	for s := range agentWaitStates {
		names = append(names, s)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

package hub

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Waiting for an agent.
//
// An agent that delegates work can send a prompt and read output, but without
// this it has no way to wait: it either polls the screen in a loop, burning
// turns on output that has not changed, or guesses at a sleep. Waiting is what
// turns several panes into a pipeline, and it is one call because the Host
// already knows every session's state.
//
// It lives here rather than in a caller because all three faces of MTUI need
// it: the GUI's MCP server, the CLI, and anything later that speaks the
// protocol directly. A second implementation would be a second vocabulary.
//
// That vocabulary is deliberately small, and about the agent rather than the
// process: "done" is finished, "blocked" is waiting for a human (a permission
// prompt or a question), "idle" is neither, and "exited" is the process being
// gone.

// AgentStates are the states WaitForAgent accepts.
var AgentStates = map[string]bool{
	"done":    true,
	"blocked": true,
	"idle":    true,
	"exited":  true,
}

// ErrWaitTimeout means a wait ended on its deadline rather than on a state.
// It is a sentinel so that a caller can tell "the agent is still working" from
// "the call failed" without reading the message, which those two deserve
// different handling for: the first is news, the second is a bug.
var ErrWaitTimeout = errors.New("hub: wait timed out")

// WaitTimeout carries what the session was doing when the deadline passed.
// That is the part worth reporting: "still active" and "still idle" mean very
// different things about an agent nobody heard from.
type WaitTimeout struct {
	SessionID int
	State     string
	Timeout   time.Duration
}

func (e *WaitTimeout) Error() string {
	return fmt.Sprintf("session %d was still %q after %s", e.SessionID, e.State, e.Timeout)
}

func (e *WaitTimeout) Unwrap() error { return ErrWaitTimeout }

const (
	// AgentWaitPoll is how often the session's state is checked. Short enough
	// that a caller does not sit on a finished agent, long enough that a wait
	// of several minutes costs nothing measurable.
	AgentWaitPoll = 250 * time.Millisecond
	// AgentWaitDefault applies when the caller names no timeout.
	AgentWaitDefault = 5 * time.Minute
	// AgentWaitMax bounds it. A wait is a call somebody is holding open, and
	// an agent that would wait forever should be told to come back instead.
	AgentWaitMax = 30 * time.Minute
)

// WaitForAgent blocks until a session reaches one of the given states and
// returns the state it reached.
//
// An empty list means "done or blocked", which is the question worth asking
// about another agent: it has either finished or it needs a human. A session
// that is already in one of the states returns at once.
func WaitForAgent(ctx context.Context, h Host, sessionID int, until []string, timeout time.Duration) (string, error) {
	wanted, err := NormalizeAgentStates(until)
	if err != nil {
		return "", err
	}
	switch {
	case timeout <= 0:
		timeout = AgentWaitDefault
	case timeout > AgentWaitMax:
		timeout = AgentWaitMax
	}

	if _, err := h.Get(sessionID); err != nil {
		return "", fmt.Errorf("session %d not found", sessionID)
	}

	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(AgentWaitPoll)
	defer ticker.Stop()
	for {
		state := AgentStateOf(h, sessionID)
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
			return "", &WaitTimeout{
				SessionID: sessionID,
				State:     AgentStateOf(h, sessionID),
				Timeout:   timeout,
			}
		}
	}
}

// AgentStateOf collapses one session's current state into the wait vocabulary.
// A session the Host does not have has exited as far as a caller is concerned.
func AgentStateOf(h Host, sessionID int) string {
	summary, err := h.Get(sessionID)
	if err != nil {
		return "exited" // gone from the host entirely
	}
	return AgentState(summary)
}

// AgentState collapses a summary into the wait vocabulary.
func AgentState(summary SessionSummary) string {
	switch summary.Status {
	case StatusExited, StatusError:
		return "exited"
	case StatusSuspended, StatusSuspending:
		// A sleeping pane is finished work with its process released; for a
		// caller waiting on the outcome that is the same as done.
		return "done"
	}
	switch summary.Activity {
	case ActivityDone:
		return "done"
	case ActivityWaitingPermission, ActivityWaitingAnswer:
		return "blocked"
	case ActivityError:
		return "blocked"
	default:
		return "idle"
	}
}

// NormalizeAgentStates validates the requested states and returns them as a
// set. An empty request means "done or blocked".
func NormalizeAgentStates(until []string) (map[string]bool, error) {
	if len(until) == 0 {
		return map[string]bool{"done": true, "blocked": true}, nil
	}
	wanted := make(map[string]bool, len(until))
	for _, s := range until {
		s = strings.ToLower(strings.TrimSpace(s))
		if !AgentStates[s] {
			return nil, fmt.Errorf("unknown state %q, want one of %s", s, KnownAgentStates())
		}
		wanted[s] = true
	}
	return wanted, nil
}

// KnownAgentStates lists the vocabulary, for error messages and help text.
func KnownAgentStates() string {
	names := make([]string, 0, len(AgentStates))
	for s := range AgentStates {
		names = append(names, s)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

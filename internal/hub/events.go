package hub

import "encoding/json"

// What a Host reports, and to whom.
//
// Split out of types.go for the file-size rule, along the line that was
// already there: everything above is the vocabulary of a request, everything
// here is the vocabulary of a notification.

// Event names emitted by a Host. They are deliberately not the Wails event
// names: a client translates them into whatever its UI listens for.
const (
	// EventSessionScan carries a whole scan tick. It is one event for all
	// sessions rather than one per session because the consumer's own work
	// (debouncing, advancing a queue) is per tick, not per session.
	EventSessionScan = "session.scan"
	// EventTmuxCommand reports what the tmux shim was asked to run. It is
	// informational: MTUI has no tmux, and the shim exists so an agent that
	// reaches for one gets an answer instead of an error.
	EventTmuxCommand = "tmux.command"
	// EventSessionHook carries one lifecycle event the agent reported, after
	// the host has recorded what it said about the session's state.
	EventSessionHook    = "session.hook"
	EventSessionCreated = "session.created"
	EventSessionExited  = "session.exited"
	// EventQueueUpdate carries a session's whole prompt queue after any change
	// to it. The whole list rather than a delta: a client that missed one
	// event would otherwise be wrong until the next, and the list is short.
	EventQueueUpdate = "queue.update"
	// EventQueueItemDone reports that a queued prompt finished. The host has
	// no view on what that means; MTUI's worktree-finish flow is one consumer.
	EventQueueItemDone    = "queue.item_done"
	EventSessionSuspended = "session.suspended"
	EventSessionResumed   = "session.resumed"
)

// SessionCreated is the payload of EventSessionCreated.
type SessionCreated struct {
	Session SessionSummary `json:"session" yaml:"session"`
}

// SessionExited is the payload of EventSessionExited.
type SessionExited struct {
	ID       int `json:"id" yaml:"id"`
	ExitCode int `json:"exit_code" yaml:"exit_code"`
}

// ScanReport is the payload of EventSessionScan.
type ScanReport struct {
	Results []ScanResult `json:"results" yaml:"results"`
}

// TmuxCommand is the payload of EventTmuxCommand.
type TmuxCommand struct {
	Args []string `json:"args" yaml:"args"`
	Dir  string   `json:"dir" yaml:"dir"`
	Env  string   `json:"env" yaml:"env"`
}

// HookReport is the payload of EventSessionHook: one lifecycle event, plus
// what the host made of it.
type HookReport struct {
	Session        int    `json:"session" yaml:"session"`
	Event          string `json:"event" yaml:"event"`
	AgentSessionID string `json:"agent_session_id" yaml:"agent_session_id"`
	Tool           string `json:"tool" yaml:"tool"`
	Message        string `json:"message" yaml:"message"`
	Cwd            string `json:"cwd" yaml:"cwd"`
	WorktreePath   string `json:"worktree_path" yaml:"worktree_path"`
	WorktreeBranch string `json:"worktree_branch" yaml:"worktree_branch"`
	BlockedPath    string `json:"blocked_path" yaml:"blocked_path"`
	BlockReason    string `json:"block_reason" yaml:"block_reason"`
	// Activity is what this event said about the session's state, empty when
	// it said nothing. The host has already applied it.
	Activity Activity `json:"activity,omitempty" yaml:"activity,omitempty"`
}

// SessionSuspended is the payload of EventSessionSuspended: the pane's process
// tree is gone on purpose and the session object is waiting to be resumed.
type SessionSuspended struct {
	ID       int    `json:"id" yaml:"id"`
	ResumeID string `json:"resume_id" yaml:"resume_id"`
}

// SessionResumed is the payload of EventSessionResumed.
type SessionResumed struct {
	ID       int    `json:"id" yaml:"id"`
	ResumeID string `json:"resume_id" yaml:"resume_id"`
}

// EventSink receives Host events. The daemon's sink fans them out to connected
// clients; the GUI's sink re-emits them as Wails events so the frontend keeps
// listening for exactly what it listens for today.
type EventSink interface {
	Emit(name string, payload any)
}

// SinkFunc adapts a function to EventSink.
type SinkFunc func(name string, payload any)

// Emit implements EventSink.
func (f SinkFunc) Emit(name string, payload any) { f(name, payload) }

// DecodePayload reads an event payload as T.
//
// The same event arrives in two shapes depending on where the host is: an
// in-process host hands the payload struct straight to the sink, while a
// remote one delivers the JSON the daemon sent. A handler that only type-
// asserts works against one of them and silently ignores the other, which is
// how a client can end up never hearing about an exit at all.
func DecodePayload[T any](payload any) (T, bool) {
	if typed, ok := payload.(T); ok {
		return typed, true
	}
	var out T
	switch raw := payload.(type) {
	case json.RawMessage:
		if json.Unmarshal(raw, &out) == nil {
			return out, true
		}
	case []byte:
		if json.Unmarshal(raw, &out) == nil {
			return out, true
		}
	case string:
		if json.Unmarshal([]byte(raw), &out) == nil {
			return out, true
		}
	}
	var zero T
	return zero, false
}

// QueueUpdate is the payload of EventQueueUpdate.
type QueueUpdate struct {
	SessionID int         `json:"session_id" yaml:"session_id"`
	Items     []QueueItem `json:"items" yaml:"items"`
}

// QueueItemDone is the payload of EventQueueItemDone.
type QueueItemDone struct {
	SessionID int `json:"session_id" yaml:"session_id"`
	ItemID    int `json:"item_id" yaml:"item_id"`
}

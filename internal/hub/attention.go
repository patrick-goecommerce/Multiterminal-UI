package hub

import (
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// What a session wants from the person at the screen.
//
// The confirmed state says what the agent is doing. Attention adds whether the
// person has seen it, what the agent wants (the detail) and where the state
// came from. It lives on the host next to the debounce, for the same reason
// the debounce does: the daemon, `mt ls` and the MCP server need the same
// answer as the window, and one rank function is what keeps the badge, the
// list and the counter from disagreeing (spec 2026-10-07, Teil A).

// Attention is one session's attention record.
type Attention struct {
	// State is the last confirmed state that is not a lifecycle label: a
	// suspend does not turn "done, unseen" into nothing.
	State Activity  `json:"state" yaml:"state"`
	Since time.Time `json:"since" yaml:"since"`
	// Seen is set when the person looked at the pane (MarkSeen) and cleared
	// by every newly confirmed state.
	Seen   bool   `json:"seen" yaml:"seen"`
	Detail string `json:"detail" yaml:"detail"`
	Source string `json:"source" yaml:"source"`
	// Rank is AttentionRank(State, Seen), carried along for convenience. A
	// reader that may face an older peer recomputes it instead.
	Rank int `json:"rank" yaml:"rank"`
}

// Ranks, most urgent first. Everything up to RankDone is listed as waiting.
const (
	RankPermission = 0
	RankQuestion   = 1
	RankError      = 2
	RankDone       = 3
	RankWorking    = 4
	RankQuiet      = 5
)

// AttentionRank is the only place the urgency order is decided. Seeing a pane
// clears "done" and "error" but not a permission or a question: those block
// the agent until it moves on, and looking does not answer them.
func AttentionRank(state Activity, seen bool) int {
	switch state {
	case ActivityWaitingPermission:
		return RankPermission
	case ActivityWaitingAnswer:
		return RankQuestion
	case ActivityError:
		if seen {
			return RankQuiet
		}
		return RankError
	case ActivityDone:
		if seen {
			return RankQuiet
		}
		return RankDone
	case ActivityActive:
		return RankWorking
	default:
		return RankQuiet
	}
}

// detailMax bounds a detail, in runes. The list shows one line of it.
const detailMax = 200

// clipDetail collapses whitespace and cuts the text to detailMax runes.
func clipDetail(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= detailMax {
		return s
	}
	r := []rune(s)
	return string(r[:detailMax-1]) + "…"
}

// attentionNote is what the latest report said about the state it set.
type attentionNote struct {
	state  Activity
	source string
	detail string
}

// attentionBook holds the per-session records. It has its own mutex, like the
// debouncer: the scan, the hook reader and client calls all write to it, and
// none of them may wait on the host's session map.
type attentionBook struct {
	mu      sync.Mutex
	entries map[int]Attention
	notes   map[int]attentionNote
}

func newAttentionBook() *attentionBook {
	return &attentionBook{entries: map[int]Attention{}, notes: map[int]attentionNote{}}
}

// noteHook records what a lifecycle hook said. A Notification for the state
// already noted does not replace a detail: PermissionRequest carries the
// command, the Notification six seconds later only "Claude needs your
// permission".
func (b *attentionBook) noteHook(id int, state Activity, event, detail string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if prev, ok := b.notes[id]; ok && event == "Notification" && prev.state == state && prev.detail != "" {
		return
	}
	source := "Hook: " + event
	if event == "Stop" && state == ActivityWaitingAnswer {
		source = "Hook: Stop, Frage im letzten Absatz"
	}
	b.notes[id] = attentionNote{state: state, source: source, detail: clipDetail(detail)}
}

// noteFact records a state the host worked out itself: an exit, a hang.
func (b *attentionBook) noteFact(id int, state Activity, source, detail string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.notes[id] = attentionNote{state: state, source: source, detail: clipDetail(detail)}
}

// confirmed records a newly confirmed state and returns the new record. It
// reports false for the lifecycle labels, which leave the record alone: a
// pane put to sleep while "done, unseen" is still done and unseen.
func (b *attentionBook) confirmed(id int, state Activity, since time.Time) (Attention, bool) {
	if state == "" || state == ActivitySleeping || state == ActivityResuming {
		return Attention{}, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	a := Attention{State: state, Since: since, Source: "Bildschirm-Scan"}
	if n, ok := b.notes[id]; ok {
		switch {
		case n.state == state:
			a.Source, a.Detail = n.source, n.detail
			delete(b.notes, id) // a note is used by the one confirmation it describes
		case state == ActivityWaitingAnswer && n.state == ActivityDone:
			// The screen saw a question the Stop text did not end with.
			a.Source, a.Detail = "Bildschirm-Scan nach "+n.source, n.detail
			delete(b.notes, id)
		}
	}
	a.Rank = AttentionRank(a.State, false)
	b.entries[id] = a
	return a, true
}

// markSeen sets or clears the seen flag and reports whether it changed.
func (b *attentionBook) markSeen(id int, seen bool) (Attention, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	a, ok := b.entries[id]
	if !ok {
		return Attention{Rank: RankQuiet}, false
	}
	if a.Seen == seen {
		return a, false
	}
	a.Seen = seen
	a.Rank = AttentionRank(a.State, seen)
	b.entries[id] = a
	return a, true
}

// get returns the record, or a quiet one for a session that has none yet.
func (b *attentionBook) get(id int) Attention {
	b.mu.Lock()
	defer b.mu.Unlock()
	if a, ok := b.entries[id]; ok {
		return a
	}
	return Attention{Rank: RankQuiet}
}

// forget drops a closed session's record.
func (b *attentionBook) forget(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.entries, id)
	delete(b.notes, id)
}

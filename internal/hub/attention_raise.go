package hub

import "time"

// raise brings a session back to the waiting list because something outside
// it went wrong: CI turned red, the PR got blocked (spec C4). The record
// becomes an unseen error, rank 2, which may toast.
//
// A session that is working or waits for a permission or an answer keeps its
// record and raise reports false: the agent is on it already, or the pane is
// at the top anyway. The next confirmed state replaces a raised record like
// any other.
func (b *attentionBook) raise(id int, source, detail string, at time.Time) (Attention, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	cur, ok := b.entries[id]
	if !ok {
		cur = Attention{Rank: RankQuiet}
	}
	switch cur.State {
	case ActivityActive, ActivityWaitingPermission, ActivityWaitingAnswer:
		return cur, false
	}
	a := Attention{State: ActivityError, Since: at, Source: source, Detail: clipDetail(detail)}
	a.Rank = AttentionRank(a.State, a.Seen)
	b.entries[id] = a
	return a, true
}

// raiseFor raises the session and tells the clients. The book's lock is
// released before the event goes out.
func (h *Embedded) raiseFor(id int, source, detail string, at time.Time) {
	if a, ok := h.attention.raise(id, source, detail, at); ok {
		h.emit(EventSessionAttention, AttentionUpdate{ID: id, Attention: a})
	}
}

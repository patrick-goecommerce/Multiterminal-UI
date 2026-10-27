package hub

import (
	"sync"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hooks"
)

// turn holds the facts of one agent turn while it runs: what was asked, where
// it ran, the tree at its start and the checks it ran.
type turn struct {
	promptID, prompt, cwd, startTree string
	keepAlive, publish, closed       bool
	checks                           []CheckResult
}

// turnBook keeps the current turn of every session. It is called from the
// hook reader and from build goroutines, so it has its own lock.
type turnBook struct {
	mu        sync.Mutex
	turns     map[int]*turn
	keepAlive map[int]bool
}

func newTurnBook() *turnBook {
	return &turnBook{turns: map[int]*turn{}, keepAlive: map[int]bool{}}
}

// expectKeepAlive marks the next turn of a session as the keep-alive's own.
func (b *turnBook) expectKeepAlive(id int) {
	b.mu.Lock()
	b.keepAlive[id] = true
	b.mu.Unlock()
}

// begin starts a turn, replacing the previous one, and reports whether it is
// a keep-alive turn.
func (b *turnBook) begin(id int, promptID, prompt, cwd string) (keepAlive bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	keepAlive = b.keepAlive[id]
	delete(b.keepAlive, id)
	b.turns[id] = &turn{
		promptID: promptID, prompt: clipDetail(prompt), cwd: cwd, keepAlive: keepAlive,
	}
	return keepAlive
}

// setStartTree records the work-tree snapshot taken at the start of the turn.
func (b *turnBook) setStartTree(id int, promptID, tree string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if t := b.turns[id]; t != nil && !t.closed && sameTurn(t, promptID) {
		t.startTree = tree
	}
}

// observe folds a hook event into the open turn of a session.
func (b *turnBook) observe(id int, ev hooks.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	t := b.turns[id]
	if t == nil || t.closed || !sameTurn(t, ev.PromptID) || ev.Interrupt {
		return
	}
	if ev.Cwd != "" {
		t.cwd = ev.Cwd
	}
	if ev.Tool != "Bash" || (ev.Event != "PostToolUse" && ev.Event != "PostToolUseFailure") {
		return
	}
	if publishCommand(ev.Command) {
		t.publish = true
	}
	res, ok := classifyCheck(ev.Command, ev.Event == "PostToolUseFailure", ev.OutputTail)
	if !ok {
		return
	}
	for i := range t.checks {
		if t.checks[i].Label == res.Label {
			t.checks[i] = res
			return
		}
	}
	t.checks = append(t.checks, res)
}

// close ends the turn and returns a copy; a second close, or one for another
// prompt, returns false.
func (b *turnBook) close(id int, promptID string) (turn, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	t := b.turns[id]
	if t == nil || t.closed || !sameTurn(t, promptID) {
		return turn{}, false
	}
	t.closed = true
	out := *t
	out.checks = append([]CheckResult(nil), t.checks...)
	return out, true
}

// forget drops everything known about a session.
func (b *turnBook) forget(id int) {
	b.mu.Lock()
	delete(b.turns, id)
	delete(b.keepAlive, id)
	b.mu.Unlock()
}

// sameTurn is false only when both sides name a prompt and the names differ.
func sameTurn(t *turn, promptID string) bool {
	return t.promptID == "" || promptID == "" || t.promptID == promptID
}

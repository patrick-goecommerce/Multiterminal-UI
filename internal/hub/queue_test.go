package hub

import (
	"strings"
	"testing"
	"time"
)

// These came over from internal/backend with the queue. What changed is that
// the host now owns the state, so the tests drive real sessions instead of a
// map, and the send path actually reaches a PTY.

// queueHost returns a host with one live session to queue against.
func queueHost(t *testing.T) (*Embedded, int) {
	t.Helper()
	h := NewEmbedded(Options{Version: "test"})
	t.Cleanup(h.Release)
	id, err := h.Create(CreateSpec{Argv: shellArgv(), Dir: t.TempDir(), Rows: 24, Cols: 80})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return h, id
}

// statuses renders a queue compactly, so a failure shows the whole picture.
func statuses(items []QueueItem) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, it.Status)
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// The first item goes out at once. Waiting for a transition that already
// happened is how a queue with one item in it hangs forever.
func TestQueue_FirstItemIsSentImmediately(t *testing.T) {
	h, id := queueHost(t)

	item, err := h.QueueAdd(id, "erster prompt")
	if err != nil {
		t.Fatalf("QueueAdd: %v", err)
	}
	if item.ID == 0 || item.Prompt != "erster prompt" {
		t.Fatalf("returned item = %+v", item)
	}
	items := h.QueueList(id)
	if len(items) != 1 || items[0].Status != QueueSent {
		t.Errorf("queue = %s, want one sent item", statuses(items))
	}
}

// A second item waits: one prompt is in flight and the agent has it.
func TestQueue_SecondItemWaits(t *testing.T) {
	h, id := queueHost(t)
	mustAdd(t, h, id, "eins")
	mustAdd(t, h, id, "zwei")

	items := h.QueueList(id)
	if len(items) != 2 || items[0].Status != QueueSent || items[1].Status != QueuePending {
		t.Errorf("queue = %s, want [sent pending]", statuses(items))
	}
}

// Advancing marks the item in flight done and sends the next one. This is
// what runs on every confirmed "done", with or without a window open.
func TestQueue_AdvanceCompletesAndSendsTheNext(t *testing.T) {
	h, id := queueHost(t)
	mustAdd(t, h, id, "eins")
	mustAdd(t, h, id, "zwei")

	h.QueueAdvance(id)

	items := h.QueueList(id)
	if len(items) != 2 || items[0].Status != QueueDone || items[1].Status != QueueSent {
		t.Errorf("queue = %s, want [done sent]", statuses(items))
	}
}

func TestQueue_AdvanceOnAnEmptyQueueIsHarmless(t *testing.T) {
	h, id := queueHost(t)
	h.QueueAdvance(id)
	if items := h.QueueList(id); len(items) != 0 {
		t.Errorf("queue = %s, want empty", statuses(items))
	}
	h.QueueAdvance(4242) // a session that does not exist
}

func TestQueue_ListIsNeverNil(t *testing.T) {
	h, id := queueHost(t)
	if items := h.QueueList(id); items == nil {
		t.Error("QueueList returned nil for a session with no queue")
	}
	if items := h.QueueList(4242); items == nil {
		t.Error("QueueList returned nil for an unknown session")
	}
}

func TestQueue_RemovePending(t *testing.T) {
	h, id := queueHost(t)
	mustAdd(t, h, id, "eins")
	second := mustAdd(t, h, id, "zwei")

	removed, err := h.QueueRemove(id, second.ID, false)
	if err != nil {
		t.Fatalf("QueueRemove: %v", err)
	}
	if !removed {
		t.Error("a pending item was not removed")
	}
	if items := h.QueueList(id); len(items) != 1 {
		t.Errorf("queue = %s, want one item left", statuses(items))
	}
}

// An item in flight stays: the agent already has the prompt, so removing the
// row would only hide what is happening.
func TestQueue_RemoveRefusesAnItemInFlight(t *testing.T) {
	h, id := queueHost(t)
	first := mustAdd(t, h, id, "eins")

	removed, err := h.QueueRemove(id, first.ID, false)
	if err != nil {
		t.Fatalf("QueueRemove: %v", err)
	}
	if removed {
		t.Error("an item in flight was removed")
	}
	if items := h.QueueList(id); len(items) != 1 {
		t.Errorf("queue = %s, want the item still there", statuses(items))
	}
}

// force is for a caller that owns the item outright and is tearing down the
// flow it belongs to, which is how MTUI cancels a worktree finish.
func TestQueue_ForceRemovesAnItemInFlight(t *testing.T) {
	h, id := queueHost(t)
	first := mustAdd(t, h, id, "eins")

	removed, err := h.QueueRemove(id, first.ID, true)
	if err != nil {
		t.Fatalf("QueueRemove: %v", err)
	}
	if !removed {
		t.Error("force did not remove an item in flight")
	}
	if items := h.QueueList(id); len(items) != 0 {
		t.Errorf("queue = %s, want empty", statuses(items))
	}
}

func TestQueue_ClearDoneKeepsTheRest(t *testing.T) {
	h, id := queueHost(t)
	mustAdd(t, h, id, "eins")
	mustAdd(t, h, id, "zwei")
	h.QueueAdvance(id) // first → done, second → sent

	if err := h.QueueClear(id, true); err != nil {
		t.Fatalf("QueueClear: %v", err)
	}
	items := h.QueueList(id)
	if len(items) != 1 || items[0].Status != QueueSent {
		t.Errorf("queue = %s, want only the item in flight", statuses(items))
	}
}

func TestQueue_ClearEverything(t *testing.T) {
	h, id := queueHost(t)
	mustAdd(t, h, id, "eins")
	mustAdd(t, h, id, "zwei")

	if err := h.QueueClear(id, false); err != nil {
		t.Fatalf("QueueClear: %v", err)
	}
	if items := h.QueueList(id); len(items) != 0 {
		t.Errorf("queue = %s, want empty", statuses(items))
	}
}

// A closed session takes its queue with it, or every closed pane leaves its
// prompts behind for the life of the process.
func TestQueue_CloseForgetsTheQueue(t *testing.T) {
	h, id := queueHost(t)
	mustAdd(t, h, id, "eins")

	if err := h.Close(id); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if items := h.QueueList(id); len(items) != 0 {
		t.Errorf("queue after close = %s, want empty", statuses(items))
	}
}

func TestQueue_AddToAnUnknownSessionFails(t *testing.T) {
	h := newTestHost(t, nil)
	if _, err := h.QueueAdd(4242, "prompt"); err == nil {
		t.Error("adding to an unknown session succeeded")
	}
}

// The events are how a window learns about a queue it does not own.
func TestQueue_EmitsUpdatesAndCompletions(t *testing.T) {
	var got []string
	sink := SinkFunc(func(name string, _ any) {
		if strings.HasPrefix(name, "queue.") {
			got = append(got, name)
		}
	})
	h := NewEmbedded(Options{Version: "test", Sink: sink})
	t.Cleanup(h.Release)
	id, err := h.Create(CreateSpec{Argv: shellArgv(), Dir: t.TempDir(), Rows: 24, Cols: 80})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	mustAdd(t, h, id, "eins")
	mustAdd(t, h, id, "zwei")
	h.QueueAdvance(id)

	joined := strings.Join(got, " ")
	if !strings.Contains(joined, EventQueueUpdate) {
		t.Errorf("events = %v, want an update", got)
	}
	if !strings.Contains(joined, EventQueueItemDone) {
		t.Errorf("events = %v, want an item-done", got)
	}
}

// The prompt has to reach the PTY, with a Return after it. Writing both in one
// chunk lets an agent's redraw swallow the Return, which is why they are
// separate writes with a pause between them.
func TestQueue_ThePromptReachesTheTerminal(t *testing.T) {
	h, id := queueHost(t)
	mustAdd(t, h, id, "echo queue-marker")

	deadline := time.Now().Add(15 * time.Second)
	for {
		text, _ := h.PlainText(id)
		for _, line := range strings.Split(text, "\n") {
			if strings.TrimSpace(line) == "queue-marker" {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("the queued prompt never ran:\n%s", text)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func mustAdd(t *testing.T, h *Embedded, id int, prompt string) QueueItem {
	t.Helper()
	item, err := h.QueueAdd(id, prompt)
	if err != nil {
		t.Fatalf("QueueAdd(%q): %v", prompt, err)
	}
	return item
}

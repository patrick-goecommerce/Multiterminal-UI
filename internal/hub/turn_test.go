package hub

import (
	"strings"
	"testing"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hooks"
)

func bashEv(event, prompt, cmd string) hooks.Event {
	return hooks.Event{Event: event, Tool: "Bash", PromptID: prompt, Command: cmd}
}

func TestRedThenGreenIsOK(t *testing.T) {
	b := newTurnBook()
	b.begin(1, "p1", "fix it", "/w")
	b.observe(1, bashEv("PostToolUseFailure", "p1", "go test ./..."))
	b.observe(1, bashEv("PostToolUse", "p1", "go test ./..."))
	tr, ok := b.close(1, "p1")
	if !ok || len(tr.checks) != 1 || tr.checks[0].Result != CheckOK {
		t.Fatalf("got %+v ok=%v, want one ok check", tr.checks, ok)
	}
}

func TestKeepAliveTurnIsMarked(t *testing.T) {
	b := newTurnBook()
	b.expectKeepAlive(1)
	if !b.begin(1, "p1", "x", "") {
		t.Fatal("first begin should be keep-alive")
	}
	if b.begin(1, "p2", "x", "") {
		t.Fatal("second begin should not be keep-alive")
	}
}

func TestEventsAfterCloseIgnored(t *testing.T) {
	b := newTurnBook()
	b.begin(1, "p1", "x", "")
	b.close(1, "p1")
	b.observe(1, bashEv("PostToolUse", "p1", "go test ./..."))
	if _, ok := b.close(1, "p1"); ok {
		t.Fatal("turn was closed already")
	}
	if got := b.turns[1]; len(got.checks) != 0 {
		t.Fatalf("closed turn took an event: %+v", got.checks)
	}
}

func TestForeignPromptIgnored(t *testing.T) {
	b := newTurnBook()
	b.begin(1, "p1", "x", "")
	b.observe(1, bashEv("PostToolUse", "other", "go test ./..."))
	tr, _ := b.close(1, "p1")
	if len(tr.checks) != 0 {
		t.Fatalf("foreign prompt counted: %+v", tr.checks)
	}
}

func TestInterruptIgnored(t *testing.T) {
	b := newTurnBook()
	b.begin(1, "p1", "x", "")
	ev := bashEv("PostToolUseFailure", "p1", "go test ./...")
	ev.Interrupt = true
	b.observe(1, ev)
	tr, _ := b.close(1, "p1")
	if len(tr.checks) != 0 {
		t.Fatalf("interrupt counted: %+v", tr.checks)
	}
}

func TestPublishNoted(t *testing.T) {
	b := newTurnBook()
	b.begin(1, "p1", "x", "")
	b.observe(1, bashEv("PostToolUse", "p1", "git push origin x"))
	tr, _ := b.close(1, "p1")
	if !tr.publish {
		t.Fatal("git push should set publish")
	}
}

func TestCloseTwice(t *testing.T) {
	b := newTurnBook()
	b.begin(1, "p1", "x", "")
	if _, ok := b.close(1, "p1"); !ok {
		t.Fatal("first close should succeed")
	}
	if _, ok := b.close(1, "p1"); ok {
		t.Fatal("second close should fail")
	}
}

func TestPromptClipped(t *testing.T) {
	b := newTurnBook()
	b.begin(1, "p1", strings.Repeat("ä", 300), "")
	tr, _ := b.close(1, "p1")
	if n := len([]rune(tr.prompt)); n != 200 {
		t.Fatalf("prompt has %d runes, want 200", n)
	}
}

func TestCwdAndStartTree(t *testing.T) {
	b := newTurnBook()
	b.begin(1, "p1", "x", "/a")
	b.observe(1, hooks.Event{Event: "PreToolUse", PromptID: "p1", Cwd: "/b"})
	b.setStartTree(1, "other", "tree0", "/x")
	b.setStartTree(1, "p1", "tree1", "/a")
	tr, _ := b.close(1, "p1")
	if tr.cwd != "/b" || tr.startTree != "tree1" || tr.startTop != "/a" {
		t.Fatalf("cwd=%q startTree=%q startTop=%q", tr.cwd, tr.startTree, tr.startTop)
	}
}

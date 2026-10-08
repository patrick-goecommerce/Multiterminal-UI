package hub

import (
	"strings"
	"testing"
	"time"
)

func TestAttentionRank_Order(t *testing.T) {
	cases := []struct {
		state Activity
		seen  bool
		want  int
	}{
		{ActivityWaitingPermission, false, RankPermission},
		{ActivityWaitingPermission, true, RankPermission},
		{ActivityWaitingAnswer, true, RankQuestion},
		{ActivityError, false, RankError},
		{ActivityError, true, RankQuiet},
		{ActivityDone, false, RankDone},
		{ActivityDone, true, RankQuiet},
		{ActivityActive, false, RankWorking},
		{ActivityIdle, false, RankQuiet},
		{ActivitySleeping, false, RankQuiet},
		// A daemon from before attention sends a zero record: it must be quiet,
		// not rank 0, which would read as a permission request.
		{"", false, RankQuiet},
	}
	for _, c := range cases {
		if got := AttentionRank(c.state, c.seen); got != c.want {
			t.Errorf("AttentionRank(%q, %v) = %d, want %d", c.state, c.seen, got, c.want)
		}
	}
}

func TestAttentionBook_NewStateCarriesTheHookDetail(t *testing.T) {
	b := newAttentionBook()
	b.noteHook(1, ActivityWaitingPermission, "PermissionRequest", "Bash: npm install")
	at := time.Unix(1000, 0)
	a, ok := b.confirmed(1, ActivityWaitingPermission, at)
	if !ok || a.Seen || a.Detail != "Bash: npm install" || a.Source != "Hook: PermissionRequest" ||
		a.Rank != RankPermission || !a.Since.Equal(at) {
		t.Fatalf("got %+v, %v", a, ok)
	}
}

func TestAttentionBook_SeenClearsDoneButNotAPermission(t *testing.T) {
	b := newAttentionBook()
	b.confirmed(1, ActivityDone, time.Now())
	if a, changed := b.markSeen(1, true); !changed || a.Rank != RankQuiet {
		t.Errorf("done after seen: %+v changed=%v, want quiet", a, changed)
	}
	b.confirmed(2, ActivityWaitingPermission, time.Now())
	if a, _ := b.markSeen(2, true); a.Rank != RankPermission {
		t.Errorf("permission after seen: rank %d, want %d", a.Rank, RankPermission)
	}
}

func TestAttentionBook_ANewStateIsUnseenAgain(t *testing.T) {
	b := newAttentionBook()
	b.confirmed(1, ActivityDone, time.Now())
	b.markSeen(1, true)
	b.confirmed(1, ActivityActive, time.Now())
	a, _ := b.confirmed(1, ActivityDone, time.Now())
	if a.Seen || a.Rank != RankDone {
		t.Errorf("second done: %+v, want unseen rank %d", a, RankDone)
	}
}

func TestAttentionBook_SleepKeepsTheRecord(t *testing.T) {
	b := newAttentionBook()
	b.confirmed(1, ActivityDone, time.Now())
	if _, changed := b.confirmed(1, ActivitySleeping, time.Now()); changed {
		t.Error("sleeping replaced the record")
	}
	if a := b.get(1); a.Rank != RankDone {
		t.Errorf("after sleep: rank %d, want %d", a.Rank, RankDone)
	}
}

func TestAttentionBook_NotificationKeepsThePermissionDetail(t *testing.T) {
	b := newAttentionBook()
	b.noteHook(1, ActivityWaitingPermission, "PermissionRequest", "Bash: rm -rf build")
	b.noteHook(1, ActivityWaitingPermission, "Notification", "Claude needs your permission to use Bash")
	a, _ := b.confirmed(1, ActivityWaitingPermission, time.Now())
	if a.Detail != "Bash: rm -rf build" {
		t.Errorf("detail = %q, want the command", a.Detail)
	}
}

func TestAttentionBook_ScreenQuestionAfterStopKeepsTheText(t *testing.T) {
	b := newAttentionBook()
	b.noteHook(1, ActivityDone, "Stop", "Alles grün. Was liegt an?")
	a, _ := b.confirmed(1, ActivityWaitingAnswer, time.Now())
	if a.Detail != "Alles grün. Was liegt an?" || !strings.HasPrefix(a.Source, "Bildschirm-Scan nach Hook: Stop") {
		t.Errorf("got %+v", a)
	}
}

func TestAttentionBook_UnknownSessionIsQuiet(t *testing.T) {
	if a := newAttentionBook().get(42); a.Rank != RankQuiet || a.State != "" {
		t.Errorf("got %+v", a)
	}
}

func TestAttentionBook_ANoteIsUsedOnce(t *testing.T) {
	b := newAttentionBook()
	b.noteHook(1, ActivityWaitingPermission, "PermissionRequest", "Bash: rm -rf build")
	if a, _ := b.confirmed(1, ActivityWaitingPermission, time.Unix(1, 0)); a.Detail != "Bash: rm -rf build" {
		t.Fatalf("first confirmation: detail %q, want the command", a.Detail)
	}
	b.confirmed(1, ActivityActive, time.Unix(2, 0))
	a, _ := b.confirmed(1, ActivityWaitingPermission, time.Unix(3, 0))
	if a.Detail != "" || a.Source != "Bildschirm-Scan" {
		t.Errorf("later confirmation reused a consumed note: %+v", a)
	}
}

func TestAttentionBook_ANonMatchingNoteSurvives(t *testing.T) {
	b := newAttentionBook()
	b.noteHook(1, ActivityDone, "Stop", "x")
	b.confirmed(1, ActivityActive, time.Unix(1, 0))
	if a, _ := b.confirmed(1, ActivityDone, time.Unix(2, 0)); a.Detail != "x" {
		t.Errorf("done after active: detail %q, want the surviving note %q", a.Detail, "x")
	}
}

func TestAttentionBook_MarkSeenUnknownIsQuiet(t *testing.T) {
	a, changed := newAttentionBook().markSeen(99, true)
	if a.Rank != RankQuiet || changed {
		t.Errorf("markSeen on unknown id: %+v changed=%v, want quiet and unchanged", a, changed)
	}
}

func TestClipDetail_CollapsesAndCuts(t *testing.T) {
	if got := clipDetail("  a \n\n b\tc  "); got != "a b c" {
		t.Errorf("collapse: %q", got)
	}
	long := strings.Repeat("ä", 250)
	got := clipDetail(long)
	if n := len([]rune(got)); n != detailMax || !strings.HasSuffix(got, "…") {
		t.Errorf("cut: %d runes, suffix %q", n, got[len(got)-3:])
	}
}

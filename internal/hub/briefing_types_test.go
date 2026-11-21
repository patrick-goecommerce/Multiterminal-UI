package hub

import (
	"strings"
	"testing"
)

func TestWarningsOrder(t *testing.T) {
	b := Briefing{
		Checks: []CheckResult{{Label: "go test", Result: CheckFail}},
		Git:    &GitFacts{Uncommitted: 2},
		PR:     &PRFacts{State: "OPEN", Checks: "red", Conflicting: true},
	}
	got := warningsFor(b, true, true)
	want := []string{WarnCIRed, WarnPRBlocked, WarnCheckFailed, WarnEndedError, WarnDirty, WarnBigDiff}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}
	if w := warningsFor(Briefing{}, false, false); len(w) != 0 {
		t.Fatalf("empty briefing warns: %v", w)
	}
	w := warningsFor(Briefing{PR: &PRFacts{State: "OPEN", Review: "CHANGES_REQUESTED"}}, false, false)
	if len(w) != 1 || w[0] != WarnPRBlocked {
		t.Fatalf("changes requested: %v", w)
	}
}

// I3: after a merge or a close the watcher stops, so the PR's last state is
// frozen. Its red rollup or old conflict must not keep warning: spec C3 makes
// "gemergt" info.
func TestClosedPRDoesNotWarn(t *testing.T) {
	for _, state := range []string{"MERGED", "CLOSED"} {
		b := Briefing{PR: &PRFacts{State: state, Checks: "red", Conflicting: true, Review: "CHANGES_REQUESTED"}}
		if w := warningsFor(b, false, false); len(w) != 0 {
			t.Errorf("%s PR warns: %v", state, w)
		}
	}
}

func TestClosingWords(t *testing.T) {
	if got := closingWords("Erster.\n\nZweiter Absatz\nmit Umbruch."); got != "Zweiter Absatz mit Umbruch." {
		t.Fatalf("got %q", got)
	}
	if n := len([]rune(closingWords(strings.Repeat("ö", 700)))); n != 600 {
		t.Fatalf("got %d runes, want 600", n)
	}
	if closingWords("") != "" {
		t.Fatal("empty message")
	}
}

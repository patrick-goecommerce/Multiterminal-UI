package tsmodels

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

// The agent state travels as a plain string from the host (hub.Activity) to
// the frontend's Pane['activity']. Nothing checks that string on the way
// (tabs.ts casts it), so a state added on one side and not the other is
// silently wrong: a badge without a label, a pane missing from the waiting
// list. This test holds the two vocabularies together.

// frontendOnly are states the frontend sets itself and no host produces:
// "starting" is a pane's state between addPane and its first report.
var frontendOnly = map[string]bool{"starting": true}

var (
	goActivity = regexp.MustCompile(`Activity\w*\s+Activity\s*=\s*"(\w+)"`)
	tsActivity = regexp.MustCompile(`(?m)^\s*activity:\s*((?:'\w+'\s*\|?\s*)+);`)
	tsLiteral  = regexp.MustCompile(`'(\w+)'`)
)

func goActivities(t *testing.T, root string) map[string]bool {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(root, "internal/hub/types.go"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, m := range goActivity.FindAllStringSubmatch(string(src), -1) {
		out[m[1]] = true
	}
	if len(out) < 6 {
		t.Fatalf("found only %d hub.Activity constants; the pattern no longer matches types.go", len(out))
	}
	return out
}

func tsActivities(t *testing.T, root string) map[string]bool {
	t.Helper()
	src, err := os.ReadFile(filepath.Join(root, "frontend/src/stores/tabs.ts"))
	if err != nil {
		t.Fatal(err)
	}
	m := tsActivity.FindStringSubmatch(string(src))
	if m == nil {
		t.Fatal("Pane.activity union not found in tabs.ts; the pattern no longer matches")
	}
	out := map[string]bool{}
	for _, l := range tsLiteral.FindAllStringSubmatch(m[1], -1) {
		out[l[1]] = true
	}
	return out
}

func TestActivityVocabularyMatches(t *testing.T) {
	root := repoRoot(t)
	goSet, tsSet := goActivities(t, root), tsActivities(t, root)

	var missingTS, missingGo []string
	for a := range goSet {
		if !tsSet[a] {
			missingTS = append(missingTS, a)
		}
	}
	for a := range tsSet {
		if !goSet[a] && !frontendOnly[a] {
			missingGo = append(missingGo, a)
		}
	}
	sort.Strings(missingTS)
	sort.Strings(missingGo)
	for _, a := range missingTS {
		t.Errorf("hub.Activity %q is not in Pane['activity'] (frontend/src/stores/tabs.ts)", a)
	}
	for _, a := range missingGo {
		t.Errorf("Pane['activity'] has %q, which no hub.Activity constant produces; add it to internal/hub/types.go or to frontendOnly here", a)
	}
}

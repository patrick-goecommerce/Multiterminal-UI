package hub

import (
	"encoding/json"
	"os"
	"testing"
)

// fixturePRs loads the measured gh response and lets a test change single
// fields of the first entry before it is marshalled again.
func fixturePRs(t *testing.T, edit func(pr map[string]any)) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/gh_pr_list_214.json")
	if err != nil {
		t.Fatal(err)
	}
	if edit == nil {
		return raw
	}
	var prs []map[string]any
	if err := json.Unmarshal(raw, &prs); err != nil {
		t.Fatal(err)
	}
	edit(prs[0])
	out, err := json.Marshal(prs)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func rollup(pr map[string]any) []any { return pr["statusCheckRollup"].([]any) }

func setCheck(pr map[string]any, i int, kv map[string]any) {
	c := rollup(pr)[i].(map[string]any)
	for k, v := range kv {
		c[k] = v
	}
}

func addContext(pr map[string]any, state string) {
	pr["statusCheckRollup"] = append(rollup(pr),
		map[string]any{"__typename": "StatusContext", "context": "ci/legacy", "state": state})
}

func TestPickPR_Fixture(t *testing.T) {
	pr, ok, err := pickPR(fixturePRs(t, nil), "patrick-goecommerce")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	want := PRFacts{
		Number: 214, State: "MERGED", Base: "alpha-main",
		HeadOID: "437bedd24c5d77058f04e9b7d126a2e75ca852ac",
		URL:     "https://github.com/patrick-goecommerce/Multiterminal-UI/pull/214",
		Checks:  "green", ChecksTotal: 2, ChecksPassed: 2,
	}
	if pr != want {
		t.Fatalf("got %+v\nwant %+v", pr, want)
	}
}

func TestPickPR_OwnerCaseInsensitive(t *testing.T) {
	if _, ok, _ := pickPR(fixturePRs(t, nil), "Patrick-GoEcommerce"); !ok {
		t.Fatal("owner match must ignore case")
	}
}

func TestPickPR_OtherOwnerNotFound(t *testing.T) {
	_, ok, err := pickPR(fixturePRs(t, nil), "someone-else")
	if err != nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestPickPR_EmptyAndBadInput(t *testing.T) {
	if _, ok, err := pickPR([]byte("[]"), "x"); ok || err != nil {
		t.Fatalf("empty list: ok=%v err=%v", ok, err)
	}
	if _, _, err := pickPR([]byte("not json"), "x"); err == nil {
		t.Fatal("want an error for invalid JSON")
	}
}

func TestCISignals_Checks(t *testing.T) {
	cases := []struct {
		name    string
		edit    func(pr map[string]any)
		checks  string
		failed  string
		passed  int
		red     bool
		blocked bool
	}{
		{"failure", func(pr map[string]any) { setCheck(pr, 0, map[string]any{"conclusion": "FAILURE"}) },
			"red", "backend", 1, true, false},
		{"in progress", func(pr map[string]any) {
			setCheck(pr, 0, map[string]any{"status": "IN_PROGRESS", "conclusion": ""})
		}, "pending", "", 1, false, false},
		{"timed out", func(pr map[string]any) { setCheck(pr, 1, map[string]any{"conclusion": "TIMED_OUT"}) },
			"red", "frontend", 1, true, false},
		{"cancelled", func(pr map[string]any) { setCheck(pr, 0, map[string]any{"conclusion": "CANCELLED"}) },
			"red", "backend", 1, true, false},
		{"action required", func(pr map[string]any) { setCheck(pr, 0, map[string]any{"conclusion": "ACTION_REQUIRED"}) },
			"red", "backend", 1, true, false},
		{"startup failure", func(pr map[string]any) { setCheck(pr, 0, map[string]any{"conclusion": "STARTUP_FAILURE"}) },
			"red", "backend", 1, true, false},
		{"skipped counts as passed", func(pr map[string]any) { setCheck(pr, 0, map[string]any{"conclusion": "SKIPPED"}) },
			"green", "", 2, false, false},
		{"status context failure", func(pr map[string]any) { addContext(pr, "FAILURE") },
			"red", "ci/legacy", 2, true, false},
		{"status context error", func(pr map[string]any) { addContext(pr, "ERROR") },
			"red", "ci/legacy", 2, true, false},
		{"status context pending", func(pr map[string]any) { addContext(pr, "PENDING") },
			"pending", "", 2, false, false},
		{"status context success", func(pr map[string]any) { addContext(pr, "SUCCESS") },
			"green", "", 3, false, false},
		{"red beats pending", func(pr map[string]any) {
			setCheck(pr, 0, map[string]any{"conclusion": "FAILURE"})
			setCheck(pr, 1, map[string]any{"status": "QUEUED", "conclusion": ""})
		}, "red", "backend", 0, true, false},
		{"conflicting", func(pr map[string]any) { pr["mergeable"] = "CONFLICTING" },
			"green", "", 2, false, true},
		{"changes requested", func(pr map[string]any) { pr["reviewDecision"] = "CHANGES_REQUESTED" },
			"green", "", 2, false, true},
		{"empty rollup", func(pr map[string]any) { pr["statusCheckRollup"] = []any{} },
			"none", "", 0, false, false},
		{"null rollup", func(pr map[string]any) { pr["statusCheckRollup"] = nil },
			"none", "", 0, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			pr, ok, err := pickPR(fixturePRs(t, c.edit), "patrick-goecommerce")
			if err != nil || !ok {
				t.Fatalf("ok=%v err=%v", ok, err)
			}
			if pr.Checks != c.checks || pr.FailedCheck != c.failed || pr.ChecksPassed != c.passed {
				t.Fatalf("checks=%q failed=%q passed=%d, want %q %q %d",
					pr.Checks, pr.FailedCheck, pr.ChecksPassed, c.checks, c.failed, c.passed)
			}
			red, blocked := ciSignals(pr)
			if red != c.red || blocked != c.blocked {
				t.Fatalf("ciSignals = %v,%v want %v,%v", red, blocked, c.red, c.blocked)
			}
		})
	}
}

func TestPickPR_ConflictingFlag(t *testing.T) {
	pr, _, _ := pickPR(fixturePRs(t, func(pr map[string]any) { pr["mergeable"] = "CONFLICTING" }), "patrick-goecommerce")
	if !pr.Conflicting {
		t.Fatal("Conflicting must follow mergeable")
	}
}

func TestPickPR_OpenBeforeClosed(t *testing.T) {
	raw := []byte(`[
	 {"number":12,"state":"CLOSED","headRepositoryOwner":{"login":"me"}},
	 {"number":10,"state":"CLOSED","headRepositoryOwner":{"login":"me"}},
	 {"number":11,"state":"OPEN","headRepositoryOwner":{"login":"me"}},
	 {"number":13,"state":"OPEN","headRepositoryOwner":{"login":"other"}}]`)
	pr, ok, err := pickPR(raw, "me")
	if err != nil || !ok || pr.Number != 11 {
		t.Fatalf("got %+v ok=%v err=%v", pr, ok, err)
	}
	raw = []byte(`[{"number":10,"state":"CLOSED","headRepositoryOwner":{"login":"me"}},{"number":12,"state":"MERGED","headRepositoryOwner":{"login":"me"}}]`)
	if pr, _, _ := pickPR(raw, "me"); pr.Number != 12 {
		t.Fatalf("without an open PR the highest number wins, got %d", pr.Number)
	}
}

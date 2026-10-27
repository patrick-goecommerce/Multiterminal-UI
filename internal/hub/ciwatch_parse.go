package hub

import (
	"encoding/json"
	"strings"
)

// ghPR is the part of one `gh pr list --json` entry the watcher reads.
type ghPR struct {
	Number              int    `json:"number"`
	URL                 string `json:"url"`
	State               string `json:"state"`
	IsDraft             bool   `json:"isDraft"`
	BaseRefName         string `json:"baseRefName"`
	HeadRefOid          string `json:"headRefOid"`
	Mergeable           string `json:"mergeable"`
	ReviewDecision      string `json:"reviewDecision"`
	HeadRepositoryOwner struct {
		Login string `json:"login"`
	} `json:"headRepositoryOwner"`
	StatusCheckRollup []ghCheck `json:"statusCheckRollup"`
}

// ghCheck is one rollup entry: a CheckRun (name, status, conclusion) or a
// StatusContext (context, state).
type ghCheck struct {
	Typename   string `json:"__typename"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Context    string `json:"context"`
	State      string `json:"state"`
}

// pickPR picks the pull request of the branch from the output of
// `gh pr list --head <branch> --state all`. Only PRs whose head repository
// belongs to owner count (a fork's branch of the same name is not ours). An
// open one wins over the others; among equals the highest number wins.
func pickPR(raw []byte, owner string) (PRFacts, bool, error) {
	var prs []ghPR
	if err := json.Unmarshal(raw, &prs); err != nil {
		return PRFacts{}, false, err
	}
	var best *ghPR
	for i := range prs {
		p := &prs[i]
		if !strings.EqualFold(p.HeadRepositoryOwner.Login, owner) {
			continue
		}
		if best == nil || prBetter(p, best) {
			best = p
		}
	}
	if best == nil {
		return PRFacts{}, false, nil
	}
	return best.facts(), true, nil
}

func prBetter(a, b *ghPR) bool {
	aOpen, bOpen := a.State == "OPEN", b.State == "OPEN"
	if aOpen != bOpen {
		return aOpen
	}
	return a.Number > b.Number
}

func (p *ghPR) facts() PRFacts {
	f := PRFacts{
		Number: p.Number, URL: p.URL, State: p.State, Base: p.BaseRefName,
		HeadOID: p.HeadRefOid, Review: p.ReviewDecision, Draft: p.IsDraft,
		Conflicting: p.Mergeable == "CONFLICTING",
	}
	f.Checks, f.FailedCheck, f.ChecksTotal, f.ChecksPassed = summarizeChecks(p.StatusCheckRollup)
	return f
}

// Check outcomes.
const (
	checkPass = iota
	checkPending
	checkFail
)

// classify maps one rollup entry to an outcome and its display name. A
// StatusContext's states are GitHub's GraphQL StatusState (SUCCESS, PENDING,
// EXPECTED, FAILURE, ERROR).
func (c ghCheck) classify() (int, string) {
	if c.Typename == "StatusContext" {
		switch strings.ToUpper(c.State) {
		case "SUCCESS":
			return checkPass, c.Context
		case "FAILURE", "ERROR":
			return checkFail, c.Context
		}
		return checkPending, c.Context
	}
	if !strings.EqualFold(c.Status, "COMPLETED") {
		return checkPending, c.Name
	}
	switch strings.ToUpper(c.Conclusion) {
	case "FAILURE", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED", "STARTUP_FAILURE":
		return checkFail, c.Name
	}
	return checkPass, c.Name
}

// summarizeChecks reduces the rollup to none, pending, green or red. A failed
// check wins over a pending one; failed is the first failing check's name.
func summarizeChecks(rollup []ghCheck) (checks, failed string, total, passed int) {
	pending := false
	for _, c := range rollup {
		total++
		switch kind, name := c.classify(); kind {
		case checkPass:
			passed++
		case checkPending:
			pending = true
		case checkFail:
			if failed == "" {
				failed = name
			}
		}
	}
	switch {
	case total == 0:
		return "none", "", 0, 0
	case failed != "":
		return "red", failed, total, passed
	case pending:
		return "pending", "", total, passed
	}
	return "green", "", total, passed
}

// ciSignals says whether the checks failed and whether the PR is blocked by a
// merge conflict or requested changes.
func ciSignals(pr PRFacts) (red, blocked bool) {
	return pr.Checks == "red", pr.Conflicting || pr.Review == "CHANGES_REQUESTED"
}

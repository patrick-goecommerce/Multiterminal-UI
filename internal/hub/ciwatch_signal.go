package hub

import (
	"sort"
	"strconv"
)

// What the CI watcher raises a session with, and the small helpers of its
// state. Everything here runs under ciWatch.mu or touches no shared state.

// newSignalLocked returns the detail to raise with when the PR shows a signal
// not seen before for its head commit; empty otherwise. Red wins over blocked.
func (c *ciWatch) newSignalLocked(key ciKey, pr PRFacts) string {
	red, blocked := ciSignals(pr)
	first := func(signal string) bool {
		k := key.Owner + "/" + key.Name + "/" + key.Branch + "|" + pr.HeadOID + "|" + signal
		if c.raised[k] {
			return false
		}
		c.raised[k] = true
		return true
	}
	newRed := red && first("red")
	newBlocked := blocked && first("blocked")
	switch {
	case newRed:
		return "CI rot: " + pr.FailedCheck
	case newBlocked && pr.Conflicting:
		return "PR #" + strconv.Itoa(pr.Number) + ": Konflikte"
	case newBlocked:
		return "PR #" + strconv.Itoa(pr.Number) + ": Änderungen angefordert"
	}
	return ""
}

func copyPR(p *PRFacts) *PRFacts {
	if p == nil {
		return nil
	}
	c := *p
	return &c
}

func sortedIDs(m map[int]bool) []int {
	ids := make([]int, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func lessKey(a, b ciKey) bool {
	if a.Owner != b.Owner {
		return a.Owner < b.Owner
	}
	if a.Name != b.Name {
		return a.Name < b.Name
	}
	return a.Branch < b.Branch
}

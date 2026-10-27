package hub

import (
	"sort"
	"strconv"
	"sync"
	"time"
)

// The CI and PR watcher (spec Teil C). It asks GitHub, through gh, about the
// branches the sessions are on, puts what it learns into their briefings and
// brings a session back to the waiting list when CI turns red or the PR gets
// blocked. It runs on the host because that happens while no window is open.
// It only reads.
//
// Locking: ciWatch.mu guards the watch state only. gh never runs under it,
// and it is never held together with the briefing desk's or the attention
// book's lock.

// ciPause is how long a key rests after gh failed (rate limit, network).
const ciPause = 15 * time.Minute

// ciKey names a branch of a GitHub repository. Sessions on the same key share
// one query.
type ciKey struct {
	Owner, Name, Branch string
}

// ciKeyOf is the key of a build's result; false when the repository is not on
// GitHub or HEAD is detached.
func ciKeyOf(res gitResult) (ciKey, bool) {
	k := ciKey{Owner: res.Repo.Owner, Name: res.Repo.Name, Branch: res.Facts.Branch}
	return k, k.Owner != "" && k.Name != "" && k.Branch != ""
}

// Login state of gh, checked once per host start.
const (
	authUnknown = iota
	authOK
	authNone
)

// ciTarget is one watched key.
type ciTarget struct {
	dir          string
	ids          map[int]bool
	fresh        map[int]bool // touched since the last poll
	due          time.Time
	paused       bool
	pendingSince time.Time
	pr           *PRFacts // last PR found
}

type ciWatch struct {
	gh   GHRunner
	now  func() time.Time
	wake chan struct{}

	mu     sync.Mutex
	auth   int
	keys   map[ciKey]*ciTarget
	byID   map[int]ciKey
	raised map[string]bool // owner/name/branch|head|signal already raised
}

func newCIWatch(gh GHRunner, now func() time.Time) *ciWatch {
	return &ciWatch{
		gh: gh, now: now, wake: make(chan struct{}, 1),
		keys: map[ciKey]*ciTarget{}, byID: map[int]ciKey{}, raised: map[string]bool{},
	}
}

// touch registers the session on key and makes the key due now. A paused key
// stays paused; the session gets what is known at the next pass.
func (c *ciWatch) touch(id int, key ciKey, dir string) {
	c.mu.Lock()
	if old, ok := c.byID[id]; ok && old != key {
		c.removeLocked(id, old)
	}
	t := c.keys[key]
	if t == nil {
		t = &ciTarget{ids: map[int]bool{}, fresh: map[int]bool{}}
		c.keys[key] = t
	}
	t.ids[id], t.fresh[id] = true, true
	c.byID[id] = key
	if dir != "" {
		t.dir = dir
	}
	if !t.paused {
		t.due = c.now()
	}
	c.mu.Unlock()
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// drop stops watching for the session. Nil-safe: a host without a watcher
// calls it on every close.
func (c *ciWatch) drop(id int) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if key, ok := c.byID[id]; ok {
		c.removeLocked(id, key)
	}
}

func (c *ciWatch) removeLocked(id int, key ciKey) {
	delete(c.byID, id)
	if t := c.keys[key]; t != nil {
		delete(t.ids, id)
		delete(t.fresh, id)
		if len(t.ids) == 0 {
			delete(c.keys, key)
		}
	}
}

// base is the base branch of the key's PR, if one is known.
func (c *ciWatch) base(key ciKey) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if t := c.keys[key]; t != nil && t.pr != nil {
		return t.pr.Base
	}
	return ""
}

// ciJob is a key to ask gh about.
type ciJob struct {
	key ciKey
	dir string
}

// ciOutcome is what the sessions of a key get after a poll or a refill.
type ciOutcome struct {
	ids    []int
	pr     *PRFacts
	paused bool
	detail string // non-empty: raise the idle sessions with it
}

// collect returns the keys due now, and for paused keys the sessions touched
// since, which get the known state without asking gh.
func (c *ciWatch) collect(now time.Time) (due []ciJob, refill []ciOutcome) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key, t := range c.keys {
		if !now.Before(t.due) {
			due = append(due, ciJob{key: key, dir: t.dir})
			continue
		}
		if t.paused && len(t.fresh) > 0 {
			refill = append(refill, ciOutcome{ids: sortedIDs(t.fresh), pr: copyPR(t.pr), paused: true})
			t.fresh = map[int]bool{}
		}
	}
	sort.Slice(due, func(i, j int) bool { return lessKey(due[i].key, due[j].key) })
	return due, refill
}

// authState returns the remembered login state.
func (c *ciWatch) authState() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.auth
}

func (c *ciWatch) setAuth(ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.auth = authNone
	if ok {
		c.auth = authOK
	}
}

// record files a poll's result for key and works out what its sessions get.
// A failed poll pauses the key and makes the login unknown again; no PR, or a
// merged or closed one, ends the watch of the key.
func (c *ciWatch) record(key ciKey, pr PRFacts, found bool, err error, now time.Time) ciOutcome {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := c.keys[key]
	if t == nil {
		return ciOutcome{} // its last session went away meanwhile
	}
	out := ciOutcome{ids: sortedIDs(t.ids)}
	t.fresh = map[int]bool{}
	if err != nil {
		c.auth = authUnknown
		t.paused, t.due = true, now.Add(ciPause)
		out.pr, out.paused = copyPR(t.pr), true
		return out
	}
	t.paused, t.pr = false, nil
	if found {
		t.pr, out.pr = copyPR(&pr), copyPR(&pr)
	}
	if found && pr.Checks == "pending" {
		if t.pendingSince.IsZero() {
			t.pendingSince = now
		}
	} else {
		t.pendingSince = time.Time{}
	}
	next, keep := nextPoll(pr, found, t.pendingSince, now)
	if !keep {
		for id := range t.ids {
			delete(c.byID, id)
		}
		delete(c.keys, key)
	} else {
		t.due = now.Add(next)
	}
	if found && pr.State == "OPEN" {
		out.detail = c.newSignalLocked(key, pr)
	}
	return out
}

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

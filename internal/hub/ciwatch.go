package hub

import (
	"sort"
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

// Login state of gh. A failed check holds for ciPause, then gh is asked
// again; a failed poll makes the state unknown again.
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

	mu        sync.Mutex
	auth      int
	authRetry time.Time // with authNone: when to check the login again
	keys      map[ciKey]*ciTarget
	byID      map[int]ciKey
	raised    map[string]bool // owner/name/branch|head|signal already raised
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

// known is the key's last PR found, if any.
func (c *ciWatch) known(key ciKey) *PRFacts {
	c.mu.Lock()
	defer c.mu.Unlock()
	if t := c.keys[key]; t != nil {
		return copyPR(t.pr)
	}
	return nil
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
	noPR   bool   // gh answered and knows no PR for the key
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

// authState returns the remembered login state; a failed check whose pause
// is over reads as unknown, so the login is checked again.
func (c *ciWatch) authState(now time.Time) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.auth == authNone && !now.Before(c.authRetry) {
		return authUnknown
	}
	return c.auth
}

// settleAuth files the result of a login check. A failed check cannot tell
// "not logged in" from an outage, so it never holds for good: the watcher
// asks gh nothing for ciPause and then checks the login again. The keys stay
// due and are polled as soon as a check passes.
func (c *ciWatch) settleAuth(ok bool, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ok {
		c.auth = authOK
		return
	}
	c.auth, c.authRetry = authNone, now.Add(ciPause)
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
	out.noPR = !found
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

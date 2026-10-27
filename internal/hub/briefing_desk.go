package hub

import (
	"context"
	"sync"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/gitx"
)

// The briefing desk keeps what the host knows about each session's last
// finished turn. It has its own lock: builds run in goroutines, and none of
// them may ever hold Embedded.mu while git runs.

const (
	// briefingLimit bounds all git calls of one build.
	briefingLimit = 5 * time.Second
	// snapshotLimit bounds the snapshot taken when a turn starts.
	snapshotLimit = 5 * time.Second
	// maxBuilds is how many builds run at once across all sessions.
	maxBuilds = 4
)

// deskEntry is a session's last published briefing, with what it was derived
// from, so a later PR update can recompute the warnings.
type deskEntry struct {
	briefing   Briefing
	endedError bool
	bigDiff    bool
	res        gitResult
}

// deskBuild is the build currently running for a session. A newer one, or
// forgetting the session, cancels it; gen tells the two apart afterwards.
type deskBuild struct {
	gen    uint64
	cancel context.CancelFunc
}

type briefingDesk struct {
	turns    *turnBook
	policy   func() BriefingPolicy // nil: defaults, CI watch off
	snapshot snapshotFunc          // a seam for tests
	limit    time.Duration
	sem      chan struct{}
	// afterBuild sees every published build. Nil until the CI watch uses it.
	afterBuild func(id int, res gitResult, publish bool)

	mu      sync.Mutex
	entries map[int]deskEntry
	targets map[int]string
	builds  map[int]deskBuild
	gen     uint64
}

func newBriefingDesk(policy func() BriefingPolicy) *briefingDesk {
	return &briefingDesk{
		turns:    newTurnBook(),
		policy:   policy,
		snapshot: gitx.SnapshotTree,
		limit:    briefingLimit,
		sem:      make(chan struct{}, maxBuilds),
		entries:  map[int]deskEntry{},
		targets:  map[int]string{},
		builds:   map[int]deskBuild{},
	}
}

// currentPolicy reads the policy now, with unset thresholds defaulted.
func (d *briefingDesk) currentPolicy() BriefingPolicy {
	if d.policy == nil {
		return BriefingPolicy{}.normalized()
	}
	return d.policy().normalized()
}

// startSnapshot records the work tree at the start of a turn. A snapshot that
// fails or lands after the turn ended leaves the turn without numbers.
func (d *briefingDesk) startSnapshot(id int, promptID, dir string) {
	ctx, cancel := context.WithTimeout(context.Background(), snapshotLimit)
	defer cancel()
	if tree, err := d.snapshot(ctx, dir); err == nil {
		d.turns.setStartTree(id, promptID, tree)
	}
}

// begin registers a new build for a session, cancelling the one still
// running, and returns its context and generation.
func (d *briefingDesk) begin(id int) (context.Context, uint64) {
	ctx, cancel := context.WithCancel(context.Background())
	d.mu.Lock()
	defer d.mu.Unlock()
	if b, ok := d.builds[id]; ok {
		b.cancel()
	}
	d.gen++
	d.builds[id] = deskBuild{gen: d.gen, cancel: cancel}
	return ctx, d.gen
}

// finish ends a build. If it is still the session's current one, e (when not
// nil) is stored and the result is true.
func (d *briefingDesk) finish(id int, gen uint64, e *deskEntry) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	b, ok := d.builds[id]
	if !ok || b.gen != gen {
		return false
	}
	b.cancel()
	delete(d.builds, id)
	if e == nil {
		return false
	}
	d.entries[id] = *e
	return true
}

func (d *briefingDesk) target(id int) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.targets[id]
}

func (d *briefingDesk) setTarget(id int, branch string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if branch == "" {
		delete(d.targets, id)
		return
	}
	d.targets[id] = branch
}

// get returns a copy of the session's briefing. Stored slices are never
// changed in place, so the copy is made outside the lock.
func (d *briefingDesk) get(id int) (Briefing, bool) {
	d.mu.Lock()
	e, ok := d.entries[id]
	d.mu.Unlock()
	if !ok {
		return Briefing{}, false
	}
	return e.briefing.clone(), true
}

// updatePR replaces what is known about the session's PR and its CI notes,
// keeps the build's own notes, and recomputes the warnings.
func (d *briefingDesk) updatePR(id int, pr *PRFacts, notes []string) (Briefing, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	e, ok := d.entries[id]
	if !ok {
		return Briefing{}, false
	}
	b := e.briefing.clone()
	b.PR = nil
	if pr != nil {
		p := *pr
		b.PR = &p
	}
	kept := []string{}
	for _, n := range b.Notes {
		if n != NoteCIPaused && n != NoteCIStale {
			kept = append(kept, n)
		}
	}
	b.Notes = append(kept, notes...)
	b.Warnings = warningsFor(b, e.endedError, e.bigDiff)
	e.briefing = b.clone()
	d.entries[id] = e
	return e.briefing.clone(), true
}

// forget drops everything about a session and cancels its build.
func (d *briefingDesk) forget(id int) {
	d.turns.forget(id)
	d.mu.Lock()
	defer d.mu.Unlock()
	if b, ok := d.builds[id]; ok {
		b.cancel()
		delete(d.builds, id)
	}
	delete(d.entries, id)
	delete(d.targets, id)
}

// clone copies a briefing deeply. Its slices come out empty, never nil, so a
// published briefing always reads [] in JSON.
func (b Briefing) clone() Briefing {
	b.Checks = append([]CheckResult{}, b.Checks...)
	b.Warnings = append([]string{}, b.Warnings...)
	b.Notes = append([]string{}, b.Notes...)
	if b.Git != nil {
		g := *b.Git
		b.Git = &g
	}
	if b.PR != nil {
		p := *b.PR
		b.PR = &p
	}
	return b
}

package hub

import (
	"context"
	"log"
	"reflect"
	"time"
)

// The host's side of the CI watcher: registering a session's branch after a
// build, the loop that asks gh, and handing the answers to the briefing desk
// and the attention book. Each of those is called in turn, never nested, and
// gh runs under no lock at all.

// ciTick is how often the loop looks for due keys. A touch wakes it at once.
const ciTick = 15 * time.Second

// ghPRFields are the fields of `gh pr list --json` the watcher reads.
const ghPRFields = "number,state,isDraft,baseRefName,headRefOid,headRepositoryOwner," +
	"mergeable,reviewDecision,statusCheckRollup,url"

// startCIWatch turns the watcher on and starts its loop.
func (h *Embedded) startCIWatch(gh GHRunner) {
	h.attachCI(newCIWatch(gh, time.Now))
	go h.ciLoop()
}

// attachCI wires a watcher into the host without starting the loop, which is
// what a test drives by hand.
func (h *Embedded) attachCI(c *ciWatch) {
	h.ci = c
	h.desk.afterBuild = h.ciAfterBuild
	h.desk.prBase = func(r ghRepo, branch string) string {
		return c.base(ciKey{Owner: r.Owner, Name: r.Name, Branch: branch})
	}
	h.desk.prKnown = func(r ghRepo, branch string) *PRFacts {
		return c.known(ciKey{Owner: r.Owner, Name: r.Name, Branch: branch})
	}
}

// ciAfterBuild registers the branch a build found the session on and asks
// about it at once. A session no longer on a GitHub branch is not watched.
func (h *Embedded) ciAfterBuild(id int, res gitResult, _ bool) {
	key, ok := ciKeyOf(res)
	if !ok {
		h.ci.drop(id)
		return
	}
	h.ci.touch(id, key, h.sessionDir(id))
}

// ciPublished asks again at once after the agent pushed or created or merged
// a PR, for the branch the session's last build found.
func (h *Embedded) ciPublished(id int) {
	if h.ci == nil {
		return
	}
	if res, ok := h.desk.result(id); ok {
		h.ciAfterBuild(id, res, true)
	}
}

// ciLoop polls due keys for as long as the host lives.
func (h *Embedded) ciLoop() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-h.stop:
			cancel() // ends a gh call in flight
		case <-ctx.Done():
		}
	}()
	ticker := time.NewTicker(ciTick)
	defer ticker.Stop()
	for {
		select {
		case <-h.stop:
			return
		case <-ticker.C:
		case <-h.ci.wake:
		}
		h.ciPass(ctx)
	}
}

// ciPass asks gh about every due key, one after the other.
func (h *Embedded) ciPass(ctx context.Context) {
	if !h.desk.currentPolicy().CIWatch {
		return
	}
	c := h.ci
	due, refill := c.collect(c.now())
	for _, o := range refill {
		h.ciApply(o, c.now())
	}
	if len(due) == 0 || !h.ciAuthorized(ctx) {
		return
	}
	for _, j := range due {
		raw, err := c.gh(ctx, j.dir, "pr", "list", "--repo", j.key.Owner+"/"+j.key.Name,
			"--head", j.key.Branch, "--state", "all", "--limit", "5", "--json", ghPRFields)
		if ctx.Err() != nil {
			return
		}
		var pr PRFacts
		var found bool
		if err == nil {
			pr, found, err = pickPR(raw, j.key.Owner)
		}
		if err != nil {
			log.Printf("[hub] ci watch: %s/%s %s: %v; pausing %v", j.key.Owner, j.key.Name, j.key.Branch, err, ciPause)
		}
		now := c.now()
		h.ciApply(c.record(j.key, pr, found, err, now), now)
	}
}

// ciAuthorized checks the gh login for github.com and remembers it; again
// after a failed poll, and ciPause after a failed check. Without a login
// there is no PR line, and that is not an error. --hostname keeps a second gh
// host with an expired token from failing the check.
func (h *Embedded) ciAuthorized(ctx context.Context) bool {
	c := h.ci
	switch c.authState(c.now()) {
	case authOK:
		return true
	case authNone:
		return false
	}
	_, err := c.gh(ctx, "", "auth", "status", "--hostname", "github.com")
	if ctx.Err() != nil {
		return false
	}
	if err != nil {
		log.Printf("[hub] ci watch: gh auth status: %v", err)
	}
	c.settleAuth(err == nil, c.now())
	return err == nil
}

// ciApply puts a key's PR into the briefing of each of its sessions, tells
// the clients when that changed it, and raises the idle ones when the outcome
// says so.
func (h *Embedded) ciApply(o ciOutcome, now time.Time) {
	for _, id := range o.ids {
		if _, err := h.lookup(id); err != nil {
			continue
		}
		res, _ := h.desk.result(id)
		var notes []string
		if o.paused {
			notes = append(notes, NoteCIPaused)
		}
		if o.noPR {
			notes = append(notes, NoteNoPR)
		}
		if ciStale(o.pr, res.Head) {
			notes = append(notes, NoteCIStale)
		}
		before, had := h.desk.get(id)
		if after, ok := h.desk.updatePR(id, o.pr, notes); ok && (!had || !reflect.DeepEqual(before, after)) {
			h.emit(EventSessionBriefing, BriefingUpdate{ID: id, Briefing: after})
		}
		if o.detail != "" {
			h.raiseFor(id, "ci", o.detail, now)
		}
	}
}

// ciStale reports a PR whose head is not the commit the build saw.
func ciStale(pr *PRFacts, head string) bool {
	return pr != nil && pr.HeadOID != "" && head != "" && head != pr.HeadOID
}

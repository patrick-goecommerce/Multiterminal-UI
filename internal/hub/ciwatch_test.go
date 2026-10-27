package hub

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

const (
	ciOwner = "patrick-goecommerce"
	ciName  = "Multiterminal-UI"
	ciHead  = "437bedd24c5d77058f04e9b7d126a2e75ca852ac" // headRefOid of the fixture
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// fakeGH answers `gh auth status` with authErr and the n-th `gh pr list`
// (1-based) with prList(n).
type fakeGH struct {
	mu      sync.Mutex
	calls   []string
	prCalls int
	authErr error
	prList  func(n int) ([]byte, error)
}

func (f *fakeGH) run(_ context.Context, _ string, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, strings.Join(args, " "))
	if len(args) > 0 && args[0] == "auth" {
		return nil, f.authErr
	}
	f.prCalls++
	return f.prList(f.prCalls)
}

// count returns how many calls started with prefix ("auth status", "pr list").
func (f *fakeGH) count(prefix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
}

// serve answers every `gh pr list` with the fixtures in turn, repeating the
// last one.
func serve(fixtures ...[]byte) func(int) ([]byte, error) {
	return func(n int) ([]byte, error) {
		if n > len(fixtures) {
			n = len(fixtures)
		}
		return fixtures[n-1], nil
	}
}

type prMod func(pr map[string]any)

// prFixture is the measured response for #214, turned into an open,
// mergeable PR with green checks, then changed by mods.
func prFixture(t *testing.T, mods ...prMod) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/gh_pr_list_214.json")
	if err != nil {
		t.Fatal(err)
	}
	var prs []map[string]any
	if err := json.Unmarshal(raw, &prs); err != nil {
		t.Fatal(err)
	}
	prs[0]["state"] = "OPEN"
	prs[0]["mergeable"] = "MERGEABLE"
	for _, m := range mods {
		m(prs[0])
	}
	out, err := json.Marshal(prs)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func firstCheck(pr map[string]any) map[string]any {
	return pr["statusCheckRollup"].([]any)[0].(map[string]any) // "backend"
}

func pending(pr map[string]any) {
	c := firstCheck(pr)
	c["status"], c["conclusion"] = "IN_PROGRESS", ""
}

func red(pr map[string]any) { firstCheck(pr)["conclusion"] = "FAILURE" }

func merged(pr map[string]any) { pr["state"], pr["mergeable"] = "MERGED", "UNKNOWN" }

func headOID(oid string) prMod { return func(pr map[string]any) { pr["headRefOid"] = oid } }

func field(k, v string) prMod { return func(pr map[string]any) { pr[k] = v } }

// ciSink keeps the briefing and attention updates.
type ciSink struct {
	mu        sync.Mutex
	briefings []BriefingUpdate
	attention []AttentionUpdate
	ch        chan BriefingUpdate
}

func newCISink() *ciSink { return &ciSink{ch: make(chan BriefingUpdate, 64)} }

func (s *ciSink) Emit(name string, payload any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch name {
	case EventSessionBriefing:
		if u, ok := DecodePayload[BriefingUpdate](payload); ok {
			s.briefings = append(s.briefings, u)
			select {
			case s.ch <- u:
			default:
			}
		}
	case EventSessionAttention:
		if u, ok := DecodePayload[AttentionUpdate](payload); ok {
			s.attention = append(s.attention, u)
		}
	}
}

func (s *ciSink) raised() []AttentionUpdate {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]AttentionUpdate(nil), s.attention...)
}

// lastBriefing returns the latest briefing published for id.
func (s *ciSink) lastBriefing(t *testing.T, id int) Briefing {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.briefings) - 1; i >= 0; i-- {
		if s.briefings[i].ID == id {
			return s.briefings[i].Briefing
		}
	}
	t.Fatalf("no briefing published for session %d", id)
	return Briefing{}
}

// ciHost is a host with a CI watcher on a fake runner and a fake clock. The
// loop does not run: the test drives it with poll.
func ciHost(t *testing.T, gh *fakeGH, clock *fakeClock, watch bool) (*Embedded, *ciSink, func()) {
	t.Helper()
	sink := newCISink()
	h := NewEmbedded(Options{Version: "test", Sink: sink, Briefing: func() BriefingPolicy {
		return BriefingPolicy{CIWatch: watch}
	}})
	t.Cleanup(h.Release)
	h.attachCI(newCIWatch(gh.run, clock.now))
	return h, sink, func() { h.ciPass(context.Background()) }
}

// seedSession adds a session whose last build found it on branch at head of
// the GitHub repo, and hands that build to the watcher as a build would.
func seedSession(t *testing.T, h *Embedded, id int, branch, head string) {
	t.Helper()
	h.AdoptForTest(id, terminal.NewSession(id, 24, 80))
	seedBuild(h, id, branch, head)
}

// seedBuild stores a fresh build's briefing (no PR yet) and hands it to the
// watcher, as buildBriefing does.
func seedBuild(h *Embedded, id int, branch, head string) {
	res := gitResult{
		Facts: GitFacts{Branch: branch, Target: "main"},
		Repo:  ghRepo{Owner: ciOwner, Name: ciName},
		Head:  head,
	}
	b := Briefing{PromptID: "p1", Prompt: "mach", Git: &res.Facts}.clone()
	h.desk.mu.Lock()
	h.desk.entries[id] = deskEntry{briefing: b, res: res}
	h.desk.mu.Unlock()
	h.desk.afterBuild(id, res, false)
}

func TestPendingThenRedRaisesOnce(t *testing.T) {
	gh := &fakeGH{prList: serve(prFixture(t, pending), prFixture(t, red), prFixture(t, red))}
	clock := newFakeClock()
	h, sink, poll := ciHost(t, gh, clock, true)
	seedSession(t, h, 1, "feature-x", ciHead)

	poll()
	if n := gh.count("pr list"); n != 1 {
		t.Fatalf("pr list calls = %d, want 1", n)
	}
	if got := sink.lastBriefing(t, 1); got.PR == nil || got.PR.Checks != "pending" {
		t.Fatalf("briefing PR = %+v, want pending checks", got.PR)
	}
	if r := sink.raised(); len(r) != 0 {
		t.Fatalf("pending raised %+v", r)
	}

	clock.advance(time.Minute)
	poll()
	r := sink.raised()
	if len(r) != 1 {
		t.Fatalf("raises after red = %d, want 1: %+v", len(r), r)
	}
	want := Attention{State: ActivityError, Since: clock.now(), Seen: false, Source: "ci", Detail: "CI rot: backend", Rank: RankError}
	if r[0].ID != 1 || !r[0].Attention.Since.Equal(want.Since) {
		t.Errorf("raise = %+v, want id 1 since %v", r[0], want.Since)
	}
	r[0].Attention.Since = want.Since
	if r[0].Attention != want {
		t.Errorf("raise = %+v, want %+v", r[0].Attention, want)
	}

	clock.advance(ciPollSlow)
	poll()
	if n := gh.count("pr list"); n != 3 {
		t.Fatalf("pr list calls = %d, want 3", n)
	}
	if r := sink.raised(); len(r) != 1 {
		t.Errorf("still red on the same head raised again: %+v", r)
	}
}

func TestNewHeadRedRaisesAgain(t *testing.T) {
	const newHead = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	gh := &fakeGH{prList: serve(prFixture(t, red), prFixture(t, red, headOID(newHead)))}
	clock := newFakeClock()
	h, sink, poll := ciHost(t, gh, clock, true)
	seedSession(t, h, 1, "feature-x", ciHead)

	poll()
	if len(sink.raised()) != 1 {
		t.Fatalf("first red raised %d times, want 1", len(sink.raised()))
	}
	clock.advance(ciPollSlow)
	poll()
	if r := sink.raised(); len(r) != 2 {
		t.Fatalf("red on a new head: raises = %d, want 2", len(r))
	}
	// The session's last build saw the older head.
	if b := sink.lastBriefing(t, 1); !slices.Contains(b.Notes, NoteCIStale) {
		t.Errorf("Notes = %v, want %s", b.Notes, NoteCIStale)
	}
}

func TestActiveSessionOnlyGetsBriefing(t *testing.T) {
	gh := &fakeGH{prList: serve(prFixture(t, red))}
	clock := newFakeClock()
	h, sink, poll := ciHost(t, gh, clock, true)
	seedSession(t, h, 1, "feature-x", ciHead)
	seedSession(t, h, 2, "feature-x", ciHead)
	h.attention.confirmed(1, ActivityActive, clock.now())
	h.attention.confirmed(2, ActivityDone, clock.now())
	h.attention.markSeen(2, true)

	poll()
	if n := gh.count("pr list"); n != 1 {
		t.Errorf("two sessions on one branch: pr list calls = %d, want 1", n)
	}
	r := sink.raised()
	if len(r) != 1 || r[0].ID != 2 {
		t.Fatalf("raises = %+v, want only session 2", r)
	}
	if a := h.attention.get(1); a.State != ActivityActive {
		t.Errorf("active session's record = %+v", a)
	}
	for _, id := range []int{1, 2} {
		b := sink.lastBriefing(t, id)
		if b.PR == nil || b.PR.Checks != "red" || !slices.Contains(b.Warnings, WarnCIRed) {
			t.Errorf("session %d briefing PR = %+v, warnings %v", id, b.PR, b.Warnings)
		}
		if b.Prompt != "mach" {
			t.Errorf("session %d lost its turn: %+v", id, b)
		}
	}

	clock.advance(ciPollSlow)
	poll()
	if r := sink.raised(); len(r) != 1 {
		t.Errorf("second poll on the same red head raised: %+v", r)
	}
}

func TestBlockedByConflict(t *testing.T) {
	cases := []struct {
		name string
		mod  prMod
		want string
	}{
		{"conflict", field("mergeable", "CONFLICTING"), "PR #214: Konflikte"},
		{"changes requested", field("reviewDecision", "CHANGES_REQUESTED"), "PR #214: Änderungen angefordert"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gh := &fakeGH{prList: serve(prFixture(t, c.mod))}
			clock := newFakeClock()
			h, sink, poll := ciHost(t, gh, clock, true)
			seedSession(t, h, 1, "feature-x", ciHead)
			poll()
			r := sink.raised()
			if len(r) != 1 || r[0].Attention.Detail != c.want || r[0].Attention.Source != "ci" {
				t.Fatalf("raises = %+v, want one with detail %q", r, c.want)
			}
			if b := sink.lastBriefing(t, 1); !slices.Contains(b.Warnings, WarnPRBlocked) {
				t.Errorf("Warnings = %v, want %s", b.Warnings, WarnPRBlocked)
			}
		})
	}
}

func TestMergedStopsWatching(t *testing.T) {
	gh := &fakeGH{prList: serve(prFixture(t, red, merged))}
	clock := newFakeClock()
	h, sink, poll := ciHost(t, gh, clock, true)
	seedSession(t, h, 1, "feature-x", ciHead)

	poll()
	if b := sink.lastBriefing(t, 1); b.PR == nil || b.PR.State != "MERGED" {
		t.Fatalf("briefing PR = %+v, want the merged PR", b.PR)
	}
	clock.advance(time.Hour)
	poll()
	if n := gh.count("pr list"); n != 1 {
		t.Errorf("pr list calls after merge = %d, want 1", n)
	}
	if r := sink.raised(); len(r) != 0 {
		t.Errorf("a merged PR raised %+v", r)
	}
}

func TestGHErrorPauses(t *testing.T) {
	prev := log.Writer()
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(prev) })
	gh := &fakeGH{prList: func(int) ([]byte, error) { return nil, errors.New("HTTP 403: API rate limit exceeded") }}
	clock := newFakeClock()
	h, sink, poll := ciHost(t, gh, clock, true)
	seedSession(t, h, 1, "feature-x", ciHead)

	poll()
	if b := sink.lastBriefing(t, 1); !slices.Contains(b.Notes, NoteCIPaused) {
		t.Fatalf("Notes = %v, want %s", b.Notes, NoteCIPaused)
	}
	// A new turn during the pause does not ask gh, but its briefing gets the
	// note too.
	clock.advance(time.Minute)
	seedBuild(h, 1, "feature-x", ciHead)
	poll()
	if n := gh.count("pr list"); n != 1 {
		t.Fatalf("pr list calls after a build during the pause = %d, want 1", n)
	}
	if b, _ := h.desk.get(1); !slices.Contains(b.Notes, NoteCIPaused) {
		t.Fatalf("Notes after a build during the pause = %v, want %s", b.Notes, NoteCIPaused)
	}
	clock.advance(ciPause - time.Minute - time.Second)
	poll()
	if n := gh.count("pr list"); n != 1 {
		t.Fatalf("pr list calls during the pause = %d, want 1", n)
	}
	clock.advance(time.Second)
	poll()
	if n := gh.count("pr list"); n != 2 {
		t.Errorf("pr list calls after the pause = %d, want 2", n)
	}
	// The failure put the login back to unknown, so it was checked again.
	if n := gh.count("auth status"); n != 2 {
		t.Errorf("auth status calls = %d, want 2", n)
	}
}

func TestCIWatchOffNoCalls(t *testing.T) {
	gh := &fakeGH{prList: serve(prFixture(t, red))}
	clock := newFakeClock()
	h, sink, poll := ciHost(t, gh, clock, false)
	seedSession(t, h, 1, "feature-x", ciHead)

	poll()
	clock.advance(time.Hour)
	poll()
	if len(gh.calls) != 0 {
		t.Errorf("gh calls with the CI watch off: %v", gh.calls)
	}
	if r := sink.raised(); len(r) != 0 {
		t.Errorf("raised %+v", r)
	}
}

func TestAuthCheckedOnce(t *testing.T) {
	gh := &fakeGH{prList: serve(prFixture(t, pending))}
	clock := newFakeClock()
	h, _, poll := ciHost(t, gh, clock, true)
	seedSession(t, h, 1, "feature-x", ciHead)
	seedSession(t, h, 2, "feature-y", ciHead)

	poll()
	clock.advance(time.Minute)
	poll()
	if n := gh.count("auth status"); n != 1 {
		t.Errorf("auth status calls = %d, want 1", n)
	}
	if n := gh.count("pr list"); n != 4 {
		t.Errorf("pr list calls = %d, want 4 (two keys, two polls)", n)
	}
}

func TestNoLoginNoPRCalls(t *testing.T) {
	gh := &fakeGH{authErr: errors.New("not logged in"), prList: serve(prFixture(t, red))}
	clock := newFakeClock()
	h, sink, poll := ciHost(t, gh, clock, true)
	seedSession(t, h, 1, "feature-x", ciHead)

	poll()
	clock.advance(time.Hour)
	poll()
	if n := gh.count("pr list"); n != 0 {
		t.Errorf("pr list without a login: %d calls", n)
	}
	if n := gh.count("auth status"); n != 1 {
		t.Errorf("auth status calls = %d, want 1 (remembered)", n)
	}
	if b, _ := h.desk.get(1); b.PR != nil || len(b.Notes) != 0 {
		t.Errorf("briefing without a login = %+v", b)
	}
	if len(sink.raised()) != 0 {
		t.Error("raised without a login")
	}
}

// A repository that is not on GitHub has no owner: it is never asked about.
func TestNoOwnerNotWatched(t *testing.T) {
	gh := &fakeGH{prList: serve(prFixture(t, red))}
	clock := newFakeClock()
	h, _, poll := ciHost(t, gh, clock, true)
	h.AdoptForTest(1, terminal.NewSession(1, 24, 80))
	h.desk.afterBuild(1, gitResult{Facts: GitFacts{Branch: "feature-x"}}, false)
	poll()
	if len(gh.calls) != 0 {
		t.Errorf("gh calls for a session without a GitHub remote: %v", gh.calls)
	}
}

func TestCloseDropsSession(t *testing.T) {
	gh := &fakeGH{prList: serve(prFixture(t, pending))}
	clock := newFakeClock()
	h, _, poll := ciHost(t, gh, clock, true)
	seedSession(t, h, 1, "feature-x", ciHead)
	poll()
	if err := h.Close(1); err != nil {
		t.Fatal(err)
	}
	clock.advance(time.Hour)
	poll()
	if n := gh.count("pr list"); n != 1 {
		t.Errorf("pr list calls after the only session closed = %d, want 1", n)
	}
}

// A publish command seen during a turn asks again at once, even for a branch
// the watcher had stopped watching.
func TestPublishTouchesAtOnce(t *testing.T) {
	gh := &fakeGH{prList: serve(prFixture(t, merged), prFixture(t, pending))}
	clock := newFakeClock()
	h, _, poll := ciHost(t, gh, clock, true)
	seedSession(t, h, 1, "feature-x", ciHead)
	poll() // merged: stops watching
	bash(h, 1, "p2", "git push -u origin feature-x", false)
	poll()
	if n := gh.count("pr list"); n != 2 {
		t.Errorf("pr list calls after git push = %d, want 2", n)
	}
}

// End to end through the real loop: a finished turn registers its branch, the
// loop asks gh, the briefing gets its PR, and the next build takes the PR's
// base as its target.
func TestBuildWakesLoopAndUsesPRBase(t *testing.T) {
	gh := &fakeGH{prList: serve(prFixture(t, pending))}
	sink := newCISink()
	h := NewEmbedded(Options{Version: "test", Sink: sink, GH: gh.run, Briefing: func() BriefingPolicy {
		return BriefingPolicy{CIWatch: true}
	}})
	t.Cleanup(h.Release)
	h.AdoptForTest(1, terminal.NewSession(1, 24, 80))
	repo := newRepoWithOrigin(t)
	tgit(t, repo, "checkout", "-q", "-b", "feature-x")
	tgit(t, repo, "remote", "set-url", "origin", "https://github.com/"+ciOwner+"/"+ciName+".git")

	submit(h, 1, "p1", "mach", repo)
	stop(h, 1, "p1", "Fertig.")
	deadline := time.After(10 * time.Second)
	for {
		var u BriefingUpdate
		select {
		case u = <-sink.ch:
		case <-deadline:
			t.Fatal("no briefing with a PR within 10 s")
		}
		if u.Briefing.PR != nil {
			if u.Briefing.PR.Number != 214 || u.Briefing.PR.Base != "alpha-main" {
				t.Fatalf("PR = %+v", u.Briefing.PR)
			}
			break
		}
	}

	submit(h, 1, "p2", "weiter", repo)
	stop(h, 1, "p2", "Fertig.")
	select {
	case u := <-sink.ch:
		if u.Briefing.PromptID != "p2" || u.Briefing.Git == nil || u.Briefing.Git.Target != "alpha-main" {
			t.Errorf("second build = %+v, git %+v; want target alpha-main", u.Briefing, u.Briefing.Git)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no second briefing")
	}
}

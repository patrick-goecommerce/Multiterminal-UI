package hub

import "time"

const (
	ciPollFast     = time.Minute
	ciPollMedium   = 5 * time.Minute
	ciPollSlow     = 15 * time.Minute
	ciFastWindow   = 30 * time.Minute
	ciMediumWindow = 3 * time.Hour
)

// nextPoll says when the CI watcher asks GitHub again. false means stop: no
// PR was found, or it is merged or closed. Pending checks are polled every
// minute for 30 minutes, every 5 minutes up to 3 hours, then like an open PR
// with finished checks, every 15 minutes. pendingSince is when the checks
// first showed pending; zero means just now.
func nextPoll(pr PRFacts, found bool, pendingSince, now time.Time) (time.Duration, bool) {
	if !found || pr.State != "OPEN" {
		return 0, false
	}
	if pr.Checks != "pending" {
		return ciPollSlow, true
	}
	var waited time.Duration
	if !pendingSince.IsZero() {
		waited = now.Sub(pendingSince)
	}
	switch {
	case waited < ciFastWindow:
		return ciPollFast, true
	case waited < ciMediumWindow:
		return ciPollMedium, true
	}
	return ciPollSlow, true
}

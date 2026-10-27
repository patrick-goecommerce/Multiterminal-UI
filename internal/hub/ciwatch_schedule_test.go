package hub

import (
	"testing"
	"time"
)

func TestNextPoll(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) time.Time { return now.Add(-d) }
	open := func(checks string) PRFacts { return PRFacts{State: "OPEN", Checks: checks} }

	cases := []struct {
		name    string
		pr      PRFacts
		found   bool
		pending time.Time
		want    time.Duration
		goOn    bool
	}{
		{"pending 10 min", open("pending"), true, ago(10 * time.Minute), time.Minute, true},
		{"pending just under 30 min", open("pending"), true, ago(30*time.Minute - time.Second), time.Minute, true},
		{"pending 30 min", open("pending"), true, ago(30 * time.Minute), 5 * time.Minute, true},
		{"pending 40 min", open("pending"), true, ago(40 * time.Minute), 5 * time.Minute, true},
		{"pending just under 3 h", open("pending"), true, ago(3*time.Hour - time.Second), 5 * time.Minute, true},
		{"pending 3 h 1 min", open("pending"), true, ago(3*time.Hour + time.Minute), 15 * time.Minute, true},
		{"pending, no start known", open("pending"), true, time.Time{}, time.Minute, true},
		{"open green", open("green"), true, time.Time{}, 15 * time.Minute, true},
		{"open red", open("red"), true, time.Time{}, 15 * time.Minute, true},
		{"open no checks", open("none"), true, time.Time{}, 15 * time.Minute, true},
		{"merged", PRFacts{State: "MERGED", Checks: "green"}, true, time.Time{}, 0, false},
		{"closed", PRFacts{State: "CLOSED", Checks: "pending"}, true, ago(time.Minute), 0, false},
		{"not found", PRFacts{}, false, time.Time{}, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d, ok := nextPoll(c.pr, c.found, c.pending, now)
			if d != c.want || ok != c.goOn {
				t.Fatalf("nextPoll = %v,%v want %v,%v", d, ok, c.want, c.goOn)
			}
		})
	}
}

package engine

import (
	"testing"
	"time"
)

func TestCheckpoint_NoTimeoutOnFirstCheck(t *testing.T) {
	g := NewCheckpointGuard(3)
	g.Record(ProgressSnapshot{DiffHash: "abc", FailingTests: 5})

	timeout, count := g.ShouldTimeout()
	if timeout {
		t.Error("expected no timeout after single snapshot")
	}
	if count != 0 {
		t.Errorf("expected 0 consecutive no-progress, got %d", count)
	}
}

func TestCheckpoint_ProgressDetected_DiffChanged(t *testing.T) {
	g := NewCheckpointGuard(3)
	g.Record(ProgressSnapshot{DiffHash: "abc"})
	g.Record(ProgressSnapshot{DiffHash: "def"})

	timeout, count := g.ShouldTimeout()
	if timeout {
		t.Error("expected no timeout when diff changed")
	}
	if count != 0 {
		t.Errorf("expected 0 no-progress, got %d", count)
	}
}

func TestCheckpoint_ProgressDetected_TestsImproved(t *testing.T) {
	g := NewCheckpointGuard(3)
	g.Record(ProgressSnapshot{DiffHash: "same", FailingTests: 5})
	g.Record(ProgressSnapshot{DiffHash: "same", FailingTests: 3})

	timeout, _ := g.ShouldTimeout()
	if timeout {
		t.Error("expected no timeout when failing tests decreased")
	}
}

func TestCheckpoint_ProgressDetected_NewCommit(t *testing.T) {
	g := NewCheckpointGuard(3)
	g.Record(ProgressSnapshot{DiffHash: "same"})
	g.Record(ProgressSnapshot{DiffHash: "same", HasNewCommit: true})

	timeout, _ := g.ShouldTimeout()
	if timeout {
		t.Error("expected no timeout when new commit detected")
	}
}

func TestCheckpoint_ProgressDetected_NewFiles(t *testing.T) {
	g := NewCheckpointGuard(3)
	g.Record(ProgressSnapshot{DiffHash: "same", FilesExist: 2})
	g.Record(ProgressSnapshot{DiffHash: "same", FilesExist: 3})

	timeout, _ := g.ShouldTimeout()
	if timeout {
		t.Error("expected no timeout when new files created")
	}
}

func TestCheckpoint_ProgressDetected_ErrorClassChanged(t *testing.T) {
	g := NewCheckpointGuard(3)
	g.Record(ProgressSnapshot{DiffHash: "same", ErrorClass: "build"})
	g.Record(ProgressSnapshot{DiffHash: "same", ErrorClass: "test"})

	timeout, _ := g.ShouldTimeout()
	if timeout {
		t.Error("expected no timeout when error class changed")
	}
}

func TestCheckpoint_NoProgressTimeout(t *testing.T) {
	g := NewCheckpointGuard(3)
	snap := ProgressSnapshot{DiffHash: "same", FailingTests: 5, ErrorClass: "build"}
	g.Record(snap)
	g.Record(snap)
	g.Record(snap)
	g.Record(snap) // 3 consecutive no-progress transitions (between 4 snapshots)

	timeout, count := g.ShouldTimeout()
	if !timeout {
		t.Error("expected timeout after 3 consecutive no-progress checks")
	}
	if count != 3 {
		t.Errorf("expected 3 consecutive no-progress, got %d", count)
	}
}

func TestCheckpoint_WarningAtMaxMinusOne(t *testing.T) {
	g := NewCheckpointGuard(3)
	snap := ProgressSnapshot{DiffHash: "same", FailingTests: 5}
	g.Record(snap)
	g.Record(snap)
	g.Record(snap) // 2 consecutive no-progress transitions

	if !g.IsWarning() {
		t.Error("expected warning at maxNoProgress-1")
	}
	timeout, _ := g.ShouldTimeout()
	if timeout {
		t.Error("expected no timeout yet at warning stage")
	}
}

func TestCheckpoint_ProgressResetsCounter(t *testing.T) {
	g := NewCheckpointGuard(3)
	snap := ProgressSnapshot{DiffHash: "same", FailingTests: 5}

	// 2 no-progress
	g.Record(snap)
	g.Record(snap)
	g.Record(snap)

	// Progress (diff changed)
	g.Record(ProgressSnapshot{DiffHash: "changed", FailingTests: 5})

	// 2 more no-progress
	g.Record(ProgressSnapshot{DiffHash: "changed", FailingTests: 5})
	g.Record(ProgressSnapshot{DiffHash: "changed", FailingTests: 5})

	timeout, count := g.ShouldTimeout()
	if timeout {
		t.Error("expected no timeout — progress reset the counter")
	}
	if count != 2 {
		t.Errorf("expected 2 consecutive no-progress after reset, got %d", count)
	}
}

func TestCheckpoint_ResetClearsHistory(t *testing.T) {
	g := NewCheckpointGuard(3)
	snap := ProgressSnapshot{DiffHash: "same"}
	g.Record(snap)
	g.Record(snap)
	g.Record(snap)
	g.Record(snap)

	// Confirm timeout before reset
	timeout, _ := g.ShouldTimeout()
	if !timeout {
		t.Fatal("expected timeout before reset")
	}

	g.Reset()

	timeout, count := g.ShouldTimeout()
	if timeout {
		t.Error("expected no timeout after reset")
	}
	if count != 0 {
		t.Errorf("expected 0 no-progress after reset, got %d", count)
	}
	if len(g.history) != 0 {
		t.Errorf("expected empty history after reset, got %d", len(g.history))
	}
}

func TestDiffHash_Deterministic(t *testing.T) {
	input := "diff --git a/foo.go b/foo.go\n+hello world\n"
	h1 := DiffHash(input)
	h2 := DiffHash(input)
	if h1 != h2 {
		t.Errorf("DiffHash not deterministic: %s != %s", h1, h2)
	}
	if len(h1) != 16 { // 8 bytes = 16 hex chars
		t.Errorf("expected 16 hex chars, got %d: %s", len(h1), h1)
	}
}

func TestCheckpoint_FailingTestsIncreaseIsNotProgress(t *testing.T) {
	prev := ProgressSnapshot{DiffHash: "same", FailingTests: 3}
	curr := ProgressSnapshot{DiffHash: "same", FailingTests: 5}

	if HasProgress(prev, curr) {
		t.Error("failing tests increasing should NOT count as progress")
	}
}

func TestHasProgress_EmptyErrorClassNotProgress(t *testing.T) {
	prev := ProgressSnapshot{DiffHash: "same", ErrorClass: "build"}
	curr := ProgressSnapshot{DiffHash: "same", ErrorClass: ""}

	if HasProgress(prev, curr) {
		t.Error("changing to empty error class should NOT count as progress")
	}
}

func TestCheckpoint_TimestampAutoFilled(t *testing.T) {
	g := NewCheckpointGuard(3)
	before := time.Now()
	g.Record(ProgressSnapshot{DiffHash: "abc"})
	after := time.Now()

	ts := g.history[0].Timestamp
	if ts.Before(before) || ts.After(after) {
		t.Errorf("auto-filled timestamp %v not in [%v, %v]", ts, before, after)
	}
}

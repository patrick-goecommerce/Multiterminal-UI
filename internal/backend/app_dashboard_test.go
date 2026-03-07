package backend

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// readGitHeadBranch — reads branch from .git/HEAD without exec
// ---------------------------------------------------------------------------

func TestReadGitHeadBranch_ValidRef(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/feature/foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readGitHeadBranch(dir)
	if got != "feature/foo" {
		t.Errorf("readGitHeadBranch() = %q, want %q", got, "feature/foo")
	}
}

func TestReadGitHeadBranch_DetachedHEAD(t *testing.T) {
	dir := t.TempDir()
	gitDir := filepath.Join(dir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hash := "abc123def456789"
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte(hash+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := readGitHeadBranch(dir)
	if got != "abc123de" {
		t.Errorf("readGitHeadBranch() = %q, want %q", got, "abc123de")
	}
}

func TestReadGitHeadBranch_NoGitDir(t *testing.T) {
	dir := t.TempDir()
	got := readGitHeadBranch(dir)
	if got != "" {
		t.Errorf("readGitHeadBranch() = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// formatDashCost — dollar formatting
// ---------------------------------------------------------------------------

func TestFormatDashCost(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0.0, "$0.00"},
		{1.5, "$1.50"},
		{99.999, "$100.00"},
		{0.123456, "$0.12"},
	}
	for _, tt := range tests {
		got := formatDashCost(tt.in)
		if got != tt.want {
			t.Errorf("formatDashCost(%f) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// GetDashboardStats — aggregation with empty sessions
// ---------------------------------------------------------------------------

func TestGetDashboardStats_NoSessions(t *testing.T) {
	app := newTestApp()
	stats := app.GetDashboardStats()
	if stats.TotalSessions != 0 {
		t.Errorf("expected 0 sessions, got %d", stats.TotalSessions)
	}
	if len(stats.Projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(stats.Projects))
	}
}

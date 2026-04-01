package board

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const defaultRemote = "origin"
const syncTimeout = 30 * time.Second

// Syncer handles pull/push of board refs with a remote.
type Syncer struct {
	repoDir string
	remote  string
}

// NewSyncer creates a Syncer that syncs with the "origin" remote.
func NewSyncer(repoDir string) *Syncer {
	return &Syncer{repoDir: repoDir, remote: defaultRemote}
}

// NewSyncerWithRemote creates a Syncer that syncs with the specified remote.
func NewSyncerWithRemote(repoDir, remote string) *Syncer {
	return &Syncer{repoDir: repoDir, remote: remote}
}

// Pull fetches board refs from remote.
// Returns nil if successful, or an error describing what went wrong.
// Remote not reachable returns an error (caller decides whether to warn or ignore).
func (s *Syncer) Pull() error {
	ctx, cancel := context.WithTimeout(context.Background(), syncTimeout)
	defer cancel()

	args := []string{"fetch", s.remote, "refs/mtui/*:refs/mtui/*"}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = s.repoDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s failed: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// Push pushes board refs to remote.
// Returns nil if successful, or an error describing what went wrong.
func (s *Syncer) Push() error {
	ctx, cancel := context.WithTimeout(context.Background(), syncTimeout)
	defer cancel()

	args := []string{"push", s.remote, "refs/mtui/*"}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = s.repoDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s failed: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

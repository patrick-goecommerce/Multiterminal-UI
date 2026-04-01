package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// WorktreeSlotManager manages a pool of git worktree slots.
// It provides backpressure by blocking Allocate when all slots are in use.
type WorktreeSlotManager struct {
	mu       sync.Mutex
	repoDir  string
	slotsDir string // e.g. ".mtui/worktrees/"
	maxSlots int
	slots    map[int]*WorktreeSlot
	nextSlot int
	waitCh   chan struct{} // signals when a slot frees up
}

// WorktreeSlot represents one managed git worktree.
type WorktreeSlot struct {
	ID      int
	WorkDir string // absolute path to worktree
	Branch  string // branch name
	InUse   bool
	Parked  bool // kept on disk but slot freed
}

// NewWorktreeSlotManager creates a slot manager for the given repo.
// slotsDir defaults to "<repoDir>/.mtui/worktrees/".
func NewWorktreeSlotManager(repoDir string, maxSlots int) *WorktreeSlotManager {
	return &WorktreeSlotManager{
		repoDir:  repoDir,
		slotsDir: filepath.Join(repoDir, ".mtui", "worktrees"),
		maxSlots: maxSlots,
		slots:    make(map[int]*WorktreeSlot),
		waitCh:   make(chan struct{}, 1),
	}
}

// Allocate gets a free slot, creates a worktree, and returns the slot ID
// and work directory. If all slots are in use, it blocks until one frees
// up or ctx is cancelled. branch should be like "mtui/<card-id>/<step-id>".
func (m *WorktreeSlotManager) Allocate(ctx context.Context, branch string) (int, string, error) {
	for {
		m.mu.Lock()
		slotID, found := m.findFreeSlot()
		if found {
			slot := m.initSlot(slotID, branch)
			m.mu.Unlock()
			if err := m.createWorktree(slot); err != nil {
				m.mu.Lock()
				slot.InUse = false
				slot.WorkDir = ""
				slot.Branch = ""
				m.mu.Unlock()
				return 0, "", fmt.Errorf("git worktree add: %w", err)
			}
			return slot.ID, slot.WorkDir, nil
		}
		m.mu.Unlock()

		// All slots busy — wait for a signal or context cancellation.
		select {
		case <-ctx.Done():
			return 0, "", ctx.Err()
		case <-m.waitCh:
			// A slot was freed, retry.
		}
	}
}

// Release removes the worktree from disk, deletes the branch, and frees the slot.
func (m *WorktreeSlotManager) Release(slotID int) error {
	m.mu.Lock()
	slot, ok := m.slots[slotID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("slot %d not found", slotID)
	}
	workDir := slot.WorkDir
	branch := slot.Branch
	m.mu.Unlock()

	// Remove worktree from disk.
	if workDir != "" {
		_ = m.runGit("worktree", "remove", "--force", workDir)
	}
	// Delete the branch.
	if branch != "" {
		_ = m.runGit("branch", "-D", branch)
	}

	m.mu.Lock()
	slot.InUse = false
	slot.Parked = false
	slot.WorkDir = ""
	slot.Branch = ""
	m.mu.Unlock()

	m.signal()
	return nil
}

// Park keeps the worktree on disk but frees the slot for reuse.
// Useful for paused cards that might resume later.
func (m *WorktreeSlotManager) Park(slotID int) error {
	m.mu.Lock()
	slot, ok := m.slots[slotID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("slot %d not found", slotID)
	}
	slot.InUse = false
	slot.Parked = true
	m.mu.Unlock()

	m.signal()
	return nil
}

// Prune removes all parked worktree directories and runs git worktree prune.
func (m *WorktreeSlotManager) Prune() error {
	// Collect parked slots.
	m.mu.Lock()
	var parked []*WorktreeSlot
	for _, s := range m.slots {
		if s.Parked {
			parked = append(parked, s)
		}
	}
	m.mu.Unlock()

	// Remove parked worktree directories.
	for _, s := range parked {
		if s.WorkDir != "" {
			_ = os.RemoveAll(s.WorkDir)
		}
		if s.Branch != "" {
			_ = m.runGit("branch", "-D", s.Branch)
		}
	}

	// Run git worktree prune.
	_ = m.runGit("worktree", "prune")

	// Reset parked slots.
	m.mu.Lock()
	for _, s := range parked {
		s.Parked = false
		s.WorkDir = ""
		s.Branch = ""
	}
	m.mu.Unlock()
	return nil
}

// ActiveSlots returns the number of currently in-use slots.
func (m *WorktreeSlotManager) ActiveSlots() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, s := range m.slots {
		if s.InUse {
			count++
		}
	}
	return count
}

// findFreeSlot returns the ID of a free slot (not InUse, not Parked).
// Must be called with mu held.
func (m *WorktreeSlotManager) findFreeSlot() (int, bool) {
	// Reuse an existing free slot.
	for id, s := range m.slots {
		if !s.InUse && !s.Parked {
			return id, true
		}
	}
	// Create a new slot if under maxSlots.
	if len(m.slots) < m.maxSlots {
		return m.nextSlot, true
	}
	return 0, false
}

// initSlot marks the slot as in-use and returns it. Must be called with mu held.
func (m *WorktreeSlotManager) initSlot(slotID int, branch string) *WorktreeSlot {
	slot, exists := m.slots[slotID]
	if !exists {
		slot = &WorktreeSlot{ID: slotID}
		m.slots[slotID] = slot
		m.nextSlot = slotID + 1
	}
	slotDir := filepath.Join(m.slotsDir, fmt.Sprintf("slot-%d", slotID))
	slot.WorkDir = slotDir
	slot.Branch = branch
	slot.InUse = true
	slot.Parked = false
	return slot
}

// createWorktree runs git worktree add for the slot.
func (m *WorktreeSlotManager) createWorktree(slot *WorktreeSlot) error {
	// Ensure slots directory exists.
	if err := os.MkdirAll(m.slotsDir, 0o755); err != nil {
		return err
	}
	return m.runGit("worktree", "add", slot.WorkDir, "-b", slot.Branch)
}

// runGit executes a git command in the repo directory.
func (m *WorktreeSlotManager) runGit(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = m.repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s", err, out)
	}
	return nil
}

// signal notifies one blocked Allocate that a slot is available.
func (m *WorktreeSlotManager) signal() {
	select {
	case m.waitCh <- struct{}{}:
	default:
	}
}

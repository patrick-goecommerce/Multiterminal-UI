package board

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ErrAlreadyLocked is returned when a task is locked by another agent.
var ErrAlreadyLocked = errors.New("task is already locked")

// LockInfo stores who holds a lock and when it was acquired.
type LockInfo struct {
	AgentName string    `json:"agent_name"`
	LockedAt  time.Time `json:"locked_at"`
}

// Locker manages atomic task locks stored as git refs.
type Locker struct {
	store        *RefStore
	staleTimeout time.Duration
}

// NewLocker creates a Locker with the default 5-minute stale timeout.
func NewLocker(store *RefStore) *Locker {
	return &Locker{store: store, staleTimeout: 5 * time.Minute}
}

// NewLockerWithTimeout creates a Locker with a custom stale timeout.
func NewLockerWithTimeout(store *RefStore, timeout time.Duration) *Locker {
	return &Locker{store: store, staleTimeout: timeout}
}

// lockRef returns the git ref path for a task lock.
func lockRef(taskID string) string {
	return fmt.Sprintf("refs/mtui/tasks/%s/lock", taskID)
}

// AcquireLock attempts to lock a task for an agent.
// If the task is already locked by another agent and the lock is NOT stale,
// returns ErrAlreadyLocked. If the lock IS stale (older than staleTimeout),
// overwrites it. If the task is already locked by the SAME agent, refreshes
// the lock timestamp.
func (l *Locker) AcquireLock(taskID, agentName string) error {
	ref := lockRef(taskID)

	// Check for existing lock.
	data, err := l.store.ReadRef(ref)
	if err == nil {
		var existing LockInfo
		if jsonErr := json.Unmarshal(data, &existing); jsonErr == nil {
			isStale := time.Since(existing.LockedAt) > l.staleTimeout
			isSameAgent := existing.AgentName == agentName
			if !isStale && !isSameAgent {
				return fmt.Errorf("%w: held by %q since %s",
					ErrAlreadyLocked, existing.AgentName, existing.LockedAt.Format(time.RFC3339))
			}
		}
	} else if !errors.Is(err, ErrRefNotFound) {
		return fmt.Errorf("check lock for task %q: %w", taskID, err)
	}

	// Write new lock.
	info := LockInfo{
		AgentName: agentName,
		LockedAt:  time.Now(),
	}
	blob, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal lock info: %w", err)
	}
	if err := l.store.WriteRef(ref, blob); err != nil {
		return fmt.Errorf("write lock for task %q: %w", taskID, err)
	}
	return nil
}

// ReleaseLock releases a lock. Only the agent that holds the lock can release it.
// Returns error if the lock doesn't exist or is held by a different agent.
func (l *Locker) ReleaseLock(taskID, agentName string) error {
	ref := lockRef(taskID)

	data, err := l.store.ReadRef(ref)
	if err != nil {
		if errors.Is(err, ErrRefNotFound) {
			return fmt.Errorf("release lock for task %q: %w", taskID, ErrRefNotFound)
		}
		return fmt.Errorf("read lock for task %q: %w", taskID, err)
	}

	var info LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return fmt.Errorf("unmarshal lock for task %q: %w", taskID, err)
	}

	if info.AgentName != agentName {
		return fmt.Errorf("release lock for task %q: held by %q, not %q",
			taskID, info.AgentName, agentName)
	}

	if err := l.store.DeleteRef(ref); err != nil {
		return fmt.Errorf("delete lock for task %q: %w", taskID, err)
	}
	return nil
}

// IsLocked checks if a task is locked and the lock is not stale.
func (l *Locker) IsLocked(taskID string) (bool, error) {
	ref := lockRef(taskID)

	data, err := l.store.ReadRef(ref)
	if err != nil {
		if errors.Is(err, ErrRefNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("check lock for task %q: %w", taskID, err)
	}

	var info LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return false, fmt.Errorf("unmarshal lock for task %q: %w", taskID, err)
	}

	if time.Since(info.LockedAt) > l.staleTimeout {
		return false, nil
	}
	return true, nil
}

// GetLockInfo returns the lock info for a task, or ErrRefNotFound if not locked.
func (l *Locker) GetLockInfo(taskID string) (LockInfo, error) {
	ref := lockRef(taskID)

	data, err := l.store.ReadRef(ref)
	if err != nil {
		if errors.Is(err, ErrRefNotFound) {
			return LockInfo{}, ErrRefNotFound
		}
		return LockInfo{}, fmt.Errorf("read lock for task %q: %w", taskID, err)
	}

	var info LockInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return LockInfo{}, fmt.Errorf("unmarshal lock for task %q: %w", taskID, err)
	}
	return info, nil
}

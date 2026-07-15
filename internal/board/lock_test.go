package board

import (
	"errors"
	"testing"
	"time"
)

func TestAcquireLockAndGetInfo(t *testing.T) {
	dir := setupTestRepo(t)
	store := NewRefStore(dir)
	locker := NewLocker(store)

	if err := locker.AcquireLock("task-1", "agent-a"); err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}

	info, err := locker.GetLockInfo("task-1")
	if err != nil {
		t.Fatalf("GetLockInfo: %v", err)
	}
	if info.AgentName != "agent-a" {
		t.Errorf("expected agent-a, got %q", info.AgentName)
	}
	if time.Since(info.LockedAt) > 5*time.Second {
		t.Errorf("LockedAt too old: %v", info.LockedAt)
	}
}

func TestAcquireLockAlreadyLockedByOther(t *testing.T) {
	dir := setupTestRepo(t)
	store := NewRefStore(dir)
	locker := NewLocker(store)

	if err := locker.AcquireLock("task-1", "agent-a"); err != nil {
		t.Fatalf("AcquireLock agent-a: %v", err)
	}

	err := locker.AcquireLock("task-1", "agent-b")
	if err == nil {
		t.Fatal("expected ErrAlreadyLocked, got nil")
	}
	if !errors.Is(err, ErrAlreadyLocked) {
		t.Errorf("expected ErrAlreadyLocked, got: %v", err)
	}
}

func TestAcquireLockSameAgentRefreshes(t *testing.T) {
	dir := setupTestRepo(t)
	store := NewRefStore(dir)
	locker := NewLocker(store)

	if err := locker.AcquireLock("task-1", "agent-a"); err != nil {
		t.Fatalf("AcquireLock first: %v", err)
	}
	info1, _ := locker.GetLockInfo("task-1")

	time.Sleep(10 * time.Millisecond)

	if err := locker.AcquireLock("task-1", "agent-a"); err != nil {
		t.Fatalf("AcquireLock refresh: %v", err)
	}
	info2, _ := locker.GetLockInfo("task-1")

	if !info2.LockedAt.After(info1.LockedAt) {
		t.Errorf("expected refreshed timestamp, got %v <= %v", info2.LockedAt, info1.LockedAt)
	}
}

func TestAcquireLockStaleLockOverwritten(t *testing.T) {
	dir := setupTestRepo(t)
	store := NewRefStore(dir)
	locker := NewLockerWithTimeout(store, 100*time.Millisecond)

	if err := locker.AcquireLock("task-1", "agent-a"); err != nil {
		t.Fatalf("AcquireLock agent-a: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	// Lock is stale — agent-b should be able to take it.
	if err := locker.AcquireLock("task-1", "agent-b"); err != nil {
		t.Fatalf("AcquireLock agent-b after stale: %v", err)
	}

	info, _ := locker.GetLockInfo("task-1")
	if info.AgentName != "agent-b" {
		t.Errorf("expected agent-b, got %q", info.AgentName)
	}
}

func TestReleaseLockCorrectAgent(t *testing.T) {
	dir := setupTestRepo(t)
	store := NewRefStore(dir)
	locker := NewLocker(store)

	if err := locker.AcquireLock("task-1", "agent-a"); err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}

	if err := locker.ReleaseLock("task-1", "agent-a"); err != nil {
		t.Fatalf("ReleaseLock: %v", err)
	}

	locked, err := locker.IsLocked("task-1")
	if err != nil {
		t.Fatalf("IsLocked: %v", err)
	}
	if locked {
		t.Error("expected task to be unlocked after release")
	}
}

func TestReleaseLockWrongAgent(t *testing.T) {
	dir := setupTestRepo(t)
	store := NewRefStore(dir)
	locker := NewLocker(store)

	if err := locker.AcquireLock("task-1", "agent-a"); err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}

	err := locker.ReleaseLock("task-1", "agent-b")
	if err == nil {
		t.Fatal("expected error for wrong agent, got nil")
	}
}

func TestReleaseLockNotLocked(t *testing.T) {
	dir := setupTestRepo(t)
	store := NewRefStore(dir)
	locker := NewLocker(store)

	err := locker.ReleaseLock("task-1", "agent-a")
	if err == nil {
		t.Fatal("expected error for unlocked task, got nil")
	}
	if !errors.Is(err, ErrRefNotFound) {
		t.Errorf("expected ErrRefNotFound, got: %v", err)
	}
}

func TestIsLockedActive(t *testing.T) {
	dir := setupTestRepo(t)
	store := NewRefStore(dir)
	locker := NewLocker(store)

	if err := locker.AcquireLock("task-1", "agent-a"); err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}

	locked, err := locker.IsLocked("task-1")
	if err != nil {
		t.Fatalf("IsLocked: %v", err)
	}
	if !locked {
		t.Error("expected task to be locked")
	}
}

func TestIsLockedStale(t *testing.T) {
	dir := setupTestRepo(t)
	store := NewRefStore(dir)
	locker := NewLockerWithTimeout(store, 100*time.Millisecond)

	if err := locker.AcquireLock("task-1", "agent-a"); err != nil {
		t.Fatalf("AcquireLock: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	locked, err := locker.IsLocked("task-1")
	if err != nil {
		t.Fatalf("IsLocked: %v", err)
	}
	if locked {
		t.Error("expected stale lock to report as not locked")
	}
}

func TestIsLockedNoLock(t *testing.T) {
	dir := setupTestRepo(t)
	store := NewRefStore(dir)
	locker := NewLocker(store)

	locked, err := locker.IsLocked("task-1")
	if err != nil {
		t.Fatalf("IsLocked: %v", err)
	}
	if locked {
		t.Error("expected no lock to report as not locked")
	}
}

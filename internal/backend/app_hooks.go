package backend

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// rawHookEvent is the JSONL structure written by hook_handler.ps1.
type rawHookEvent struct {
	Ts        int64  `json:"ts"`
	Event     string `json:"event"`
	SessionID string `json:"session_id"`
	MtID      int    `json:"mt_id"`
	Tool      string `json:"tool"`
	Message   string `json:"message"`
}

// hookEventToActivity maps a Claude Code event name to an ActivityState.
func hookEventToActivity(event string) terminal.ActivityState {
	switch event {
	case "PreToolUse", "PostToolUse", "UserPromptSubmit":
		return terminal.ActivityActive
	case "PostToolUseFailure":
		return terminal.ActivityError
	case "PermissionRequest":
		return terminal.ActivityWaitingPermission
	case "Notification":
		return terminal.ActivityIdle
	case "Stop":
		return terminal.ActivityDone
	default:
		return terminal.ActivityIdle
	}
}

// HookManager polls the hooks directory and dispatches events to sessions.
type HookManager struct {
	dir        string
	lookupFn   func(mtID int) *terminal.Session
	onActivity func(sessionID int, activity string, cost string)

	mu      sync.Mutex
	offsets map[string]int64 // filename → bytes already read
}

func newHookManager(
	dir string,
	lookupFn func(mtID int) *terminal.Session,
	onActivity func(sessionID int, activity string, cost string),
) *HookManager {
	return &HookManager{
		dir:        dir,
		lookupFn:   lookupFn,
		onActivity: onActivity,
		offsets:    make(map[string]int64),
	}
}

// Start begins polling the hooks directory every 100ms.
func (hm *HookManager) Start(ctx context.Context) {
	if err := os.MkdirAll(hm.dir, 0755); err != nil {
		log.Printf("[hooks] could not create hooks dir: %v — hook integration disabled", err)
		return
	}
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				hm.processDirectory()
			}
		}
	}()
}

// processDirectory scans the hooks directory for new JSONL events.
func (hm *HookManager) processDirectory() {
	entries, err := os.ReadDir(hm.dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		hm.processFile(filepath.Join(hm.dir, entry.Name()), entry.Name())
	}
}

// processFile reads new lines from a JSONL file since the last read offset.
// Events are collected while the file is open, then dispatched after closing
// so that handleEvent (which may delete the file on Windows) never races with
// an open file handle.
func (hm *HookManager) processFile(path, name string) {
	hm.mu.Lock()
	offset := hm.offsets[name]
	hm.mu.Unlock()

	events, newOffset := hm.readEvents(path, offset)

	hm.mu.Lock()
	hm.offsets[name] = newOffset
	hm.mu.Unlock()

	for _, ev := range events {
		hm.handleEvent(ev)
	}
}

// readEvents opens the file, seeks to offset, and collects all new events.
// Returns the parsed events and the new file offset. The file is closed before
// returning so callers can safely delete it on Windows.
func (hm *HookManager) readEvents(path string, offset int64) ([]rawHookEvent, int64) {
	f, err := os.Open(path)
	if err != nil {
		return nil, offset
	}
	defer f.Close()

	if offset > 0 {
		if _, err := f.Seek(offset, 0); err != nil {
			return nil, offset
		}
	}

	var events []rawHookEvent
	scanner := bufio.NewScanner(f)
	newOffset := offset
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		newOffset += int64(len(scanner.Bytes())) + 1 // +1 for newline
		if line == "" {
			continue
		}
		var ev rawHookEvent
		if err := json.Unmarshal([]byte(line), &ev); err == nil {
			events = append(events, ev)
		}
	}
	return events, newOffset
}

// handleEvent applies a hook event to the appropriate session.
func (hm *HookManager) handleEvent(ev rawHookEvent) {
	if ev.MtID == 0 {
		return
	}
	sess := hm.lookupFn(ev.MtID)
	if sess == nil {
		return
	}

	// Record Claude's session UUID on first event
	if ev.SessionID != "" && sess.HookSessionID() == "" {
		sess.SetHookSessionID(ev.SessionID)
	}

	if ev.Event == "SessionEnd" {
		sess.ClearHookData()
		hm.cleanupFile(ev.SessionID + ".jsonl")
		return
	}

	newState := hookEventToActivity(ev.Event)
	sess.SetHookActivity(newState)

	if hm.onActivity != nil {
		hm.onActivity(ev.MtID, activityString(newState), "")
	}
}

// cleanupFile removes a finished session's JSONL file and its offset entry.
func (hm *HookManager) cleanupFile(name string) {
	hm.mu.Lock()
	delete(hm.offsets, name)
	hm.mu.Unlock()
	_ = os.Remove(filepath.Join(hm.dir, name))
}

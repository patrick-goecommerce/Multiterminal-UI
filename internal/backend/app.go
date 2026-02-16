// Package backend provides the Wails-bound application struct that bridges
// the Go PTY session management with the Svelte frontend.
package backend

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// sessionIssue tracks which GitHub issue a session is working on.
type sessionIssue struct {
	Number int
	Title  string
	Branch string
	Dir    string // working directory (for gh CLI calls)
}

// App is the main Wails application struct. All exported methods are
// automatically available to the frontend via generated TypeScript bindings.
type App struct {
	ctx           context.Context
	cfg           config.Config
	health        config.HealthState
	sessions      map[int]*terminal.Session
	queues        map[int]*sessionQueue
	sessionIssues map[int]*sessionIssue // issue linked to each session
	mu                sync.Mutex
	nextID            int
	cancelAll         context.CancelFunc
	resolvedClaudePath string
	claudeDetected     bool
}

// NewApp creates a new App instance with the given configuration.
func NewApp(cfg config.Config) *App {
	return &App{
		cfg:           cfg,
		sessions:      make(map[int]*terminal.Session),
		queues:        make(map[int]*sessionQueue),
		sessionIssues: make(map[int]*sessionIssue),
	}
}

// Startup is called when the Wails app starts. It receives the app context.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	// Load health state and mark this session as started (dirty)
	a.health = config.LoadHealth()
	config.MarkStarting(&a.health)
	_ = config.SaveHealth(a.health)

	// Resolve Claude CLI path before anything else needs it
	a.resolveClaudeOnStartup()

	// Start periodic scanner for activity and token detection
	scanCtx, cancel := context.WithCancel(ctx)
	a.cancelAll = cancel
	go a.scanLoop(scanCtx)

	// Start focus listener and register custom protocol for notification clicks
	a.startFocusListener()
	registerProtocol()
}

// Shutdown is called when the Wails app is closing. Clean up all sessions.
func (a *App) Shutdown(ctx context.Context) {
	if a.cancelAll != nil {
		a.cancelAll()
	}
	a.mu.Lock()
	sessions := make([]*terminal.Session, 0, len(a.sessions))
	for _, s := range a.sessions {
		sessions = append(sessions, s)
	}
	a.mu.Unlock()

	for _, s := range sessions {
		s.Close()
	}

	// Mark clean shutdown and auto-disable logging if stable
	config.MarkCleanShutdown(&a.health)
	if config.ShouldAutoDisableLogging(&a.health) {
		config.DisableAutoLogging(&a.health)
		a.cfg.LoggingEnabled = false
		_ = config.Save(a.cfg)
		log.Println("[Shutdown] Auto-logging disabled after 3 clean shutdowns")
	}
	_ = config.SaveHealth(a.health)
	log.Println("[Shutdown] Clean shutdown recorded")
}

// SessionInfo is the JSON-serialisable session metadata sent to the frontend.
type SessionInfo struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Running  bool   `json:"running"`
	ExitCode int    `json:"exitCode"`
}

// CreateSession spawns a new PTY session and starts streaming its output
// to the frontend. Returns the session ID.
func (a *App) CreateSession(argv []string, dir string, rows int, cols int) int {
	a.mu.Lock()
	a.nextID++
	id := a.nextID
	a.mu.Unlock()

	if dir == "" {
		dir, _ = os.Getwd()
	}
	if rows < 5 {
		rows = 24
	}
	if cols < 20 {
		cols = 80
	}

	log.Printf("[CreateSession] id=%d argv=%v dir=%q rows=%d cols=%d", id, argv, dir, rows, cols)

	// Use configured default shell when no command specified
	if len(argv) == 0 && a.cfg.DefaultShell != "" {
		argv = []string{a.cfg.DefaultShell}
	}

	sess := terminal.NewSession(id, rows, cols)
	if err := sess.Start(argv, dir, nil); err != nil {
		errMsg := fmt.Sprintf("Session start failed: %v", err)
		log.Printf("[CreateSession] ERROR: %s", errMsg)
		runtime.EventsEmit(a.ctx, "terminal:error", id, errMsg)
		return -1
	}
	log.Printf("[CreateSession] session %d started successfully", id)

	a.mu.Lock()
	a.sessions[id] = sess
	a.mu.Unlock()

	// Stream PTY output to frontend
	go a.streamOutput(id, sess)

	// Watch for process exit
	go a.watchExit(id, sess)

	return id
}

// WriteToSession sends raw input data (base64-encoded) to a session's PTY.
func (a *App) WriteToSession(id int, b64data string) {
	a.mu.Lock()
	sess := a.sessions[id]
	a.mu.Unlock()
	if sess == nil {
		return
	}
	data, err := base64.StdEncoding.DecodeString(b64data)
	if err != nil {
		return
	}
	sess.Write(data)
}

// ResizeSession updates the PTY and screen buffer dimensions.
func (a *App) ResizeSession(id int, rows int, cols int) {
	a.mu.Lock()
	sess := a.sessions[id]
	a.mu.Unlock()
	if sess == nil {
		return
	}
	sess.Resize(rows, cols)
}

// CloseSession terminates a session and removes it.
// The session is closed asynchronously but removed from the map only
// after Close() completes, ensuring streamOutput drains all buffered
// data before the session is gone.
func (a *App) CloseSession(id int) {
	a.mu.Lock()
	sess := a.sessions[id]
	a.mu.Unlock()
	if sess == nil {
		return
	}
	// Report "close" progress before removing the issue link
	a.reportIssueProgress(id, progressClose, a.getSessionCost(id))

	go func() {
		sess.Close() // blocks until process exits and readLoop closes RawOutputCh
		a.mu.Lock()
		delete(a.sessions, id)
		delete(a.queues, id)
		delete(a.sessionIssues, id)
		a.mu.Unlock()
		// Clean up per-session activity tracking to prevent memory leak
		cleanupActivityTracking(id)
	}()
}

// GetConfig returns the current application configuration.
func (a *App) GetConfig() config.Config {
	return a.cfg
}

// SaveConfig saves the given config to disk and updates the in-memory copy.
func (a *App) SaveConfig(cfg config.Config) error {
	log.Printf("[SaveConfig] theme=%q terminal_color=%q", cfg.Theme, cfg.TerminalColor)
	a.cfg = cfg
	if err := config.Save(cfg); err != nil {
		log.Printf("[SaveConfig] error: %v", err)
		return fmt.Errorf("config save failed: %w", err)
	}
	// Re-detect Claude path in case claude_command changed
	a.resolveClaudeOnStartup()
	return nil
}

// SaveTabs persists the current tab/pane layout to disk so it can be
// restored on next startup.
func (a *App) SaveTabs(state config.SessionState) {
	log.Printf("[SaveTabs] saving %d tabs", len(state.Tabs))
	if err := config.SaveSession(state); err != nil {
		log.Printf("[SaveTabs] error: %v", err)
	}
}

// LoadTabs returns the previously saved tab/pane layout, or nil.
func (a *App) LoadTabs() *config.SessionState {
	if !a.cfg.ShouldRestoreSession() {
		log.Printf("[LoadTabs] restore_session disabled")
		return nil
	}
	state := config.LoadSession()
	if state == nil {
		log.Printf("[LoadTabs] no saved session found")
	} else {
		log.Printf("[LoadTabs] loaded %d tabs", len(state.Tabs))
	}
	return state
}

// GetWorkingDir returns the effective working directory (from config or cwd).
func (a *App) GetWorkingDir() string {
	if a.cfg.DefaultDir != "" {
		return a.cfg.DefaultDir
	}
	dir, _ := os.Getwd()
	return dir
}

// SelectDirectory opens a native directory picker dialog and returns the
// selected path, or an empty string if the user cancelled.
func (a *App) SelectDirectory(startDir string) string {
	if startDir == "" {
		startDir = a.GetWorkingDir()
	}
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Arbeitsverzeichnis wählen",
		DefaultDirectory: startDir,
	})
	if err != nil {
		return ""
	}
	return dir
}

// streamOutput reads raw PTY bytes from the session and emits them as
// base64-encoded chunks to the frontend via Wails events.
// It coalesces rapid output over a short time window so that TUI redraws
// (which produce many small chunks) arrive as a single event, preventing
// cursor flicker in xterm.js.
func (a *App) streamOutput(id int, sess *terminal.Session) {
	const coalesceDelay = 4 * time.Millisecond
	for {
		select {
		case data, ok := <-sess.RawOutputCh:
			if !ok {
				return
			}
			buf := append([]byte(nil), data...)
			// Wait briefly for more chunks — TUI apps redraw in bursts
			deadline := time.After(coalesceDelay)
		collect:
			for {
				select {
				case more, ok := <-sess.RawOutputCh:
					if !ok {
						b64 := base64.StdEncoding.EncodeToString(buf)
						runtime.EventsEmit(a.ctx, "terminal:output", id, b64)
						return
					}
					buf = append(buf, more...)
				case <-deadline:
					break collect
				case <-a.ctx.Done():
					return
				}
			}
			b64 := base64.StdEncoding.EncodeToString(buf)
			runtime.EventsEmit(a.ctx, "terminal:output", id, b64)
		case <-a.ctx.Done():
			return
		}
	}
}

// watchExit waits for a session to exit and notifies the frontend.
func (a *App) watchExit(id int, sess *terminal.Session) {
	<-sess.Done()
	runtime.EventsEmit(a.ctx, "terminal:exit", id, sess.ExitCode)
}

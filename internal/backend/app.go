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

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// sessionIssue tracks which GitHub issue a session is working on.
type sessionIssue struct {
	Number int
	Title  string
	Branch string
	Dir    string // working directory (for gh CLI calls)
}

// AppService is the main Wails application struct. All exported methods are
// automatically available to the frontend via generated TypeScript bindings.
type AppService struct {
	app            *application.App           // Wails v3 application instance
	mainWindow     *application.WebviewWindow  // main window reference for dialogs
	serviceCtx     context.Context             // context from ServiceStartup
	cfg            config.Config
	health         config.HealthState
	sessions       map[int]*terminal.Session
	queues         map[int]*sessionQueue
	sessionIssues  map[int]*sessionIssue // issue linked to each session
	mu                sync.Mutex
	nextID            int
	cancelAll         context.CancelFunc
	batcher           *outputBatcher
	resolvedClaudePath string
	claudeDetected     bool
	winMgr            *windowManager // tracks all open windows for multi-window support
	detachCount       int            // monotonic counter for detached window IDs
}

// NewAppService creates a new AppService instance for Wails v3 service pattern.
func NewAppService(app *application.App, cfg config.Config) *AppService {
	return &AppService{
		app:           app,
		cfg:           cfg,
		sessions:      make(map[int]*terminal.Session),
		queues:        make(map[int]*sessionQueue),
		sessionIssues: make(map[int]*sessionIssue),
		winMgr:        newWindowManager(app),
	}
}

// SetMainWindow stores the main window reference for dialog and focus operations.
func (a *AppService) SetMainWindow(w *application.WebviewWindow) {
	a.mainWindow = w
	a.winMgr.register("main", w, nil)
}

// ServiceStartup implements the Wails v3 Service interface.
func (a *AppService) ServiceStartup(ctx context.Context, opts application.ServiceOptions) error {
	a.serviceCtx = ctx

	// Load health state and mark this session as started (dirty)
	a.health = config.LoadHealth()
	config.MarkStarting(&a.health)
	_ = config.SaveHealth(a.health)

	// Resolve Claude CLI path before anything else needs it
	a.resolveClaudeOnStartup()

	// Start periodic scanner for activity and token detection
	scanCtx, cancel := context.WithCancel(ctx)
	a.cancelAll = cancel
	a.batcher = newOutputBatcher()
	go a.scanLoop(scanCtx)
	go a.batchLoop(scanCtx)

	// Start focus listener and register custom protocol for notification clicks
	a.startFocusListener()
	registerProtocol()
	return nil
}

// ServiceShutdown implements the Wails v3 Service interface.
func (a *AppService) ServiceShutdown() error {
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
	return nil
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
func (a *AppService) CreateSession(argv []string, dir string, rows int, cols int) int {
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
		a.app.Event.Emit("terminal:error", TerminalErrorEvent{ID: id, Message: errMsg})
		return -1
	}
	log.Printf("[CreateSession] session %d started successfully", id)

	a.mu.Lock()
	a.sessions[id] = sess
	a.mu.Unlock()

	// Stream PTY output to frontend
	go a.collectOutput(id, sess, a.serviceCtx)

	// Watch for process exit
	go a.watchExit(id, sess)

	return id
}

// WriteToSession sends raw input data (base64-encoded) to a session's PTY.
func (a *AppService) WriteToSession(id int, b64data string) {
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
func (a *AppService) ResizeSession(id int, rows int, cols int) {
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
func (a *AppService) CloseSession(id int) {
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
func (a *AppService) GetConfig() config.Config {
	return a.cfg
}

// SaveConfig saves the given config to disk and updates the in-memory copy.
func (a *AppService) SaveConfig(cfg config.Config) error {
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
func (a *AppService) SaveTabs(state config.SessionState) {
	log.Printf("[SaveTabs] saving %d tabs", len(state.Tabs))
	if err := config.SaveSession(state); err != nil {
		log.Printf("[SaveTabs] error: %v", err)
	}
}

// LoadTabs returns the previously saved tab/pane layout, or nil.
func (a *AppService) LoadTabs() *config.SessionState {
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
func (a *AppService) GetWorkingDir() string {
	if a.cfg.DefaultDir != "" {
		return a.cfg.DefaultDir
	}
	dir, _ := os.Getwd()
	return dir
}


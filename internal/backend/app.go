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
	safeMode      bool
	sessionBackup *config.SessionState // populated in safe-mode; restored on shutdown
	hookMgr       *HookManager
	resolvedCodexPath  string
	codexDetected      bool
	resolvedGeminiPath string
	geminiDetected     bool
	tmuxAPIPort        int // port for the tmux shim HTTP API
}

// NewAppService creates a new AppService instance for Wails v3 service pattern.
func NewAppService(app *application.App, cfg config.Config, safeMode bool) *AppService {
	svc := &AppService{
		app:           app,
		cfg:           cfg,
		sessions:      make(map[int]*terminal.Session),
		queues:        make(map[int]*sessionQueue),
		sessionIssues: make(map[int]*sessionIssue),
		winMgr:        newWindowManager(app),
		safeMode:      safeMode,
	}
	if safeMode {
		svc.sessionBackup = config.LoadSession() // may be nil — that's fine
		log.Println("[SafeMode] active: sessions will not be loaded or saved")
	}
	return svc
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

	// Resolve CLI paths before anything else needs them
	a.resolveClaudeOnStartup()
	a.resolveCodexOnStartup()
	a.resolveGeminiOnStartup()

	// Setup Claude Code hook integration
	go a.setupHooks(ctx)

	// Auto-setup statusline in ~/.claude/settings.json if not already configured
	go a.setupStatusLine()

	// Start periodic scanner for activity and token detection
	scanCtx, cancel := context.WithCancel(ctx)
	a.cancelAll = cancel
	a.batcher = newOutputBatcher()
	go a.scanLoop(scanCtx)
	go a.batchLoop(scanCtx)
	go a.scheduleLoop(scanCtx)

	// Start focus listener and register custom protocol for notification clicks
	a.startFocusListener()
	registerProtocol()

	// Start tmux shim API server
	if port, err := a.startTmuxAPI(); err != nil {
		log.Printf("[tmux-api] failed to start: %v", err)
	} else {
		a.tmuxAPIPort = port
	}

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

	if a.safeMode {
		if a.sessionBackup != nil {
			if err := config.SaveSession(*a.sessionBackup); err != nil {
				log.Printf("[SafeMode] failed to restore session backup: %v", err)
			}
		} else {
			config.ClearSession()
		}
	}
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
// mode must be "shell", "claude", or "claude-yolo"; it controls env injection.
func (a *AppService) CreateSession(argv []string, dir string, rows int, cols int, mode string) int {
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

	log.Printf("[CreateSession] id=%d argv=%v dir=%q rows=%d cols=%d mode=%q", id, argv, dir, rows, cols, mode)

	// Use configured default shell when no command specified
	if len(argv) == 0 && a.cfg.DefaultShell != "" {
		argv = []string{a.cfg.DefaultShell}
	}

	// Inject env vars for all sessions
	var env []string
	if a.tmuxAPIPort > 0 {
		env = append(env, fmt.Sprintf("MTUI_PORT=%d", a.tmuxAPIPort))
	}
	if mode == "claude" || mode == "claude-yolo" {
		env = append(env, fmt.Sprintf("MULTITERMINAL_SESSION_ID=%d", id))
	}

	sess := terminal.NewSession(id, rows, cols)
	if err := sess.Start(argv, dir, env); err != nil {
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

// SaveTabs persists the current tab/pane layout to disk so it can be
// restored on next startup.
func (a *AppService) SaveTabs(state config.SessionState) {
	if a.safeMode {
		log.Println("[SaveTabs] skipped (safe-mode)")
		return
	}
	log.Printf("[SaveTabs] saving %d tabs", len(state.Tabs))
	if err := config.SaveSession(state); err != nil {
		log.Printf("[SaveTabs] error: %v", err)
	}
}

// LoadTabs returns the previously saved tab/pane layout, or nil.
func (a *AppService) LoadTabs() *config.SessionState {
	if a.safeMode {
		log.Println("[LoadTabs] skipped (safe-mode)")
		return nil
	}
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



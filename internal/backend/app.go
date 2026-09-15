// Package backend provides the Wails-bound application struct that bridges
// the Go PTY session management with the Svelte frontend.
package backend

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/config"
	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hub"
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
	app        *application.App           // Wails v3 application instance
	mainWindow *application.WebviewWindow // main window reference for dialogs
	serviceCtx context.Context            // context from ServiceStartup
	cfg        config.Config
	health     config.HealthState
	// host owns the terminal sessions: a hub.Embedded runs them in this
	// process and ends them with it, a hub.Remote leaves them to the daemon.
	// Which one is decided by config.SessionHost (see newSessionHost).
	host               hub.Host
	launches           map[int]launchSpec // how each session was started (for ResumeSession)
	queues             map[int]*sessionQueue
	finishStates       map[int]*finishState  // active worktree-finish flows, keyed by session ID
	sessionIssues      map[int]*sessionIssue // issue linked to each session
	mu                 sync.Mutex
	finishMu           sync.Mutex // serializes merge+cleanup globally (index.lock, TOCTOU)
	cancelAll          context.CancelFunc
	batcher            *outputBatcher
	batcherOnce        sync.Once
	resolvedClaudePath string
	claudeDetected     bool
	winMgr             *windowManager // tracks all open windows for multi-window support
	detachCount        int            // monotonic counter for detached window IDs
	safeMode           bool
	sessionBackup      *config.SessionState // populated in safe-mode; restored on shutdown
	hookMgr            *HookManager
	resolvedCodexPath  string
	codexDetected      bool
	resolvedGeminiPath string
	geminiDetected     bool
	tmuxAPIPort        int           // port for the tmux shim HTTP API
	mcpServerPort      int           // port for the agent-control MCP server
	focusToken         string        // token a focus request must present (see app_notify.go)
	bindWarnings       []BindWarning // listeners that failed to start, surfaced via CheckHealth
	bindWarningsMu     sync.Mutex
	agentSessions      map[int]AgentSessionInfo    // sessions spawned via SpawnAgentSession (agent-control)
	sessionMode        map[int]string              // mode ("claude"/"shell"/...) each session was created with, across all windows
	chatSessions       map[string]*ChatSession     // active chat sessions keyed by conversation ID
	chatBuffers        map[string]*strings.Builder // buffered assistant text per conversation
	worktreeStateMu    sync.Mutex
	worktreeState      map[int]worktreeState
	// lastProbedCwd is the cwd each session was last probed for with git, kept
	// regardless of the verdict. Guarded by worktreeStateMu. Hook events arrive
	// every 100ms, so without this the worktree probe in
	// handleWorktreeCwdUpdate would run git twice per tick per session.
	lastProbedCwd map[int]string
	// emitWorktreeEvent is a seam for testing; production wiring assigns it to
	// a.app.Event.Emit in setupHooks. Never nil-checked directly — callers use
	// the emitWorktreeEventSafe helper below.
	emitWorktreeEvent func(name string, payload any)
	// worktreeProbe is a seam for testing, defaulting to defaultWorktreeProbe.
	// It answers "is dir a linked worktree, and on which branch" — see
	// app_worktree_detect.go.
	worktreeProbe func(dir string) (path, branch string, ok bool)
	// issueProgressHook is a seam for testing, nil in production. It fires on
	// every reportIssueProgress call ahead of any config gate, so a test can
	// count how often one real completion reports progress — the duplicate
	// reporting from #188, where the hook callback and the scan loop each ran
	// the side effects of the same transition.
	issueProgressHook func(sessionID int, event issueProgressEvent)
}

// NewAppService creates a new AppService instance for Wails v3 service pattern.
func NewAppService(app *application.App, cfg config.Config, safeMode bool) *AppService {
	svc := &AppService{
		app:           app,
		cfg:           cfg,
		launches:      make(map[int]launchSpec),
		queues:        make(map[int]*sessionQueue),
		finishStates:  make(map[int]*finishState),
		sessionIssues: make(map[int]*sessionIssue),
		agentSessions: make(map[int]AgentSessionInfo),
		sessionMode:   make(map[int]string),
		chatSessions:  make(map[string]*ChatSession),
		chatBuffers:   make(map[string]*strings.Builder),
		worktreeState: make(map[int]worktreeState),
		lastProbedCwd: make(map[int]string),
		winMgr:        newWindowManager(app),
		safeMode:      safeMode,
	}
	svc.host = svc.newSessionHost()
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

	// Enable Claude voice dictation by default (settings.json only — no CLI flag exists)
	go a.setupVoice()

	// Start periodic scanner for activity and token detection
	scanCtx, cancel := context.WithCancel(ctx)
	a.cancelAll = cancel
	a.outputBatch() // ensure the batcher is initialized before batchLoop starts
	go a.scanLoop(scanCtx)
	go a.batchLoop(scanCtx)
	go a.scheduleLoop(scanCtx)
	go a.idleSuspendLoop(scanCtx)

	// Register custom protocol for notification clicks, then bring up the
	// loopback listeners (focus signal, agent-control MCP server).
	registerProtocol()
	a.startLocalListeners()

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
	// Releasing the host means different things by design: the embedded host
	// owns its sessions and ends them (killing each process tree first, which
	// the old shutdown loop did not, leaving descendants holding handles
	// inside worktrees, #185), while a daemon host is merely disconnected and
	// keeps every agent running for the next window.
	a.host.Release()

	// Withdraw the published loopback ports so no helper process dials a port
	// this instance no longer owns.
	a.releaseDiscoveryRecords()

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
// mode must be "shell", "claude", "claude-auto", or "claude-yolo"; it controls env injection.
func (a *AppService) CreateSession(argv []string, dir string, rows int, cols int, mode string) int {
	// The ID has to exist before the environment can, because part of that
	// environment names the session: the hook and the statusline shim report
	// back with MULTITERMINAL_SESSION_ID.
	id, err := a.host.Reserve()
	if err != nil {
		log.Printf("[CreateSession] ERROR: reserve: %v", err)
		return -1
	}

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

	// Inject env vars for all sessions. Shared with ResumeSession so a woken
	// pane gets an identical environment (app_suspend.go).
	env := a.sessionEnv(id, dir, mode)

	spec := hub.CreateSpec{
		ID: id, Argv: argv, Dir: dir, Rows: rows, Cols: cols, Mode: mode, Env: env,
	}
	// The Claude session UUID lives only in the argv the frontend built. Pass
	// it in so a pane can be resumed even before the first hook event; the
	// hook-reported id overwrites it as soon as it arrives.
	if isClaudeMode(mode) {
		spec.ResumeID = claudeSessionIDFromArgv(argv)
	}

	a.rememberLaunch(id, argv, dir, mode)
	if _, err := a.host.Create(spec); err != nil {
		errMsg := fmt.Sprintf("Session start failed: %v", err)
		log.Printf("[CreateSession] ERROR: %s", errMsg)
		a.mu.Lock()
		delete(a.launches, id)
		a.mu.Unlock()
		if a.app != nil {
			a.app.Event.Emit("terminal:error", TerminalErrorEvent{ID: id, Message: errMsg})
		}
		return -1
	}
	log.Printf("[CreateSession] session %d started successfully", id)

	a.mu.Lock()
	a.sessionMode[id] = mode
	a.mu.Unlock()

	a.streamSession(id)
	return id
}

// WriteToSession sends raw input data (base64-encoded) to a session's PTY.
func (a *AppService) WriteToSession(id int, b64data string) {
	summary, err := a.host.Get(id)
	if err != nil {
		return
	}
	data, err := base64.StdEncoding.DecodeString(b64data)
	if err != nil {
		return
	}
	// A sleeping pane has no PTY: the write would fail with ErrSuspended and the
	// keystroke would vanish. Wake it instead — typing into a pane is exactly
	// the user gesture that means "I want this back" (design D7). The keystroke
	// that triggered the wake is dropped on purpose; replaying it into a Claude
	// TUI that is still replaying its transcript would land somewhere random.
	if summary.Asleep() {
		log.Printf("[suspend] session %d: input received while asleep — waking up", id)
		a.wakeSession(id)
		return
	}
	_ = a.host.Write(id, data)
}

// ResizeSession updates the PTY and screen buffer dimensions.
func (a *AppService) ResizeSession(id int, rows int, cols int) {
	_ = a.host.Resize(id, rows, cols)
}

// CloseSession terminates a session and removes it.
// The session is closed asynchronously but removed from the map only
// after Close() completes, ensuring streamOutput drains all buffered
// data before the session is gone.
func (a *AppService) CloseSession(id int) {
	if !a.hasSession(id) {
		return
	}
	// Report "close" progress before removing the issue link
	a.reportIssueProgress(id, progressClose, a.getSessionCost(id))

	go func() {
		// The host kills the whole process tree before closing and blocks
		// until the process is gone, so the output pump has drained first.
		// Killing the tree matters because Close only ends the root process;
		// once it is gone taskkill /T can no longer reach its descendants
		// (#185).
		_ = a.host.Close(id)
		a.mu.Lock()
		delete(a.launches, id)
		delete(a.queues, id)
		delete(a.finishStates, id)
		delete(a.sessionIssues, id)
		delete(a.sessionMode, id)
		a.mu.Unlock()
		a.worktreeStateMu.Lock()
		delete(a.worktreeState, id)
		delete(a.lastProbedCwd, id)
		a.worktreeStateMu.Unlock()
		// Clean up per-session activity tracking to prevent memory leak
		cleanupActivityTracking(id)
		cleanupNameTracking(id)
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

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

	"github.com/patrick-goecommerce/multiterminal/internal/config"
	"github.com/patrick-goecommerce/multiterminal/internal/terminal"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main Wails application struct. All exported methods are
// automatically available to the frontend via generated TypeScript bindings.
type App struct {
	ctx       context.Context
	cfg       config.Config
	sessions  map[int]*terminal.Session
	mu        sync.Mutex
	nextID    int
	cancelAll context.CancelFunc
}

// NewApp creates a new App instance with the given configuration.
func NewApp(cfg config.Config) *App {
	return &App{
		cfg:      cfg,
		sessions: make(map[int]*terminal.Session),
	}
}

// Startup is called when the Wails app starts. It receives the app context.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	// Start periodic scanner for activity and token detection
	scanCtx, cancel := context.WithCancel(ctx)
	a.cancelAll = cancel
	go a.scanLoop(scanCtx)
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
func (a *App) CloseSession(id int) {
	a.mu.Lock()
	sess := a.sessions[id]
	delete(a.sessions, id)
	a.mu.Unlock()
	if sess != nil {
		go sess.Close()
	}
}

// GetConfig returns the current application configuration.
func (a *App) GetConfig() config.Config {
	return a.cfg
}

// SaveConfig saves the given config to disk and updates the in-memory copy.
func (a *App) SaveConfig(cfg config.Config) {
	a.cfg = cfg
	if err := config.Save(cfg); err != nil {
		log.Printf("[SaveConfig] error: %v", err)
	}
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
func (a *App) streamOutput(id int, sess *terminal.Session) {
	for {
		select {
		case data, ok := <-sess.RawOutputCh:
			if !ok {
				return
			}
			b64 := base64.StdEncoding.EncodeToString(data)
			runtime.EventsEmit(a.ctx, "terminal:output", id, b64)
		case <-sess.Done():
			return
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

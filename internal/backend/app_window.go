package backend

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// detachedTabStates temporarily holds serialised tab state for windows being
// created via DetachTab. The secondary window fetches and clears it via
// GetDetachedTabState once it has loaded.
var detachedTabStates = struct {
	mu     sync.Mutex
	states map[string]string
}{states: make(map[string]string)}

// windowEntry tracks one open window and the tab IDs it currently owns.
type windowEntry struct {
	Window       *application.WebviewWindow
	TabIDs       []string
	tabStateJSON string // latest state pushed by the secondary window's store subscription
}

// windowManager tracks all open windows.
type windowManager struct {
	mu      sync.Mutex
	windows map[string]*windowEntry
	app     *application.App
	// quitting makes the main window's quit happen once: Quit closes the
	// remaining windows, and closing the main one again must not start a
	// second quit on top of the first.
	quitting sync.Once
}

func newWindowManager(app *application.App) *windowManager {
	return &windowManager{
		windows: make(map[string]*windowEntry),
		app:     app,
	}
}

// register records a window. tabState is the state its tabs start out with,
// so a window that closes before it ever called SaveWindowTabs still hands
// its tabs back (see DetachTab).
func (wm *windowManager) register(id string, win *application.WebviewWindow, tabIDs []string, tabState string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.windows[id] = &windowEntry{Window: win, TabIDs: tabIDs, tabStateJSON: tabState}
}

// others counts the open windows apart from the one named.
func (wm *windowManager) others(id string) int {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	n := len(wm.windows)
	if _, ok := wm.windows[id]; ok {
		n--
	}
	return n
}

// SetMainWindow stores the main window reference for dialog and focus
// operations, and ties the app's life to it.
func (a *AppService) SetMainWindow(w *application.WebviewWindow) {
	a.mainWindow = w
	a.winMgr.register("main", w, nil, "")
	w.RegisterHook(events.Common.WindowClosing, func(*application.WindowEvent) {
		a.onMainWindowClosing()
	})
}

// onMainWindowClosing quits the app when the main window closes while a
// detached window is still open.
//
// Wails only quits once the last window is gone, so the app used to live on
// in the detached window. The main window's tabs were gone, but their
// sessions kept running with nothing on screen, and the window left behind
// cannot stand in for the main one: it saves no layout, draws no panes for
// agent sessions and runs no keep-alive. Everything that happens on a normal
// quit happens here too: the sessions end with the in-process host and stay
// with the daemon.
func (a *AppService) onMainWindowClosing() {
	if a.winMgr.others("main") == 0 {
		return // the last window; Wails quits on its own
	}
	a.winMgr.quitting.Do(func() {
		log.Printf("[WindowManager] main window closing with detached windows open — quitting")
		go a.app.Quit() // not from inside the hook: Quit waits on the main thread
	})
}

func (wm *windowManager) unregister(id string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	delete(wm.windows, id)
}

// WindowInfo is returned to the frontend.
type WindowInfo struct {
	ID     string   `json:"id"     yaml:"id"`
	TabIDs []string `json:"tabIds" yaml:"tab_ids"`
}

// DetachTab creates a new Wails window for the given tab.
// tabStateJSON is the serialised tab object from the frontend store; it is
// stored temporarily so the new window can retrieve it via GetDetachedTabState.
// Returns the new window ID.
func (a *AppService) DetachTab(tabID string, sourceWindowID string, tabStateJSON string) (string, error) {
	newID := fmt.Sprintf("win-%d", a.nextDetachID())
	url := fmt.Sprintf("/?windowId=%s&tabs=%s", newID, tabID)

	// Get source window position for offset
	var x, y int
	a.winMgr.mu.Lock()
	if src, ok := a.winMgr.windows[sourceWindowID]; ok && src.Window != nil {
		x, y = src.Window.Position()
	}
	a.winMgr.mu.Unlock()

	win := a.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Multiterminal",
		Width:  1200,
		Height: 800,
		X:      x + 30,
		Y:      y + 30,
		URL:    url,
	})

	// Seeded with the tab it was created for. The window replaces it via
	// SaveWindowTabs, but only 300 ms after its store first changes, and a
	// window closed before that used to take its tab (and the sessions in it)
	// with it.
	a.winMgr.register(newID, win, []string{tabID}, windowStateOf(tabStateJSON))

	// Store serialised tab state for the new window to pick up on load.
	if tabStateJSON != "" {
		detachedTabStates.mu.Lock()
		detachedTabStates.states[newID] = tabStateJSON
		detachedTabStates.mu.Unlock()
	}

	// On close: emit window:tabs-merged with the last state pushed by SaveWindowTabs.
	// This avoids the fire-and-forget IPC race: the secondary window saves its tab
	// state proactively on every change, so the backend always has a fresh copy.
	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		a.winMgr.mu.Lock()
		var tabState string
		if entry, ok := a.winMgr.windows[newID]; ok {
			tabState = entry.tabStateJSON
			delete(a.winMgr.windows, newID)
		}
		a.winMgr.mu.Unlock()

		if tabState != "" {
			a.app.Event.Emit("window:tabs-merged", map[string]interface{}{
				"fromWindowId": newID,
				"tabState":     tabState,
			})
		}
		log.Printf("[WindowManager] window %s closing, tabState present=%v", newID, tabState != "")
	})

	win.Show()
	log.Printf("[DetachTab] created window %s for tab %s", newID, tabID)
	return newID, nil
}

// windowStateOf wraps one tab's state in the shape SaveWindowTabs stores and
// window:tabs-merged carries, {"tabs": [...]}. Anything that is not a JSON
// object yields no state rather than one the main window cannot parse.
func windowStateOf(tabStateJSON string) string {
	tab := json.RawMessage(strings.TrimSpace(tabStateJSON))
	if !json.Valid(tab) || len(tab) == 0 || tab[0] != '{' {
		return ""
	}
	out, err := json.Marshal(struct {
		Tabs []json.RawMessage `json:"tabs"`
	}{Tabs: []json.RawMessage{tab}})
	if err != nil {
		return ""
	}
	return string(out)
}

// GetDetachedTabState returns and clears the serialised tab state stored for
// the given window ID during DetachTab. Returns empty string if not found.
func (a *AppService) GetDetachedTabState(windowID string) string {
	detachedTabStates.mu.Lock()
	defer detachedTabStates.mu.Unlock()
	state := detachedTabStates.states[windowID]
	delete(detachedTabStates.states, windowID)
	return state
}

// SaveWindowTabs is called by a secondary window whenever its tab store changes.
// The state is persisted in the window entry so the backend can emit
// window:tabs-merged reliably when the window closes (no IPC race).
func (a *AppService) SaveWindowTabs(windowID string, tabStateJSON string) {
	a.winMgr.mu.Lock()
	defer a.winMgr.mu.Unlock()
	if entry, ok := a.winMgr.windows[windowID]; ok {
		entry.tabStateJSON = tabStateJSON
	}
}

// MergeWindowToMain is kept for compatibility but is no longer the primary
// merge path. The WindowClosing hook now handles the merge directly.
func (a *AppService) MergeWindowToMain(windowID string, tabState string) {
	log.Printf("[MergeWindowToMain] called for window %s (legacy path)", windowID)
	a.app.Event.Emit("window:tabs-merged", map[string]interface{}{
		"fromWindowId": windowID,
		"tabState":     tabState,
	})
	a.winMgr.unregister(windowID)
}

// GetOpenWindows returns info about all open windows.
func (a *AppService) GetOpenWindows() []WindowInfo {
	a.winMgr.mu.Lock()
	defer a.winMgr.mu.Unlock()
	result := make([]WindowInfo, 0, len(a.winMgr.windows))
	for id, entry := range a.winMgr.windows {
		result = append(result, WindowInfo{ID: id, TabIDs: entry.TabIDs})
	}
	return result
}

// OpenDashboardWindow opens the dashboard in a new window.
func (a *AppService) OpenDashboardWindow() (string, error) {
	newID := fmt.Sprintf("win-%d", a.nextDetachID())
	url := fmt.Sprintf("/?windowId=%s&view=dashboard", newID)

	win := a.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Multiterminal — Dashboard",
		Width:  900,
		Height: 600,
		URL:    url,
	})

	a.winMgr.register(newID, win, nil, "")

	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		a.winMgr.unregister(newID)
	})

	win.Show()
	log.Printf("[OpenDashboardWindow] created window %s", newID)
	return newID, nil
}

// nextDetachID returns a monotonically increasing ID for new windows.
func (a *AppService) nextDetachID() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.detachCount++
	return a.detachCount
}

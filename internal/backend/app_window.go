package backend

import (
	"fmt"
	"log"
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

// draggingTab holds info about the tab currently being dragged between windows.
// Set on dragstart, cleared on dragend or when claimed by a target window.
var draggingTab = struct {
	mu           sync.Mutex
	tabID        string
	windowID     string
	tabStateJSON string
}{}

// SetDraggingTab is called by the source window when a tab drag begins.
func (a *AppService) SetDraggingTab(tabID, windowID, tabStateJSON string) {
	draggingTab.mu.Lock()
	defer draggingTab.mu.Unlock()
	draggingTab.tabID = tabID
	draggingTab.windowID = windowID
	draggingTab.tabStateJSON = tabStateJSON
}

// ClaimDraggedTab is called by the target window when a tab is dropped on its
// tab bar. It returns the tab state JSON so the target can import the tab, and
// emits window:tab-claimed so the source window removes the tab.
// Returns empty string if nothing is being dragged (or wrong tabID).
func (a *AppService) ClaimDraggedTab(tabID string) string {
	draggingTab.mu.Lock()
	// Accept by specific ID or by "whatever is currently dragging" (empty ID).
	if draggingTab.tabID == "" || (tabID != "" && draggingTab.tabID != tabID) {
		draggingTab.mu.Unlock()
		return ""
	}
	sourceWindowID := draggingTab.windowID
	tabStateJSON := draggingTab.tabStateJSON
	claimedTabID := draggingTab.tabID
	draggingTab.tabID = ""
	draggingTab.windowID = ""
	draggingTab.tabStateJSON = ""
	draggingTab.mu.Unlock()

	// Tell the source window to close this tab.
	a.app.Event.Emit("window:tab-claimed", map[string]string{
		"windowId": sourceWindowID,
		"tabId":    claimedTabID,
	})
	return tabStateJSON
}

// ClearDraggingTab is called when a drag ends without a cross-window drop.
func (a *AppService) ClearDraggingTab() {
	draggingTab.mu.Lock()
	defer draggingTab.mu.Unlock()
	draggingTab.tabID = ""
	draggingTab.windowID = ""
	draggingTab.tabStateJSON = ""
}

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
}

func newWindowManager(app *application.App) *windowManager {
	return &windowManager{
		windows: make(map[string]*windowEntry),
		app:     app,
	}
}

func (wm *windowManager) register(id string, win *application.WebviewWindow, tabIDs []string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.windows[id] = &windowEntry{Window: win, TabIDs: tabIDs}
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

	a.winMgr.register(newID, win, []string{tabID})

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

// nextDetachID returns a monotonically increasing ID for new windows.
func (a *AppService) nextDetachID() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.detachCount++
	return a.detachCount
}

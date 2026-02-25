package backend

import (
	"fmt"
	"log"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// windowEntry tracks one open window and the tab IDs it currently owns.
type windowEntry struct {
	Window *application.WebviewWindow
	TabIDs []string
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

func (wm *windowManager) getWindowForTab(tabID string) string {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	for id, entry := range wm.windows {
		for _, t := range entry.TabIDs {
			if t == tabID {
				return id
			}
		}
	}
	return ""
}

func (wm *windowManager) moveTab(tabID, targetWindowID string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	// Remove from current window
	for _, entry := range wm.windows {
		for i, t := range entry.TabIDs {
			if t == tabID {
				entry.TabIDs = append(entry.TabIDs[:i], entry.TabIDs[i+1:]...)
				break
			}
		}
	}
	// Add to target
	if target, ok := wm.windows[targetWindowID]; ok {
		target.TabIDs = append(target.TabIDs, tabID)
	}
}

// WindowInfo is returned to the frontend.
type WindowInfo struct {
	ID     string   `json:"id"`
	TabIDs []string `json:"tabIds"`
}

// DetachTab creates a new Wails window for the given tab.
// Returns the new window ID.
func (a *AppService) DetachTab(tabID string, sourceWindowID string) (string, error) {
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

	win.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		a.app.Event.Emit("window:before-close", map[string]string{"windowId": newID})
		a.winMgr.unregister(newID)
		log.Printf("[WindowManager] window %s closing", newID)
	})

	win.Show()
	log.Printf("[DetachTab] created window %s for tab %s", newID, tabID)
	return newID, nil
}

// MergeWindowToMain is called by a secondary window before it closes.
// tabState is the serialized tab state JSON from the frontend.
func (a *AppService) MergeWindowToMain(windowID string, tabState string) {
	log.Printf("[MergeWindowToMain] merging window %s to main", windowID)
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

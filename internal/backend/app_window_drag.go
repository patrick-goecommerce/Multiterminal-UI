package backend

import "sync"

// Dragging a tab from one window onto another.

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

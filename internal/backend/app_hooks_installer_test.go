package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHookInstaller_Empty(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	os.WriteFile(settingsPath, []byte(`{}`), 0644)

	hi := newHookInstaller(settingsPath, "powershell -File hook.ps1")
	if err := hi.Install(); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	var result map[string]any
	json.Unmarshal(data, &result)
	hooks, ok := result["hooks"].(map[string]any)
	if !ok {
		t.Fatalf("hooks key missing in settings.json")
	}
	if _, exists := hooks["PreToolUse"]; !exists {
		t.Error("PreToolUse hook not installed")
	}
	if _, exists := hooks["PermissionRequest"]; !exists {
		t.Error("PermissionRequest hook not installed")
	}
	if _, exists := hooks["UserPromptSubmit"]; !exists {
		t.Error("UserPromptSubmit hook not installed — needed for auto pane naming")
	}
}

func TestHookInstaller_Idempotent(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	os.WriteFile(settingsPath, []byte(`{}`), 0644)

	hi := newHookInstaller(settingsPath, "powershell -File hook.ps1")
	hi.Install()

	data1, _ := os.ReadFile(settingsPath)
	hi.Install() // second install
	data2, _ := os.ReadFile(settingsPath)

	if string(data1) != string(data2) {
		t.Error("second Install() changed the file — not idempotent")
	}
}

func TestHookInstaller_Backup(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	os.WriteFile(settingsPath, []byte(`{"someKey": true}`), 0644)

	hi := newHookInstaller(settingsPath, "powershell -File hook.ps1")
	hi.Install()

	entries, _ := os.ReadDir(dir)
	hasBak := false
	for _, e := range entries {
		if strings.Contains(e.Name(), ".bak") {
			hasBak = true
		}
	}
	if !hasBak {
		t.Error("no .bak backup file created")
	}
}

func TestHookInstaller_MissingFile(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	// File does not exist — installer should create it

	hi := newHookInstaller(settingsPath, "powershell -File hook.ps1")
	if err := hi.Install(); err != nil {
		t.Fatalf("Install() on missing file should not error: %v", err)
	}

	if _, err := os.Stat(settingsPath); err != nil {
		t.Error("settings.json should have been created")
	}
}

func TestHookInstaller_BackfillsMissingEvents(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	// Simulate an OLDER Multiterminal install: only PreToolUse carries our
	// marker (the event set did not yet include UserPromptSubmit).
	oldCmd := "powershell -File hook.ps1 PreToolUse " + hookMarker
	existing := `{"hooks":{"PreToolUse":[{"hooks":[{"type":"command","command":"` + oldCmd + `"}]}]}}`
	os.WriteFile(settingsPath, []byte(existing), 0644)

	hi := newHookInstaller(settingsPath, "powershell -File hook.ps1")
	if err := hi.Install(); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	var result map[string]any
	json.Unmarshal(data, &result)
	hooks := result["hooks"].(map[string]any)

	// The new event must be backfilled.
	if _, exists := hooks["UserPromptSubmit"]; !exists {
		t.Error("UserPromptSubmit not backfilled into an older install")
	}

	// PreToolUse must NOT be duplicated — still exactly one marker entry.
	preToolUse, _ := hooks["PreToolUse"].([]any)
	markerCount := 0
	for _, entry := range preToolUse {
		e := entry.(map[string]any)
		inner := e["hooks"].([]any)
		for _, ih := range inner {
			cmd, _ := ih.(map[string]any)["command"].(string)
			if strings.Contains(cmd, hookMarker) {
				markerCount++
			}
		}
	}
	if markerCount != 1 {
		t.Errorf("PreToolUse has %d marker entries, want 1 (no duplication)", markerCount)
	}
}

// TestHookInstaller_RemovesLegacyPS1Registration reproduces the reported bug:
// an install that predates the GUI hook binary registered
// `powershell -File …\hook_handler.ps1 <Event>`. Newer builds delete that .ps1
// file but left the settings.json entry behind, so every hook event failed with
// "Das Argument … für den -File-Parameter ist nicht vorhanden".
func TestHookInstaller_RemovesLegacyPS1Registration(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	legacy := `powershell -NonInteractive -File \"C:\\Users\\x\\AppData\\Roaming\\Multiterminal\\hook_handler.ps1\" Stop`
	existing := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"` + legacy + `"}]}]}}`
	os.WriteFile(settingsPath, []byte(existing), 0644)

	hi := newHookInstaller(settingsPath, `"C:\mtui\mtui-hook.exe"`)
	if err := hi.Install(); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	if strings.Contains(string(data), "hook_handler.ps1") {
		t.Error("legacy hook_handler.ps1 registration still present in settings.json")
	}
	if !strings.Contains(string(data), "mtui-hook.exe") {
		t.Error("new hook binary was not registered")
	}
}

// TestHookInstaller_RemovesLegacyPS1EvenWhenMarkersComplete covers the exact
// user-visible state: the new binary is already registered for every event, so
// the early "already installed" return skipped cleanup and the dead .ps1 entry
// survived forever.
func TestHookInstaller_RemovesLegacyPS1EvenWhenMarkersComplete(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	command := `"C:\mtui\mtui-hook.exe"`

	// First install: every event carries our marker.
	os.WriteFile(settingsPath, []byte(`{}`), 0644)
	hi := newHookInstaller(settingsPath, command)
	if err := hi.Install(); err != nil {
		t.Fatalf("first Install() error: %v", err)
	}

	// Now inject a leftover legacy entry alongside the current one.
	data, _ := os.ReadFile(settingsPath)
	var settings map[string]any
	json.Unmarshal(data, &settings)
	hooks := settings["hooks"].(map[string]any)
	legacy := map[string]any{"hooks": []any{map[string]any{
		"type":    "command",
		"command": `powershell -NonInteractive -File "C:\Users\x\AppData\Roaming\Multiterminal\hook_handler.ps1" Stop`,
	}}}
	hooks["Stop"] = append(hooks["Stop"].([]any), legacy)
	out, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(settingsPath, out, 0644)

	if err := hi.Install(); err != nil {
		t.Fatalf("second Install() error: %v", err)
	}

	after, _ := os.ReadFile(settingsPath)
	if strings.Contains(string(after), "hook_handler.ps1") {
		t.Error("legacy hook_handler.ps1 entry survived because Install() returned early")
	}
}

// TestHookInstaller_LeavesForeignPS1HooksAlone guards the cleanup against
// collateral damage: only Multiterminal's own legacy script may be removed.
func TestHookInstaller_LeavesForeignPS1HooksAlone(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	foreign := `powershell -File \"C:\\other-tool\\hook_handler.ps1\" Stop`
	existing := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"` + foreign + `"}]}]}}`
	os.WriteFile(settingsPath, []byte(existing), 0644)

	hi := newHookInstaller(settingsPath, `"C:\mtui\mtui-hook.exe"`)
	hi.Install()

	data, _ := os.ReadFile(settingsPath)
	if !strings.Contains(string(data), `other-tool`) {
		t.Error("a foreign tool's hook was removed — cleanup must only target Multiterminal's own script")
	}
}

// TestHookInstaller_RepointsMovedBinary covers the portable-exe case: MTUI was
// moved to a different directory, so our own marker entries point at a binary
// that no longer exists. The marker check alone considers those "installed",
// which would leave every hook broken.
func TestHookInstaller_RepointsMovedBinary(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	os.WriteFile(settingsPath, []byte(`{}`), 0644)
	if err := newHookInstaller(settingsPath, `"C:\old\mtui-hook.exe"`).Install(); err != nil {
		t.Fatalf("first Install() error: %v", err)
	}

	// MTUI moved — install again from the new location.
	if err := newHookInstaller(settingsPath, `"C:\new\mtui-hook.exe"`).Install(); err != nil {
		t.Fatalf("second Install() error: %v", err)
	}

	data, _ := os.ReadFile(settingsPath)
	if strings.Contains(string(data), `C:\\old\\mtui-hook.exe`) {
		t.Error("hook entry still points at the old binary location")
	}

	var result map[string]any
	json.Unmarshal(data, &result)
	hooks := result["hooks"].(map[string]any)
	for _, event := range hookEvents {
		entries, _ := hooks[event].([]any)
		markerCount := 0
		for _, entry := range entries {
			for _, ih := range entry.(map[string]any)["hooks"].([]any) {
				cmd, _ := ih.(map[string]any)["command"].(string)
				if strings.Contains(cmd, hookMarker) {
					markerCount++
				}
			}
		}
		if markerCount != 1 {
			t.Errorf("%s has %d marker entries, want exactly 1", event, markerCount)
		}
	}
}

func TestHookInstaller_PreservesExistingHooks(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	// Pre-existing hook from another tool
	existing := `{"hooks":{"PreToolUse":[{"hooks":[{"type":"command","command":"other-tool"}]}]}}`
	os.WriteFile(settingsPath, []byte(existing), 0644)

	hi := newHookInstaller(settingsPath, "powershell -File hook.ps1")
	hi.Install()

	data, _ := os.ReadFile(settingsPath)
	var result map[string]any
	json.Unmarshal(data, &result)
	hooks := result["hooks"].(map[string]any)
	preToolUse := hooks["PreToolUse"].([]any)

	// Should have 2 entries: ours (prepended) + other-tool (preserved)
	if len(preToolUse) != 2 {
		t.Errorf("PreToolUse has %d entries, want 2 (ours + other-tool)", len(preToolUse))
	}
}

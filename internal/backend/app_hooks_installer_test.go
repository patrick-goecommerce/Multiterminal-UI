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

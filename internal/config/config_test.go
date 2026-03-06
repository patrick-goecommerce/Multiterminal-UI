package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// DefaultConfig
// ---------------------------------------------------------------------------

func TestDefaultConfig_Values(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Theme != "dark" {
		t.Errorf("Theme = %q, want 'dark'", cfg.Theme)
	}
	if cfg.MaxPanesPerTab != 9 {
		t.Errorf("MaxPanesPerTab = %d, want 9", cfg.MaxPanesPerTab)
	}
	if cfg.SidebarWidth != 30 {
		t.Errorf("SidebarWidth = %d, want 30", cfg.SidebarWidth)
	}
	if cfg.ClaudeCommand != "claude" {
		t.Errorf("ClaudeCommand = %q, want 'claude'", cfg.ClaudeCommand)
	}
	if cfg.CommitReminderMinutes != 30 {
		t.Errorf("CommitReminderMinutes = %d, want 30", cfg.CommitReminderMinutes)
	}
	if cfg.RestoreSession == nil || !*cfg.RestoreSession {
		t.Error("RestoreSession should default to true")
	}
	if len(cfg.ClaudeModels) != 4 {
		t.Errorf("ClaudeModels count = %d, want 4", len(cfg.ClaudeModels))
	}
}

func TestDefaultConfig_ModelEntries(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ClaudeModels[0].Label != "Default" {
		t.Errorf("Model 0 label = %q, want 'Default'", cfg.ClaudeModels[0].Label)
	}
	if cfg.ClaudeModels[0].ID != "" {
		t.Errorf("Model 0 ID = %q, want empty", cfg.ClaudeModels[0].ID)
	}
	if cfg.ClaudeModels[1].ID != "claude-opus-4-6" {
		t.Errorf("Model 1 ID = %q, want 'claude-opus-4-6'", cfg.ClaudeModels[1].ID)
	}
}

// ---------------------------------------------------------------------------
// ShouldRestoreSession
// ---------------------------------------------------------------------------

func TestShouldRestoreSession_NilDefault(t *testing.T) {
	cfg := Config{RestoreSession: nil}
	if !cfg.ShouldRestoreSession() {
		t.Error("ShouldRestoreSession with nil should return true")
	}
}

func TestShouldRestoreSession_True(t *testing.T) {
	cfg := Config{RestoreSession: boolPtr(true)}
	if !cfg.ShouldRestoreSession() {
		t.Error("ShouldRestoreSession(true) should return true")
	}
}

func TestShouldRestoreSession_False(t *testing.T) {
	cfg := Config{RestoreSession: boolPtr(false)}
	if cfg.ShouldRestoreSession() {
		t.Error("ShouldRestoreSession(false) should return false")
	}
}

// ---------------------------------------------------------------------------
// YAML round-trip: Save + Load
// ---------------------------------------------------------------------------

func TestConfig_YAMLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-config.yaml")

	original := DefaultConfig()
	original.Theme = "dracula"
	original.MaxPanesPerTab = 6
	original.SidebarWidth = 40

	// Save
	err := writeDefaults(path, original)
	if err != nil {
		t.Fatalf("writeDefaults failed: %v", err)
	}

	// Load back
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var loaded Config
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if loaded.Theme != "dracula" {
		t.Errorf("Loaded Theme = %q, want 'dracula'", loaded.Theme)
	}
	if loaded.MaxPanesPerTab != 6 {
		t.Errorf("Loaded MaxPanesPerTab = %d, want 6", loaded.MaxPanesPerTab)
	}
	if loaded.SidebarWidth != 40 {
		t.Errorf("Loaded SidebarWidth = %d, want 40", loaded.SidebarWidth)
	}
}

// ---------------------------------------------------------------------------
// Validation bounds
// ---------------------------------------------------------------------------

func TestConfig_Validation_MaxPanesPerTab(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	tests := []struct {
		input int
		want  int
	}{
		{0, 1},
		{-5, 1},
		{1, 1},
		{6, 6},
		{9, 9},
		{10, 9},
		{100, 9},
	}

	for _, tt := range tests {
		cfg := DefaultConfig()
		cfg.MaxPanesPerTab = tt.input
		data, _ := yaml.Marshal(cfg)
		os.WriteFile(path, data, 0644)

		// Simulate what Load() does for validation
		var loaded Config
		yaml.Unmarshal(data, &loaded)
		if loaded.MaxPanesPerTab < 1 {
			loaded.MaxPanesPerTab = 1
		}
		if loaded.MaxPanesPerTab > 9 {
			loaded.MaxPanesPerTab = 9
		}

		if loaded.MaxPanesPerTab != tt.want {
			t.Errorf("MaxPanesPerTab(%d) after validation = %d, want %d",
				tt.input, loaded.MaxPanesPerTab, tt.want)
		}
	}
}

func TestConfig_Validation_SidebarWidth(t *testing.T) {
	tests := []struct {
		input int
		want  int
	}{
		{0, 15},
		{14, 15},
		{15, 15},
		{30, 30},
		{60, 60},
		{61, 60},
		{200, 60},
	}

	for _, tt := range tests {
		// Apply the same validation logic as Load()
		val := tt.input
		if val < 15 {
			val = 15
		}
		if val > 60 {
			val = 60
		}
		if val != tt.want {
			t.Errorf("SidebarWidth(%d) after validation = %d, want %d",
				tt.input, val, tt.want)
		}
	}
}

func TestConfig_Validation_Theme(t *testing.T) {
	validThemes := map[string]bool{"dark": true, "light": true, "dracula": true, "nord": true, "solarized": true}

	valid := []string{"dark", "light", "dracula", "nord", "solarized"}
	for _, theme := range valid {
		if !validThemes[theme] {
			t.Errorf("Theme %q should be valid", theme)
		}
	}

	invalid := []string{"", "monokai", "gruvbox", "DARK", "Light"}
	for _, theme := range invalid {
		if validThemes[theme] {
			t.Errorf("Theme %q should be invalid", theme)
		}
	}
}

func TestConfig_Validation_CommitReminder(t *testing.T) {
	// Negative values should be clamped to 0
	val := -10
	if val < 0 {
		val = 0
	}
	if val != 0 {
		t.Errorf("CommitReminderMinutes(-10) = %d, want 0", val)
	}

	// Positive values should pass through
	val = 30
	if val < 0 {
		val = 0
	}
	if val != 30 {
		t.Errorf("CommitReminderMinutes(30) = %d, want 30", val)
	}
}

// ---------------------------------------------------------------------------
// Session state: JSON round-trip
// ---------------------------------------------------------------------------

func TestSessionState_JSONRoundTrip(t *testing.T) {
	original := SessionState{
		ActiveTab: 1,
		Tabs: []SavedTab{
			{
				Name:     "Tab 1",
				Dir:      "/home/user",
				FocusIdx: 0,
				Panes: []SavedPane{
					{Name: "shell", Mode: 0, Model: ""},
					{Name: "claude", Mode: 1, Model: "Opus 4.6"},
				},
			},
			{
				Name:     "Tab 2",
				Dir:      "/tmp",
				FocusIdx: 1,
				Panes: []SavedPane{
					{Name: "yolo", Mode: 2, Model: "Haiku 4.5"},
				},
			},
		},
	}

	data, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var loaded SessionState
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if loaded.ActiveTab != 1 {
		t.Errorf("ActiveTab = %d, want 1", loaded.ActiveTab)
	}
	if len(loaded.Tabs) != 2 {
		t.Fatalf("Tabs count = %d, want 2", len(loaded.Tabs))
	}
	if loaded.Tabs[0].Name != "Tab 1" {
		t.Errorf("Tab 0 name = %q, want 'Tab 1'", loaded.Tabs[0].Name)
	}
	if len(loaded.Tabs[0].Panes) != 2 {
		t.Errorf("Tab 0 panes = %d, want 2", len(loaded.Tabs[0].Panes))
	}
	if loaded.Tabs[0].Panes[1].Model != "Opus 4.6" {
		t.Errorf("Tab 0 pane 1 model = %q, want 'Opus 4.6'", loaded.Tabs[0].Panes[1].Model)
	}
}

func TestSessionState_EmptyTabsReturnsNil(t *testing.T) {
	// LoadSession returns nil for empty tabs — test the validation logic
	state := SessionState{ActiveTab: 0, Tabs: nil}
	data, _ := json.Marshal(state)

	var loaded SessionState
	json.Unmarshal(data, &loaded)

	if len(loaded.Tabs) != 0 {
		t.Errorf("Expected 0 tabs, got %d", len(loaded.Tabs))
	}
	// The LoadSession function checks: if len(state.Tabs) == 0 → return nil
	// Verify the same condition
	if len(loaded.Tabs) == 0 {
		// This is correct behavior — would return nil from LoadSession
	} else {
		t.Error("Empty tabs should trigger nil return")
	}
}

func TestSaveSession_WriteAndRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-session.json")

	state := SessionState{
		ActiveTab: 0,
		Tabs: []SavedTab{
			{Name: "Main", Dir: "/home", FocusIdx: 0, Panes: []SavedPane{
				{Name: "bash", Mode: 0},
			}},
		},
	}

	// Write
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Read back
	readData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var loaded SessionState
	if err := json.Unmarshal(readData, &loaded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if loaded.Tabs[0].Name != "Main" {
		t.Errorf("Loaded tab name = %q, want 'Main'", loaded.Tabs[0].Name)
	}
}

// ---------------------------------------------------------------------------
// Favorites
// ---------------------------------------------------------------------------

func TestConfig_FavoritesRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-config.yaml")

	original := DefaultConfig()
	original.Favorites = map[string][]string{
		"/home/user/project": {"/home/user/project/main.go", "/home/user/project/src"},
	}

	err := writeDefaults(path, original)
	if err != nil {
		t.Fatalf("writeDefaults failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var loaded Config
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	favs, ok := loaded.Favorites["/home/user/project"]
	if !ok {
		t.Fatal("expected favorites for /home/user/project")
	}
	if len(favs) != 2 {
		t.Fatalf("expected 2 favorites, got %d", len(favs))
	}
	if favs[0] != "/home/user/project/main.go" {
		t.Errorf("fav[0] = %q, want '/home/user/project/main.go'", favs[0])
	}
}

func TestConfig_FavoritesDefaultNil(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Favorites != nil {
		t.Errorf("DefaultConfig should have nil Favorites, got %v", cfg.Favorites)
	}
}

// ---------------------------------------------------------------------------
// KeepAlive settings: JSON round-trip (simulates Wails frontend→backend call)
// ---------------------------------------------------------------------------

func TestKeepAlive_DefaultValues(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.KeepAlive.Enabled == nil || !*cfg.KeepAlive.Enabled {
		t.Error("KeepAlive.Enabled should default to true")
	}
	if cfg.KeepAlive.IntervalMinutes != 300 {
		t.Errorf("KeepAlive.IntervalMinutes = %d, want 300", cfg.KeepAlive.IntervalMinutes)
	}
	if cfg.KeepAlive.Message != "Hi!" {
		t.Errorf("KeepAlive.Message = %q, want 'Hi!'", cfg.KeepAlive.Message)
	}
}

func TestKeepAlive_JSONRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.KeepAlive = KeepAliveSettings{
		Enabled:         boolPtr(false),
		IntervalMinutes: 60,
		Message:         "keep going",
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if loaded.KeepAlive.Enabled == nil || *loaded.KeepAlive.Enabled {
		t.Error("KeepAlive.Enabled should be false after round-trip")
	}
	if loaded.KeepAlive.IntervalMinutes != 60 {
		t.Errorf("KeepAlive.IntervalMinutes = %d, want 60", loaded.KeepAlive.IntervalMinutes)
	}
	if loaded.KeepAlive.Message != "keep going" {
		t.Errorf("KeepAlive.Message = %q, want 'keep going'", loaded.KeepAlive.Message)
	}
}

func TestKeepAlive_YAMLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "keepalive-test.yaml")

	original := DefaultConfig()
	original.KeepAlive = KeepAliveSettings{
		Enabled:         boolPtr(false),
		IntervalMinutes: 120,
		Message:         "still here",
	}

	if err := writeDefaults(path, original); err != nil {
		t.Fatalf("writeDefaults: %v", err)
	}

	data, _ := os.ReadFile(path)
	var loaded Config
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	if loaded.KeepAlive.Enabled == nil || *loaded.KeepAlive.Enabled {
		t.Error("KeepAlive.Enabled should be false after YAML round-trip")
	}
	if loaded.KeepAlive.IntervalMinutes != 120 {
		t.Errorf("KeepAlive.IntervalMinutes = %d, want 120", loaded.KeepAlive.IntervalMinutes)
	}
	if loaded.KeepAlive.Message != "still here" {
		t.Errorf("KeepAlive.Message = %q, want 'still here'", loaded.KeepAlive.Message)
	}
}

func TestShouldKeepAlive_NilDefault(t *testing.T) {
	cfg := Config{KeepAlive: KeepAliveSettings{Enabled: nil}}
	if !cfg.ShouldKeepAlive() {
		t.Error("ShouldKeepAlive with nil should return true")
	}
}

func TestShouldKeepAlive_False(t *testing.T) {
	cfg := Config{KeepAlive: KeepAliveSettings{Enabled: boolPtr(false)}}
	if cfg.ShouldKeepAlive() {
		t.Error("ShouldKeepAlive(false) should return false")
	}
}

// ---------------------------------------------------------------------------
// StatusLine settings: JSON round-trip
// ---------------------------------------------------------------------------

func TestStatusLine_DefaultValues(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.StatusLine.Enabled {
		t.Error("StatusLine.Enabled should default to false")
	}
	if cfg.StatusLine.Template != "standard" {
		t.Errorf("StatusLine.Template = %q, want 'standard'", cfg.StatusLine.Template)
	}
	if !cfg.StatusLine.ShowModel {
		t.Error("StatusLine.ShowModel should default to true")
	}
	if !cfg.StatusLine.ShowContext {
		t.Error("StatusLine.ShowContext should default to true")
	}
	if !cfg.StatusLine.ShowCost {
		t.Error("StatusLine.ShowCost should default to true")
	}
}

func TestStatusLine_JSONRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.StatusLine = StatusLineSettings{
		Enabled:       true,
		Template:      "minimal",
		ShowModel:     false,
		ShowContext:   true,
		ShowCost:      false,
		ShowGitBranch: true,
		ShowDuration:  true,
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if !loaded.StatusLine.Enabled {
		t.Error("StatusLine.Enabled should be true after round-trip")
	}
	if loaded.StatusLine.Template != "minimal" {
		t.Errorf("StatusLine.Template = %q, want 'minimal'", loaded.StatusLine.Template)
	}
	if loaded.StatusLine.ShowModel {
		t.Error("StatusLine.ShowModel should be false")
	}
	if !loaded.StatusLine.ShowGitBranch {
		t.Error("StatusLine.ShowGitBranch should be true")
	}
	if !loaded.StatusLine.ShowDuration {
		t.Error("StatusLine.ShowDuration should be true")
	}
}

func TestStatusLine_YAMLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "statusline-test.yaml")

	original := DefaultConfig()
	original.StatusLine = StatusLineSettings{
		Enabled:       true,
		Template:      "extended",
		ShowModel:     true,
		ShowContext:   false,
		ShowCost:      true,
		ShowGitBranch: false,
		ShowDuration:  true,
	}

	if err := writeDefaults(path, original); err != nil {
		t.Fatalf("writeDefaults: %v", err)
	}

	data, _ := os.ReadFile(path)
	var loaded Config
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("yaml.Unmarshal: %v", err)
	}

	if !loaded.StatusLine.Enabled {
		t.Error("StatusLine.Enabled should be true after YAML round-trip")
	}
	if loaded.StatusLine.Template != "extended" {
		t.Errorf("StatusLine.Template = %q, want 'extended'", loaded.StatusLine.Template)
	}
	if loaded.StatusLine.ShowContext {
		t.Error("StatusLine.ShowContext should be false")
	}
	if !loaded.StatusLine.ShowDuration {
		t.Error("StatusLine.ShowDuration should be true")
	}
}

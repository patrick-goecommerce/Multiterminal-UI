package backend

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readVoice(t *testing.T, path string) map[string]any {
	t.Helper()
	data, _ := os.ReadFile(path)
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("parse settings.json: %v", err)
	}
	voice, ok := result["voice"].(map[string]any)
	if !ok {
		t.Fatalf("voice key missing in settings.json")
	}
	return voice
}

func TestVoiceInstaller_Empty(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	os.WriteFile(settingsPath, []byte(`{}`), 0644)

	vi := newVoiceInstaller(settingsPath, "hold")
	if err := vi.Install(); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	voice := readVoice(t, settingsPath)
	if voice["enabled"] != true {
		t.Errorf("voice.enabled = %v, want true", voice["enabled"])
	}
	if voice["mode"] != "hold" {
		t.Errorf("voice.mode = %v, want hold", voice["mode"])
	}
}

func TestVoiceInstaller_MissingFile(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	vi := newVoiceInstaller(settingsPath, "tap")
	if err := vi.Install(); err != nil {
		t.Fatalf("Install() on missing file should not error: %v", err)
	}
	if _, err := os.Stat(settingsPath); err != nil {
		t.Error("settings.json should have been created")
	}
	voice := readVoice(t, settingsPath)
	if voice["mode"] != "tap" {
		t.Errorf("voice.mode = %v, want tap", voice["mode"])
	}
}

func TestVoiceInstaller_Idempotent(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	os.WriteFile(settingsPath, []byte(`{}`), 0644)

	vi := newVoiceInstaller(settingsPath, "hold")
	vi.Install()
	data1, _ := os.ReadFile(settingsPath)
	vi.Install()
	data2, _ := os.ReadFile(settingsPath)

	if string(data1) != string(data2) {
		t.Error("second Install() changed the file — not idempotent")
	}
}

// A user who already configured voice (e.g. ran /voice off) must not be overridden.
func TestVoiceInstaller_PreservesExistingVoice(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	os.WriteFile(settingsPath, []byte(`{"voice":{"enabled":false,"mode":"tap"}}`), 0644)

	vi := newVoiceInstaller(settingsPath, "hold")
	vi.Install()

	voice := readVoice(t, settingsPath)
	if voice["enabled"] != false {
		t.Errorf("voice.enabled = %v, want false (user setting preserved)", voice["enabled"])
	}
	if voice["mode"] != "tap" {
		t.Errorf("voice.mode = %v, want tap (user setting preserved)", voice["mode"])
	}
}

func TestVoiceInstaller_PreservesOtherKeys(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	os.WriteFile(settingsPath, []byte(`{"statusLine":{"type":"command"},"someKey":true}`), 0644)

	vi := newVoiceInstaller(settingsPath, "hold")
	vi.Install()

	data, _ := os.ReadFile(settingsPath)
	var result map[string]any
	json.Unmarshal(data, &result)
	if _, ok := result["statusLine"]; !ok {
		t.Error("statusLine key was lost")
	}
	if result["someKey"] != true {
		t.Error("someKey was lost")
	}
}

func TestVoiceInstaller_Backup(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")
	os.WriteFile(settingsPath, []byte(`{"someKey": true}`), 0644)

	vi := newVoiceInstaller(settingsPath, "hold")
	vi.Install()

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

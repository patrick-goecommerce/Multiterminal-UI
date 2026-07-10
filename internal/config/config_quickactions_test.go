package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultConfig_QuickActionsEmpty(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.FinishPrepPrompt != "" {
		t.Errorf("FinishPrepPrompt default = %q, want empty (falls back to built-in prompt)", cfg.FinishPrepPrompt)
	}
	if len(cfg.QuickActions) != 0 {
		t.Errorf("QuickActions default = %+v, want empty", cfg.QuickActions)
	}
}

func TestConfig_QuickActions_YAMLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-config.yaml")

	original := DefaultConfig()
	original.FinishPrepPrompt = "Push {{branch}}, open a PR against {{targetBranch}}, then /loop until merged."
	original.QuickActions = []QuickAction{
		{Label: "🔁", Prompt: "/loop 5m check status"},
		{Label: "🧪", Prompt: "Run the test suite and report failures"},
	}

	if err := writeDefaults(path, original); err != nil {
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

	if loaded.FinishPrepPrompt != original.FinishPrepPrompt {
		t.Errorf("loaded FinishPrepPrompt = %q, want %q", loaded.FinishPrepPrompt, original.FinishPrepPrompt)
	}
	if len(loaded.QuickActions) != 2 {
		t.Fatalf("loaded QuickActions len = %d, want 2", len(loaded.QuickActions))
	}
	if loaded.QuickActions[0].Label != "🔁" || loaded.QuickActions[0].Prompt != "/loop 5m check status" {
		t.Errorf("loaded QuickActions[0] = %+v, want {🔁, /loop 5m check status}", loaded.QuickActions[0])
	}
	if loaded.QuickActions[1].Label != "🧪" || loaded.QuickActions[1].Prompt != "Run the test suite and report failures" {
		t.Errorf("loaded QuickActions[1] = %+v, want {🧪, Run the test suite and report failures}", loaded.QuickActions[1])
	}
}

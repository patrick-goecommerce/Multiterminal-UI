package config

import (
	"path/filepath"
	"testing"
)

func TestRemoveTab_RemovesMatchingTab(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")

	state := SessionState{
		ActiveTab: 0,
		Tabs: []SavedTab{
			{Name: "Alpha", Dir: "/a"},
			{Name: "Beta", Dir: "/b"},
			{Name: "Gamma", Dir: "/c"},
		},
	}
	if err := saveSessionTo(path, state); err != nil {
		t.Fatal(err)
	}

	found, err := removeTabFrom(path, "Beta")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected tab to be found")
	}

	result := loadSessionFrom(path)
	if result == nil {
		t.Fatal("expected session to exist after removal")
	}
	if len(result.Tabs) != 2 {
		t.Fatalf("expected 2 tabs, got %d", len(result.Tabs))
	}
	for _, tab := range result.Tabs {
		if tab.Name == "Beta" {
			t.Fatal("Beta should have been removed")
		}
	}
}

func TestRemoveTab_ReturnsFalseWhenNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")

	state := SessionState{
		Tabs: []SavedTab{{Name: "Alpha", Dir: "/a"}},
	}
	if err := saveSessionTo(path, state); err != nil {
		t.Fatal(err)
	}

	found, err := removeTabFrom(path, "DoesNotExist")
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Fatal("expected tab not to be found")
	}
}

func TestRemoveTab_NoSessionFile(t *testing.T) {
	found, err := removeTabFrom(filepath.Join(t.TempDir(), "no-such-file.json"), "X")
	if err != nil {
		t.Fatal("expected no error when file missing")
	}
	if found {
		t.Fatal("expected false when file missing")
	}
}

func TestRemoveTab_ActiveTabClamped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")

	state := SessionState{
		ActiveTab: 2,
		Tabs: []SavedTab{
			{Name: "Alpha", Dir: "/a"},
			{Name: "Beta", Dir: "/b"},
			{Name: "Gamma", Dir: "/c"},
		},
	}
	if err := saveSessionTo(path, state); err != nil {
		t.Fatal(err)
	}

	found, err := removeTabFrom(path, "Gamma")
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("expected tab to be found")
	}

	result := loadSessionFrom(path)
	if result.ActiveTab >= len(result.Tabs) {
		t.Fatalf("ActiveTab %d out of bounds for %d tabs", result.ActiveTab, len(result.Tabs))
	}
}

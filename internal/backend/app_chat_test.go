package backend

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Conversation persistence — save/load cycle
// ---------------------------------------------------------------------------

func TestSaveAndLoadConversation(t *testing.T) {
	dir := t.TempDir()
	conv := Conversation{
		ID:        "test-conv-1",
		Title:     "Test Chat",
		Provider:  "claude",
		Model:     "claude-sonnet-4-5-20250929",
		Scope:     dir,
		CreatedAt: "2026-03-07T10:00:00Z",
		UpdatedAt: "2026-03-07T10:00:00Z",
		Messages: []ChatMessage{
			{ID: "m1", Role: "user", Content: "Hello", Timestamp: "2026-03-07T10:00:00Z"},
		},
	}

	if err := saveConversation(dir, conv); err != nil {
		t.Fatalf("save error: %v", err)
	}

	// Verify file exists
	path := conversationPath(dir, "test-conv-1")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("conversation file not created")
	}

	// Load and verify
	app := newTestApp()
	loaded, err := app.GetConversation(dir, "test-conv-1")
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if loaded.Title != "Test Chat" {
		t.Errorf("title = %q, want %q", loaded.Title, "Test Chat")
	}
	if loaded.Provider != "claude" {
		t.Errorf("provider = %q, want %q", loaded.Provider, "claude")
	}
	if len(loaded.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(loaded.Messages))
	}
	if loaded.Messages[0].Content != "Hello" {
		t.Errorf("message content = %q, want %q", loaded.Messages[0].Content, "Hello")
	}
}

func TestConversationPath(t *testing.T) {
	got := conversationPath("/project", "abc123")
	want := filepath.Join("/project", ".mtui", "chat", "conv-abc123.json")
	if got != want {
		t.Errorf("conversationPath() = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// GetConversations — lists all conversations
// ---------------------------------------------------------------------------

func TestGetConversations_Empty(t *testing.T) {
	dir := t.TempDir()
	app := newTestApp()
	convs := app.GetConversations(dir)
	if len(convs) != 0 {
		t.Errorf("expected 0 conversations, got %d", len(convs))
	}
}

func TestGetConversations_Multiple(t *testing.T) {
	dir := t.TempDir()

	for _, id := range []string{"conv1", "conv2"} {
		conv := Conversation{
			ID:       id,
			Title:    "Chat " + id,
			Messages: []ChatMessage{},
		}
		if err := saveConversation(dir, conv); err != nil {
			t.Fatal(err)
		}
	}

	app := newTestApp()
	convs := app.GetConversations(dir)
	if len(convs) != 2 {
		t.Errorf("expected 2 conversations, got %d", len(convs))
	}
}

// ---------------------------------------------------------------------------
// DeleteConversation
// ---------------------------------------------------------------------------

func TestDeleteConversation(t *testing.T) {
	dir := t.TempDir()
	conv := Conversation{ID: "del1", Title: "Delete me", Messages: []ChatMessage{}}
	saveConversation(dir, conv)

	app := newTestApp()
	if err := app.DeleteConversation(dir, "del1"); err != nil {
		t.Fatalf("delete error: %v", err)
	}

	path := conversationPath(dir, "del1")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("conversation file should be deleted")
	}
}

// ---------------------------------------------------------------------------
// RenameConversation
// ---------------------------------------------------------------------------

func TestRenameConversation(t *testing.T) {
	dir := t.TempDir()
	conv := Conversation{ID: "ren1", Title: "Old Title", Messages: []ChatMessage{}}
	saveConversation(dir, conv)

	app := newTestApp()
	if err := app.RenameConversation(dir, "ren1", "New Title"); err != nil {
		t.Fatalf("rename error: %v", err)
	}

	loaded, _ := app.GetConversation(dir, "ren1")
	if loaded.Title != "New Title" {
		t.Errorf("title = %q, want %q", loaded.Title, "New Title")
	}
}

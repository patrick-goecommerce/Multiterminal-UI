// Package backend provides chat conversation management via CLI tools.
package backend

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// Conversation represents a chat session with an AI provider.
type Conversation struct {
	ID             string        `json:"id" yaml:"id"`
	Title          string        `json:"title" yaml:"title"`
	Provider       string        `json:"provider" yaml:"provider"`
	Model          string        `json:"model" yaml:"model"`
	Scope          string        `json:"scope" yaml:"scope"`
	CreatedAt      string        `json:"created_at" yaml:"created_at"`
	UpdatedAt      string        `json:"updated_at" yaml:"updated_at"`
	Messages       []ChatMessage `json:"messages" yaml:"messages"`
	SessionID      string        `json:"session_id,omitempty" yaml:"session_id,omitempty"`
	PermissionMode string        `json:"permission_mode,omitempty" yaml:"permission_mode,omitempty"`
}

// ChatMessage represents a single message in a conversation.
type ChatMessage struct {
	ID         string `json:"id" yaml:"id"`
	Role       string `json:"role" yaml:"role"`
	Content    string `json:"content" yaml:"content"`
	Timestamp  string `json:"timestamp" yaml:"timestamp"`
	Cost       string `json:"cost" yaml:"cost"`
	Tokens     int    `json:"tokens" yaml:"tokens"`
	ToolName   string `json:"tool_name,omitempty" yaml:"tool_name,omitempty"`
	ToolInput  string `json:"tool_input,omitempty" yaml:"tool_input,omitempty"`
	ToolResult string `json:"tool_result,omitempty" yaml:"tool_result,omitempty"`
}

// CreateConversation creates a new chat conversation. When resumeID is set,
// the conversation adopts that claude session id so the first message resumes
// (`--resume`) an existing session instead of starting a fresh one — this is
// how a terminal pane keeps Claude's context when toggled to chat display.
func (a *AppService) CreateConversation(provider, model, scope, permissionMode, resumeID string) (Conversation, error) {
	if permissionMode == "" {
		permissionMode = "plan"
	}
	conv := Conversation{
		ID:             generateID(),
		Title:          "Neue Konversation",
		Provider:       provider,
		Model:          model,
		Scope:          scope,
		PermissionMode: permissionMode,
		SessionID:      resumeID,
		CreatedAt:      time.Now().Format(time.RFC3339),
		UpdatedAt:      time.Now().Format(time.RFC3339),
		Messages:       []ChatMessage{},
	}

	if err := saveConversation(scope, conv); err != nil {
		return conv, fmt.Errorf("save conversation: %w", err)
	}

	log.Printf("[chat] created conversation %s (provider=%s, scope=%s)", conv.ID, provider, scope)
	return conv, nil
}

// GetConversations returns all conversations for a project directory.
func (a *AppService) GetConversations(dir string) []Conversation {
	chatDir := filepath.Join(dir, ".mtui", "chat")
	entries, err := os.ReadDir(chatDir)
	if err != nil {
		return []Conversation{}
	}

	convs := make([]Conversation, 0)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(chatDir, e.Name()))
		if err != nil {
			continue
		}
		var conv Conversation
		if err := json.Unmarshal(data, &conv); err != nil {
			continue
		}
		convs = append(convs, conv)
	}
	return convs
}

// GetConversation returns a single conversation by ID.
func (a *AppService) GetConversation(dir string, convID string) (Conversation, error) {
	path := conversationPath(dir, convID)
	data, err := os.ReadFile(path)
	if err != nil {
		return Conversation{}, fmt.Errorf("read conversation: %w", err)
	}
	var conv Conversation
	if err := json.Unmarshal(data, &conv); err != nil {
		return Conversation{}, fmt.Errorf("parse conversation: %w", err)
	}
	return conv, nil
}

// DeleteConversation removes a conversation file.
func (a *AppService) DeleteConversation(dir string, convID string) error {
	path := conversationPath(dir, convID)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	log.Printf("[chat] deleted conversation %s", convID)
	return nil
}

// AddChatMessage adds a user message and sends it to the persistent chat session.
func (a *AppService) AddChatMessage(dir string, convID string, content string) error {
	conv, err := a.GetConversation(dir, convID)
	if err != nil {
		return fmt.Errorf("load conversation: %w", err)
	}

	msg := ChatMessage{ID: generateID(), Role: "user", Content: content, Timestamp: time.Now().Format(time.RFC3339)}
	conv.Messages = append(conv.Messages, msg)
	conv.UpdatedAt = time.Now().Format(time.RFC3339)

	if conv.Title == "Neue Konversation" && len(conv.Messages) == 1 {
		title := content
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		conv.Title = title
	}

	if err := saveConversation(dir, conv); err != nil {
		return fmt.Errorf("save conversation: %w", err)
	}

	sess, err := a.ensureChatSession(conv)
	if err != nil {
		a.emitChatError(convID, err.Error())
		return err
	}
	return sess.SendTurn(content)
}

// ensureChatSession returns the running session for conv, starting it if needed.
func (a *AppService) ensureChatSession(conv Conversation) (*ChatSession, error) {
	a.mu.Lock()
	if s, ok := a.chatSessions[conv.ID]; ok {
		a.mu.Unlock()
		return s, nil
	}
	a.mu.Unlock()

	sess, err := a.startChatProcess(conv.ID, conv.Scope, conv.Model, conv.PermissionMode, conv.SessionID)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	if existing, ok := a.chatSessions[conv.ID]; ok {
		// Lost the race: another caller started one first. Discard ours.
		a.mu.Unlock()
		sess.Close()
		return existing, nil
	}
	a.chatSessions[conv.ID] = sess
	a.mu.Unlock()
	return sess, nil
}

// CloseChatSession stops and forgets a chat session (called when a chat pane closes).
func (a *AppService) CloseChatSession(convID string) {
	a.mu.Lock()
	sess := a.chatSessions[convID]
	delete(a.chatSessions, convID)
	delete(a.chatBuffers, convID)
	a.mu.Unlock()
	if sess != nil {
		sess.Close()
	}
}

// RenameConversation updates a conversation's title.
func (a *AppService) RenameConversation(dir string, convID string, title string) error {
	conv, err := a.GetConversation(dir, convID)
	if err != nil {
		return err
	}
	conv.Title = title
	conv.UpdatedAt = time.Now().Format(time.RFC3339)
	return saveConversation(dir, conv)
}

// conversationPath returns the file path for a conversation.
func conversationPath(dir string, convID string) string {
	return filepath.Join(dir, ".mtui", "chat", "conv-"+convID+".json")
}

// saveConversation writes a conversation to disk.
func saveConversation(dir string, conv Conversation) error {
	chatDir := filepath.Join(dir, ".mtui", "chat")
	if err := os.MkdirAll(chatDir, 0o755); err != nil {
		return fmt.Errorf("create chat dir: %w", err)
	}
	data, err := json.MarshalIndent(conv, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal conversation: %w", err)
	}
	return os.WriteFile(conversationPath(dir, conv.ID), data, 0o644)
}

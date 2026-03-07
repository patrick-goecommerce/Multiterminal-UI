// Package backend provides chat response streaming via CLI tools.
package backend

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// streamChatResponse starts a CLI tool to get an AI response and streams it
// to the frontend via events.
func (a *AppService) streamChatResponse(dir string, conv Conversation) {
	convID := conv.ID

	// Build message history for context
	prompt := buildPrompt(conv.Messages)

	// Determine CLI command based on provider
	cmd, err := a.buildChatCommand(conv.Provider, conv.Model, conv.Scope, prompt)
	if err != nil {
		a.emitChatError(convID, err.Error())
		return
	}

	cmd.Dir = conv.Scope
	// Strip CLAUDECODE env var (same as session.go:Start)
	cmd.Env = filterEnv(os.Environ(), "CLAUDECODE")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		a.emitChatError(convID, fmt.Sprintf("stdout pipe: %v", err))
		return
	}

	if err := cmd.Start(); err != nil {
		a.emitChatError(convID, fmt.Sprintf("start: %v", err))
		return
	}

	// Read streaming output
	scanner := bufio.NewScanner(stdout)
	var fullResponse strings.Builder

	for scanner.Scan() {
		line := scanner.Text()
		delta := parseStreamDelta(line)
		if delta != "" {
			fullResponse.WriteString(delta)
			a.emitChatStream(convID, delta)
		}
	}

	if err := cmd.Wait(); err != nil {
		log.Printf("[chat] command error for conv %s: %v", convID, err)
	}

	// Save the complete response as an assistant message
	response := fullResponse.String()
	if response == "" {
		response = "(Keine Antwort)"
	}

	assistantMsg := ChatMessage{
		ID:        generateID(),
		Role:      "assistant",
		Content:   strings.TrimSpace(response),
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Reload conv and append message
	conv, err = a.GetConversation(dir, convID)
	if err == nil {
		conv.Messages = append(conv.Messages, assistantMsg)
		conv.UpdatedAt = time.Now().Format(time.RFC3339)
		if saveErr := saveConversation(dir, conv); saveErr != nil {
			log.Printf("[chat] save error: %v", saveErr)
		}
	}

	a.emitChatDone(convID, assistantMsg)
}

// buildChatCommand creates the exec.Cmd for the chosen provider.
func (a *AppService) buildChatCommand(provider string, model string, scope string, prompt string) (*exec.Cmd, error) {
	switch provider {
	case "claude":
		path := a.resolvedClaudePath
		if path == "" {
			path = "claude"
		}
		args := []string{"--output-format", "stream-json", "--print", prompt}
		if model != "" {
			args = append([]string{"--model", model}, args...)
		}
		return exec.Command(path, args...), nil

	case "codex":
		path := a.resolvedCodexPath
		if path == "" {
			path = "codex"
		}
		args := []string{"--quiet", prompt}
		if model != "" {
			args = append([]string{"--model", model}, args...)
		}
		return exec.Command(path, args...), nil

	case "gemini":
		path := a.resolvedGeminiPath
		if path == "" {
			path = "gemini"
		}
		return exec.Command(path, prompt), nil

	default:
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
}

// buildPrompt constructs the full prompt from message history.
func buildPrompt(messages []ChatMessage) string {
	if len(messages) == 0 {
		return ""
	}
	// For --print mode, send the last user message
	// The CLI doesn't support multi-turn in --print mode
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return messages[i].Content
		}
	}
	return messages[len(messages)-1].Content
}

// streamDelta represents a parsed chunk from stream-json output.
type streamDelta struct {
	Type    string `json:"type"`
	Content string `json:"content"`
	Text    string `json:"text"`
}

// parseStreamDelta extracts text content from a stream-json line.
func parseStreamDelta(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || line[0] != '{' {
		return ""
	}

	var delta streamDelta
	if err := json.Unmarshal([]byte(line), &delta); err != nil {
		// Not JSON, treat as plain text
		return line + "\n"
	}

	// Claude stream-json format
	if delta.Type == "content_block_delta" || delta.Type == "text" {
		if delta.Text != "" {
			return delta.Text
		}
		if delta.Content != "" {
			return delta.Content
		}
	}

	// Plain text response (codex/gemini)
	if delta.Content != "" {
		return delta.Content
	}

	return ""
}

// filterEnv returns environment variables with the named var removed.
func filterEnv(env []string, name string) []string {
	prefix := name + "="
	result := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			result = append(result, e)
		}
	}
	return result
}

// Chat event emitters

func (a *AppService) emitChatStream(convID string, delta string) {
	if a.app == nil {
		return
	}
	a.app.Event.Emit("chat:stream", map[string]string{
		"conversationId": convID,
		"delta":          delta,
	})
}

func (a *AppService) emitChatDone(convID string, msg ChatMessage) {
	if a.app == nil {
		return
	}
	a.app.Event.Emit("chat:done", map[string]interface{}{
		"conversationId": convID,
		"message":        msg,
	})
}

func (a *AppService) emitChatError(convID string, errMsg string) {
	if a.app == nil {
		return
	}
	log.Printf("[chat] error for conv %s: %s", convID, errMsg)
	a.app.Event.Emit("chat:error", map[string]string{
		"conversationId": convID,
		"error":          errMsg,
	})
}

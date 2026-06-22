// Package backend — chat conversation persistence helpers.
package backend

import (
	"fmt"
	"strings"
	"time"
)

// convScope returns the working directory for an active chat session.
func (a *AppService) convScope(convID string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if s, ok := a.chatSessions[convID]; ok {
		return s.Scope
	}
	return ""
}

// appendChatText accumulates streaming assistant text for a conversation.
func (a *AppService) appendChatText(convID, delta string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, ok := a.chatBuffers[convID]
	if !ok {
		b = &strings.Builder{}
		a.chatBuffers[convID] = b
	}
	b.WriteString(delta)
}

// takeChatText returns and clears the accumulated assistant text.
func (a *AppService) takeChatText(convID string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, ok := a.chatBuffers[convID]
	if !ok {
		return ""
	}
	text := b.String()
	delete(a.chatBuffers, convID)
	return text
}

// persistSessionID stores the claude session_id for resume.
func (a *AppService) persistSessionID(convID, sessionID string) {
	dir := a.convScope(convID)
	if dir == "" || sessionID == "" {
		return
	}
	conv, err := a.GetConversation(dir, convID)
	if err != nil {
		return
	}
	conv.SessionID = sessionID
	conv.UpdatedAt = time.Now().Format(time.RFC3339)
	_ = saveConversation(dir, conv)
}

// clearPersistedSessionID removes a stale claude session id so the next start
// launches a fresh session instead of a failing --resume.
func (a *AppService) clearPersistedSessionID(dir, convID string) {
	if dir == "" {
		return
	}
	conv, err := a.GetConversation(dir, convID)
	if err != nil {
		return
	}
	conv.SessionID = ""
	conv.UpdatedAt = time.Now().Format(time.RFC3339)
	_ = saveConversation(dir, conv)
}

// appendMessage loads, appends a message, and saves a conversation.
func (a *AppService) appendMessage(convID string, msg ChatMessage) {
	dir := a.convScope(convID)
	if dir == "" {
		return
	}
	conv, err := a.GetConversation(dir, convID)
	if err != nil {
		return
	}
	conv.Messages = append(conv.Messages, msg)
	conv.UpdatedAt = time.Now().Format(time.RFC3339)
	_ = saveConversation(dir, conv)
}

// flushAssistantMessage finalizes the streamed assistant text into a message.
func (a *AppService) flushAssistantMessage(convID string, ev ChatEvent) {
	text := strings.TrimSpace(a.takeChatText(convID))
	cost := ""
	if ev.CostUSD > 0 {
		cost = fmt.Sprintf("$%.4f", ev.CostUSD)
	}
	msg := ChatMessage{
		ID:        generateID(),
		Role:      "assistant",
		Content:   text,
		Timestamp: time.Now().Format(time.RFC3339),
		Cost:      cost,
		Tokens:    ev.OutputTokens,
	}
	// Persist only non-empty turns; a pure thinking/tool turn has no text and
	// would otherwise leave a blank bubble that reappears after reload.
	if text != "" {
		a.appendMessage(convID, msg)
	}
	a.emitChat("chat:done", map[string]interface{}{"conversationId": convID, "message": msg})
}

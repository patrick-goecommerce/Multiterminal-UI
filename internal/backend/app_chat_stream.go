// Package backend provides chat event dispatch and frontend emission helpers.
package backend

import (
	"log"
	"strings"
	"time"
)

// dispatchChatEvent maps a parsed ChatEvent to frontend events + persistence.
func (a *AppService) dispatchChatEvent(convID string, ev ChatEvent) {
	switch ev.Kind {
	case ChatEventInit:
		a.persistSessionID(convID, ev.SessionID)
		a.emitChat("chat:init", map[string]interface{}{"conversationId": convID, "model": ev.Model})
	case ChatEventText:
		a.appendChatText(convID, ev.Text)
		a.emitChat("chat:stream", map[string]interface{}{"conversationId": convID, "delta": ev.Text})
	case ChatEventToolUse, ChatEventToolResult:
		if m := chatEventToMessage(ev); m != nil {
			a.appendMessage(convID, *m)
			a.emitChat("chat:tool", map[string]interface{}{"conversationId": convID, "message": m})
		}
	case ChatEventResult:
		a.flushAssistantMessage(convID, ev)
	}
}

// chatEventToMessage converts tool events into persistable messages; nil otherwise.
func chatEventToMessage(ev ChatEvent) *ChatMessage {
	switch ev.Kind {
	case ChatEventToolUse:
		return &ChatMessage{ID: generateID(), Role: "tool", ToolName: ev.ToolName, Timestamp: time.Now().Format(time.RFC3339)}
	case ChatEventToolResult:
		return &ChatMessage{ID: generateID(), Role: "tool", ToolResult: ev.ToolResult, Timestamp: time.Now().Format(time.RFC3339)}
	}
	return nil
}

// emitChat emits a chat event to the frontend (no-op if app is nil, e.g. in tests).
func (a *AppService) emitChat(name string, payload map[string]interface{}) {
	if a.app == nil {
		return
	}
	a.app.Event.Emit(name, payload)
}

// emitChatError emits a chat:error event to the frontend.
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

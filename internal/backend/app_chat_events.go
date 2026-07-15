// Package backend — stream-json event parsing for chat sessions.
package backend

import "encoding/json"

// ChatEventKind classifies a parsed stream-json event.
type ChatEventKind string

const (
	ChatEventInit       ChatEventKind = "init"
	ChatEventText       ChatEventKind = "text"
	ChatEventToolUse    ChatEventKind = "tool_use"
	ChatEventToolResult ChatEventKind = "tool_result"
	ChatEventResult     ChatEventKind = "result"
)

// ChatEvent is the normalized domain event emitted to the frontend.
type ChatEvent struct {
	Kind         ChatEventKind `json:"kind"`
	Text         string        `json:"text,omitempty"`
	SessionID    string        `json:"sessionId,omitempty"`
	Model        string        `json:"model,omitempty"`
	ToolID       string        `json:"toolId,omitempty"`
	ToolName     string        `json:"toolName,omitempty"`
	ToolResult   string        `json:"toolResult,omitempty"`
	CostUSD      float64       `json:"costUsd,omitempty"`
	OutputTokens int           `json:"outputTokens,omitempty"`
}

// raw mirrors the relevant subset of Claude's stream-json schema.
type rawStreamLine struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	SessionID string `json:"session_id"`
	Model     string `json:"model"`
	// result
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	// tool_result (top-level user message form)
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	// nested stream_event
	Event struct {
		Type  string `json:"type"`
		Delta struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"delta"`
		ContentBlock struct {
			Type string `json:"type"`
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"content_block"`
	} `json:"event"`
}

// parseChatEvent normalizes one NDJSON line. ok=false means "ignore this line".
func parseChatEvent(line []byte) (ChatEvent, bool) {
	var r rawStreamLine
	if err := json.Unmarshal(line, &r); err != nil {
		return ChatEvent{}, false
	}
	switch r.Type {
	case "system":
		if r.Subtype == "init" {
			return ChatEvent{Kind: ChatEventInit, SessionID: r.SessionID, Model: r.Model}, true
		}
	case "result":
		return ChatEvent{Kind: ChatEventResult, CostUSD: r.TotalCostUSD, OutputTokens: r.Usage.OutputTokens}, true
	case "tool_result":
		return ChatEvent{Kind: ChatEventToolResult, ToolID: r.ToolUseID, ToolResult: r.Content}, true
	case "stream_event":
		switch r.Event.Type {
		case "content_block_delta":
			if r.Event.Delta.Type == "text_delta" && r.Event.Delta.Text != "" {
				return ChatEvent{Kind: ChatEventText, Text: r.Event.Delta.Text}, true
			}
		case "content_block_start":
			if r.Event.ContentBlock.Type == "tool_use" {
				return ChatEvent{Kind: ChatEventToolUse, ToolID: r.Event.ContentBlock.ID, ToolName: r.Event.ContentBlock.Name}, true
			}
		}
	}
	return ChatEvent{}, false
}

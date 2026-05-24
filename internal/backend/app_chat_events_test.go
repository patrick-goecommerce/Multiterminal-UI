package backend

import "testing"

func TestParseChatEvent_TextDelta(t *testing.T) {
	line := `{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hallo"}}}`
	ev, ok := parseChatEvent([]byte(line))
	if !ok || ev.Kind != ChatEventText || ev.Text != "Hallo" {
		t.Fatalf("got %+v ok=%v, want text 'Hallo'", ev, ok)
	}
}

func TestParseChatEvent_Init(t *testing.T) {
	line := `{"type":"system","subtype":"init","session_id":"abc-123","model":"claude-opus-4-7"}`
	ev, ok := parseChatEvent([]byte(line))
	if !ok || ev.Kind != ChatEventInit || ev.SessionID != "abc-123" || ev.Model != "claude-opus-4-7" {
		t.Fatalf("got %+v ok=%v, want init session abc-123", ev, ok)
	}
}

func TestParseChatEvent_ToolUse(t *testing.T) {
	line := `{"type":"stream_event","event":{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"t1","name":"Edit","input":{}}}}`
	ev, ok := parseChatEvent([]byte(line))
	if !ok || ev.Kind != ChatEventToolUse || ev.ToolName != "Edit" || ev.ToolID != "t1" {
		t.Fatalf("got %+v ok=%v, want tool_use Edit", ev, ok)
	}
}

func TestParseChatEvent_Result(t *testing.T) {
	line := `{"type":"result","subtype":"success","total_cost_usd":0.0123,"usage":{"output_tokens":42}}`
	ev, ok := parseChatEvent([]byte(line))
	if !ok || ev.Kind != ChatEventResult || ev.CostUSD != 0.0123 || ev.OutputTokens != 42 {
		t.Fatalf("got %+v ok=%v, want result cost 0.0123", ev, ok)
	}
}

func TestParseChatEvent_Ignored(t *testing.T) {
	if _, ok := parseChatEvent([]byte(`{"type":"stream_event","event":{"type":"message_start"}}`)); ok {
		t.Fatal("message_start should be ignored")
	}
	if _, ok := parseChatEvent([]byte("not json")); ok {
		t.Fatal("non-JSON should be ignored")
	}
}

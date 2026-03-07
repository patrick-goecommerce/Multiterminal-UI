package backend

import (
	"testing"
)

// ---------------------------------------------------------------------------
// buildPrompt — extracts last user message
// ---------------------------------------------------------------------------

func TestBuildPrompt_LastUserMessage(t *testing.T) {
	messages := []ChatMessage{
		{Role: "user", Content: "first question"},
		{Role: "assistant", Content: "answer"},
		{Role: "user", Content: "follow-up"},
	}
	got := buildPrompt(messages)
	if got != "follow-up" {
		t.Errorf("buildPrompt() = %q, want %q", got, "follow-up")
	}
}

func TestBuildPrompt_Empty(t *testing.T) {
	got := buildPrompt(nil)
	if got != "" {
		t.Errorf("buildPrompt(nil) = %q, want empty", got)
	}
}

func TestBuildPrompt_NoUserMessages(t *testing.T) {
	messages := []ChatMessage{
		{Role: "assistant", Content: "Hello"},
	}
	got := buildPrompt(messages)
	if got != "Hello" {
		t.Errorf("buildPrompt() = %q, want %q (last message fallback)", got, "Hello")
	}
}

// ---------------------------------------------------------------------------
// parseStreamDelta — extracts text from stream-json
// ---------------------------------------------------------------------------

func TestParseStreamDelta_ContentBlockDelta(t *testing.T) {
	line := `{"type":"content_block_delta","text":"Hello "}`
	got := parseStreamDelta(line)
	if got != "Hello " {
		t.Errorf("parseStreamDelta() = %q, want %q", got, "Hello ")
	}
}

func TestParseStreamDelta_TextType(t *testing.T) {
	line := `{"type":"text","text":"World"}`
	got := parseStreamDelta(line)
	if got != "World" {
		t.Errorf("parseStreamDelta() = %q, want %q", got, "World")
	}
}

func TestParseStreamDelta_ContentField(t *testing.T) {
	line := `{"content":"plain text response"}`
	got := parseStreamDelta(line)
	if got != "plain text response" {
		t.Errorf("parseStreamDelta() = %q, want %q", got, "plain text response")
	}
}

func TestParseStreamDelta_EmptyLine(t *testing.T) {
	got := parseStreamDelta("")
	if got != "" {
		t.Errorf("parseStreamDelta('') = %q, want empty", got)
	}
}

func TestParseStreamDelta_NonJSON(t *testing.T) {
	got := parseStreamDelta("plain text output")
	if got != "" {
		t.Errorf("parseStreamDelta(plain) = %q, want empty (non-JSON skipped)", got)
	}
}

func TestParseStreamDelta_UnrelatedJSON(t *testing.T) {
	line := `{"type":"message_start","message":{}}`
	got := parseStreamDelta(line)
	if got != "" {
		t.Errorf("parseStreamDelta(message_start) = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// filterEnv — removes named env var
// ---------------------------------------------------------------------------

func TestFilterEnv_Removes(t *testing.T) {
	env := []string{"PATH=/usr/bin", "CLAUDECODE=xxx", "HOME=/home/user"}
	got := filterEnv(env, "CLAUDECODE")
	if len(got) != 2 {
		t.Fatalf("expected 2 vars, got %d", len(got))
	}
	for _, e := range got {
		if e == "CLAUDECODE=xxx" {
			t.Error("CLAUDECODE should be removed")
		}
	}
}

func TestFilterEnv_NoMatch(t *testing.T) {
	env := []string{"PATH=/usr/bin", "HOME=/home/user"}
	got := filterEnv(env, "NONEXISTENT")
	if len(got) != 2 {
		t.Errorf("expected 2 vars unchanged, got %d", len(got))
	}
}

func TestFilterEnv_Empty(t *testing.T) {
	got := filterEnv(nil, "FOO")
	if len(got) != 0 {
		t.Errorf("expected empty result, got %d", len(got))
	}
}

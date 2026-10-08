package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestToolDetail(t *testing.T) {
	cases := []struct {
		tool, input, want string
	}{
		{"Bash", `{"command":"npm install @shipcloud/sdk --save"}`, "Bash: npm install @shipcloud/sdk --save"},
		{"Edit", `{"file_path":"src/app.ts","old_string":"a","new_string":"b"}`, "Edit: src/app.ts"},
		{"WebFetch", `{"url":"https://example.org/x","prompt":"p"}`, "WebFetch: https://example.org/x"},
		{"AskUserQuestion", `{"questions":[{"question":"Auf v3 heben?","header":"API"}]}`, "Auf v3 heben?"},
		{"TodoWrite", `{"todos":[]}`, "TodoWrite"},
		{"Bash", `not json`, ""},
		{"Bash", ``, ""},
	}
	for _, c := range cases {
		if got := toolDetail(c.tool, json.RawMessage(c.input)); got != c.want {
			t.Errorf("%s %s: got %q, want %q", c.tool, c.input, got, c.want)
		}
	}
	long := toolDetail("Bash", json.RawMessage(`{"command":"`+strings.Repeat("x", 500)+`"}`))
	if n := len([]rune(long)); n > 200 {
		t.Errorf("not clipped: %d runes", n)
	}
}

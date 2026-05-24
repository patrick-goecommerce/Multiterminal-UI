package backend

import (
	"strings"
	"testing"
)

func TestBuildChatArgs_Defaults(t *testing.T) {
	args := buildChatArgs("", "plan", "")
	joined := strings.Join(args, " ")
	for _, want := range []string{"--output-format stream-json", "--input-format stream-json", "--verbose", "--include-partial-messages", "--permission-mode plan", "-p"} {
		if !strings.Contains(joined, want) {
			t.Errorf("args missing %q; got %q", want, joined)
		}
	}
}

func TestBuildChatArgs_ModelAndResume(t *testing.T) {
	args := buildChatArgs("claude-opus-4-7", "acceptEdits", "sess-9")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "--model claude-opus-4-7") {
		t.Errorf("missing model; got %q", joined)
	}
	if !strings.Contains(joined, "--resume sess-9") {
		t.Errorf("missing resume; got %q", joined)
	}
}

func TestScanNDJSON_SplitsObjects(t *testing.T) {
	input := `{"a":1}` + "\n" + `{"b":2}` + "\n"
	var got []string
	scanNDJSON(strings.NewReader(input), func(line []byte) {
		got = append(got, string(line))
	})
	if len(got) != 2 || got[0] != `{"a":1}` || got[1] != `{"b":2}` {
		t.Fatalf("got %v, want two objects", got)
	}
}

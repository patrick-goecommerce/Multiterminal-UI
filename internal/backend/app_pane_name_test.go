package backend

import (
	"strings"
	"testing"
)

func TestBuildNamePrompt_IncludesPromptAndInstruction(t *testing.T) {
	p := buildNamePrompt("Fix the auth bug in login flow")
	if !strings.Contains(p, "Fix the auth bug in login flow") {
		t.Errorf("prompt should contain the user prompt, got: %q", p)
	}
	if !strings.Contains(strings.ToLower(p), "title") {
		t.Errorf("prompt should instruct for a short title, got: %q", p)
	}
}

func TestBuildNamePrompt_TruncatesLongInput(t *testing.T) {
	long := strings.Repeat("x", 5000)
	p := buildNamePrompt(long)
	if len([]rune(p)) > paneNamePromptMaxInput+200 {
		t.Errorf("prompt not bounded: len=%d", len([]rune(p)))
	}
}

func TestPaneNameArgs(t *testing.T) {
	args := paneNameArgs("claude-haiku-4-5", "some prompt")
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-p") {
		t.Errorf("args must contain -p, got %v", args)
	}
	if !strings.Contains(joined, "--model claude-haiku-4-5") {
		t.Errorf("args must contain model flag, got %v", args)
	}
	// the prompt must be passed as its own argv element (not split)
	found := false
	for _, a := range args {
		if a == "some prompt" {
			found = true
		}
	}
	if !found {
		t.Errorf("prompt must be a single argv element, got %v", args)
	}
}

func TestPaneNameArgs_NoModel(t *testing.T) {
	args := paneNameArgs("", "p")
	if strings.Contains(strings.Join(args, " "), "--model") {
		t.Errorf("no --model flag expected when model empty, got %v", args)
	}
}

func TestSanitizePaneName(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "auth-refactor", "auth-refactor"},
		{"trims whitespace", "  fix login  ", "fix login"},
		{"strips surrounding double quotes", `"Fix Auth Bug"`, "Fix Auth Bug"},
		{"strips surrounding single quotes", "'rename panes'", "rename panes"},
		{"first line only", "kanban tests\nmore detail here", "kanban tests"},
		{"strips trailing period", "refactor parser.", "refactor parser"},
		{"strips markdown backticks", "`config loader`", "config loader"},
		{"empty stays empty", "", ""},
		{"whitespace only becomes empty", "   \n  ", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizePaneName(tt.in); got != tt.want {
				t.Errorf("sanitizePaneName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestShouldRegenerateName(t *testing.T) {
	tests := []struct {
		name        string
		lastGenUnix int64
		nowUnix     int64
		want        bool
	}{
		{"never generated", 0, 1000, true},
		{"too soon after last", 1000, 1000 + paneNameMinIntervalSec - 1, false},
		{"exactly at interval", 1000, 1000 + paneNameMinIntervalSec, true},
		{"well past interval", 1000, 1000 + 3600, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRegenerateName(tt.lastGenUnix, tt.nowUnix); got != tt.want {
				t.Errorf("shouldRegenerateName(%d, %d) = %v, want %v", tt.lastGenUnix, tt.nowUnix, got, tt.want)
			}
		})
	}
}

func TestSanitizePaneName_Truncates(t *testing.T) {
	long := "this is a really long pane name that goes way past any sensible limit"
	got := sanitizePaneName(long)
	if len([]rune(got)) > paneNameMaxLen {
		t.Errorf("sanitizePaneName did not truncate: len=%d (%q), max=%d", len([]rune(got)), got, paneNameMaxLen)
	}
}

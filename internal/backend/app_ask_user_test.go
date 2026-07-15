package backend

import (
	"testing"
)

// ---------------------------------------------------------------------------
// extractQuestion — pattern detection for various prompt styles
// ---------------------------------------------------------------------------

func TestExtractQuestion_YnBracket(t *testing.T) {
	lines := []string{"Do you want to continue? [Y/n]"}
	q, opts := extractQuestion(lines)
	if q == "" {
		t.Fatal("expected question to be found")
	}
	if len(opts) != 2 || opts[0] != "Y" || opts[1] != "n" {
		t.Errorf("options = %v, want [Y, n]", opts)
	}
}

func TestExtractQuestion_ynBracket(t *testing.T) {
	lines := []string{"Proceed? [y/N]"}
	q, opts := extractQuestion(lines)
	if q == "" {
		t.Fatal("expected question to be found")
	}
	if len(opts) != 2 || opts[0] != "y" || opts[1] != "N" {
		t.Errorf("options = %v, want [y, N]", opts)
	}
}

func TestExtractQuestion_YesNoBracket(t *testing.T) {
	lines := []string{"Save changes? [Yes/No]"}
	q, opts := extractQuestion(lines)
	if q == "" {
		t.Fatal("expected question to be found")
	}
	if len(opts) != 2 || opts[0] != "Yes" || opts[1] != "No" {
		t.Errorf("options = %v, want [Yes, No]", opts)
	}
}

func TestExtractQuestion_AllowPrompt(t *testing.T) {
	lines := []string{"Allow Bash(npm test)?"}
	q, opts := extractQuestion(lines)
	if q == "" {
		t.Fatal("expected question to be found")
	}
	if len(opts) != 2 {
		t.Errorf("expected 2 options for Allow prompt, got %d", len(opts))
	}
}

func TestExtractQuestion_DoYouPrompt(t *testing.T) {
	lines := []string{"Do you want to install dependencies?"}
	q, opts := extractQuestion(lines)
	if q == "" {
		t.Fatal("expected question to be found")
	}
	if len(opts) != 2 {
		t.Errorf("expected 2 options for Do you prompt, got %d", len(opts))
	}
}

func TestExtractQuestion_ConfirmPrompt(t *testing.T) {
	lines := []string{"Confirm deletion?"}
	q, opts := extractQuestion(lines)
	if q == "" {
		t.Fatal("expected question to be found")
	}
	if len(opts) != 2 {
		t.Errorf("expected y/n options, got %v", opts)
	}
}

func TestExtractQuestion_GenericQuestion(t *testing.T) {
	lines := []string{"What should the file name be?"}
	q, opts := extractQuestion(lines)
	if q == "" {
		t.Fatal("expected question to be found")
	}
	// Generic questions have no predefined options
	if len(opts) != 0 {
		t.Errorf("generic question should have no options, got %v", opts)
	}
}

func TestExtractQuestion_ParenYN(t *testing.T) {
	lines := []string{"Continue (y/n)"}
	q, opts := extractQuestion(lines)
	if q == "" {
		t.Fatal("expected question to be found")
	}
	if len(opts) != 2 {
		t.Errorf("expected y/n options, got %v", opts)
	}
}

func TestExtractQuestion_EmptyLines(t *testing.T) {
	lines := []string{"", "", ""}
	q, _ := extractQuestion(lines)
	if q != "" {
		t.Errorf("expected no question from empty lines, got %q", q)
	}
}

func TestExtractQuestion_SkipsBlankLinesBeforeQuestion(t *testing.T) {
	lines := []string{"some output", "", "Continue? [Y/n]", "", ""}
	q, opts := extractQuestion(lines)
	if q != "Continue? [Y/n]" {
		t.Errorf("question = %q, want %q", q, "Continue? [Y/n]")
	}
	if len(opts) != 2 {
		t.Errorf("expected 2 options, got %v", opts)
	}
}

func TestExtractQuestion_ShortQuestionIgnored(t *testing.T) {
	// Questions under 10 chars with just ? should not match as generic
	lines := []string{"Why?"}
	q, _ := extractQuestion(lines)
	// Falls through to the fallback (returns last non-empty line)
	if q != "Why?" {
		t.Errorf("expected fallback to return %q, got %q", "Why?", q)
	}
}

func TestExtractQuestion_NilSlice(t *testing.T) {
	q, opts := extractQuestion(nil)
	if q != "" || opts != nil {
		t.Errorf("expected empty result for nil input, got q=%q opts=%v", q, opts)
	}
}

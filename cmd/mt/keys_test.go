package main

import (
	"strings"
	"testing"
)

func TestKeyBytes(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"enter", "\r"},
		{"Enter", "\r"},
		{"  RETURN  ", "\r"},
		{"tab", "\t"},
		{"escape", "\x1b"},
		{"esc", "\x1b"},
		{"up", "\x1b[A"},
		{"down", "\x1b[B"},
		{"backspace", "\x7f"},
		{"space", " "},
		// The answers to a permission prompt, which is the reason this command
		// exists at all.
		{"yes", "y"},
		{"no", "n"},
		// ctrl- is generated rather than tabulated, so both ends of the range
		// and the most-used one in the middle are worth pinning.
		{"ctrl-a", "\x01"},
		{"ctrl-c", "\x03"},
		{"ctrl-z", "\x1a"},
		{"ctrl+c", "\x03"},
		{"c-c", "\x03"},
		{"^c", "\x03"},
		{"CTRL-C", "\x03"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := keyBytes(tc.name)
			if !ok {
				t.Fatalf("keyBytes(%q) not found", tc.name)
			}
			if string(got) != tc.want {
				t.Errorf("keyBytes(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestKeyBytes_RejectsNonsense(t *testing.T) {
	for _, name := range []string{"", "ctrl-", "ctrl-ab", "ctrl-1", "meta-x", "supercalifragilistic"} {
		if _, ok := keyBytes(name); ok {
			t.Errorf("keyBytes(%q) was accepted", name)
		}
	}
}

// The help text is the only place a user learns the vocabulary, so it has to
// carry it.
func TestKnownKeyNames(t *testing.T) {
	got := knownKeyNames()
	for _, want := range []string{"enter", "escape", "up", "ctrl-<buchstabe>"} {
		if !strings.Contains(got, want) {
			t.Errorf("knownKeyNames() = %q, missing %q", got, want)
		}
	}
}

// A terminal screen is padded to its full height with blanks, so "the last
// three lines" of a mostly empty pane must not be three empty strings.
func TestLastLines_SkipsThePaddingBlanks(t *testing.T) {
	screen := "erste\nzweite\ndritte\n\n\n\n"
	got := lastLines(screen, 2)
	if got != "zweite\ndritte" {
		t.Errorf("lastLines = %q, want %q", got, "zweite\ndritte")
	}
}

func TestLastLines_AsksForMoreThanThereIs(t *testing.T) {
	if got := lastLines("nur eine\n", 5); got != "nur eine" {
		t.Errorf("lastLines = %q", got)
	}
}

func TestLastLines_OnAnEmptyScreen(t *testing.T) {
	if got := lastLines("\n\n\n", 3); got != "" {
		t.Errorf("lastLines = %q, want empty", got)
	}
}

func TestGatherText_JoinsArgumentsWithSpaces(t *testing.T) {
	got, err := gatherText([]string{"refactor", "the", "parser"})
	if err != nil {
		t.Fatalf("gatherText: %v", err)
	}
	if got != "refactor the parser" {
		t.Errorf("gatherText = %q", got)
	}
}

// A heredoc ends with a newline, and that newline would submit the prompt
// before the Enter the command sends, splitting it in two.
func TestGatherText_StdinLosesItsTrailingNewline(t *testing.T) {
	old := readAllStdin
	t.Cleanup(func() { readAllStdin = old })
	readAllStdin = func() ([]byte, error) { return []byte("mach mal\nund dann\n"), nil }

	got, err := gatherText([]string{"-"})
	if err != nil {
		t.Fatalf("gatherText: %v", err)
	}
	if got != "mach mal\nund dann" {
		t.Errorf("gatherText = %q", got)
	}
}

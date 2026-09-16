package main

import (
	"bytes"
	"flag"
	"testing"
	"time"
)

// The silent-flag bug.
//
// flag.Parse stops at the first non-flag argument, so `mt wait 3 --timeout
// 30s` parsed the ID and then ignored the timeout without a word: the caller
// asked for thirty seconds and waited five minutes. Nothing about it was
// visible, which is what makes it worth a test rather than a comment.
func TestParseArgs_ReadsFlagsBehindThePositional(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(&bytes.Buffer{})
	timeout := fs.Duration("timeout", time.Minute, "")
	asJSON := fs.Bool("json", false, "")

	positional, err := parseArgs(fs, []string{"3", "--timeout", "30s", "--json"})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if *timeout != 30*time.Second {
		t.Errorf("timeout = %s, want 30s", *timeout)
	}
	if !*asJSON {
		t.Error("--json behind the ID was ignored")
	}
	if len(positional) != 1 || positional[0] != "3" {
		t.Errorf("positional = %v, want [3]", positional)
	}
}

func TestParseArgs_StillReadsFlagsInFront(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(&bytes.Buffer{})
	asJSON := fs.Bool("json", false, "")

	positional, err := parseArgs(fs, []string{"--json", "7"})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if !*asJSON {
		t.Error("--json in front was ignored")
	}
	if len(positional) != 1 || positional[0] != "7" {
		t.Errorf("positional = %v, want [7]", positional)
	}
}

func TestParseArgs_KeepsSeveralPositionalsInOrder(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(&bytes.Buffer{})
	force := fs.Bool("force", false, "")

	positional, err := parseArgs(fs, []string{"3", "--force", "5", "9"})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if !*force {
		t.Error("--force between positionals was ignored")
	}
	want := []string{"3", "5", "9"}
	if len(positional) != len(want) {
		t.Fatalf("positional = %v, want %v", positional, want)
	}
	for i := range want {
		if positional[i] != want[i] {
			t.Fatalf("positional = %v, want %v", positional, want)
		}
	}
}

// "--" is how a caller passes an argument that looks like a flag.
func TestParseArgs_DoubleDashEndsTheFlags(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(&bytes.Buffer{})
	asJSON := fs.Bool("json", false, "")

	positional, err := parseArgs(fs, []string{"--", "--json"})
	if err != nil {
		t.Fatalf("parseArgs: %v", err)
	}
	if *asJSON {
		t.Error("--json after -- was treated as a flag")
	}
	if len(positional) != 1 || positional[0] != "--json" {
		t.Errorf("positional = %v, want [--json]", positional)
	}
}

func TestParseArgs_ReportsAnUnknownFlag(t *testing.T) {
	fs := flag.NewFlagSet("t", flag.ContinueOnError)
	fs.SetOutput(&bytes.Buffer{})

	if _, err := parseArgs(fs, []string{"3", "--nope"}); err == nil {
		t.Error("an unknown flag behind the positional was accepted")
	}
}

func TestSplitList(t *testing.T) {
	got := splitList(" done , blocked ,, ")
	if len(got) != 2 || got[0] != "done" || got[1] != "blocked" {
		t.Errorf("splitList = %v, want [done blocked]", got)
	}
	if splitList("") != nil {
		t.Errorf("splitList(\"\") = %v, want nil", splitList(""))
	}
}

func TestShortDir(t *testing.T) {
	cases := map[string]string{
		"":                     "-",
		"/home/p/code/mtui":    ".../code/mtui",
		"/home/p/code/mtui/":   ".../code/mtui",
		"/tmp":                 "/tmp",
		"C:/Users/p/code/mtui": ".../code/mtui",
		`C:\Users\p\code\mtui`: ".../code/mtui",
	}
	for in, want := range cases {
		if got := shortDir(in); got != want {
			t.Errorf("shortDir(%q) = %q, want %q", in, got, want)
		}
	}
}

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A hook whose stdin is never closed must still write its line and return.
// The pipe's write end can outlive Claude Code in a child that inherited it;
// a SessionEnd hook waiting for EOF on it hung for a month and kept its own
// exe locked, so no later build could replace it.
func TestRunReturnsWithoutStdinEOF(t *testing.T) {
	appdata := t.TempDir()
	t.Setenv("APPDATA", appdata)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer w.Close() // deliberately open while run() reads
	_, _ = w.WriteString(`{"session_id":"s1"}`)
	oldIn, oldArgs := os.Stdin, os.Args
	os.Stdin, os.Args = r, []string{"mtui-hook", "SessionEnd"}
	defer func() { os.Stdin, os.Args = oldIn, oldArgs }()

	done := make(chan struct{})
	go func() { run(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("run() blocked waiting for EOF on stdin")
	}
	if _, err := os.Stat(filepath.Join(appdata, "Multiterminal", "hooks", "s1.jsonl")); err != nil {
		t.Fatalf("no hook line written: %v", err)
	}
}

package engine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/orchestrator"
)

func skipIfNoClaude(t *testing.T) {
	t.Helper()
	_, err := exec.LookPath("claude")
	if err != nil {
		t.Skip("claude CLI not found, skipping integration test")
	}
}

func TestNewHeadlessEngine(t *testing.T) {
	eng := NewHeadlessEngine("/tmp/test-repo", 3)
	if eng.slots == nil {
		t.Fatal("slots manager should be initialized")
	}
	if eng.stepDetect == nil {
		t.Fatal("step loop detector should be initialized")
	}
	if eng.repoDetect == nil {
		t.Fatal("repo loop detector should be initialized")
	}
	if eng.checkpoint == nil {
		t.Fatal("checkpoint guard should be initialized")
	}
	if eng.cancels == nil {
		t.Fatal("cancels map should be initialized")
	}
	if eng.Slots().ActiveSlots() != 0 {
		t.Fatal("should start with 0 active slots")
	}
}

func TestRunVerifySuccess(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	steps := []orchestrator.VerifyStep{
		{Command: "echo hello", Description: "echo test"},
	}
	results := runVerify(ctx, dir, steps)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Passed {
		t.Fatalf("expected pass, got exit code %d: %s", results[0].ExitCode, results[0].Output)
	}
	if !strings.Contains(results[0].Output, "hello") {
		t.Fatalf("expected output to contain 'hello', got: %s", results[0].Output)
	}
}

func TestRunVerifyFailure(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	steps := []orchestrator.VerifyStep{
		{Command: "exit 1", Description: "should fail"},
	}
	results := runVerify(ctx, dir, steps)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Passed {
		t.Fatal("expected failure")
	}
	if results[0].ExitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", results[0].ExitCode)
	}
}

func TestRunVerifyOutputTruncation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	// Generate output larger than 2KB.
	// "echo" on Windows via COMSPEC: generate a long string.
	longStr := strings.Repeat("A", 3000)
	steps := []orchestrator.VerifyStep{
		{Command: "echo " + longStr, Description: "large output"},
	}
	results := runVerify(ctx, dir, steps)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if len(results[0].Output) > 2100 {
		t.Fatalf("output should be truncated to ~2KB, got %d bytes", len(results[0].Output))
	}
	if !strings.Contains(results[0].Output, "... (truncated)") {
		t.Fatal("truncated output should contain truncation marker")
	}
}

func TestGetFilesChanged(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// Initialize a git repo.
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	// Create and commit a file.
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "a.txt")
	run("commit", "-m", "initial")

	// Modify the file.
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("world"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := getFilesChanged(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "a.txt" {
		t.Fatalf("expected [a.txt], got %v", files)
	}
}

func TestGetFilesChangedNoChanges(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %s\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "a.txt")
	run("commit", "-m", "initial")

	files, err := getFilesChanged(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("expected no files, got %v", files)
	}
}

func TestCancelNoExecution(t *testing.T) {
	eng := NewHeadlessEngine("/tmp/test-repo", 3)
	err := eng.Cancel("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent step")
	}
	if !strings.Contains(err.Error(), "no active execution") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDetermineStatus(t *testing.T) {
	// All pass, no loops → success.
	status := determineStatus(
		[]orchestrator.VerifyResult{{Passed: true}, {Passed: true}},
		nil,
	)
	if status != orchestrator.StepSuccess {
		t.Fatalf("expected success, got %s", status)
	}

	// One failure → failed.
	status = determineStatus(
		[]orchestrator.VerifyResult{{Passed: true}, {Passed: false, ExitCode: 1}},
		nil,
	)
	if status != orchestrator.StepFailed {
		t.Fatalf("expected failed, got %s", status)
	}

	// Loop signals → stuck (even if verify passed).
	status = determineStatus(
		[]orchestrator.VerifyResult{{Passed: true}},
		[]orchestrator.LoopSignal{{Type: "same_error"}},
	)
	if status != orchestrator.StepStuck {
		t.Fatalf("expected stuck, got %s", status)
	}

	// No verify steps, no loops → success.
	status = determineStatus(nil, nil)
	if status != orchestrator.StepSuccess {
		t.Fatalf("expected success, got %s", status)
	}
}

func TestSummarizeFailures(t *testing.T) {
	verify := []orchestrator.VerifyResult{
		{Command: "go vet ./...", Passed: true},
		{Command: "go test ./...", Description: "unit tests", ExitCode: 1, Passed: false},
		{Command: "lint", ExitCode: 2, Passed: false},
	}
	msg := summarizeFailures(verify)
	if !strings.Contains(msg, "unit tests (exit 1)") {
		t.Fatalf("expected 'unit tests (exit 1)' in message, got: %s", msg)
	}
	if !strings.Contains(msg, "lint (exit 2)") {
		t.Fatalf("expected 'lint (exit 2)' in message, got: %s", msg)
	}
}

func TestParseCostFromOutput(t *testing.T) {
	raw := []byte(`{"cost_usd": 0.42, "result": "done"}`)
	cost := parseCostFromOutput(raw)
	if cost != 0.42 {
		t.Fatalf("expected 0.42, got %f", cost)
	}

	// Invalid JSON.
	cost = parseCostFromOutput([]byte("not json"))
	if cost != 0 {
		t.Fatalf("expected 0 for invalid json, got %f", cost)
	}
}

func TestStripClaudeEnv(t *testing.T) {
	env := []string{
		"PATH=/usr/bin",
		"CLAUDECODE=true",
		"HOME=/home/user",
		"CLAUDECODE=nested",
	}
	filtered := stripClaudeEnv(env)
	for _, e := range filtered {
		if strings.HasPrefix(e, "CLAUDECODE=") {
			t.Fatalf("CLAUDECODE should be stripped, found: %s", e)
		}
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(filtered), filtered)
	}
}

func TestRunVerifyContextCancelled(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	dir := t.TempDir()
	// Use a command that takes a while — ping localhost (Windows ping does 4 by default).
	steps := []orchestrator.VerifyStep{
		{Command: "ping -n 10 127.0.0.1", Description: "slow command"},
	}
	results := runVerify(ctx, dir, steps)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// Should fail due to context cancellation.
	if results[0].Passed {
		t.Fatal("expected failure due to context cancellation")
	}
}

package engine

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/orchestrator"
)

// runClaude executes claude -p --output-format json and returns the raw stdout.
func runClaude(ctx context.Context, workDir, prompt, systemPrompt, model string) ([]byte, error) {
	args := []string{"-p", "--output-format", "json"}
	if model != "" {
		args = append(args, "--model", model)
	}
	if systemPrompt != "" {
		args = append(args, "--system-prompt", systemPrompt)
	}

	// On Windows, claude is a .cmd shim — must use COMSPEC.
	comspec := os.Getenv("COMSPEC")
	if comspec == "" {
		comspec = "cmd.exe"
	}
	cmdArgs := append([]string{"/c", "claude"}, args...)

	cmd := exec.CommandContext(ctx, comspec, cmdArgs...)
	cmd.Dir = workDir
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Env = stripClaudeEnv(os.Environ())

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("cancelled: %w", ctx.Err())
		}
		return nil, fmt.Errorf("claude -p failed (exit=%v): %s", err, stderr.String())
	}

	raw := bytes.TrimSpace(stdout.Bytes())
	if len(raw) == 0 {
		return nil, fmt.Errorf("claude -p returned empty output")
	}
	return raw, nil
}

// stripClaudeEnv removes CLAUDECODE= entries from the environment.
func stripClaudeEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, "CLAUDECODE=") {
			out = append(out, e)
		}
	}
	return out
}

// maxVerifyOutput is the maximum output length per verify command (2KB).
const maxVerifyOutput = 2048

// runVerify executes verification commands and returns their results.
func runVerify(ctx context.Context, workDir string, steps []orchestrator.VerifyStep) []orchestrator.VerifyResult {
	results := make([]orchestrator.VerifyResult, 0, len(steps))
	for _, step := range steps {
		comspec := os.Getenv("COMSPEC")
		if comspec == "" {
			comspec = "cmd.exe"
		}
		cmd := exec.CommandContext(ctx, comspec, "/c", step.Command)
		cmd.Dir = workDir

		output, err := cmd.CombinedOutput()
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = -1
			}
		}

		outStr := string(output)
		if len(outStr) > maxVerifyOutput {
			outStr = outStr[:maxVerifyOutput] + "\n... (truncated)"
		}

		results = append(results, orchestrator.VerifyResult{
			Command:     step.Command,
			Description: step.Description,
			ExitCode:    exitCode,
			Output:      outStr,
			Passed:      exitCode == 0,
		})
	}
	return results
}

// getFilesChanged returns files modified in the worktree relative to HEAD.
func getFilesChanged(ctx context.Context, workDir string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "diff", "--name-only", "HEAD")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var files []string
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			files = append(files, l)
		}
	}
	return files, nil
}

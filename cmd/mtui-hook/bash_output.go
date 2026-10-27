package main

import (
	"encoding/json"
	"strings"
)

const (
	// outputTailMax is how much of a Bash call's output a hook line carries:
	// enough to see why a check failed, small enough for the hook file.
	outputTailMax = 300
	// commandMax bounds the command stored in a hook line.
	commandMax = 1000
)

// bashOutputTail is the last outputTailMax runes of what a Bash call printed.
// On a failure that is Claude Code's `error` text ("Exit code 3\n<output>");
// on success it is stdout followed by stderr, empty parts skipped.
func bashOutputTail(eventType string, response json.RawMessage, errText string) string {
	if eventType == "PostToolUseFailure" {
		return tail(errText, outputTailMax)
	}
	if len(response) == 0 {
		return ""
	}
	var r struct {
		Stdout string `json:"stdout"`
		Stderr string `json:"stderr"`
	}
	if json.Unmarshal(response, &r) != nil {
		return ""
	}
	var parts []string
	for _, p := range []string{r.Stdout, r.Stderr} {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return tail(strings.Join(parts, "\n"), outputTailMax)
}

// bashCommand is the command of a Bash call collapsed to one line, at most
// commandMax runes.
func bashCommand(input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	var in struct {
		Command string `json:"command"`
	}
	if json.Unmarshal(input, &in) != nil {
		return ""
	}
	// A newline ends a command like ";" does; a backslash-newline continues it.
	cmd := strings.ReplaceAll(strings.TrimSpace(in.Command), "\r\n", "\n")
	cmd = strings.ReplaceAll(cmd, "\\\n", " ")
	cmd = strings.ReplaceAll(cmd, "\n", "; ")
	s := strings.Join(strings.Fields(cmd), " ")
	if r := []rune(s); len(r) > commandMax {
		return string(r[:commandMax])
	}
	return s
}

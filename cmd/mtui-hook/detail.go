package main

import (
	"encoding/json"
	"strings"
)

// detailMax matches the hub's limit; the hub clips again, this keeps the hook
// file small.
const detailMax = 200

// toolDetail is one line on what a tool call is about to do, for MTUI's
// waiting list: "Bash: npm install", "Edit: src/app.ts", or the question of an
// AskUserQuestion. Empty when the input cannot be read.
func toolDetail(tool string, input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	var in struct {
		Command      string `json:"command"`
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
		URL          string `json:"url"`
		Pattern      string `json:"pattern"`
		Questions    []struct {
			Question string `json:"question"`
		} `json:"questions"`
	}
	if json.Unmarshal(input, &in) != nil {
		return ""
	}
	if tool == "AskUserQuestion" {
		if len(in.Questions) > 0 {
			return clip(in.Questions[0].Question)
		}
		return ""
	}
	for _, what := range []string{in.Command, in.FilePath, in.NotebookPath, in.URL, in.Pattern} {
		if what != "" {
			return clip(tool + ": " + what)
		}
	}
	return tool
}

func clip(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > detailMax {
		return string(r[:detailMax-1]) + "…"
	}
	return s
}

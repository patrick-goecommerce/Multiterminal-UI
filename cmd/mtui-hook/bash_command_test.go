package main

import (
	"encoding/json"
	"testing"
)

func TestBashCommandNewlines(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"newline separates", "go test ./...\necho done", "go test ./...; echo done"},
		{"crlf separates", "go test ./...\r\necho done", "go test ./...; echo done"},
		{"continuation joins", "go test \\\n  ./...", "go test ./..."},
		{"crlf continuation joins", "go test \\\r\n  ./...", "go test ./..."},
		{"trailing newline leaves no separator", "go test ./...\n", "go test ./..."},
		{"plain", "  go   test\t./... ", "go test ./..."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]string{"command": c.in})
			if got := bashCommand(raw); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

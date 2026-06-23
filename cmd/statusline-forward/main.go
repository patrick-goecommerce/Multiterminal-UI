// Command statusline-forward wraps a Claude Code statusLine command: it relays
// the wrapped command's output (so the displayed status line is unchanged) and
// fire-and-forget POSTs Claude's statusline JSON to MTUI for telemetry capture.
// It must never block or break the wrapped statusline.
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"
)

func main() {
	data, _ := io.ReadAll(os.Stdin)
	// Display first: relay the wrapped statusline so the POST never delays it.
	code := runWrapped(os.Args[1:], data, os.Stdout, os.Stderr)
	// Then capture, best-effort.
	forward(data, os.Getenv("MTUI_PORT"), os.Getenv("MULTITERMINAL_SESSION_ID"))
	os.Exit(code)
}

// runWrapped runs args[0] with args[1:], feeding stdin and relaying stdout/stderr.
// Returns the wrapped process exit code (0 when there is no wrapped command).
func runWrapped(args []string, stdin []byte, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return 0
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = bytes.NewReader(stdin)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		return 1
	}
	return 0
}

// forward POSTs the raw statusline JSON to MTUI. Silent on any failure.
func forward(payload []byte, port, sid string) {
	if port == "" || sid == "" {
		return
	}
	id, err := strconv.Atoi(sid)
	if err != nil {
		return
	}
	body, err := json.Marshal(struct {
		SessionID int             `json:"sessionId"`
		Payload   json.RawMessage `json:"payload"`
	}{SessionID: id, Payload: json.RawMessage(payload)})
	if err != nil {
		return
	}
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Post("http://127.0.0.1:"+port+"/api/statusline",
		"application/json", bytes.NewReader(body))
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}

// Package backend — persistent claude stream-json chat session lifecycle.
package backend

import (
	"bufio"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
)

// ChatSession is a long-lived claude subprocess driven via stream-json.
type ChatSession struct {
	ConvID string
	Scope  string
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	mu     sync.Mutex
	closed bool
}

// buildChatArgs builds the claude argv (without the executable) for a chat session.
func buildChatArgs(model, permissionMode, resumeID string) []string {
	args := []string{
		"-p",
		"--output-format", "stream-json",
		"--input-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
	}
	if permissionMode == "" {
		permissionMode = "plan"
	}
	args = append(args, "--permission-mode", permissionMode)
	if model != "" {
		args = append(args, "--model", model)
	}
	if resumeID != "" {
		args = append(args, "--resume", resumeID)
	}
	return args
}

// scanNDJSON reads newline-delimited JSON from r and calls fn per complete line.
// Buffer is enlarged so large tool payloads spanning chunks are not truncated.
func scanNDJSON(r io.Reader, fn func(line []byte)) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 256*1024)
	scanner.Buffer(buf, 8*1024*1024) // up to 8 MiB per line
	for scanner.Scan() {
		b := scanner.Bytes()
		if len(b) == 0 {
			continue
		}
		line := make([]byte, len(b))
		copy(line, b)
		fn(line)
	}
	if err := scanner.Err(); err != nil {
		log.Printf("[chat] stream read error: %v", err)
	}
}

// startChatProcess launches the claude subprocess for a chat session.
// On Windows, claude is a .cmd shim → wrap via COMSPEC (see session.go).
func (a *AppService) startChatProcess(convID, scope, model, permissionMode, resumeID string) (*ChatSession, error) {
	path := a.resolvedClaudePath
	if path == "" {
		path = "claude"
	}
	args := buildChatArgs(model, permissionMode, resumeID)
	cmd := wrapClaudeCmd(path, args)
	cmd.Dir = scope
	cmd.Env = filterEnv(os.Environ(), "CLAUDECODE")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	sess := &ChatSession{ConvID: convID, Scope: scope, cmd: cmd, stdin: stdin}
	go scanNDJSON(stdout, func(line []byte) {
		if ev, ok := parseChatEvent(line); ok {
			a.dispatchChatEvent(sess.ConvID, ev)
		}
	})
	return sess, nil
}

// SendTurn writes one user turn to the session's stdin as stream-json.
func (s *ChatSession) SendTurn(content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.ErrClosedPipe
	}
	payload := `{"type":"user","message":{"role":"user","content":` + jsonQuote(content) + `}}` + "\n"
	_, err := s.stdin.Write([]byte(payload))
	return err
}

// Close terminates the session.
func (s *ChatSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	_ = s.stdin.Close()
	if s.cmd != nil && s.cmd.Process != nil {
		c := s.cmd
		_ = c.Process.Kill()
		go func() { _ = c.Wait() }() // reap to avoid zombie; async so we don't block under lock
	}
}


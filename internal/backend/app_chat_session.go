// Package backend — persistent claude stream-json chat session lifecycle.
package backend

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// ChatSession is a long-lived claude subprocess driven via stream-json.
type ChatSession struct {
	ConvID   string
	Scope    string
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	mu       sync.Mutex
	closed   bool
	waitOnce sync.Once
	stderr   []byte // capped tail of the process's stderr, for diagnostics
	resumeID string // claude session id this process was launched with (for --resume)
	lastTurn string // most recent user turn, for resend after a recovery restart
}

// getLastTurn returns the last user turn sent (thread-safe).
func (s *ChatSession) getLastTurn() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastTurn
}

const chatStderrCap = 4096

// appendStderr keeps the last chatStderrCap bytes of stderr for error reporting.
func (s *ChatSession) appendStderr(line string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stderr = append(s.stderr, line...)
	s.stderr = append(s.stderr, '\n')
	if len(s.stderr) > chatStderrCap {
		s.stderr = s.stderr[len(s.stderr)-chatStderrCap:]
	}
}

// stderrTail returns a trimmed copy of the captured stderr.
func (s *ChatSession) stderrTail() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.TrimSpace(string(s.stderr))
}

// wait reaps the process exactly once, regardless of who calls it.
func (s *ChatSession) wait() error {
	var err error
	s.waitOnce.Do(func() {
		if s.cmd != nil {
			err = s.cmd.Wait()
		}
	})
	return err
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
// Returns the scanner error (if any) so the caller can decide whether it is
// worth logging — a closed pipe during intentional teardown is not.
func scanNDJSON(r io.Reader, fn func(line []byte)) error {
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
	return scanner.Err()
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
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	sess := &ChatSession{ConvID: convID, Scope: scope, cmd: cmd, stdin: stdin, resumeID: resumeID}

	// Drain stderr so the process never blocks on a full pipe, and keep a tail
	// for diagnostics — without this, a failed claude launch (bad flag, auth,
	// missing binary) produces a silent "no answer" in the UI.
	go func() {
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			log.Printf("[chat %s] stderr: %s", convID, line)
			sess.appendStderr(line)
		}
	}()

	go func() {
		err := scanNDJSON(stdout, func(line []byte) {
			if ev, ok := parseChatEvent(line); ok {
				a.dispatchChatEvent(sess.ConvID, ev)
			}
		})
		waitErr := sess.wait()
		// A read/exit error after an intentional Close (e.g. terminal⇄chat
		// toggle) is expected teardown noise, not a fault.
		if sess.isClosed() {
			return
		}
		if err != nil {
			log.Printf("[chat] stream read error: %v", err)
		}
		if waitErr == nil {
			return
		}
		tail := sess.stderrTail()
		// Self-heal a stale resume: the persisted session id no longer exists
		// in claude's store (e.g. the original process never saved it). Drop it
		// and transparently restart with a fresh session, resending the turn.
		if resumeID != "" && strings.Contains(strings.ToLower(tail), "no conversation found") {
			log.Printf("[chat %s] resume of %s failed, restarting fresh", convID, resumeID)
			a.recoverChatResume(sess)
			return
		}
		// Otherwise claude exited on its own (crash / bad invocation) — surface why.
		msg := fmt.Sprintf("claude beendet (%v)", waitErr)
		if tail != "" {
			msg += ": " + tail
		}
		a.emitChatError(sess.ConvID, msg)
	}()
	return sess, nil
}

// recoverChatResume restarts a conversation's session without --resume after a
// stale-resume failure, then resends the last user turn so the message the user
// just sent still gets answered.
func (a *AppService) recoverChatResume(dead *ChatSession) {
	convID := dead.ConvID
	a.clearPersistedSessionID(dead.Scope, convID)

	a.mu.Lock()
	if a.chatSessions[convID] == dead {
		delete(a.chatSessions, convID)
	}
	a.mu.Unlock()

	conv, err := a.GetConversation(dead.Scope, convID)
	if err != nil {
		a.emitChatError(convID, "Neustart fehlgeschlagen: "+err.Error())
		return
	}
	fresh, err := a.startChatProcess(convID, conv.Scope, conv.Model, conv.PermissionMode, "")
	if err != nil {
		a.emitChatError(convID, "Neustart fehlgeschlagen: "+err.Error())
		return
	}
	a.mu.Lock()
	a.chatSessions[convID] = fresh
	a.mu.Unlock()

	if turn := dead.getLastTurn(); turn != "" {
		if err := fresh.SendTurn(turn); err != nil {
			a.emitChatError(convID, "Neustart fehlgeschlagen: "+err.Error())
		}
	}
}

// isClosed reports whether Close has been called (thread-safe).
func (s *ChatSession) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

// SendTurn writes one user turn to the session's stdin as stream-json.
func (s *ChatSession) SendTurn(content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastTurn = content // remembered so a recovery restart can resend it
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
		_ = s.cmd.Process.Kill()
		go func() { _ = s.wait() }() // reap once; async so we don't block under lock
	}
}

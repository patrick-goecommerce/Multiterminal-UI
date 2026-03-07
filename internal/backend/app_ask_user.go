// Package backend provides Ask-User Bridging: detecting when CLI agents
// need user input and forwarding the question to the frontend.
package backend

import (
	"log"
	"strings"
	"time"
)

// AskUserQuestion represents an agent's question that needs user input.
type AskUserQuestion struct {
	SessionID   int      `json:"session_id" yaml:"session_id"`
	SessionName string   `json:"session_name" yaml:"session_name"`
	Question    string   `json:"question" yaml:"question"`
	Options     []string `json:"options" yaml:"options"`
	Timestamp   string   `json:"timestamp" yaml:"timestamp"`
}

// CheckAskUser analyzes a session's screen buffer for pending questions.
// Called when activity transitions to waitingPermission or waitingAnswer.
func (a *AppService) CheckAskUser(sessionID int) *AskUserQuestion {
	a.mu.Lock()
	sess := a.sessions[sessionID]
	a.mu.Unlock()

	if sess == nil {
		return nil
	}

	// Get the last few lines from the screen buffer
	rows := sess.Screen.PlainTextRows(0, -1)
	if len(rows) == 0 {
		return nil
	}

	// Analyze last 5 lines for question patterns
	start := len(rows) - 5
	if start < 0 {
		start = 0
	}
	lastLines := rows[start:]

	question, options := extractQuestion(lastLines)
	if question == "" {
		return nil
	}

	return &AskUserQuestion{
		SessionID:   sessionID,
		SessionName: sess.Title,
		Question:    question,
		Options:     options,
		Timestamp:   time.Now().Format(time.RFC3339),
	}
}

// AnswerAskUser sends a response to a session that is waiting for input.
func (a *AppService) AnswerAskUser(sessionID int, answer string) error {
	a.mu.Lock()
	sess := a.sessions[sessionID]
	a.mu.Unlock()

	if sess == nil {
		return nil
	}

	_, err := sess.Write([]byte(answer + "\r"))
	if err != nil {
		log.Printf("[ask-user] write error for session %d: %v", sessionID, err)
		return err
	}

	log.Printf("[ask-user] answered session %d: %q", sessionID, answer)

	// Emit answered event
	if a.app != nil {
		a.app.Event.Emit("ask_user:answered", map[string]interface{}{
			"sessionId": sessionID,
			"answer":    answer,
		})
	}

	return nil
}

// DismissAskUser dismisses a pending question without answering.
func (a *AppService) DismissAskUser(sessionID int) {
	if a.app != nil {
		a.app.Event.Emit("ask_user:dismissed", map[string]interface{}{
			"sessionId": sessionID,
		})
	}
}

// extractQuestion analyzes screen lines for question patterns.
// Returns the question text and available options (e.g., ["Y", "n"]).
func extractQuestion(lines []string) (string, []string) {
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Pattern: [Y/n] or [y/N] style prompts
		if idx := strings.Index(line, "[Y/n]"); idx >= 0 {
			return line, []string{"Y", "n"}
		}
		if idx := strings.Index(line, "[y/N]"); idx >= 0 {
			return line, []string{"y", "N"}
		}
		if idx := strings.Index(line, "[Yes/No]"); idx >= 0 {
			return line, []string{"Yes", "No"}
		}

		// Pattern: "Allow" permission prompts (Claude Code)
		if strings.Contains(line, "Allow") && strings.HasSuffix(line, "?") {
			return line, []string{"y", "n"}
		}

		// Pattern: "Do you want to" prompts
		if strings.HasPrefix(line, "Do you") && strings.HasSuffix(line, "?") {
			return line, []string{"y", "n"}
		}

		// Pattern: "Confirm" prompts
		if strings.Contains(line, "Confirm") && strings.HasSuffix(line, "?") {
			return line, []string{"y", "n"}
		}

		// Pattern: line ending with ? (generic question)
		if strings.HasSuffix(line, "?") && len(line) > 10 {
			return line, []string{}
		}

		// Pattern: (y/n) anywhere
		if strings.Contains(line, "(y/n)") || strings.Contains(line, "(Y/N)") {
			return line, []string{"y", "n"}
		}
	}

	// Fallback: return last non-empty line as context
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line, []string{}
		}
	}
	return "", nil
}

// emitAskUserQuestion sends the question event to the frontend.
// Called from the scan loop when activity becomes waitingPermission/waitingAnswer.
func (a *AppService) emitAskUserQuestion(q *AskUserQuestion) {
	if a.app == nil || q == nil {
		return
	}
	a.app.Event.Emit("ask_user:question", q)
	log.Printf("[ask-user] question from session %d: %q", q.SessionID, q.Question)
}

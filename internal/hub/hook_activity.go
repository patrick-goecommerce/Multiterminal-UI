package hub

import (
	"regexp"
	"strings"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/hooks"
)

// askUserTool is Claude Code's tool for asking the user a multiple-choice
// question. Its PreToolUse means the agent now waits for an answer, not that
// it is working.
const askUserTool = "AskUserQuestion"

// hookActivity maps a Claude Code hook event to an agent state.
//
// The bool reports whether the event carries any state information at all.
// Not every event does: an idle reminder says nothing about whether the turn
// ended, and an unknown event says nothing at all. Returning a state anyway
// meant inventing one, which tore running sessions to "done" and idle ones to
// "idle" (#188). Callers must leave the current state untouched when this is
// false.
func hookActivity(ev hooks.Event) (Activity, bool) {
	switch ev.Event {
	case "PreToolUse", "PermissionRequest":
		if ev.Tool == askUserTool {
			return ActivityWaitingAnswer, true
		}
		if ev.Event == "PermissionRequest" {
			return ActivityWaitingPermission, true
		}
		return ActivityActive, true
	case "PostToolUse", "UserPromptSubmit":
		return ActivityActive, true
	case "PostToolUseFailure":
		return ActivityError, true
	case "Notification":
		return notificationActivity(ev.NotificationType, ev.Message)
	case "Stop":
		// The turn is over. Whether the user is now expected to answer is in
		// the last message, which mtui-hook passes along; the screen cannot
		// be trusted with it, because Claude Code prints a timing line and a
		// recap between the question and the prompt.
		if endsWithQuestion(ev.Message) {
			return ActivityWaitingAnswer, true
		}
		return ActivityDone, true
	default:
		return ActivityIdle, false
	}
}

// notificationActivity reads a Notification by its type. Claude Code versions
// that do not send a type fall back to the old reading, a "?" in the text.
func notificationActivity(kind, message string) (Activity, bool) {
	switch kind {
	case "permission_prompt":
		return ActivityWaitingPermission, true
	case "elicitation_dialog", "elicitation_url_dialog", "agent_needs_input":
		return ActivityWaitingAnswer, true
	case "elicitation_complete", "elicitation_response":
		return ActivityActive, true
	case "":
		if strings.Contains(message, "?") {
			return ActivityWaitingAnswer, true
		}
		return ActivityIdle, false
	default:
		// idle_prompt, auth_success, agent_completed, quota_*: reminders and
		// news, none of which changes what the session is doing.
		return ActivityIdle, false
	}
}

var (
	codeFence = regexp.MustCompile("(?s)```.*?(```|$)")
	// A question mark that ends a sentence: followed by the end, whitespace,
	// or closing quotes/brackets/markdown. "?tab=1" in a URL is not a question.
	sentenceQuestion = regexp.MustCompile(`[?？]["'»“”)\]*_~]*(\s|$)`)
	listItem         = regexp.MustCompile(`^\s*([-*•]|\d+[.)])\s`)
)

// endsWithQuestion reports whether a message ends by asking the user
// something: the last paragraph contains a question. When the last paragraph
// is a list, the question usually introduces it ("Wie weiter?\n\n1. ...
// 2. ..."), so the paragraph before it counts too. Code blocks are ignored.
func endsWithQuestion(msg string) bool {
	// mtui-hook sends only the tail of a long message, which can start inside
	// a code block. An odd number of fences means the first one closes a block
	// whose start was cut off; without this it would be read as an opening
	// fence and swallow the rest, question included.
	if strings.Count(msg, "```")%2 == 1 {
		msg = msg[strings.Index(msg, "```")+3:]
	}
	msg = codeFence.ReplaceAllString(msg, "")
	var paras []string
	for _, p := range strings.Split(strings.ReplaceAll(msg, "\r\n", "\n"), "\n\n") {
		if strings.TrimSpace(p) != "" {
			paras = append(paras, p)
		}
	}
	if len(paras) == 0 {
		return false
	}
	last := paras[len(paras)-1]
	if sentenceQuestion.MatchString(last) {
		return true
	}
	if isList(last) && len(paras) > 1 {
		return sentenceQuestion.MatchString(paras[len(paras)-2])
	}
	return false
}

func isList(para string) bool {
	for _, line := range strings.Split(para, "\n") {
		if strings.TrimSpace(line) != "" && !listItem.MatchString(line) {
			return false
		}
	}
	return true
}

package hub

import (
	"regexp"
	"strings"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// permissionDialog matches what Claude Code draws for a tool permission: the
// question, or the option that sends the user's own instructions instead.
// Checked against Claude Code 2.1.
var permissionDialog = regexp.MustCompile(`(?i)(do you want to (proceed|make this edit|create|allow)|no, and tell claude what to do differently)`)

// declineMark matches the line Claude Code prints when a permission was
// declined with Esc ("Interrupted · What should Claude do instead?").
var declineMark = regexp.MustCompile(`(?i)interrupted`)

const (
	// dialogRows is how many bottom rows the dialog check reads.
	dialogRows = 14
	// declineRows is how many bottom rows the decline check reads. Claude Code
	// prints the decline line directly above its input box, so a small window
	// keeps an old "Interrupted" further up from matching.
	declineRows = 8
)

// bottomRows returns the last n rows of the session's content as text.
//
// The content, not the screen: a young session's text does not reach the
// bottom of the pane, and the screen's last n rows are then blank. Trailing
// blank rows are cut first, so a decline in a half-empty pane is seen too.
func bottomRows(sess *terminal.Session, n int) string {
	lines := strings.Split(sess.Screen.PlainText(), "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " ")
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// permissionDialogVisible reports whether a permission dialog is on screen.
func permissionDialogVisible(sess *terminal.Session) bool {
	return permissionDialog.MatchString(bottomRows(sess, dialogRows))
}

// declineVisible reports the mark of a declined permission. The Esc fallback
// in classifyForScan needs it: a dialog worded differently from the regex
// above (plan mode, say) shows no decline mark and so simply keeps waiting
// for the next hook.
func declineVisible(sess *terminal.Session) bool {
	return declineMark.MatchString(bottomRows(sess, declineRows))
}

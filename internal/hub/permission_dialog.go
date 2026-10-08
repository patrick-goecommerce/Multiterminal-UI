package hub

import (
	"regexp"
	"strings"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

// permissionDialog matches what Claude Code draws for a tool permission: the
// question, or the option that sends the user's own instructions instead.
// Checked against Claude Code 2.1. If a later version words it differently,
// the Esc fallback in classifyForScan stops firing and a declined permission
// waits for the next hook again, which is how it behaved before.
var permissionDialog = regexp.MustCompile(`(?i)(do you want to (proceed|make this edit|create|allow)|no, and tell claude what to do differently)`)

// dialogRows is how many bottom rows the check reads.
const dialogRows = 14

// permissionDialogVisible reports whether a permission dialog is on screen.
func permissionDialogVisible(sess *terminal.Session) bool {
	lines := strings.Split(sess.Screen.PlainText(), "\n")
	if len(lines) > dialogRows {
		lines = lines[len(lines)-dialogRows:]
	}
	return permissionDialog.MatchString(strings.Join(lines, "\n"))
}

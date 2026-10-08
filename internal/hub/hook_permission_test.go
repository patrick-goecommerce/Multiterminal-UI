package hub

import (
	"strings"
	"testing"
	"time"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/terminal"
)

func permissionSession(t *testing.T, screen string, hookAge, outputAge time.Duration) *terminal.Session {
	t.Helper()
	sess := terminal.NewSession(1, 24, 80)
	// The classifier reads the bottom rows, where Claude Code draws its dialog.
	sess.Screen.Write([]byte(strings.Repeat("\r\n", 22) + screen))
	sess.SetHookActivity(terminal.ActivityWaitingPermission)
	sess.SetHookActivityAtForTest(time.Now().Add(-hookAge))
	sess.SetLastOutputAtForTest(time.Now().Add(-outputAge))
	return sess
}

// After the user approved, the next hook (PostToolUse) comes only when the
// tool is done. Until then the pane must not keep asking for permission.
func TestClassifyForScan_ApprovedPermissionRunsAgain(t *testing.T) {
	sess := permissionSession(t, "● Bash(go test ./...)\r\n  running tests\r\n", 5*time.Second, 200*time.Millisecond)
	if got := classifyForScan(sess); got != terminal.ActivityActive {
		t.Errorf("dialog gone and output flowing: got %q, want active", got)
	}
}

func TestClassifyForScan_PermissionHoldsWhileTheDialogIsShown(t *testing.T) {
	sess := permissionSession(t, "Bash command\r\n  rm -rf build\r\nDo you want to proceed?\r\n❯ 1. Yes\r\n  2. No\r\n", 5*time.Second, 200*time.Millisecond)
	if got := classifyForScan(sess); got != terminal.ActivityWaitingPermission {
		t.Errorf("dialog still on screen: got %q, want waitingPermission", got)
	}
}

// Claude Code runs the hook before it draws the dialog, so output right after
// the hook is the tool call being printed, not an approval.
func TestClassifyForScan_PermissionIsTrustedRightAfterTheHook(t *testing.T) {
	sess := permissionSession(t, "● Bash(rm -rf build)\r\n", 300*time.Millisecond, 100*time.Millisecond)
	if got := classifyForScan(sess); got != terminal.ActivityWaitingPermission {
		t.Errorf("hook just fired: got %q, want waitingPermission", got)
	}
}

func TestClassifyForScan_QuietPermissionStays(t *testing.T) {
	sess := permissionSession(t, "● Bash(rm -rf build)\r\n", 30*time.Second, 20*time.Second)
	if got := classifyForScan(sess); got != terminal.ActivityWaitingPermission {
		t.Errorf("no output since: got %q, want waitingPermission", got)
	}
}

// Declined with Esc: Claude Code prints "Interrupted" and its prompt, and no
// hook follows. Once the pane is quiet the screen's reading wins.
func TestClassifyForScan_DeclinedPermissionLetsGo(t *testing.T) {
	sess := permissionSession(t,
		"● Bash(rm -rf build)\r\n  ⎿  Interrupted · What should Claude do instead?\r\n\x1b[1;35m❯\x1b[0m ",
		30*time.Second, 500*time.Millisecond)
	if got := classifyForScan(sess); got != terminal.ActivityDone {
		t.Errorf("declined: got %v, want done", got)
	}
}

func TestClassifyForScan_UnknownDialogStillWaits(t *testing.T) {
	sess := permissionSession(t,
		"Would you like to proceed?\r\n❯ 1. Yes, and auto-accept edits\r\n  2. No, keep planning\r\n",
		30*time.Second, 20*time.Second)
	if got := classifyForScan(sess); got != terminal.ActivityWaitingPermission {
		t.Errorf("unrecognised dialog: got %v, want waitingPermission", got)
	}
}

func TestClassifyForScan_QuietDialogStillWaits(t *testing.T) {
	sess := permissionSession(t,
		"Bash command\r\n  rm -rf build\r\nDo you want to proceed?\r\n❯ 1. Yes\r\n  2. No, and tell Claude what to do differently (esc)\r\n",
		30*time.Second, 20*time.Second)
	if got := classifyForScan(sess); got != terminal.ActivityWaitingPermission {
		t.Errorf("dialog still shown: got %v, want waitingPermission", got)
	}
}

func TestScanOne_HungAgentIsAnError(t *testing.T) {
	sess := terminal.NewSession(1, 24, 80)
	sess.SetHookActivity(terminal.ActivityActive)
	sess.SetHookActivityAtForTest(time.Now().Add(-11 * time.Minute))
	sess.SetLastOutputAtForTest(time.Now().Add(-11 * time.Minute))
	r, isHung := scanOne(1, sess)
	if !isHung || r.Activity != ActivityError {
		t.Errorf("got %v hung=%v, want error and hung", r.Activity, isHung)
	}
}

func TestScanOne_ShellIsNeverHung(t *testing.T) {
	sess := terminal.NewSession(1, 24, 80) // no hook data: a shell or an agent without hooks
	sess.SetLastOutputAtForTest(time.Now().Add(-time.Hour))
	if _, isHung := scanOne(1, sess); isHung {
		t.Error("a session without hooks counted as hung")
	}
}

// youngSession is a 40-row pane whose content stops well above the bottom: a
// session a few turns old. The bottom rows of the screen are blank there, and
// no padding pushes the text down.
func youngSession(t *testing.T, state terminal.ActivityState, hookAge, outputAge time.Duration) *terminal.Session {
	t.Helper()
	sess := terminal.NewSession(1, 40, 100)
	sess.Screen.Write([]byte(
		" ✻ Welcome to Claude Code!\r\n\r\n   cwd: C:\\repos\\a\r\n\r\n" +
			"> lösch den build ordner\r\n\r\n" +
			"● Bash(rm -rf build)\r\n" +
			"  ⎿  Interrupted · What should Claude do instead?\r\n\r\n" +
			"────────────────────\r\n\x1b[1;35m❯\x1b[0m \r\n────────────────────\r\n  ? for shortcuts"))
	sess.SetHookActivity(state)
	sess.SetHookActivityAtForTest(time.Now().Add(-hookAge))
	sess.SetLastOutputAtForTest(time.Now().Add(-outputAge))
	return sess
}

func TestClassifyForScan_DeclineInAYoungSessionLetsGo(t *testing.T) {
	sess := youngSession(t, terminal.ActivityWaitingPermission, 30*time.Second, 20*time.Second)
	if got := classifyForScan(sess); got != terminal.ActivityDone {
		t.Errorf("declined in a half-empty pane: got %v, want done", got)
	}
}

// Esc during a working turn: no Stop hook follows. The turn ends as done and
// never reaches the ten-minute hang.
func TestScanOne_InterruptedTurnIsDoneNotHung(t *testing.T) {
	sess := youngSession(t, terminal.ActivityActive, 11*time.Minute, 11*time.Minute)
	if got := classifyForScan(sess); got != terminal.ActivityDone {
		t.Errorf("interrupted turn: got %v, want done", got)
	}
	r, isHung := scanOne(1, sess)
	if isHung || r.Activity != ActivityDone {
		t.Errorf("interrupted turn 11 min later: got %v hung=%v, want done, not hung", r.Activity, isHung)
	}
}

// A turn that still draws is working, whatever an older line says.
func TestClassifyForScan_InterruptMarkWhileDrawingStaysActive(t *testing.T) {
	sess := youngSession(t, terminal.ActivityActive, time.Second, 100*time.Millisecond)
	if got := classifyForScan(sess); got != terminal.ActivityActive {
		t.Errorf("output still flowing: got %v, want active", got)
	}
}

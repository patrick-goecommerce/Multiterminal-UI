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

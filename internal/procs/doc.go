// Package procs holds the process-spawning rules that every MTUI binary has to
// follow, so there is one implementation rather than one per binary.
//
// Both rules exist because of Windows:
//
//   - HideConsole must be applied to every non-PTY child. MTUI is a GUI app
//     with no console of its own, so a console-subsystem child (git, gh,
//     taskkill, anything through cmd.exe) makes Windows allocate a console
//     window that flashes on screen. PTY sessions are exempt: ConPTY has no
//     window.
//   - KillProcessTree exists because closing a session only ends the root
//     process, and its descendants then hold handles that make operations
//     like "git worktree remove" fail.
//
// The daemon (cmd/mtuid) spawns the same kinds of child process the GUI does,
// which is why these live here instead of in internal/backend.
package procs

# go-pty, patched for MTUI

A copy of github.com/aymanbagabas/go-pty v0.2.3 (MIT, see LICENSE), used
through a `replace` directive in the top-level go.mod.

## Patch

`cmd_windows.go`, `(*Cmd).start`: close `pi.Process` once `os.FindProcess` has
opened its own handle. Upstream closes only `pi.Thread`, so every PTY start
leaked one process handle and kept the exited process object, and its PID,
alive for the life of MTUI or mtuid. With idle suspend and resume that is one
more per wake-up, for as long as the daemon runs.

Closing it means a PID is free for reuse once the session's process has been
waited on. `terminal.Session.Pid` therefore reports 0 after the exit, so
nothing kills a tree by a PID that may already belong to another process.

## Dropping the copy

Once upstream closes the handle itself, remove this directory and the
`replace` line in go.mod. The `Pid` change in internal/terminal stays.

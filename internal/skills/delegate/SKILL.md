---
name: mtui-delegate
description: Hand a self-contained subtask to another agent in its own Multiterminal (MTUI) pane and wait for its result, through the mtui MCP tools or the `mt` CLI. Use only inside an MTUI pane (the environment variable MULTITERMINAL_SESSION_ID is set), when work splits into independent parts that can run in parallel, when another model should review or cross-check something, or when a long job (test suite, migration, research) should run beside you instead of blocking you.
---

<!-- mtui:managed -- Installed and updated by Multiterminal. Delete this line to keep your own edits; MTUI then leaves the file alone. -->

# Delegating to another agent pane in MTUI

MTUI shows every agent session as a pane the user can watch and take over.
You can open more of them, give each one a task, wait until it is done, and
read what it did. The user sees all of it happen.

## Check first

- `MULTITERMINAL_SESSION_ID` is not set: you are not running in MTUI and this
  skill does not apply.
- That variable is your own session ID. Never send to, wait on or close it.
- There are two ways in. Use the first one that works:
  1. The MCP tools of the `mtui` server: `open_session`, `send_input`,
     `wait_for_agent`, `read_output`, `close_session`, `list_sessions`
     (in Claude Code they appear as `mcp__mtui__open_session` and so on).
  2. The `mt` command line. It only works while MTUI runs its session daemon
     (`session_host: daemon`): `mt hub` exits 0 when one is running and 3
     when not.
- If neither is available, do the work yourself and say that delegation was
  not available.

## When to delegate, and when not

Worth it:
- independent parts of a task that can run side by side, for example the API
  and the UI of one feature, each in its own worktree
- a second opinion: a review or a cross-check by another model
- a long job whose result you only need later

Not worth it:
- small tasks: a new agent needs 10 to 20 seconds to start and costs tokens
- anything that depends on context from this conversation you cannot write
  down in the prompt
- files another agent is already editing

Keep it to three delegated agents at a time unless the user asks for more.
Each one is a full CLI process tree, with its own MCP servers, on the user's
machine.

## The loop

With the MCP tools:

1. `open_session` with `tool` (`claude`, `codex` or `gemini`), an absolute
   `dir`, optionally `model`, and the task as `prompt`. It returns the
   session ID.
2. `wait_for_agent` with that ID. It returns `done`, `blocked` (the agent is
   asking for a permission or an answer) or `exited`. The default timeout is
   300 seconds, the maximum 1800. A timeout only means it is still working:
   wait again if that is still reasonable.
3. Read the result file you asked for (see below), or `read_output` for the
   visible screen.
4. For a follow-up, `send_input` (it is queued until the agent is idle), then
   wait again.
5. `close_session` once you have what you need.

With `mt` (the same steps; exit code 4 from `mt wait` means it timed out and
the agent is still working):

```sh
id=$(mt new claude --dir /abs/path/to/worktree --prompt "...")
mt wait "$id" --timeout 20m      # prints done, blocked or exited
mt read "$id" --lines 40         # the last 40 lines of its screen
mt send "$id" "next instruction"
mt kill "$id"
mt ls                            # every session with its state
```

## Writing the task

The other agent sees nothing of this conversation. The prompt has to stand
on its own:

- the goal and when it counts as done
- the files and paths involved
- the constraints: which branch, whether it may commit, that it must not push
- where the result goes: "Write your final summary to <absolute path> and end
  with the line DONE." Use a path outside the repository, for example in the
  system temp directory. `read_output` shows only the visible screen, and a
  long answer scrolls off it; a file does not.

## Parallel edits

Two agents editing the same checkout overwrite each other. Give every agent
that edits files its own git worktree and pass that directory as `dir`:

```sh
git worktree add .claude/worktrees/<name> -b <branch>
```

Read-only work (review, research) can share the checkout. When
`MULTITERMINAL_FORCE_WORKTREE_ROOT` is set, MTUI blocks writes outside a
worktree anyway.

## When an agent is blocked

It is waiting on a permission prompt or a question. Read its screen. Answer
only what the user's instructions already cover; otherwise tell the user
which pane is waiting and why. Never approve a permission prompt on the
user's behalf. With `mt`, answer with `mt send` or with keys, for example
`mt keys "$id" down enter`.

## Cleaning up

Close every session you opened once you have its result, unless the user
wants to keep working in it: an open agent pane keeps its whole process tree
running. Tell the user which panes you left open, and why.

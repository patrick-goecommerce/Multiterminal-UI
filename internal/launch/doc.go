// Package launch decides what a session's environment must contain.
//
// It is the policy that turns "start claude in this directory" into the exact
// set of variables the pane needs: which port its helpers report to, which
// session ID it announces itself as, and whether the worktree firewall applies
// to it. None of that depends on a window; it depends on the config file and
// on git.
//
// It lives here rather than in internal/backend because the session daemon has
// to be able to build the same environment. As long as the policy sat next to
// the Wails bindings, only a process with a GUI could start a session, which
// is what kept the MCP server and the CLI from creating one. There must be
// exactly one implementation: a pane launched by the daemon that ends up with
// a different environment than one launched by the window is a bug nobody
// finds, because both of them start fine.
package launch

package launch

import "strings"

// Waking a pane up again.
//
// Resuming is a launch, so the command line for it is built here with all the
// others. It lives in this package rather than next to the window because the
// daemon has to be able to wake a pane on its own: a prompt queued for a
// sleeping agent with no window open would otherwise sit there forever.

// ResumeArgv rewrites a launch argv into a resume argv.
//
// Every existing --session-id and --resume is dropped before a single
// `--resume <id>` is appended: the two are mutually exclusive, and a command
// line carrying both is rejected by the CLI rather than ignored.
func ResumeArgv(argv []string, resumeID string) []string {
	out := make([]string, 0, len(argv)+2)
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if arg == "--session-id" || arg == "--resume" {
			if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				i++ // skip the value too
			}
			continue
		}
		if strings.HasPrefix(arg, "--session-id=") || strings.HasPrefix(arg, "--resume=") {
			continue
		}
		out = append(out, arg)
	}
	if resumeID != "" {
		out = append(out, "--resume", resumeID)
	}
	return out
}

// SessionIDFromArgv extracts the agent's own conversation ID from a launch
// argv, empty when there is none.
//
// The frontend generates the UUID and passes it as --session-id, or as
// --resume when a conversation is continued, so at launch time this is the
// only place the value exists. What a lifecycle hook later reports wins over
// it, because the agent may pick a different ID internally; this is what is
// known before the first hook event arrives.
//
// A bare `--resume` (the interactive picker) has no value and yields "".
func SessionIDFromArgv(argv []string) string {
	for i, arg := range argv {
		switch {
		case arg == "--session-id", arg == "--resume":
			if i+1 < len(argv) && !strings.HasPrefix(argv[i+1], "-") {
				return argv[i+1]
			}
		case strings.HasPrefix(arg, "--session-id="):
			return strings.TrimPrefix(arg, "--session-id=")
		case strings.HasPrefix(arg, "--resume="):
			return strings.TrimPrefix(arg, "--resume=")
		}
	}
	return ""
}

// ResumeArgv on a Policy satisfies the hub's Launcher. It is the package
// function: nothing about rewriting a command line depends on the config.
func (Policy) ResumeArgv(argv []string, resumeID string) []string {
	return ResumeArgv(argv, resumeID)
}

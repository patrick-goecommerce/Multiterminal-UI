package launch

import (
	"fmt"
	"sort"
	"strings"
)

// The agent table.
//
// herdr starts 22 agent CLIs; MTUI knows three. That difference is a table,
// not an architecture, and this is the table. Adding a fourth is an entry here
// plus a command in the config, not a change anywhere else.
//
// It lives in this package because the daemon has to start an agent without a
// window, and because the GUI and the daemon must agree on what "claude" means
// down to the argv: a session the daemon starts differently from one the
// window starts is a difference nobody sees until the agent behaves oddly.

// Agent describes one supported agent CLI.
type Agent struct {
	// Tool is the name callers use: "claude", "codex", "gemini".
	Tool string
	// DisplayName is what a human reads.
	DisplayName string
	// Resume reports whether the CLI understands --resume. Only Claude does;
	// codex and gemini have no verified resume path, so a pane running them
	// cannot be put to sleep and woken up again.
	Resume bool
	// ModelFlag is how the CLI takes a model. All three use --model today;
	// the field exists so the fourth one does not have to be a special case.
	ModelFlag string
}

// agents is the table, keyed by tool name.
var agents = map[string]Agent{
	"claude": {Tool: "claude", DisplayName: "Claude", Resume: true, ModelFlag: "--model"},
	"codex":  {Tool: "codex", DisplayName: "Codex", ModelFlag: "--model"},
	"gemini": {Tool: "gemini", DisplayName: "Gemini", ModelFlag: "--model"},
}

// LookupAgent returns the agent for a tool name.
func LookupAgent(tool string) (Agent, bool) {
	a, ok := agents[strings.ToLower(strings.TrimSpace(tool))]
	return a, ok
}

// IsAgentTool reports whether tool names a supported agent CLI.
func IsAgentTool(tool string) bool {
	_, ok := LookupAgent(tool)
	return ok
}

// AgentDisplayName returns the human-readable name, or the tool itself when it
// is not one MTUI knows.
func AgentDisplayName(tool string) string {
	if a, ok := LookupAgent(tool); ok {
		return a.DisplayName
	}
	return tool
}

// KnownAgents lists the supported tools, sorted, for help and error messages.
func KnownAgents() []string {
	out := make([]string, 0, len(agents))
	for tool := range agents {
		out = append(out, tool)
	}
	sort.Strings(out)
	return out
}

// Commands maps a tool name to the command that starts it.
//
// It is passed in rather than read from the config here because the GUI
// resolves the CLIs against a set of well-known install locations at startup
// and wants that resolved absolute path used; the daemon has only what the
// config says. Both end up in the same field, so Argv has one input.
type Commands map[string]string

// Argv builds a plain launch command for one of the supported CLIs.
//
// "Plain" means neither auto-accept nor yolo: those are the window's launch
// modes, chosen per pane in a dialog. An agent or a script asking for a
// session gets the ordinary one, and has to ask for more explicitly.
func (p Policy) Argv(tool, model string) ([]string, error) {
	agent, ok := LookupAgent(tool)
	if !ok {
		return nil, fmt.Errorf("unbekanntes Tool %q, bekannt sind: %s",
			tool, strings.Join(KnownAgents(), ", "))
	}
	cmd := p.Commands[agent.Tool]
	if cmd == "" {
		// The bare name, resolved against the session's PATH at launch. It is
		// the right fallback: a config that names no command is a config that
		// never had to, because the CLI is on the PATH.
		cmd = agent.Tool
	}
	if model == "" {
		return []string{cmd}, nil
	}
	return []string{cmd, agent.ModelFlag, model}, nil
}

// KeepAliveModes are the session modes a keep-alive may type into.
//
// The launch modes of the CLIs that understand a conversation, which today is
// Claude in its three flavours. A shell pane must never be typed into: the
// message would land in whatever the user left running there.
func KeepAliveModes() []string {
	return []string{"claude", "claude-auto", "claude-yolo"}
}

// Policy satisfies hub.Launcher. The assertion is written as a function rather
// than an import of internal/hub, because launch must not depend on the
// package that will depend on it: the hub owns sessions, this owns policy, and
// the arrow points one way only.
var _ interface {
	Argv(tool, model string) ([]string, error)
	Env(sessionID int, dir, mode string) []string
	ResumeArgv(argv []string, resumeID string) []string
} = Policy{}

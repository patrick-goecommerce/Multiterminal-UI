package backend

import (
	"context"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// PaneNameEvent is emitted to the frontend with an auto-generated pane name.
type PaneNameEvent struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// lastNameGen tracks the unix time a pane was last auto-named, to throttle
// regeneration across rapid follow-up prompts.
var (
	nameGenMu   sync.Mutex
	lastNameGen = make(map[int]int64)
)

// cleanupNameTracking removes throttle state for a closed session.
func cleanupNameTracking(id int) {
	nameGenMu.Lock()
	delete(lastNameGen, id)
	nameGenMu.Unlock()
}

// maybeGeneratePaneName is the hook-driven entry point: on a fresh user prompt
// it generates a pane name (throttled, in the background) and emits it to the
// frontend. No-op when auto-naming is disabled or throttled.
func (a *AppService) maybeGeneratePaneName(mtID int, prompt string) {
	if !a.cfg.ShouldAutoName() || strings.TrimSpace(prompt) == "" {
		return
	}

	now := time.Now().Unix()
	nameGenMu.Lock()
	if !shouldRegenerateName(lastNameGen[mtID], now) {
		nameGenMu.Unlock()
		return
	}
	lastNameGen[mtID] = now
	nameGenMu.Unlock()

	model := a.cfg.AutoNaming.Model
	go func() {
		name := a.generatePaneName(model, prompt)
		if name == "" {
			return
		}
		log.Printf("[panename] session %d → %q", mtID, name)
		if a.app != nil {
			a.app.Event.Emit("pane:autoname", PaneNameEvent{ID: mtID, Name: name})
		}
	}()
}

// paneNamePromptMaxInput caps how much of the user's prompt is sent to the
// naming model (runes), keeping the naming call cheap.
const paneNamePromptMaxInput = 600

// paneNameTimeout bounds how long a single naming call may run before it is
// abandoned (the one-shot claude process has noticeable startup overhead).
const paneNameTimeout = 25 * time.Second

// defaultNamingModel is the model used to generate pane names when the config
// does not specify one. Haiku is fast and cheap, ample for 2-4 word titles.
const defaultNamingModel = "claude-haiku-4-5"

// buildNamePrompt builds the instruction sent to the naming model. The user's
// prompt is truncated so a huge first message does not blow up the call.
func buildNamePrompt(userPrompt string) string {
	in := strings.TrimSpace(userPrompt)
	if r := []rune(in); len(r) > paneNamePromptMaxInput {
		in = string(r[:paneNamePromptMaxInput])
	}
	return "Summarize this coding task as a short pane title of 2 to 4 words. " +
		"Reply with ONLY the title — no quotes, no trailing punctuation, " +
		"in the same language as the task.\n\nTask: " + in
}

// paneNameArgs builds the claude argv for a one-shot, non-interactive naming
// call. The prompt is passed as a single argv element.
func paneNameArgs(model, prompt string) []string {
	args := []string{"-p", prompt}
	if model != "" {
		args = append(args, "--model", model)
	}
	return args
}

// generatePaneName runs a one-shot claude call to summarize userPrompt into a
// short pane name. It deliberately does NOT set MULTITERMINAL_SESSION_ID in the
// child env, so the naming call's own hook events carry mt_id=0 and are ignored
// by the HookManager (no recursive naming). Returns "" on any failure.
func (a *AppService) generatePaneName(model, userPrompt string) string {
	if strings.TrimSpace(userPrompt) == "" {
		return ""
	}
	if model == "" {
		model = defaultNamingModel
	}

	path := a.resolvedClaudePath
	if path == "" {
		path = "claude"
	}

	ctx, cancel := context.WithTimeout(context.Background(), paneNameTimeout)
	defer cancel()

	args := paneNameArgs(model, buildNamePrompt(userPrompt))
	cmd := wrapClaudeCmdContext(ctx, path, args)
	cmd.Env = filterEnv(os.Environ(), "CLAUDECODE")

	out, err := cmd.Output()
	if err != nil {
		log.Printf("[panename] generation failed: %v", err)
		return ""
	}
	return sanitizePaneName(string(out))
}

// paneNameMaxLen caps the length (in runes) of an auto-generated pane name so
// it fits the pane titlebar.
const paneNameMaxLen = 24

// paneNameMinIntervalSec throttles regeneration: after naming a pane, a new
// prompt only triggers a fresh name once this many seconds have elapsed. This
// keeps follow-up prompts ("yes", "continue") from spending a model call each
// while still letting the name follow genuine topic changes.
const paneNameMinIntervalSec = 90

// shouldRegenerateName decides whether a pane name should be (re)generated.
// It always generates the first time (lastGenUnix == 0) and otherwise only
// once paneNameMinIntervalSec have passed since the last generation.
func shouldRegenerateName(lastGenUnix, nowUnix int64) bool {
	if lastGenUnix == 0 {
		return true
	}
	return nowUnix-lastGenUnix >= paneNameMinIntervalSec
}

// sanitizePaneName cleans raw LLM output into a short, single-line pane name:
// takes the first line, strips surrounding quotes/backticks and a trailing
// period, and truncates to paneNameMaxLen runes.
func sanitizePaneName(raw string) string {
	name := raw
	if i := strings.IndexAny(name, "\r\n"); i >= 0 {
		name = name[:i]
	}
	name = strings.TrimSpace(name)
	name = strings.Trim(name, "\"'`")
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, ".")
	name = strings.TrimSpace(name)

	if r := []rune(name); len(r) > paneNameMaxLen {
		name = strings.TrimSpace(string(r[:paneNameMaxLen]))
	}
	return name
}

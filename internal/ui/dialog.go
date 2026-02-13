package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/patrick-goecommerce/multiterminal/internal/config"
)

// LaunchChoice describes what the user selected in the launch dialog.
type LaunchChoice struct {
	Type    LaunchType
	Model   string // model ID (only for Claude modes)
	Argv    []string
}

// LaunchType enumerates the kinds of processes the dialog can start.
type LaunchType int

const (
	LaunchShell       LaunchType = iota // plain shell
	LaunchClaude                        // claude (normal permissions)
	LaunchClaudeYOLO                    // claude --dangerously-skip-permissions
	LaunchCancel                        // user cancelled
)

// DialogState describes the current step in the launch dialog flow.
type DialogState int

const (
	DialogStepMode  DialogState = iota // choose Shell / Claude / YOLO
	DialogStepModel                     // choose Claude model (only shown for Claude modes)
)

// Dialog is the modal launch dialog that appears when creating a new pane.
type Dialog struct {
	Visible  bool
	Step     DialogState
	Options  []string
	Cursor   int
	Config   config.Config
	Choice   LaunchChoice

	// Internal: the mode chosen in step 1, before model selection
	chosenType LaunchType
}

// NewDialog creates a dialog pre-populated with config.
func NewDialog(cfg config.Config) Dialog {
	return Dialog{
		Config: cfg,
		Step:   DialogStepMode,
		Options: []string{
			"Shell              (plain terminal)",
			"Claude Code        (normal mode)",
			"Claude Code YOLO   (skip permissions)",
		},
	}
}

// Open makes the dialog visible and resets state.
func (d *Dialog) Open() {
	d.Visible = true
	d.Step = DialogStepMode
	d.Cursor = 0
	d.Options = []string{
		"Shell              (plain terminal)",
		"Claude Code        (normal mode)",
		"Claude Code YOLO   (skip permissions)",
	}
	d.Choice = LaunchChoice{}
}

// Close hides the dialog.
func (d *Dialog) Close() {
	d.Visible = false
}

// MoveUp moves the cursor up in the current option list.
func (d *Dialog) MoveUp() {
	if d.Cursor > 0 {
		d.Cursor--
	}
}

// MoveDown moves the cursor down in the current option list.
func (d *Dialog) MoveDown() {
	if d.Cursor < len(d.Options)-1 {
		d.Cursor++
	}
}

// Select confirms the current cursor choice.
// Returns true when the dialog flow is complete (Choice is populated).
func (d *Dialog) Select() bool {
	switch d.Step {
	case DialogStepMode:
		switch d.Cursor {
		case 0: // Shell
			d.Choice = LaunchChoice{
				Type: LaunchShell,
				Argv: nil, // will use default shell
			}
			d.Close()
			return true
		case 1: // Claude normal
			d.chosenType = LaunchClaude
			d.advanceToModelStep()
			return false
		case 2: // Claude YOLO
			d.chosenType = LaunchClaudeYOLO
			d.advanceToModelStep()
			return false
		}
	case DialogStepModel:
		model := d.Config.ClaudeModels[d.Cursor]
		argv := buildClaudeArgv(d.Config.ClaudeCommand, d.chosenType, model.ID)
		d.Choice = LaunchChoice{
			Type:  d.chosenType,
			Model: model.Label,
			Argv:  argv,
		}
		d.Close()
		return true
	}
	return false
}

// advanceToModelStep switches the dialog to the model selection step.
func (d *Dialog) advanceToModelStep() {
	d.Step = DialogStepModel
	d.Cursor = 0
	d.Options = make([]string, len(d.Config.ClaudeModels))
	for i, m := range d.Config.ClaudeModels {
		label := m.Label
		if m.ID != "" {
			label += "  (" + m.ID + ")"
		}
		d.Options[i] = label
	}
}

// buildClaudeArgv constructs the command-line for launching Claude Code.
func buildClaudeArgv(cmd string, lt LaunchType, modelID string) []string {
	if cmd == "" {
		cmd = "claude"
	}
	argv := []string{cmd}
	if lt == LaunchClaudeYOLO {
		argv = append(argv, "--dangerously-skip-permissions")
	}
	if modelID != "" {
		argv = append(argv, "--model", modelID)
	}
	return argv
}

// Render draws the dialog box.
func (d *Dialog) Render(screenW, screenH int) string {
	if !d.Visible {
		return ""
	}

	var b strings.Builder

	// Title
	switch d.Step {
	case DialogStepMode:
		b.WriteString(DialogTitle.Render("New Terminal Pane"))
		b.WriteByte('\n')
		b.WriteString(DialogHint.Render("Choose what to launch:"))
	case DialogStepModel:
		b.WriteString(DialogTitle.Render("Select Model"))
		b.WriteByte('\n')
		b.WriteString(DialogHint.Render("Choose a Claude model:"))
	}
	b.WriteByte('\n')
	b.WriteByte('\n')

	// Options
	for i, opt := range d.Options {
		prefix := "  "
		style := DialogOption
		if i == d.Cursor {
			prefix = "▸ "
			style = DialogOptionSelected
		}
		b.WriteString(style.Render(prefix + opt))
		b.WriteByte('\n')
	}

	b.WriteByte('\n')
	b.WriteString(DialogHint.Render("↑/↓: navigate  Enter: select  Esc: cancel"))

	box := DialogOverlay.Render(b.String())

	// Centre the dialog on screen
	return lipgloss.Place(screenW, screenH, lipgloss.Center, lipgloss.Center, box)
}

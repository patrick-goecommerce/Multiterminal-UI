// Multiterminal – A TUI terminal multiplexer optimised for Claude Code.
//
// Features:
//   - Tabbed workspaces, each with up to 12 terminal panes
//   - One-click Claude Code launch (normal / YOLO mode / model picker)
//   - Per-pane status: git branch, model, activity
//   - Global footer: branch (copyable), model, shortcuts
//   - Collapsible file-browser sidebar with search
//   - Working directory preset per tab
//
// Stack: Go · Bubbletea · Lipgloss · Bubbles · creack/pty
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/patrick-goecommerce/multiterminal/internal/app"
	"github.com/patrick-goecommerce/multiterminal/internal/config"
)

func main() {
	cfg := config.Load()
	model := app.New(cfg)

	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

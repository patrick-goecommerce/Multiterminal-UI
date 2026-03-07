// Package backend provides Kanban board state management with persistence.
package backend

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// KanbanState represents the full board state for a project.
type KanbanState struct {
	Columns   map[string][]KanbanCard `json:"columns" yaml:"columns"`
	Plans     []Plan                  `json:"plans" yaml:"plans"`
	Schedules []ScheduledTask         `json:"schedules" yaml:"schedules"`
}

// KanbanCard represents a single card on the board.
type KanbanCard struct {
	ID           string   `json:"id" yaml:"id"`
	IssueNumber  int      `json:"issue_number" yaml:"issue_number"`
	Title        string   `json:"title" yaml:"title"`
	Labels       []string `json:"labels" yaml:"labels"`
	Dir          string   `json:"dir" yaml:"dir"`
	SessionID    int      `json:"session_id" yaml:"session_id"`
	Priority     int      `json:"priority" yaml:"priority"`
	Dependencies []int    `json:"dependencies" yaml:"dependencies"`
	PlanID       string   `json:"plan_id" yaml:"plan_id"`
	ScheduleID   string   `json:"schedule_id" yaml:"schedule_id"`
	CreatedAt    string   `json:"created_at" yaml:"created_at"`
}

// Column IDs used by the Kanban board.
const (
	ColBacklog    = "backlog"
	ColPlanned    = "planned"
	ColInProgress = "in_progress"
	ColReview     = "review"
	ColDone       = "done"
)

// defaultColumns returns the ordered column list.
func defaultColumns() []string {
	return []string{ColBacklog, ColPlanned, ColInProgress, ColReview, ColDone}
}

// GetKanbanState loads the Kanban state for a project directory.
func (a *AppService) GetKanbanState(dir string) KanbanState {
	state, err := loadKanbanState(dir)
	if err != nil {
		log.Printf("[kanban] load error for %s: %v", dir, err)
		return newKanbanState()
	}
	return state
}

// SaveKanbanState persists the full Kanban state for a project.
func (a *AppService) SaveKanbanState(dir string, state KanbanState) error {
	return saveKanbanState(dir, state)
}

// MoveKanbanCard moves a card to a different column at a given position.
func (a *AppService) MoveKanbanCard(dir string, cardID string, toColumn string, position int) error {
	state, err := loadKanbanState(dir)
	if err != nil {
		return fmt.Errorf("load kanban: %w", err)
	}

	// Find and remove card from current column
	var card KanbanCard
	found := false
	for col, cards := range state.Columns {
		for i, c := range cards {
			if c.ID == cardID {
				card = c
				state.Columns[col] = append(cards[:i], cards[i+1:]...)
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		return fmt.Errorf("card %s not found", cardID)
	}

	// Insert at new position
	target := state.Columns[toColumn]
	if position < 0 || position > len(target) {
		position = len(target)
	}
	newCards := make([]KanbanCard, 0, len(target)+1)
	newCards = append(newCards, target[:position]...)
	newCards = append(newCards, card)
	newCards = append(newCards, target[position:]...)
	state.Columns[toColumn] = newCards

	return saveKanbanState(dir, state)
}

// AddKanbanCard adds a new card to a column (default: backlog).
func (a *AppService) AddKanbanCard(dir string, card KanbanCard) (KanbanCard, error) {
	state, err := loadKanbanState(dir)
	if err != nil {
		return card, fmt.Errorf("load kanban: %w", err)
	}

	if card.ID == "" {
		card.ID = generateID()
	}
	if card.CreatedAt == "" {
		card.CreatedAt = time.Now().Format(time.RFC3339)
	}

	col := ColBacklog
	state.Columns[col] = append(state.Columns[col], card)

	if err := saveKanbanState(dir, state); err != nil {
		return card, fmt.Errorf("save kanban: %w", err)
	}
	return card, nil
}

// RemoveKanbanCard removes a card from all columns.
func (a *AppService) RemoveKanbanCard(dir string, cardID string) error {
	state, err := loadKanbanState(dir)
	if err != nil {
		return fmt.Errorf("load kanban: %w", err)
	}

	for col, cards := range state.Columns {
		for i, c := range cards {
			if c.ID == cardID {
				state.Columns[col] = append(cards[:i], cards[i+1:]...)
				return saveKanbanState(dir, state)
			}
		}
	}
	return fmt.Errorf("card %s not found", cardID)
}

// SyncKanbanWithIssues syncs GitHub issues into the Kanban board.
// New issues go to backlog, closed issues move to done.
func (a *AppService) SyncKanbanWithIssues(dir string) KanbanState {
	state, err := loadKanbanState(dir)
	if err != nil {
		state = newKanbanState()
	}

	// Get existing issue numbers across all columns
	existing := make(map[int]bool)
	for _, cards := range state.Columns {
		for _, c := range cards {
			if c.IssueNumber > 0 {
				existing[c.IssueNumber] = true
			}
		}
	}

	// Fetch open issues
	issues := a.GetIssues(dir, "open")
	for _, iss := range issues {
		if existing[iss.Number] {
			continue
		}
		card := KanbanCard{
			ID:          generateID(),
			IssueNumber: iss.Number,
			Title:       iss.Title,
			Labels:      iss.Labels,
			Dir:         dir,
			CreatedAt:   time.Now().Format(time.RFC3339),
		}
		state.Columns[ColBacklog] = append(state.Columns[ColBacklog], card)
	}

	// Move closed issues to done
	closedIssues := a.GetIssues(dir, "closed")
	closedSet := make(map[int]bool)
	for _, iss := range closedIssues {
		closedSet[iss.Number] = true
	}

	for col, cards := range state.Columns {
		if col == ColDone {
			continue
		}
		remaining := make([]KanbanCard, 0, len(cards))
		for _, c := range cards {
			if c.IssueNumber > 0 && closedSet[c.IssueNumber] {
				state.Columns[ColDone] = append(state.Columns[ColDone], c)
			} else {
				remaining = append(remaining, c)
			}
		}
		state.Columns[col] = remaining
	}

	if err := saveKanbanState(dir, state); err != nil {
		log.Printf("[kanban] save error after sync: %v", err)
	}
	return state
}

// newKanbanState creates an empty state with all columns.
func newKanbanState() KanbanState {
	cols := make(map[string][]KanbanCard)
	for _, c := range defaultColumns() {
		cols[c] = []KanbanCard{}
	}
	return KanbanState{
		Columns:   cols,
		Plans:     []Plan{},
		Schedules: []ScheduledTask{},
	}
}

// kanbanPath returns the path to kanban.json for a project.
func kanbanPath(dir string) string {
	return filepath.Join(dir, ".mtui", "kanban.json")
}

// loadKanbanState reads kanban.json from the .mtui directory.
func loadKanbanState(dir string) (KanbanState, error) {
	data, err := os.ReadFile(kanbanPath(dir))
	if err != nil {
		if os.IsNotExist(err) {
			return newKanbanState(), nil
		}
		return KanbanState{}, fmt.Errorf("read kanban: %w", err)
	}
	var state KanbanState
	if err := json.Unmarshal(data, &state); err != nil {
		return KanbanState{}, fmt.Errorf("parse kanban: %w", err)
	}
	// Ensure all columns exist
	for _, c := range defaultColumns() {
		if state.Columns[c] == nil {
			state.Columns[c] = []KanbanCard{}
		}
	}
	if state.Plans == nil {
		state.Plans = []Plan{}
	}
	if state.Schedules == nil {
		state.Schedules = []ScheduledTask{}
	}
	return state, nil
}

// saveKanbanState writes kanban.json to the .mtui directory.
func saveKanbanState(dir string, state KanbanState) error {
	mtuiDir := filepath.Join(dir, ".mtui")
	if err := os.MkdirAll(mtuiDir, 0o755); err != nil {
		return fmt.Errorf("create .mtui dir: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal kanban: %w", err)
	}
	return os.WriteFile(kanbanPath(dir), data, 0o644)
}

// generateID creates a simple unique ID from timestamp.
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

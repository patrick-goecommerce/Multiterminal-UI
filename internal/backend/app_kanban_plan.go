// Package backend provides auto-planning for Kanban cards.
package backend

import (
	"fmt"
	"log"
	"time"
)

// Plan represents an execution plan for a set of Kanban cards.
type Plan struct {
	ID        string     `json:"id" yaml:"id"`
	Dir       string     `json:"dir" yaml:"dir"`
	CreatedAt string     `json:"created_at" yaml:"created_at"`
	Steps     []PlanStep `json:"steps" yaml:"steps"`
	Status    string     `json:"status" yaml:"status"` // draft/approved/running/done
}

// PlanStep represents a single step within a plan.
type PlanStep struct {
	IssueNumber int    `json:"issue_number" yaml:"issue_number"`
	CardID      string `json:"card_id" yaml:"card_id"`
	Title       string `json:"title" yaml:"title"`
	Order       int    `json:"order" yaml:"order"`
	Parallel    bool   `json:"parallel" yaml:"parallel"`
	SessionID   int    `json:"session_id" yaml:"session_id"`
	Status      string `json:"status" yaml:"status"` // pending/running/done/skipped
	Prompt      string `json:"prompt" yaml:"prompt"`
}

// GeneratePlan creates a plan from selected backlog cards.
// Orders by priority (higher first) and groups parallel-safe items.
func (a *AppService) GeneratePlan(dir string, cardIDs []string) (Plan, error) {
	state, err := loadKanbanState(dir)
	if err != nil {
		return Plan{}, fmt.Errorf("load kanban: %w", err)
	}

	// Collect selected cards from backlog
	cards := make([]KanbanCard, 0)
	cardMap := make(map[string]KanbanCard)
	for _, col := range defaultColumns() {
		for _, c := range state.Columns[col] {
			cardMap[c.ID] = c
		}
	}
	for _, id := range cardIDs {
		if c, ok := cardMap[id]; ok {
			cards = append(cards, c)
		}
	}

	if len(cards) == 0 {
		return Plan{}, fmt.Errorf("no cards selected")
	}

	// Sort by priority (higher first), then by issue number
	sortCards(cards)

	plan := Plan{
		ID:        generateID(),
		Dir:       dir,
		CreatedAt: time.Now().Format(time.RFC3339),
		Status:    "draft",
		Steps:     make([]PlanStep, len(cards)),
	}

	for i, c := range cards {
		prompt := fmt.Sprintf("Work on: %s", c.Title)
		if c.IssueNumber > 0 {
			prompt = fmt.Sprintf("Fix issue #%d: %s", c.IssueNumber, c.Title)
		}
		plan.Steps[i] = PlanStep{
			IssueNumber: c.IssueNumber,
			CardID:      c.ID,
			Title:       c.Title,
			Order:       i + 1,
			Parallel:    false,
			Status:      "pending",
			Prompt:      prompt,
		}
	}

	// Save plan to state
	state.Plans = append(state.Plans, plan)
	if err := saveKanbanState(dir, state); err != nil {
		return plan, fmt.Errorf("save plan: %w", err)
	}

	log.Printf("[kanban] generated plan %s with %d steps", plan.ID, len(plan.Steps))
	return plan, nil
}

// GetPlans returns all plans for a project.
func (a *AppService) GetPlans(dir string) []Plan {
	state, err := loadKanbanState(dir)
	if err != nil {
		return []Plan{}
	}
	return state.Plans
}

// ApprovePlan changes a plan's status from draft to approved.
func (a *AppService) ApprovePlan(dir string, planID string) error {
	state, err := loadKanbanState(dir)
	if err != nil {
		return fmt.Errorf("load kanban: %w", err)
	}

	for i, p := range state.Plans {
		if p.ID == planID {
			if p.Status != "draft" {
				return fmt.Errorf("plan %s is not in draft status", planID)
			}
			state.Plans[i].Status = "approved"

			// Move cards to planned column
			for _, step := range p.Steps {
				a.moveCardToColumn(&state, step.CardID, ColPlanned)
			}

			return saveKanbanState(dir, state)
		}
	}
	return fmt.Errorf("plan %s not found", planID)
}

// CancelPlan cancels a plan and moves cards back to backlog.
func (a *AppService) CancelPlan(dir string, planID string) error {
	state, err := loadKanbanState(dir)
	if err != nil {
		return fmt.Errorf("load kanban: %w", err)
	}

	for i, p := range state.Plans {
		if p.ID == planID {
			// Move non-done cards back to backlog
			for _, step := range p.Steps {
				if step.Status != "done" {
					a.moveCardToColumn(&state, step.CardID, ColBacklog)
				}
			}
			state.Plans[i].Status = "cancelled"
			return saveKanbanState(dir, state)
		}
	}
	return fmt.Errorf("plan %s not found", planID)
}

// DeletePlan removes a plan entirely.
func (a *AppService) DeletePlan(dir string, planID string) error {
	state, err := loadKanbanState(dir)
	if err != nil {
		return fmt.Errorf("load kanban: %w", err)
	}

	for i, p := range state.Plans {
		if p.ID == planID {
			state.Plans = append(state.Plans[:i], state.Plans[i+1:]...)
			return saveKanbanState(dir, state)
		}
	}
	return fmt.Errorf("plan %s not found", planID)
}

// UpdatePlanStep updates a step's order, parallel flag, or prompt.
func (a *AppService) UpdatePlanStep(dir string, planID string, step PlanStep) error {
	state, err := loadKanbanState(dir)
	if err != nil {
		return fmt.Errorf("load kanban: %w", err)
	}

	for i, p := range state.Plans {
		if p.ID == planID {
			for j, s := range p.Steps {
				if s.CardID == step.CardID {
					state.Plans[i].Steps[j].Order = step.Order
					state.Plans[i].Steps[j].Parallel = step.Parallel
					state.Plans[i].Steps[j].Prompt = step.Prompt
					return saveKanbanState(dir, state)
				}
			}
			return fmt.Errorf("step for card %s not found", step.CardID)
		}
	}
	return fmt.Errorf("plan %s not found", planID)
}

// moveCardToColumn moves a card by ID to a target column.
func (a *AppService) moveCardToColumn(state *KanbanState, cardID string, toCol string) {
	for col, cards := range state.Columns {
		for i, c := range cards {
			if c.ID == cardID {
				state.Columns[col] = append(cards[:i], cards[i+1:]...)
				state.Columns[toCol] = append(state.Columns[toCol], c)
				return
			}
		}
	}
}

// sortCards sorts cards by priority desc, then issue number asc.
func sortCards(cards []KanbanCard) {
	for i := 1; i < len(cards); i++ {
		for j := i; j > 0; j-- {
			if cards[j].Priority > cards[j-1].Priority {
				cards[j], cards[j-1] = cards[j-1], cards[j]
			} else if cards[j].Priority == cards[j-1].Priority &&
				cards[j].IssueNumber < cards[j-1].IssueNumber {
				cards[j], cards[j-1] = cards[j-1], cards[j]
			}
		}
	}
}

package orchestrator

import (
	"errors"
	"fmt"
	"sync"
)

var ErrBudgetExhausted = errors.New("budget exhausted")
var ErrCardNotAllocated = errors.New("card has no allocated budget")

// BudgetTracker manages per-card spending limits.
type BudgetTracker struct {
	mu       sync.Mutex
	budgets  map[string]float64 // cardID → remaining USD
	initial  map[string]float64 // cardID → initial allocation
	defaults map[string]float64 // complexity → default budget
}

// DefaultBudgets returns the spec-defined budget defaults.
func DefaultBudgets() map[string]float64 {
	return map[string]float64{
		"trivial": 0.50,
		"medium":  2.00,
		"complex": 10.00,
	}
}

// NewBudgetTracker creates a BudgetTracker with the given complexity defaults.
// If defaults is nil, DefaultBudgets() is used.
func NewBudgetTracker(defaults map[string]float64) *BudgetTracker {
	if defaults == nil {
		defaults = DefaultBudgets()
	}
	return &BudgetTracker{
		budgets:  make(map[string]float64),
		initial:  make(map[string]float64),
		defaults: defaults,
	}
}

// Allocate sets the initial budget for a card based on its complexity.
// If complexity is unknown, uses "medium" as fallback.
func (bt *BudgetTracker) Allocate(cardID, complexity string) error {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	budget, ok := bt.defaults[complexity]
	if !ok {
		budget = bt.defaults["medium"]
	}

	bt.budgets[cardID] = budget
	bt.initial[cardID] = budget
	return nil
}

// Spend deducts an amount from a card's budget.
// Returns ErrBudgetExhausted if spending would make budget negative.
// Returns ErrCardNotAllocated if card has no budget.
func (bt *BudgetTracker) Spend(cardID string, amount float64) error {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	remaining, ok := bt.budgets[cardID]
	if !ok {
		return fmt.Errorf("%w: %s", ErrCardNotAllocated, cardID)
	}

	if amount > remaining {
		return fmt.Errorf("%w: need $%.2f but only $%.2f remaining for card %s",
			ErrBudgetExhausted, amount, remaining, cardID)
	}

	bt.budgets[cardID] = remaining - amount
	return nil
}

// Remaining returns the remaining budget for a card.
func (bt *BudgetTracker) Remaining(cardID string) (float64, error) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	remaining, ok := bt.budgets[cardID]
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrCardNotAllocated, cardID)
	}
	return remaining, nil
}

// CanSpend checks if a card has enough budget for the given amount.
func (bt *BudgetTracker) CanSpend(cardID string, amount float64) bool {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	remaining, ok := bt.budgets[cardID]
	if !ok {
		return false
	}
	return remaining >= amount
}

// TotalSpent returns how much has been spent on a card (initial - remaining).
func (bt *BudgetTracker) TotalSpent(cardID string) (float64, error) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	remaining, ok := bt.budgets[cardID]
	if !ok {
		return 0, fmt.Errorf("%w: %s", ErrCardNotAllocated, cardID)
	}

	initial := bt.initial[cardID]
	return initial - remaining, nil
}

package backend

import (
	"testing"
)

// ---------------------------------------------------------------------------
// sortCards — priority desc, issue number asc
// ---------------------------------------------------------------------------

func TestSortCards_ByPriorityDesc(t *testing.T) {
	cards := []KanbanCard{
		{ID: "a", Priority: 1, IssueNumber: 10},
		{ID: "b", Priority: 3, IssueNumber: 5},
		{ID: "c", Priority: 2, IssueNumber: 1},
	}
	sortCards(cards)
	if cards[0].ID != "b" || cards[1].ID != "c" || cards[2].ID != "a" {
		t.Errorf("sort order wrong: %s, %s, %s", cards[0].ID, cards[1].ID, cards[2].ID)
	}
}

func TestSortCards_SamePriorityByIssueAsc(t *testing.T) {
	cards := []KanbanCard{
		{ID: "a", Priority: 1, IssueNumber: 10},
		{ID: "b", Priority: 1, IssueNumber: 3},
		{ID: "c", Priority: 1, IssueNumber: 7},
	}
	sortCards(cards)
	if cards[0].ID != "b" || cards[1].ID != "c" || cards[2].ID != "a" {
		t.Errorf("sort order wrong: %s, %s, %s", cards[0].ID, cards[1].ID, cards[2].ID)
	}
}

func TestSortCards_EmptySlice(t *testing.T) {
	var cards []KanbanCard
	sortCards(cards) // should not panic
}

func TestSortCards_SingleCard(t *testing.T) {
	cards := []KanbanCard{{ID: "a", Priority: 5}}
	sortCards(cards)
	if cards[0].ID != "a" {
		t.Error("single card should be unchanged")
	}
}

// ---------------------------------------------------------------------------
// GeneratePlan — plan creation from cards
// ---------------------------------------------------------------------------

func TestGeneratePlan_Basic(t *testing.T) {
	dir := t.TempDir()
	state := newKanbanState()
	state.Columns[ColBacklog] = []KanbanCard{
		{ID: "c1", Title: "Task 1", Priority: 2, IssueNumber: 5},
		{ID: "c2", Title: "Task 2", Priority: 1},
	}
	if err := saveKanbanState(dir, state); err != nil {
		t.Fatal(err)
	}

	app := newTestApp()
	plan, err := app.GeneratePlan(dir, []string{"c1", "c2"})
	if err != nil {
		t.Fatalf("generate plan error: %v", err)
	}
	if plan.Status != "draft" {
		t.Errorf("plan status = %q, want %q", plan.Status, "draft")
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(plan.Steps))
	}
	// Higher priority card should be first
	if plan.Steps[0].CardID != "c1" {
		t.Errorf("first step should be c1 (higher priority), got %s", plan.Steps[0].CardID)
	}
	// Issue-based prompt
	if plan.Steps[0].IssueNumber != 5 {
		t.Errorf("step 0 issue = %d, want 5", plan.Steps[0].IssueNumber)
	}
}

func TestGeneratePlan_NoCards(t *testing.T) {
	dir := t.TempDir()
	if err := saveKanbanState(dir, newKanbanState()); err != nil {
		t.Fatal(err)
	}
	app := newTestApp()
	_, err := app.GeneratePlan(dir, []string{"nonexistent"})
	if err == nil {
		t.Error("expected error for no matching cards")
	}
}

// ---------------------------------------------------------------------------
// ApprovePlan — status transition and card movement
// ---------------------------------------------------------------------------

func TestApprovePlan(t *testing.T) {
	dir := t.TempDir()
	state := newKanbanState()
	state.Columns[ColBacklog] = []KanbanCard{
		{ID: "c1", Title: "Task 1"},
	}
	state.Plans = []Plan{
		{ID: "p1", Status: "draft", Steps: []PlanStep{{CardID: "c1"}}},
	}
	if err := saveKanbanState(dir, state); err != nil {
		t.Fatal(err)
	}

	app := newTestApp()
	if err := app.ApprovePlan(dir, "p1"); err != nil {
		t.Fatalf("approve error: %v", err)
	}

	loaded, _ := loadKanbanState(dir)
	if loaded.Plans[0].Status != "approved" {
		t.Errorf("plan status = %q, want %q", loaded.Plans[0].Status, "approved")
	}
	// Card should have moved to planned
	if len(loaded.Columns[ColPlanned]) != 1 {
		t.Error("card should be in planned column")
	}
}

func TestApprovePlan_NotDraft(t *testing.T) {
	dir := t.TempDir()
	state := newKanbanState()
	state.Plans = []Plan{{ID: "p1", Status: "approved"}}
	saveKanbanState(dir, state)

	app := newTestApp()
	err := app.ApprovePlan(dir, "p1")
	if err == nil {
		t.Error("expected error when plan is not in draft status")
	}
}

// ---------------------------------------------------------------------------
// CancelPlan
// ---------------------------------------------------------------------------

func TestCancelPlan(t *testing.T) {
	dir := t.TempDir()
	state := newKanbanState()
	state.Columns[ColInProgress] = []KanbanCard{{ID: "c1", Title: "T"}}
	state.Plans = []Plan{
		{ID: "p1", Status: "running", Steps: []PlanStep{
			{CardID: "c1", Status: "running"},
		}},
	}
	saveKanbanState(dir, state)

	app := newTestApp()
	if err := app.CancelPlan(dir, "p1"); err != nil {
		t.Fatalf("cancel error: %v", err)
	}

	loaded, _ := loadKanbanState(dir)
	if loaded.Plans[0].Status != "cancelled" {
		t.Errorf("plan status = %q, want %q", loaded.Plans[0].Status, "cancelled")
	}
	// Non-done cards should be back in backlog
	if len(loaded.Columns[ColBacklog]) != 1 {
		t.Error("card should be back in backlog")
	}
}

// ---------------------------------------------------------------------------
// DeletePlan
// ---------------------------------------------------------------------------

func TestDeletePlan(t *testing.T) {
	dir := t.TempDir()
	state := newKanbanState()
	state.Plans = []Plan{{ID: "p1"}, {ID: "p2"}}
	saveKanbanState(dir, state)

	app := newTestApp()
	if err := app.DeletePlan(dir, "p1"); err != nil {
		t.Fatalf("delete error: %v", err)
	}

	loaded, _ := loadKanbanState(dir)
	if len(loaded.Plans) != 1 || loaded.Plans[0].ID != "p2" {
		t.Errorf("expected only p2 to remain, got %v", loaded.Plans)
	}
}

// ---------------------------------------------------------------------------
// moveCardToColumn — internal helper
// ---------------------------------------------------------------------------

func TestMoveCardToColumn(t *testing.T) {
	state := newKanbanState()
	state.Columns[ColBacklog] = []KanbanCard{{ID: "c1", Title: "T"}}

	app := newTestApp()
	app.moveCardToColumn(&state, "c1", ColReview)

	if len(state.Columns[ColBacklog]) != 0 {
		t.Error("card should be removed from backlog")
	}
	if len(state.Columns[ColReview]) != 1 {
		t.Error("card should be in review")
	}
}

func TestMoveCardToColumn_NotFound(t *testing.T) {
	state := newKanbanState()
	app := newTestApp()
	// Should not panic
	app.moveCardToColumn(&state, "nonexistent", ColDone)
}

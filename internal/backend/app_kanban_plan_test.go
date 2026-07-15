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
	state.Columns[ColDefine] = []KanbanCard{
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
	state.Columns[ColDefine] = []KanbanCard{
		{ID: "c1", Title: "Task 1"},
	}
	state.Plans = []Plan{
		{ID: "p1", Status: "draft", Steps: []PlanStep{{CardID: "c1"}}},
	}
	if err := saveKanbanState(dir, state); err != nil {
		t.Fatal(err)
	}

	app := newTestApp()
	if err := app.ApprovePlan(dir, "p1", 0); err != nil {
		t.Fatalf("approve error: %v", err)
	}

	loaded, _ := loadKanbanState(dir)
	if loaded.Plans[0].Status != "approved" {
		t.Errorf("plan status = %q, want %q", loaded.Plans[0].Status, "approved")
	}
	// Card should have moved to planned
	if len(loaded.Columns[ColApproved]) != 0 {
		// Original card moved, plus sub-ticket generated
	}
}

func TestApprovePlan_NotDraft(t *testing.T) {
	dir := t.TempDir()
	state := newKanbanState()
	state.Plans = []Plan{{ID: "p1", Status: "approved"}}
	saveKanbanState(dir, state)

	app := newTestApp()
	err := app.ApprovePlan(dir, "p1", 0)
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
	if len(loaded.Columns[ColDefine]) != 1 {
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
	state.Columns[ColDefine] = []KanbanCard{{ID: "c1", Title: "T"}}

	app := newTestApp()
	app.moveCardToColumn(&state, "c1", ColAutoReview)

	if len(state.Columns[ColDefine]) != 0 {
		t.Error("card should be removed from backlog")
	}
	if len(state.Columns[ColAutoReview]) != 1 {
		t.Error("card should be in review")
	}
}

func TestMoveCardToColumn_NotFound(t *testing.T) {
	state := newKanbanState()
	app := newTestApp()
	// Should not panic
	app.moveCardToColumn(&state, "nonexistent", ColDone)
}

// ---------------------------------------------------------------------------
// GenerateSubTickets — creates cards from plan steps
// ---------------------------------------------------------------------------

func TestGenerateSubTickets_Basic(t *testing.T) {
	dir := t.TempDir()
	state := newKanbanState()
	state.Plans = []Plan{
		{
			ID:     "p1",
			Status: "approved",
			Steps: []PlanStep{
				{Title: "Step 1", Prompt: "Do step 1", Order: 1},
				{Title: "Step 2", Prompt: "Do step 2", Order: 2},
			},
		},
	}
	if err := saveKanbanState(dir, state); err != nil {
		t.Fatal(err)
	}

	app := newTestApp()
	if err := app.GenerateSubTickets(dir, "p1", 42); err != nil {
		t.Fatalf("generate sub-tickets error: %v", err)
	}

	loaded, _ := loadKanbanState(dir)
	// Cards should be in approved column (AutoStart defaults to false)
	if len(loaded.Columns[ColApproved]) != 2 {
		t.Errorf("expected 2 cards in approved, got %d", len(loaded.Columns[ColApproved]))
	}
	card := loaded.Columns[ColApproved][0]
	if card.ParentIssue != 42 {
		t.Errorf("parent issue = %d, want 42", card.ParentIssue)
	}
	if card.PlanID != "p1" {
		t.Errorf("plan ID = %q, want %q", card.PlanID, "p1")
	}
	if card.Prompt != "Do step 1" {
		t.Errorf("prompt = %q, want %q", card.Prompt, "Do step 1")
	}

	// Steps should have CardIDs set
	if loaded.Plans[0].Steps[0].CardID == "" {
		t.Error("step 0 should have CardID set")
	}
	if loaded.Plans[0].Steps[1].CardID == "" {
		t.Error("step 1 should have CardID set")
	}

	// Second card should have dependency on first step's order
	card2 := loaded.Columns[ColApproved][1]
	if len(card2.Dependencies) != 1 || card2.Dependencies[0] != 1 {
		t.Errorf("card2 dependencies = %v, want [1]", card2.Dependencies)
	}
}

func TestGenerateSubTickets_NotFound(t *testing.T) {
	dir := t.TempDir()
	if err := saveKanbanState(dir, newKanbanState()); err != nil {
		t.Fatal(err)
	}
	app := newTestApp()
	err := app.GenerateSubTickets(dir, "nonexistent", 0)
	if err == nil {
		t.Error("expected error for missing plan")
	}
}

// ---------------------------------------------------------------------------
// GetSubTicketProgress — counts done vs total for a parent issue
// ---------------------------------------------------------------------------

func TestGetSubTicketProgress(t *testing.T) {
	dir := t.TempDir()
	state := newKanbanState()
	state.Columns[ColApproved] = []KanbanCard{
		{ID: "c1", ParentIssue: 10},
		{ID: "c2", ParentIssue: 10},
	}
	state.Columns[ColDone] = []KanbanCard{
		{ID: "c3", ParentIssue: 10},
	}
	state.Columns[ColInProgress] = []KanbanCard{
		{ID: "c4", ParentIssue: 99}, // different parent
	}
	if err := saveKanbanState(dir, state); err != nil {
		t.Fatal(err)
	}

	app := newTestApp()
	done, total := app.GetSubTicketProgress(dir, 10)
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	if done != 1 {
		t.Errorf("done = %d, want 1", done)
	}

	// Different parent issue
	done2, total2 := app.GetSubTicketProgress(dir, 99)
	if total2 != 1 || done2 != 0 {
		t.Errorf("parent 99: done=%d total=%d, want 0/1", done2, total2)
	}
}

func TestGetSubTicketProgress_NoCards(t *testing.T) {
	dir := t.TempDir()
	if err := saveKanbanState(dir, newKanbanState()); err != nil {
		t.Fatal(err)
	}
	app := newTestApp()
	done, total := app.GetSubTicketProgress(dir, 999)
	if done != 0 || total != 0 {
		t.Errorf("expected 0/0, got %d/%d", done, total)
	}
}

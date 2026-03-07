package backend

import (
	"os"
	"path/filepath"
	"testing"
)

// ---------------------------------------------------------------------------
// Kanban state persistence — load/save cycle
// ---------------------------------------------------------------------------

func TestNewKanbanState_HasAllColumns(t *testing.T) {
	state := newKanbanState()
	for _, col := range defaultColumns() {
		if _, ok := state.Columns[col]; !ok {
			t.Errorf("missing column %q", col)
		}
	}
	if state.Plans == nil {
		t.Error("plans should be non-nil")
	}
	if state.Schedules == nil {
		t.Error("schedules should be non-nil")
	}
}

func TestKanbanPath(t *testing.T) {
	got := kanbanPath("/project")
	want := filepath.Join("/project", ".mtui", "kanban.json")
	if got != want {
		t.Errorf("kanbanPath() = %q, want %q", got, want)
	}
}

func TestLoadKanbanState_NonExistentDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nonexistent")
	state, err := loadKanbanState(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return a valid empty state
	for _, col := range defaultColumns() {
		if state.Columns[col] == nil {
			t.Errorf("column %q should be non-nil", col)
		}
	}
}

func TestSaveAndLoadKanbanState(t *testing.T) {
	dir := t.TempDir()
	state := newKanbanState()
	state.Columns[ColBacklog] = []KanbanCard{
		{ID: "card-1", Title: "Test card", Priority: 3},
	}

	if err := saveKanbanState(dir, state); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := loadKanbanState(dir)
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if len(loaded.Columns[ColBacklog]) != 1 {
		t.Fatalf("expected 1 card in backlog, got %d", len(loaded.Columns[ColBacklog]))
	}
	if loaded.Columns[ColBacklog][0].Title != "Test card" {
		t.Errorf("title mismatch: got %q", loaded.Columns[ColBacklog][0].Title)
	}
	if loaded.Columns[ColBacklog][0].Priority != 3 {
		t.Errorf("priority mismatch: got %d", loaded.Columns[ColBacklog][0].Priority)
	}
}

// ---------------------------------------------------------------------------
// MoveKanbanCard
// ---------------------------------------------------------------------------

func TestMoveKanbanCard(t *testing.T) {
	dir := t.TempDir()
	state := newKanbanState()
	state.Columns[ColBacklog] = []KanbanCard{
		{ID: "c1", Title: "Card 1"},
		{ID: "c2", Title: "Card 2"},
	}
	if err := saveKanbanState(dir, state); err != nil {
		t.Fatal(err)
	}

	app := newTestApp()
	if err := app.MoveKanbanCard(dir, "c1", ColInProgress, 0); err != nil {
		t.Fatalf("move error: %v", err)
	}

	loaded, _ := loadKanbanState(dir)
	if len(loaded.Columns[ColBacklog]) != 1 {
		t.Errorf("backlog should have 1 card, got %d", len(loaded.Columns[ColBacklog]))
	}
	if len(loaded.Columns[ColInProgress]) != 1 {
		t.Errorf("in_progress should have 1 card, got %d", len(loaded.Columns[ColInProgress]))
	}
	if loaded.Columns[ColInProgress][0].ID != "c1" {
		t.Errorf("wrong card moved: got %q", loaded.Columns[ColInProgress][0].ID)
	}
}

func TestMoveKanbanCard_NotFound(t *testing.T) {
	dir := t.TempDir()
	if err := saveKanbanState(dir, newKanbanState()); err != nil {
		t.Fatal(err)
	}
	app := newTestApp()
	err := app.MoveKanbanCard(dir, "nonexistent", ColDone, 0)
	if err == nil {
		t.Error("expected error for non-existent card")
	}
}

// ---------------------------------------------------------------------------
// AddKanbanCard
// ---------------------------------------------------------------------------

func TestAddKanbanCard(t *testing.T) {
	dir := t.TempDir()
	if err := saveKanbanState(dir, newKanbanState()); err != nil {
		t.Fatal(err)
	}

	app := newTestApp()
	card, err := app.AddKanbanCard(dir, KanbanCard{Title: "New card"})
	if err != nil {
		t.Fatalf("add error: %v", err)
	}
	if card.ID == "" {
		t.Error("card ID should be auto-generated")
	}
	if card.CreatedAt == "" {
		t.Error("created_at should be set")
	}

	loaded, _ := loadKanbanState(dir)
	if len(loaded.Columns[ColBacklog]) != 1 {
		t.Errorf("expected 1 card in backlog, got %d", len(loaded.Columns[ColBacklog]))
	}
}

// ---------------------------------------------------------------------------
// RemoveKanbanCard
// ---------------------------------------------------------------------------

func TestRemoveKanbanCard(t *testing.T) {
	dir := t.TempDir()
	state := newKanbanState()
	state.Columns[ColBacklog] = []KanbanCard{{ID: "rm1", Title: "Remove me"}}
	if err := saveKanbanState(dir, state); err != nil {
		t.Fatal(err)
	}

	app := newTestApp()
	if err := app.RemoveKanbanCard(dir, "rm1"); err != nil {
		t.Fatalf("remove error: %v", err)
	}

	loaded, _ := loadKanbanState(dir)
	if len(loaded.Columns[ColBacklog]) != 0 {
		t.Errorf("expected 0 cards, got %d", len(loaded.Columns[ColBacklog]))
	}
}

func TestRemoveKanbanCard_NotFound(t *testing.T) {
	dir := t.TempDir()
	if err := saveKanbanState(dir, newKanbanState()); err != nil {
		t.Fatal(err)
	}
	app := newTestApp()
	err := app.RemoveKanbanCard(dir, "nonexistent")
	if err == nil {
		t.Error("expected error for non-existent card")
	}
}

// ---------------------------------------------------------------------------
// generateID — basic uniqueness check
// ---------------------------------------------------------------------------

func TestGenerateID_Unique(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := generateID()
		if seen[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		seen[id] = true
	}
}

// ---------------------------------------------------------------------------
// defaultColumns — ordering
// ---------------------------------------------------------------------------

func TestDefaultColumns(t *testing.T) {
	cols := defaultColumns()
	expected := []string{ColBacklog, ColPlanned, ColInProgress, ColReview, ColDone}
	if len(cols) != len(expected) {
		t.Fatalf("expected %d columns, got %d", len(expected), len(cols))
	}
	for i, col := range expected {
		if cols[i] != col {
			t.Errorf("column[%d] = %q, want %q", i, cols[i], col)
		}
	}
}

// ---------------------------------------------------------------------------
// loadKanbanState — ensures missing columns/arrays are initialized
// ---------------------------------------------------------------------------

func TestLoadKanbanState_FillsMissingColumns(t *testing.T) {
	dir := t.TempDir()
	mtuiDir := filepath.Join(dir, ".mtui")
	os.MkdirAll(mtuiDir, 0o755)

	// Write a state with only one column
	data := `{"columns":{"backlog":[]}}`
	os.WriteFile(filepath.Join(mtuiDir, "kanban.json"), []byte(data), 0o644)

	state, err := loadKanbanState(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, col := range defaultColumns() {
		if state.Columns[col] == nil {
			t.Errorf("column %q should be non-nil after load", col)
		}
	}
	if state.Plans == nil {
		t.Error("plans should be non-nil")
	}
	if state.Schedules == nil {
		t.Error("schedules should be non-nil")
	}
}

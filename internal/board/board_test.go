package board

import (
	"errors"
	"testing"
)

func mustMarshalCard(t *testing.T, card TaskCard) []byte {
	t.Helper()
	data, err := marshalCard(card)
	if err != nil {
		t.Fatalf("marshalCard: %v", err)
	}
	return data
}

func TestCreateAndGetTask(t *testing.T) {
	dir := setupTestRepo(t)
	b := NewBoard(dir)

	card := TaskCard{
		ID:          "task-001",
		Title:       "Add auth endpoint",
		Description: "Implement JWT-based authentication",
		State:       StateBacklog,
		CardType:    CardTypeFeature,
		Complexity:  ComplexityMedium,
	}

	if _, err := b.CreateTask(card); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	got, err := b.GetTask("task-001")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.ID != "task-001" {
		t.Errorf("ID: got %q, want %q", got.ID, "task-001")
	}
	if got.Title != "Add auth endpoint" {
		t.Errorf("Title: got %q, want %q", got.Title, "Add auth endpoint")
	}
	if got.Description != "Implement JWT-based authentication" {
		t.Errorf("Description: got %q, want %q", got.Description, "Implement JWT-based authentication")
	}
	if got.State != StateBacklog {
		t.Errorf("State: got %q, want %q", got.State, StateBacklog)
	}
	if got.CardType != CardTypeFeature {
		t.Errorf("CardType: got %q, want %q", got.CardType, CardTypeFeature)
	}
	if got.Complexity != ComplexityMedium {
		t.Errorf("Complexity: got %q, want %q", got.Complexity, ComplexityMedium)
	}
}

func TestCreateTaskAutoID(t *testing.T) {
	dir := setupTestRepo(t)
	b := NewBoard(dir)

	card := TaskCard{
		Title: "Auto ID task",
		State: StateBacklog,
	}

	if _, err := b.CreateTask(card); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	tasks, err := b.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].ID == "" {
		t.Error("expected auto-generated ID, got empty")
	}
	if len(tasks[0].ID) != 8 {
		t.Errorf("expected 8-char hex ID, got %q (len %d)", tasks[0].ID, len(tasks[0].ID))
	}
}

func TestCreateTaskSetsTimestamps(t *testing.T) {
	dir := setupTestRepo(t)
	b := NewBoard(dir)

	card := TaskCard{
		ID:    "ts-test",
		Title: "Timestamp test",
		State: StateBacklog,
	}

	if _, err := b.CreateTask(card); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	got, err := b.GetTask("ts-test")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.CreatedAt == "" {
		t.Error("CreatedAt is empty")
	}
	if got.UpdatedAt == "" {
		t.Error("UpdatedAt is empty")
	}
	if got.CreatedAt != got.UpdatedAt {
		t.Errorf("CreatedAt (%s) != UpdatedAt (%s) on creation", got.CreatedAt, got.UpdatedAt)
	}
}

func TestListTasksMultiple(t *testing.T) {
	dir := setupTestRepo(t)
	b := NewBoard(dir)

	ids := []string{"list-a", "list-b", "list-c"}
	for _, id := range ids {
		card := TaskCard{ID: id, Title: "Task " + id, State: StateBacklog}
		if _, err := b.CreateTask(card); err != nil {
			t.Fatalf("CreateTask(%s): %v", id, err)
		}
	}

	tasks, err := b.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}

	found := make(map[string]bool)
	for _, task := range tasks {
		found[task.ID] = true
	}
	for _, id := range ids {
		if !found[id] {
			t.Errorf("missing task %s", id)
		}
	}
}

func TestListTasksEmpty(t *testing.T) {
	dir := setupTestRepo(t)
	b := NewBoard(dir)

	tasks, err := b.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected empty slice, got %d tasks", len(tasks))
	}
}

func TestUpdateTask(t *testing.T) {
	dir := setupTestRepo(t)
	b := NewBoard(dir)

	card := TaskCard{ID: "upd-1", Title: "Original", State: StateBacklog}
	if _, err := b.CreateTask(card); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	original, _ := b.GetTask("upd-1")

	// Force a known past timestamp so UpdateTask will produce a different one.
	original.UpdatedAt = "2020-01-01T00:00:00Z"
	if err := b.store.WriteRef(taskContentRef("upd-1"), mustMarshalCard(t, original)); err != nil {
		t.Fatalf("rewrite with old timestamp: %v", err)
	}
	original, _ = b.GetTask("upd-1")

	updated := original
	updated.Title = "Updated title"
	updated.State = StatePlanning
	updated.Description = "New description"

	if err := b.UpdateTask(updated); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	got, err := b.GetTask("upd-1")
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Title != "Updated title" {
		t.Errorf("Title: got %q, want %q", got.Title, "Updated title")
	}
	if got.State != StatePlanning {
		t.Errorf("State: got %q, want %q", got.State, StatePlanning)
	}
	if got.Description != "New description" {
		t.Errorf("Description: got %q, want %q", got.Description, "New description")
	}
	if got.CreatedAt != original.CreatedAt {
		t.Errorf("CreatedAt changed: was %s, now %s", original.CreatedAt, got.CreatedAt)
	}
	if got.UpdatedAt == original.UpdatedAt {
		t.Error("UpdatedAt was not changed by UpdateTask")
	}
}

func TestDeleteTask(t *testing.T) {
	dir := setupTestRepo(t)
	b := NewBoard(dir)

	card := TaskCard{ID: "del-1", Title: "To delete", State: StateBacklog}
	if _, err := b.CreateTask(card); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	if err := b.DeleteTask("del-1"); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}

	_, err := b.GetTask("del-1")
	if err == nil {
		t.Fatal("expected error after delete, got nil")
	}
	if !errors.Is(err, ErrRefNotFound) {
		t.Errorf("expected ErrRefNotFound, got: %v", err)
	}
}

func TestDeleteTaskRemovesPlan(t *testing.T) {
	dir := setupTestRepo(t)
	b := NewBoard(dir)

	card := TaskCard{ID: "del-plan", Title: "With plan", State: StateBacklog}
	if _, err := b.CreateTask(card); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	plan := Plan{
		CardID:     "del-plan",
		Complexity: ComplexityMedium,
		Steps: []PlanStep{
			{ID: "s1", Title: "Step 1", Status: "pending"},
		},
	}
	if err := b.SavePlan("del-plan", plan); err != nil {
		t.Fatalf("SavePlan: %v", err)
	}

	if err := b.DeleteTask("del-plan"); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}

	_, err := b.GetPlan("del-plan")
	if err == nil {
		t.Fatal("expected error for plan after delete, got nil")
	}
	if !errors.Is(err, ErrRefNotFound) {
		t.Errorf("expected ErrRefNotFound for plan, got: %v", err)
	}
}

func TestGetTaskNotFound(t *testing.T) {
	dir := setupTestRepo(t)
	b := NewBoard(dir)

	_, err := b.GetTask("nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrRefNotFound) {
		t.Errorf("expected ErrRefNotFound, got: %v", err)
	}
}


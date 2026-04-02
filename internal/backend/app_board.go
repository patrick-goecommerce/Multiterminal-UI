// Package backend — Wails-exposed Board methods that bridge the internal/board
// package to the Svelte frontend. app.go stays thin; all board logic lives here.
package backend

import (
	"fmt"
	"log"

	"github.com/patrick-goecommerce/Multiterminal-UI/internal/board"
)

// BoardTransitionEvent is emitted on "board:task-transition" after a
// successful state machine transition.
type BoardTransitionEvent struct {
	CardID   string          `json:"card_id"`
	OldState board.TaskState `json:"old_state"`
	NewState board.TaskState `json:"new_state"`
	Event    board.Event     `json:"event"`
}

// openBoard creates a Board instance for the given repo directory.
// Returns an error if the directory is not a git repository.
func openBoard(dir string) (*board.Board, error) {
	if err := board.ValidateGitRepo(dir); err != nil {
		return nil, err
	}
	return board.NewBoard(dir), nil
}

// GetBoardTasks returns all tasks from the git-ref board.
func (a *AppService) GetBoardTasks(dir string) ([]board.TaskCard, error) {
	b, err := openBoard(dir)
	if err != nil {
		return nil, err
	}
	tasks, err := b.ListTasks()
	if err != nil {
		log.Printf("[board] ListTasks error for %s: %v", dir, err)
		return nil, fmt.Errorf("list board tasks: %w", err)
	}
	if tasks == nil {
		tasks = []board.TaskCard{}
	}
	return tasks, nil
}

// CreateBoardTask creates a new task on the board.
// Returns the created card (with generated ID and timestamps).
func (a *AppService) CreateBoardTask(dir string, card board.TaskCard) (board.TaskCard, error) {
	b, err := openBoard(dir)
	if err != nil {
		return board.TaskCard{}, err
	}
	if card.State == "" {
		card.State = board.StateBacklog
	}
	created, err := b.CreateTask(card)
	if err != nil {
		log.Printf("[board] CreateTask error for %s: %v", dir, err)
		return board.TaskCard{}, fmt.Errorf("create board task: %w", err)
	}
	return created, nil
}

// GetBoardTask returns a single task by ID.
func (a *AppService) GetBoardTask(dir, id string) (board.TaskCard, error) {
	b, err := openBoard(dir)
	if err != nil {
		return board.TaskCard{}, err
	}
	card, err := b.GetTask(id)
	if err != nil {
		return board.TaskCard{}, fmt.Errorf("get board task %s: %w", id, err)
	}
	return card, nil
}

// UpdateBoardTask updates an existing task.
func (a *AppService) UpdateBoardTask(dir string, card board.TaskCard) error {
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	if err := b.UpdateTask(card); err != nil {
		log.Printf("[board] UpdateTask error for %s/%s: %v", dir, card.ID, err)
		return fmt.Errorf("update board task %s: %w", card.ID, err)
	}
	return nil
}

// DeleteBoardTask removes a task and its associated plan.
func (a *AppService) DeleteBoardTask(dir, id string) error {
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	if err := b.DeleteTask(id); err != nil {
		log.Printf("[board] DeleteTask error for %s/%s: %v", dir, id, err)
		return fmt.Errorf("delete board task %s: %w", id, err)
	}
	return nil
}

// MoveBoardTask triggers a state transition on a task.
// Returns the transition result. Emits "board:task-transition" event.
func (a *AppService) MoveBoardTask(dir, id string, event board.Event) (board.TransitionResult, error) {
	b, err := openBoard(dir)
	if err != nil {
		return board.TransitionResult{}, err
	}
	sm := board.NewStateMachine()

	// 1. Get current card
	card, err := b.GetTask(id)
	if err != nil {
		return board.TransitionResult{}, fmt.Errorf("get task for transition: %w", err)
	}

	// 2. Validate transition
	result, err := sm.Transition(card, event)
	if err != nil {
		return board.TransitionResult{}, fmt.Errorf("transition task %s: %w", id, err)
	}

	// 3. Update card state
	card.State = result.NewState

	// 4. Persist
	if err := b.UpdateTask(card); err != nil {
		return board.TransitionResult{}, fmt.Errorf("update task after transition: %w", err)
	}

	// 5. Emit event
	a.app.Event.Emit("board:task-transition", BoardTransitionEvent{
		CardID:   id,
		OldState: result.OldState,
		NewState: result.NewState,
		Event:    event,
	})

	log.Printf("[board] transition %s: %s -> %s (event=%s)", id, result.OldState, result.NewState, event)
	return result, nil
}

// SaveBoardPlan saves an execution plan for a task.
func (a *AppService) SaveBoardPlan(dir, id string, plan board.Plan) error {
	b, err := openBoard(dir)
	if err != nil {
		return err
	}
	if err := b.SavePlan(id, plan); err != nil {
		log.Printf("[board] SavePlan error for %s/%s: %v", dir, id, err)
		return fmt.Errorf("save board plan for %s: %w", id, err)
	}
	return nil
}

// GetBoardPlan returns the plan for a task.
func (a *AppService) GetBoardPlan(dir, id string) (board.Plan, error) {
	b, err := openBoard(dir)
	if err != nil {
		return board.Plan{}, err
	}
	plan, err := b.GetPlan(id)
	if err != nil {
		return board.Plan{}, fmt.Errorf("get board plan for %s: %w", id, err)
	}
	return plan, nil
}

// SyncBoard pulls and pushes board refs to/from the remote.
func (a *AppService) SyncBoard(dir string) error {
	if err := board.ValidateGitRepo(dir); err != nil {
		return err
	}
	syncer := board.NewSyncer(dir)
	if err := syncer.Pull(); err != nil {
		log.Printf("[board] sync pull error for %s: %v", dir, err)
		return fmt.Errorf("board sync pull: %w", err)
	}
	if err := syncer.Push(); err != nil {
		log.Printf("[board] sync push error for %s: %v", dir, err)
		return fmt.Errorf("board sync push: %w", err)
	}
	return nil
}

package board

import (
	"errors"
	"fmt"
)

// Event triggers a state transition.
type Event string

const (
	EventStartTriage          Event = "start_triage"
	EventComplexityTrivial    Event = "complexity_trivial"
	EventComplexityNonTrivial Event = "complexity_non_trivial"
	EventPlanReady            Event = "plan_ready"
	EventApproved             Event = "approved"
	EventRejected             Event = "rejected"
	EventStepStuck            Event = "step_stuck"
	EventModelEscalated       Event = "model_escalated"
	EventReplanCompleted      Event = "replan_completed"
	EventScopeExpansion       Event = "scope_expansion_required"
	EventMaxEscalations       Event = "max_escalations"
	EventAllStepsDone         Event = "all_steps_done"
	EventQAPassed             Event = "qa_passed"
	EventQAFailed             Event = "qa_failed"
	EventMergeSuccess         Event = "merge_success"
	EventMergeConflict        Event = "merge_conflict"

	// user_resolved variants — each encodes the target state.
	EventUserResolvedExecuting Event = "user_resolved_executing"
	EventUserResolvedDone      Event = "user_resolved_done"
	EventUserResolvedBacklog   Event = "user_resolved_backlog"
)

// Sentinel errors returned by Transition.
var (
	ErrInvalidTransition = errors.New("invalid state transition")
	ErrGuardFailed       = errors.New("transition guard failed")
)

// TransitionResult contains the outcome of a transition attempt.
type TransitionResult struct {
	OldState TaskState `yaml:"old_state" json:"old_state"`
	NewState TaskState `yaml:"new_state" json:"new_state"`
	Event    Event     `yaml:"event" json:"event"`
}

// guardFunc returns nil if the guard passes, or an error describing why not.
type guardFunc func(card TaskCard) error

// transition defines one entry in the state table.
type transition struct {
	target TaskState
	guard  guardFunc
}

// transitionKey indexes the table by (current state, event).
type transitionKey struct {
	state TaskState
	event Event
}

// StateMachine validates and executes state transitions.
// It is stateless — the caller must update the card after a successful call.
type StateMachine struct {
	table map[transitionKey]transition
}

// NewStateMachine creates a StateMachine with the full transition table.
func NewStateMachine() *StateMachine {
	sm := &StateMachine{
		table: make(map[transitionKey]transition),
	}
	sm.register(StateBacklog, EventStartTriage, StateTriage, nil)
	sm.register(StateTriage, EventComplexityTrivial, StateExecuting, nil)
	sm.register(StateTriage, EventComplexityNonTrivial, StatePlanning, nil)
	sm.register(StatePlanning, EventPlanReady, StateReview, nil)
	sm.register(StateReview, EventApproved, StateExecuting, nil)
	sm.register(StateReview, EventRejected, StatePlanning, nil)
	sm.register(StateExecuting, EventStepStuck, StateStuck, nil)
	sm.register(StateExecuting, EventAllStepsDone, StateQA, nil)
	sm.register(StateStuck, EventModelEscalated, StateExecuting, guardEscAttempts)
	sm.register(StateStuck, EventReplanCompleted, StateExecuting, guardEscAttempts)
	sm.register(StateStuck, EventScopeExpansion, StateHumanReview, nil)
	sm.register(StateStuck, EventMaxEscalations, StateHumanReview, nil)
	sm.register(StateQA, EventQAPassed, StateMerging, nil)
	sm.register(StateQA, EventQAFailed, StateExecuting, guardQAAttempts)
	sm.register(StateQA, EventStepStuck, StateStuck, nil)
	sm.register(StateMerging, EventMergeSuccess, StateDone, nil)
	sm.register(StateMerging, EventMergeConflict, StateHumanReview, nil)
	sm.register(StateHumanReview, EventUserResolvedExecuting, StateExecuting, nil)
	sm.register(StateHumanReview, EventUserResolvedDone, StateDone, nil)
	sm.register(StateHumanReview, EventUserResolvedBacklog, StateBacklog, nil)
	return sm
}

// register adds one row to the transition table.
func (sm *StateMachine) register(from TaskState, ev Event, to TaskState, g guardFunc) {
	sm.table[transitionKey{state: from, event: ev}] = transition{target: to, guard: g}
}

// Transition attempts to move a card from its current state via the given event.
// The card is NOT modified — the caller must update the card's State field.
func (sm *StateMachine) Transition(card TaskCard, event Event) (TransitionResult, error) {
	key := transitionKey{state: card.State, event: event}
	tr, ok := sm.table[key]
	if !ok {
		return TransitionResult{}, fmt.Errorf(
			"%w: cannot move from %q via %q", ErrInvalidTransition, card.State, event,
		)
	}
	if tr.guard != nil {
		if err := tr.guard(card); err != nil {
			return TransitionResult{}, err
		}
	}
	return TransitionResult{
		OldState: card.State,
		NewState: tr.target,
		Event:    event,
	}, nil
}

// --- Guards ---

const maxEscalations = 2
const maxQAAttempts = 3

func guardEscAttempts(card TaskCard) error {
	if card.EscAttempts >= maxEscalations {
		return fmt.Errorf(
			"%w: escalation attempts exhausted (%d/%d)",
			ErrGuardFailed, card.EscAttempts, maxEscalations,
		)
	}
	return nil
}

func guardQAAttempts(card TaskCard) error {
	if card.QAAttempts >= maxQAAttempts {
		return fmt.Errorf(
			"%w: QA attempts exhausted (%d/%d)",
			ErrGuardFailed, card.QAAttempts, maxQAAttempts,
		)
	}
	return nil
}

package board

import (
	"errors"
	"testing"
)

func newCard(state TaskState) TaskCard {
	return TaskCard{ID: "test-1", State: state}
}

func TestTransition_BacklogToTriage(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateBacklog)
	res, err := sm.Transition(card, EventStartTriage)
	assertOk(t, err)
	assertStates(t, res, StateBacklog, StateTriage)
}

func TestTransition_TriageToExecuting_Trivial(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateTriage)
	res, err := sm.Transition(card, EventComplexityTrivial)
	assertOk(t, err)
	assertStates(t, res, StateTriage, StateExecuting)
}

func TestTransition_TriageToPlanning_NonTrivial(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateTriage)
	res, err := sm.Transition(card, EventComplexityNonTrivial)
	assertOk(t, err)
	assertStates(t, res, StateTriage, StatePlanning)
}

func TestTransition_PlanningToReview(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StatePlanning)
	res, err := sm.Transition(card, EventPlanReady)
	assertOk(t, err)
	assertStates(t, res, StatePlanning, StateReview)
}

func TestTransition_ReviewToExecuting_Approved(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateReview)
	res, err := sm.Transition(card, EventApproved)
	assertOk(t, err)
	assertStates(t, res, StateReview, StateExecuting)
}

func TestTransition_ReviewToPlanning_Rejected(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateReview)
	res, err := sm.Transition(card, EventRejected)
	assertOk(t, err)
	assertStates(t, res, StateReview, StatePlanning)
}

func TestTransition_ExecutingToStuck(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateExecuting)
	res, err := sm.Transition(card, EventStepStuck)
	assertOk(t, err)
	assertStates(t, res, StateExecuting, StateStuck)
}

func TestTransition_ExecutingToQA(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateExecuting)
	res, err := sm.Transition(card, EventAllStepsDone)
	assertOk(t, err)
	assertStates(t, res, StateExecuting, StateQA)
}

func TestTransition_StuckToExecuting_ModelEscalated(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateStuck)
	card.EscAttempts = 0
	res, err := sm.Transition(card, EventModelEscalated)
	assertOk(t, err)
	assertStates(t, res, StateStuck, StateExecuting)
}

func TestTransition_StuckToExecuting_ReplanCompleted(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateStuck)
	card.EscAttempts = 1
	res, err := sm.Transition(card, EventReplanCompleted)
	assertOk(t, err)
	assertStates(t, res, StateStuck, StateExecuting)
}

func TestTransition_StuckToExecuting_GuardBlocks(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateStuck)
	card.EscAttempts = 2
	_, err := sm.Transition(card, EventModelEscalated)
	if err == nil {
		t.Fatal("expected guard error, got nil")
	}
	if !errors.Is(err, ErrGuardFailed) {
		t.Fatalf("expected ErrGuardFailed, got: %v", err)
	}
}

func TestTransition_StuckToHumanReview_ScopeExpansion(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateStuck)
	res, err := sm.Transition(card, EventScopeExpansion)
	assertOk(t, err)
	assertStates(t, res, StateStuck, StateHumanReview)
}

func TestTransition_StuckToHumanReview_MaxEscalations(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateStuck)
	res, err := sm.Transition(card, EventMaxEscalations)
	assertOk(t, err)
	assertStates(t, res, StateStuck, StateHumanReview)
}

func TestTransition_QAToMerging(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateQA)
	res, err := sm.Transition(card, EventQAPassed)
	assertOk(t, err)
	assertStates(t, res, StateQA, StateMerging)
}

func TestTransition_QAToExecuting_QAFailed(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateQA)
	card.QAAttempts = 0
	res, err := sm.Transition(card, EventQAFailed)
	assertOk(t, err)
	assertStates(t, res, StateQA, StateExecuting)
}

func TestTransition_QAToExecuting_GuardBlocks(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateQA)
	card.QAAttempts = 3
	_, err := sm.Transition(card, EventQAFailed)
	if err == nil {
		t.Fatal("expected guard error, got nil")
	}
	if !errors.Is(err, ErrGuardFailed) {
		t.Fatalf("expected ErrGuardFailed, got: %v", err)
	}
}

func TestTransition_MergingToDone(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateMerging)
	res, err := sm.Transition(card, EventMergeSuccess)
	assertOk(t, err)
	assertStates(t, res, StateMerging, StateDone)
}

func TestTransition_MergingToHumanReview(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateMerging)
	res, err := sm.Transition(card, EventMergeConflict)
	assertOk(t, err)
	assertStates(t, res, StateMerging, StateHumanReview)
}

func TestTransition_HumanReviewToExecuting(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateHumanReview)
	res, err := sm.Transition(card, EventUserResolvedExecuting)
	assertOk(t, err)
	assertStates(t, res, StateHumanReview, StateExecuting)
}

func TestTransition_HumanReviewToDone(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateHumanReview)
	res, err := sm.Transition(card, EventUserResolvedDone)
	assertOk(t, err)
	assertStates(t, res, StateHumanReview, StateDone)
}

func TestTransition_HumanReviewToBacklog(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateHumanReview)
	res, err := sm.Transition(card, EventUserResolvedBacklog)
	assertOk(t, err)
	assertStates(t, res, StateHumanReview, StateBacklog)
}

func TestTransition_InvalidTransition(t *testing.T) {
	sm := NewStateMachine()
	card := newCard(StateBacklog)
	_, err := sm.Transition(card, EventQAPassed)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition, got: %v", err)
	}
}

// --- helpers ---

func assertOk(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertStates(t *testing.T, res TransitionResult, wantOld, wantNew TaskState) {
	t.Helper()
	if res.OldState != wantOld {
		t.Errorf("OldState = %q, want %q", res.OldState, wantOld)
	}
	if res.NewState != wantNew {
		t.Errorf("NewState = %q, want %q", res.NewState, wantNew)
	}
}

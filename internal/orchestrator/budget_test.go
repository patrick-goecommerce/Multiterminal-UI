package orchestrator

import (
	"errors"
	"sync"
	"testing"
)

func TestAllocateComplexityBudgets(t *testing.T) {
	bt := NewBudgetTracker(nil)

	tests := []struct {
		complexity string
		want       float64
	}{
		{"trivial", 0.50},
		{"medium", 2.00},
		{"complex", 10.00},
	}

	for _, tc := range tests {
		cardID := "card-" + tc.complexity
		bt.Allocate(cardID, tc.complexity)

		got, err := bt.Remaining(cardID)
		if err != nil {
			t.Fatalf("Remaining(%s): unexpected error: %v", cardID, err)
		}
		if got != tc.want {
			t.Errorf("Allocate(%s, %s): got budget $%.2f, want $%.2f",
				cardID, tc.complexity, got, tc.want)
		}
	}
}

func TestAllocateUnknownComplexityFallsBackToMedium(t *testing.T) {
	bt := NewBudgetTracker(nil)

	bt.Allocate("card-1", "legendary")

	got, err := bt.Remaining("card-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 2.00 {
		t.Errorf("unknown complexity: got $%.2f, want $2.00", got)
	}
}

func TestSpendReducesBudget(t *testing.T) {
	bt := NewBudgetTracker(nil)
	bt.Allocate("card-1", "complex")

	if err := bt.Spend("card-1", 3.50); err != nil {
		t.Fatalf("Spend: unexpected error: %v", err)
	}

	got, _ := bt.Remaining("card-1")
	if got != 6.50 {
		t.Errorf("after spending $3.50 of $10.00: got $%.2f remaining, want $6.50", got)
	}
}

func TestSpendReturnsBudgetExhausted(t *testing.T) {
	bt := NewBudgetTracker(nil)
	bt.Allocate("card-1", "trivial") // $0.50

	err := bt.Spend("card-1", 1.00)
	if !errors.Is(err, ErrBudgetExhausted) {
		t.Errorf("expected ErrBudgetExhausted, got: %v", err)
	}

	// Budget should be unchanged after rejected spend.
	got, _ := bt.Remaining("card-1")
	if got != 0.50 {
		t.Errorf("budget should be unchanged after rejected spend: got $%.2f, want $0.50", got)
	}
}

func TestSpendOnUnallocatedCard(t *testing.T) {
	bt := NewBudgetTracker(nil)

	err := bt.Spend("nonexistent", 1.00)
	if !errors.Is(err, ErrCardNotAllocated) {
		t.Errorf("expected ErrCardNotAllocated, got: %v", err)
	}
}

func TestRemainingAfterSpending(t *testing.T) {
	bt := NewBudgetTracker(nil)
	bt.Allocate("card-1", "medium") // $2.00

	bt.Spend("card-1", 0.75)
	bt.Spend("card-1", 0.25)

	got, err := bt.Remaining("card-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 1.00 {
		t.Errorf("after spending $1.00 of $2.00: got $%.2f remaining, want $1.00", got)
	}
}

func TestCanSpendSufficient(t *testing.T) {
	bt := NewBudgetTracker(nil)
	bt.Allocate("card-1", "complex") // $10.00

	if !bt.CanSpend("card-1", 5.00) {
		t.Error("CanSpend should return true when budget is sufficient")
	}
}

func TestCanSpendInsufficient(t *testing.T) {
	bt := NewBudgetTracker(nil)
	bt.Allocate("card-1", "trivial") // $0.50

	if bt.CanSpend("card-1", 1.00) {
		t.Error("CanSpend should return false when budget is insufficient")
	}
}

func TestCanSpendUnallocated(t *testing.T) {
	bt := NewBudgetTracker(nil)

	if bt.CanSpend("nonexistent", 0.01) {
		t.Error("CanSpend should return false for unallocated card")
	}
}

func TestTotalSpent(t *testing.T) {
	bt := NewBudgetTracker(nil)
	bt.Allocate("card-1", "complex") // $10.00

	bt.Spend("card-1", 2.50)
	bt.Spend("card-1", 1.25)

	got, err := bt.TotalSpent("card-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 3.75 {
		t.Errorf("TotalSpent: got $%.2f, want $3.75", got)
	}
}

func TestMultipleCardsIndependent(t *testing.T) {
	bt := NewBudgetTracker(nil)
	bt.Allocate("card-a", "trivial") // $0.50
	bt.Allocate("card-b", "complex") // $10.00

	bt.Spend("card-a", 0.25)
	bt.Spend("card-b", 5.00)

	remA, _ := bt.Remaining("card-a")
	remB, _ := bt.Remaining("card-b")

	if remA != 0.25 {
		t.Errorf("card-a: got $%.2f remaining, want $0.25", remA)
	}
	if remB != 5.00 {
		t.Errorf("card-b: got $%.2f remaining, want $5.00", remB)
	}
}

func TestConcurrentSpend(t *testing.T) {
	bt := NewBudgetTracker(nil)
	bt.Allocate("card-1", "complex") // $10.00

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bt.Spend("card-1", 0.50)
		}()
	}
	wg.Wait()

	got, _ := bt.Remaining("card-1")
	if got != 5.00 {
		t.Errorf("after 10 concurrent $0.50 spends on $10.00: got $%.2f remaining, want $5.00", got)
	}
}

package engine

import (
	"testing"
)

func TestOperationCoordinatorAcquireAndRelease(t *testing.T) {
	coord := &OperationCoordinator{currentOp: OpIdle}

	rel1, err := coord.Acquire(OpDoctor, "user_1", false)
	if err != nil {
		t.Fatalf("Failed to acquire doctor op: %v", err)
	}

	if !coord.IsBusy() {
		t.Error("Expected coordinator to be busy")
	}

	op, owner := coord.CurrentOperation()
	if op != OpDoctor || owner != "user_1" {
		t.Errorf("CurrentOperation = (%s, %s); want (doctor, user_1)", op, owner)
	}

	// Conflict without cancel
	_, err = coord.Acquire(OpAutoTune, "user_2", false)
	if err == nil {
		t.Error("Expected conflict error when acquiring AutoTune while Doctor is running")
	}

	// Release
	rel1()
	if coord.IsBusy() {
		t.Error("Expected coordinator to be idle after release")
	}

	// Now AutoTune can be acquired
	rel2, err := coord.Acquire(OpAutoTune, "user_2", false)
	if err != nil {
		t.Fatalf("Failed to acquire AutoTune after release: %v", err)
	}
	rel2()
}

func TestOperationCoordinatorCancelActive(t *testing.T) {
	coord := &OperationCoordinator{currentOp: OpIdle}

	_, err := coord.Acquire(OpDoctor, "user_1", false)
	if err != nil {
		t.Fatal(err)
	}

	cancelled := false
	coord.SetCancelFunc(func() {
		cancelled = true
	})

	// Preempt with cancelCurrent = true
	rel2, err := coord.Acquire(OpStrategyLab, "user_2", true)
	if err != nil {
		t.Fatalf("Failed to preempt doctor with strategy lab: %v", err)
	}
	if !cancelled {
		t.Error("Expected cancelActive function to be called on preemption")
	}

	op, owner := coord.CurrentOperation()
	if op != OpStrategyLab || owner != "user_2" {
		t.Errorf("CurrentOperation = (%s, %s); want (strategy_lab, user_2)", op, owner)
	}

	rel2()
}

func TestOperationCoordinatorShutdownBlocksAll(t *testing.T) {
	coord := &OperationCoordinator{currentOp: OpShuttingDown}

	_, err := coord.Acquire(OpDoctor, "user_1", true)
	if err == nil {
		t.Error("Expected acquire to fail when shutting down")
	}
}

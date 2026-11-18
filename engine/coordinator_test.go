package engine

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

func TestOperationCoordinatorCancelActiveWaitsForTeardownRelease(t *testing.T) {
	coord := &OperationCoordinator{currentOp: OpIdle, cancelWaitTimeout: 2 * time.Second}

	rel1, err := coord.Acquire(OpDoctor, "user_1", false)
	if err != nil {
		t.Fatal(err)
	}

	var cancelled atomic.Bool
	var op1CleanedUp atomic.Bool
	coord.SetCancelFunc(func() {
		cancelled.Store(true)
		go func() {
			// Simulate async WinDivert/NFQUEUE engine teardown before Release().
			time.Sleep(80 * time.Millisecond)
			op1CleanedUp.Store(true)
			rel1()
		}()
	})

	start := time.Now()
	rel2, err := coord.Acquire(OpStrategyLab, "user_2", true)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("Failed to preempt doctor with strategy lab: %v", err)
	}
	defer rel2()

	if !cancelled.Load() {
		t.Error("Expected cancelActive function to be called on preemption")
	}
	if !op1CleanedUp.Load() {
		t.Error("Acquire returned BEFORE cancelled operation finished teardown and called Release()")
	}
	if elapsed < 60*time.Millisecond {
		t.Errorf("Acquire returned too early (%s), did not wait for Release()", elapsed)
	}

	op, owner := coord.CurrentOperation()
	if op != OpStrategyLab || owner != "user_2" {
		t.Errorf("CurrentOperation = (%s, %s); want (strategy_lab, user_2)", op, owner)
	}
}

func TestOperationCoordinatorCancelActiveTimeoutPreventsEngineCollision(t *testing.T) {
	coord := &OperationCoordinator{currentOp: OpIdle, cancelWaitTimeout: 75 * time.Millisecond}

	rel1, err := coord.Acquire(OpAutoTune, "hung_owner", false)
	if err != nil {
		t.Fatal(err)
	}
	defer rel1()

	var cancelled atomic.Bool
	coord.SetCancelFunc(func() {
		cancelled.Store(true)
		// Deliberately do NOT call rel1() to simulate a wedged teardown.
	})

	_, err = coord.Acquire(OpStrategyLab, "user_2", true)
	if err == nil {
		t.Fatal("Expected timeout error when cancelled operation fails to release within cancelWaitTimeout")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("Expected timeout error message, got: %v", err)
	}
	if !cancelled.Load() {
		t.Error("Expected cancelActive to be invoked before timing out")
	}

	// Original wedged op still owns the lock so a second engine never collides.
	op, owner := coord.CurrentOperation()
	if op != OpAutoTune || owner != "hung_owner" {
		t.Errorf("CurrentOperation = (%s, %s); want (autotune, hung_owner)", op, owner)
	}
}

func TestOperationCoordinatorConcurrentPreemptionNeverOverlaps(t *testing.T) {
	coord := &OperationCoordinator{currentOp: OpIdle, cancelWaitTimeout: 2 * time.Second}

	var activeEngines atomic.Int32
	var maxConcurrent atomic.Int32

	const workers = 12
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		workerID := i
		go func() {
			defer wg.Done()
			owner := fmt.Sprintf("worker-%d", workerID)
			rel, err := coord.Acquire(OpStrategyLab, owner, true)
			if err != nil {
				// Contention without registered cancel fn on another starter is acceptable,
				// but two workers must never hold ownership concurrently.
				return
			}
			stopCh := make(chan struct{})
			var stopOnce sync.Once
			coord.SetCancelFunc(func() {
				stopOnce.Do(func() { close(stopCh) })
			})

			cur := activeEngines.Add(1)
			for {
				prev := maxConcurrent.Load()
				if cur <= prev || maxConcurrent.CompareAndSwap(prev, cur) {
					break
				}
			}

			select {
			case <-stopCh:
				time.Sleep(10 * time.Millisecond) // teardown delay
			case <-time.After(25 * time.Millisecond):
			}

			activeEngines.Add(-1)
			rel()
		}()
		time.Sleep(3 * time.Millisecond)
	}

	wg.Wait()

	if got := maxConcurrent.Load(); got > 1 {
		t.Fatalf("Detected %d concurrent engine owners; mutual exclusion violated", got)
	}
	if coord.IsBusy() {
		t.Fatalf("Coordinator remained busy after all workers exited")
	}
}

func TestOperationCoordinatorShutdownBlocksAll(t *testing.T) {
	coord := &OperationCoordinator{currentOp: OpShuttingDown}

	_, err := coord.Acquire(OpDoctor, "user_1", true)
	if err == nil {
		t.Error("Expected acquire to fail when shutting down")
	}
}

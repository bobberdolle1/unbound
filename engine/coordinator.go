package engine

import (
	"context"
	"fmt"
	"sync"
)

// EngineOperation represents an exclusive long-running operation in UNBOUND.
type EngineOperation string

const (
	OpIdle             EngineOperation = "idle"
	OpRunningProfile   EngineOperation = "running_profile"
	OpDoctor           EngineOperation = "doctor"
	OpAutoTune         EngineOperation = "autotune"
	OpStrategyLab      EngineOperation = "strategy_lab"
	OpBypassComparison EngineOperation = "bypass_comparison"
	OpEngineUpdate     EngineOperation = "engine_update"
	OpShuttingDown     EngineOperation = "shutting_down"
)

// OperationCoordinator coordinates exclusive tasks and prevents concurrency conflicts.
type OperationCoordinator struct {
	mu           sync.Mutex
	currentOp    EngineOperation
	cancelActive context.CancelFunc
	activeOwner  string
}

var (
	globalCoordinator     *OperationCoordinator
	globalCoordinatorOnce sync.Once
)

// GetCoordinator returns the singleton OperationCoordinator.
func GetCoordinator() *OperationCoordinator {
	globalCoordinatorOnce.Do(func() {
		globalCoordinator = &OperationCoordinator{
			currentOp: OpIdle,
		}
	})
	return globalCoordinator
}

// ReleaseFunc releases the acquired operation lock.
type ReleaseFunc func()

// Acquire tries to acquire the requested operation.
// If an operation is already running and cancelCurrent is true, it cancels the active operation.
// Otherwise, it returns an error describing the conflict.
func (c *OperationCoordinator) Acquire(op EngineOperation, owner string, cancelCurrent bool) (ReleaseFunc, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.currentOp == OpShuttingDown {
		return nil, fmt.Errorf("application is shutting down, cannot start %s", op)
	}

	if c.currentOp != OpIdle && c.currentOp != OpRunningProfile {
		if cancelCurrent && c.cancelActive != nil {
			GetLogger().Warnf("Coordinator", "Cancelling active operation %s (owner: %s) to start %s (owner: %s)",
				c.currentOp, c.activeOwner, op, owner)
			c.cancelActive()
			c.cancelActive = nil
		} else {
			return nil, fmt.Errorf("operation %s is currently active (owner: %s), cannot start %s",
				c.currentOp, c.activeOwner, op)
		}
	}

	prevOp := c.currentOp
	c.currentOp = op
	c.activeOwner = owner
	GetLogger().Infof("Coordinator", "Acquired operation lock for %s (owner: %s)", op, owner)

	var released bool
	release := func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if released {
			return
		}
		released = true
		c.cancelActive = nil
		if c.currentOp == op {
			c.currentOp = prevOp
			if c.currentOp == op {
				c.currentOp = OpIdle
			}
			c.activeOwner = ""
			GetLogger().Infof("Coordinator", "Released operation lock for %s, returned to %s", op, c.currentOp)
		}
	}

	return release, nil
}

// SetCancelFunc associates an active cancel function with the current operation.
func (c *OperationCoordinator) SetCancelFunc(cancel context.CancelFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cancelActive = cancel
}

// CurrentOperation returns the currently active operation.
func (c *OperationCoordinator) CurrentOperation() (EngineOperation, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.currentOp, c.activeOwner
}

// IsBusy returns true if an exclusive operation (Doctor, AutoTune, Lab, Update) is active.
func (c *OperationCoordinator) IsBusy() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.currentOp != OpIdle && c.currentOp != OpRunningProfile
}

package engine

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DefaultCancelWaitTimeout bounds how long Acquire waits for a cancelled
// operation to finish tearing down owned packet-capture state and call Release.
const DefaultCancelWaitTimeout = 5 * time.Second

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
	mu                sync.Mutex
	currentOp         EngineOperation
	cancelActive      context.CancelFunc
	activeOwner       string
	activeDone        chan struct{}
	activeToken       uint64
	activeCancelled   bool
	nextToken         uint64
	cancelWaitTimeout time.Duration
}

var (
	globalCoordinator     *OperationCoordinator
	globalCoordinatorOnce sync.Once
)

// GetCoordinator returns the singleton OperationCoordinator.
func GetCoordinator() *OperationCoordinator {
	globalCoordinatorOnce.Do(func() {
		globalCoordinator = &OperationCoordinator{
			currentOp:         OpIdle,
			cancelWaitTimeout: DefaultCancelWaitTimeout,
		}
	})
	return globalCoordinator
}

// ReleaseFunc releases the acquired operation lock.
type ReleaseFunc func()

// Acquire tries to acquire the requested operation.
// If an operation is already running and cancelCurrent is true, it cancels the
// active operation and waits (up to DefaultCancelWaitTimeout) for the cancelled
// operation to call its ReleaseFunc before granting ownership.
// Otherwise, it returns an error describing the conflict.
func (c *OperationCoordinator) Acquire(op EngineOperation, owner string, cancelCurrent bool) (ReleaseFunc, error) {
	c.mu.Lock()
	waitTimeout := c.cancelWaitTimeout
	if waitTimeout <= 0 {
		waitTimeout = DefaultCancelWaitTimeout
	}
	var deadline time.Time

	for {
		if c.currentOp == OpShuttingDown {
			c.mu.Unlock()
			return nil, fmt.Errorf("application is shutting down, cannot start %s", op)
		}

		if c.currentOp == OpIdle || c.currentOp == OpRunningProfile {
			break
		}

		if !cancelCurrent || (c.cancelActive == nil && !c.activeCancelled) {
			curOp, curOwner := c.currentOp, c.activeOwner
			c.mu.Unlock()
			return nil, fmt.Errorf("operation %s is currently active (owner: %s), cannot start %s",
				curOp, curOwner, op)
		}

		if deadline.IsZero() {
			deadline = time.Now().Add(waitTimeout)
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			curOp, curOwner := c.currentOp, c.activeOwner
			c.mu.Unlock()
			return nil, fmt.Errorf("timed out waiting %s for cancelled operation %s (owner: %s) to release before starting %s",
				waitTimeout, curOp, curOwner, op)
		}

		cancelFn := c.cancelActive
		doneCh := c.activeDone
		cancelledOp := c.currentOp
		cancelledOwner := c.activeOwner
		cancelledToken := c.activeToken
		c.cancelActive = nil
		c.activeCancelled = true
		c.mu.Unlock()

		if cancelFn != nil {
			GetLogger().Warnf("Coordinator", "Cancelling active operation %s (owner: %s) to start %s (owner: %s)",
				cancelledOp, cancelledOwner, op, owner)
			cancelFn()
		}

		if doneCh != nil {
			timer := time.NewTimer(remaining)
			select {
			case <-doneCh:
				timer.Stop()
			case <-timer.C:
				c.mu.Lock()
				stillSame := c.activeToken == cancelledToken && c.currentOp != OpIdle && c.currentOp != OpRunningProfile
				c.mu.Unlock()
				if stillSame {
					return nil, fmt.Errorf("timed out waiting %s for cancelled operation %s (owner: %s) to release before starting %s",
						waitTimeout, cancelledOp, cancelledOwner, op)
				}
			}
		}

		c.mu.Lock()
	}

	prevOp := c.currentOp
	if prevOp != OpIdle && prevOp != OpRunningProfile {
		prevOp = OpIdle
	}
	c.nextToken++
	token := c.nextToken
	done := make(chan struct{})
	c.currentOp = op
	c.activeOwner = owner
	c.activeDone = done
	c.activeToken = token
	c.activeCancelled = false
	c.cancelActive = nil
	GetLogger().Infof("Coordinator", "Acquired operation lock for %s (owner: %s)", op, owner)
	c.mu.Unlock()

	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() {
			c.mu.Lock()
			if c.activeToken == token {
				c.cancelActive = nil
				c.activeDone = nil
				c.activeCancelled = false
				c.currentOp = prevOp
				if c.currentOp == op {
					c.currentOp = OpIdle
				}
				c.activeOwner = ""
				GetLogger().Infof("Coordinator", "Released operation lock for %s, returned to %s", op, c.currentOp)
			}
			close(done)
			c.mu.Unlock()
		})
	}

	return release, nil
}

// SetCancelFunc associates an active cancel function with the current operation.
func (c *OperationCoordinator) SetCancelFunc(cancel context.CancelFunc) {
	c.mu.Lock()
	alreadyCancelled := c.activeCancelled
	if !alreadyCancelled {
		c.cancelActive = cancel
	}
	c.mu.Unlock()
	if alreadyCancelled && cancel != nil {
		cancel()
	}
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

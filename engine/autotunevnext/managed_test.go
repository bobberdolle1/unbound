package autotunevnext

import (
	"context"
	"errors"
	"slices"
	"testing"

	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/strategyir"
)

func TestApplyVerifiedRevalidatesThenOwnsRevert(t *testing.T) {
	strategy := tlsStrategy("managed")
	fingerprint, err := strategyir.Fingerprint(strategy)
	if err != nil {
		t.Fatal(err)
	}
	executor := &fakeExecutor{}
	observer := &fakeObserver{results: []observatory.ObservationResult{
		observation("baseline", false, "192.0.2.1", "https://blocked.test/"),
		observation("before", false, "192.0.2.1", "https://blocked.test/"),
		observation("control-direct", true, "192.0.2.2", "https://control.test/"),
		observation("active", true, "192.0.2.1", "https://blocked.test/"),
		observation("control-active", true, "192.0.2.2", "https://control.test/"),
	}}
	activation, err := ApplyVerified(context.Background(), ManagedRequest{Target: Target{URL: "https://blocked.test/"}, Controls: []Target{{URL: "https://control.test/"}}, Strategy: strategy, Fingerprint: fingerprint, Backend: backendcap.Zapret2Windows}, observer, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
	if err != nil {
		t.Fatal(err)
	}
	if got := activation.Candidate(); got.Fingerprint != fingerprint || got.TargetEdge.String() != "192.0.2.1" {
		t.Fatalf("candidate=%#v", got)
	}
	if err := activation.Revert(context.Background()); err != nil {
		t.Fatal(err)
	}
	if want := []string{"snapshot", "direct", "activate", "verify-active", "deactivate", "restore", "verify-restored"}; !slices.Equal(executor.calls, want) {
		t.Fatalf("calls=%v want=%v", executor.calls, want)
	}
}

func TestApplyVerifiedFailureRestoresAndDoesNotReturnOwner(t *testing.T) {
	strategy := tlsStrategy("managed-failure")
	fingerprint, err := strategyir.Fingerprint(strategy)
	if err != nil {
		t.Fatal(err)
	}
	executor := &fakeExecutor{verifyActiveErr: errors.New("verify")}
	observer := &fakeObserver{results: []observatory.ObservationResult{
		observation("baseline", false, "192.0.2.1", "https://blocked.test/"),
		observation("before", false, "192.0.2.1", "https://blocked.test/"),
	}}
	activation, err := ApplyVerified(context.Background(), ManagedRequest{Target: Target{URL: "https://blocked.test/"}, Strategy: strategy, Fingerprint: fingerprint, Backend: backendcap.Zapret2Windows}, observer, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
	if err == nil || activation != nil {
		t.Fatalf("activation=%#v err=%v", activation, err)
	}
	if !slices.Contains(executor.calls, "deactivate") || !slices.Contains(executor.calls, "restore") || !slices.Contains(executor.calls, "verify-restored") {
		t.Fatalf("cleanup calls=%v", executor.calls)
	}
}

func TestApplyVerifiedCleanupRestoreFailureRetainsOwnerForRetry(t *testing.T) {
	strategy := tlsStrategy("managed-cleanup-failure")
	fingerprint, err := strategyir.Fingerprint(strategy)
	if err != nil {
		t.Fatal(err)
	}
	executor := &fakeExecutor{verifyActiveErr: errors.New("verify"), restoreErr: errors.New("restore")}
	observer := &fakeObserver{results: []observatory.ObservationResult{
		observation("baseline", false, "192.0.2.1", "https://blocked.test/"),
		observation("before", false, "192.0.2.1", "https://blocked.test/"),
	}}
	activation, err := ApplyVerified(context.Background(), ManagedRequest{Target: Target{URL: "https://blocked.test/"}, Strategy: strategy, Fingerprint: fingerprint, Backend: backendcap.Zapret2Windows}, observer, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
	if activation == nil || !errors.Is(err, ErrManagedStateRestoreFailed) {
		t.Fatalf("activation=%#v err=%v", activation, err)
	}
	executor.restoreErr = nil
	if err := activation.Revert(context.Background()); err != nil {
		t.Fatalf("retry restore: %v", err)
	}
	if err := activation.Revert(context.Background()); err != nil {
		t.Fatalf("cleared ownership retry: %v", err)
	}
}

func TestApplyVerifiedPreActivationRestoreFailureRetainsOwnerForRetry(t *testing.T) {
	strategy := tlsStrategy("managed-pre-activation-failure")
	fingerprint, err := strategyir.Fingerprint(strategy)
	if err != nil {
		t.Fatal(err)
	}
	executor := &fakeExecutor{restoreErr: errors.New("restore")}
	activation, err := ApplyVerified(context.Background(), ManagedRequest{Target: Target{URL: "https://blocked.test/"}, Strategy: strategy, Fingerprint: fingerprint, Backend: backendcap.Zapret2Windows}, &fakeObserver{}, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
	if activation == nil || !errors.Is(err, ErrManagedStateRestoreFailed) {
		t.Fatalf("activation=%#v err=%v", activation, err)
	}
	if slices.Contains(executor.calls, "activate") || slices.Contains(executor.calls, "deactivate") {
		t.Fatalf("pre-activation cleanup touched candidate lifecycle: %v", executor.calls)
	}
	executor.restoreErr = nil
	if err := activation.Revert(context.Background()); err != nil {
		t.Fatalf("pre-activation restore retry: %v", err)
	}
}

func TestManagedRevertRetainsOwnershipWhenRestoreFails(t *testing.T) {
	executor := &fakeExecutor{restoreErr: errors.New("restore")}
	activation := &ManagedActivation{executor: executor, snapshot: StateSnapshot{ID: "snapshot"}, restorePending: true}
	if err := activation.Revert(context.Background()); err == nil {
		t.Fatal("restore failure was reported as success")
	}
	if !activation.restorePending {
		t.Fatal("restore failure cleared managed ownership")
	}
}

func TestManagedRevertTreatsVerifiedRestoreAsFactualAfterDeactivateError(t *testing.T) {
	executor := &fakeExecutor{deactivateErr: errors.New("deactivate")}
	activation := &ManagedActivation{
		executor:        executor,
		snapshot:        StateSnapshot{ID: "snapshot"},
		candidateActive: true,
		restorePending:  true,
	}
	if err := activation.Revert(context.Background()); err != nil {
		t.Fatalf("verified restoration reported failure: %v", err)
	}
	if activation.restorePending || activation.candidateActive {
		t.Fatalf("verified restoration retained ownership: %#v", activation)
	}
	if want := []string{"deactivate", "restore", "verify-restored"}; !slices.Equal(executor.calls, want) {
		t.Fatalf("calls=%v want=%v", executor.calls, want)
	}
}

// contextBoundExecutor models a runtime process whose lifetime is owned by the
// context supplied to Activate, as exec.CommandContext does in production.
type contextBoundExecutor struct {
	calls         []string
	validationCtx context.Context
	processCtx    context.Context
	processAlive  bool
	restored      bool
}

type managedLifetimeContextKey struct{}

func (e *contextBoundExecutor) Snapshot(ctx context.Context) (StateSnapshot, error) {
	e.calls = append(e.calls, "snapshot")
	e.validationCtx = ctx
	return StateSnapshot{ID: "snapshot"}, nil
}

func (e *contextBoundExecutor) EstablishDirect(context.Context, StateSnapshot) error {
	e.calls = append(e.calls, "direct")
	return nil
}

func (e *contextBoundExecutor) Activate(ctx context.Context, _ ExecutableCandidate) error {
	e.calls = append(e.calls, "activate")
	e.processCtx = ctx
	e.processAlive = true
	return nil
}

func (e *contextBoundExecutor) VerifyActive(context.Context, ExecutableCandidate) error {
	e.calls = append(e.calls, "verify-active")
	if !e.processAlive || e.processCtx.Err() != nil {
		return errors.New("managed process is not active")
	}
	return nil
}

func (e *contextBoundExecutor) Deactivate(context.Context) error {
	e.calls = append(e.calls, "deactivate")
	e.processAlive = false
	return nil
}

func (e *contextBoundExecutor) Restore(context.Context, StateSnapshot) error {
	e.calls = append(e.calls, "restore")
	e.restored = true
	return nil
}

func (e *contextBoundExecutor) VerifyRestored(context.Context, StateSnapshot) error {
	e.calls = append(e.calls, "verify-restored")
	if !e.restored {
		return errors.New("snapshot was not restored")
	}
	return nil
}

func managedLifetimeRequest(t *testing.T, id string) (ManagedRequest, *fakeObserver) {
	t.Helper()
	strategy := tlsStrategy(id)
	fingerprint, err := strategyir.Fingerprint(strategy)
	if err != nil {
		t.Fatal(err)
	}
	return ManagedRequest{
			Target:      Target{URL: "https://blocked.test/"},
			Strategy:    strategy,
			Fingerprint: fingerprint,
			Backend:     backendcap.Zapret2Windows,
		}, &fakeObserver{results: []observatory.ObservationResult{
			observation("baseline", false, "192.0.2.1", "https://blocked.test/"),
			observation("before", false, "192.0.2.1", "https://blocked.test/"),
			observation("active", true, "192.0.2.1", "https://blocked.test/"),
		}}
}

func TestApplyVerifiedCommittedProcessSurvivesValidationReturn(t *testing.T) {
	request, observer := managedLifetimeRequest(t, "managed-process-lifetime")
	key := managedLifetimeContextKey{}
	caller := context.WithValue(context.Background(), key, "retained")
	executor := &contextBoundExecutor{}
	activation, err := ApplyVerified(caller, request, observer, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
	if err != nil {
		t.Fatal(err)
	}
	if executor.validationCtx.Err() == nil {
		t.Fatal("Apply validation context remained active after return")
	}
	if got := executor.processCtx.Value(key); got != "retained" {
		t.Fatalf("managed process context lost caller value: %v", got)
	}
	if _, hasDeadline := executor.processCtx.Deadline(); hasDeadline {
		t.Fatal("managed process inherited Apply validation deadline")
	}
	if err := activation.VerifyActive(context.Background()); err != nil {
		t.Fatalf("managed process did not survive Apply return: %v", err)
	}
	if err := activation.Revert(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !executor.restored {
		t.Fatal("Revert did not restore the original snapshot")
	}
}

func TestApplyVerifiedCallerCancelAfterCommitDoesNotStopManagedProcess(t *testing.T) {
	request, observer := managedLifetimeRequest(t, "managed-caller-cancel")
	caller, cancelCaller := context.WithCancel(context.Background())
	defer cancelCaller()
	executor := &contextBoundExecutor{}
	activation, err := ApplyVerified(caller, request, observer, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
	if err != nil {
		t.Fatal(err)
	}
	cancelCaller()
	if err := activation.VerifyActive(context.Background()); err != nil {
		t.Fatalf("caller cancellation stopped committed managed process: %v", err)
	}
	if err := activation.Revert(context.Background()); err != nil {
		t.Fatal(err)
	}
	if executor.processAlive {
		t.Fatal("Revert left the managed process active")
	}
	if !executor.restored {
		t.Fatal("Revert did not restore the original snapshot")
	}
	callsAfterRevert := append([]string(nil), executor.calls...)
	if err := activation.Revert(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(executor.calls, callsAfterRevert) {
		t.Fatalf("second Revert mutated lifecycle: %v", executor.calls)
	}
}

func TestApplyVerifiedFailureAfterActivateStopsManagedProcess(t *testing.T) {
	request, observer := managedLifetimeRequest(t, "managed-active-target-failure")
	observer.results[2] = observation("active", false, "192.0.2.1", "https://blocked.test/")
	executor := &contextBoundExecutor{}
	activation, err := ApplyVerified(context.Background(), request, observer, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
	if err == nil || activation != nil {
		t.Fatalf("activation=%#v err=%v", activation, err)
	}
	if executor.processAlive {
		t.Fatal("failed Apply leaked the managed process")
	}
	if !executor.restored {
		t.Fatal("failed Apply did not restore the original snapshot")
	}
	if want := []string{"snapshot", "direct", "activate", "verify-active", "deactivate", "restore", "verify-restored"}; !slices.Equal(executor.calls, want) {
		t.Fatalf("calls=%v want=%v", executor.calls, want)
	}
}

func TestApplyVerifiedCancellationAfterActivateStopsManagedProcess(t *testing.T) {
	request, observer := managedLifetimeRequest(t, "managed-apply-cancel")
	caller, cancelCaller := context.WithCancel(context.Background())
	defer cancelCaller()
	observer.cancel = cancelCaller
	observer.errAt = 2
	executor := &contextBoundExecutor{}
	activation, err := ApplyVerified(caller, request, observer, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
	if err == nil || activation != nil {
		t.Fatalf("activation=%#v err=%v", activation, err)
	}
	if executor.processAlive {
		t.Fatal("canceled Apply leaked the managed process")
	}
	if !executor.restored {
		t.Fatal("canceled Apply did not restore the original snapshot")
	}
	if want := []string{"snapshot", "direct", "activate", "verify-active", "deactivate", "restore", "verify-restored"}; !slices.Equal(executor.calls, want) {
		t.Fatalf("calls=%v want=%v", executor.calls, want)
	}
}

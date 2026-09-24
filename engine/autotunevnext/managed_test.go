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

func TestManagedRevertRetainsOwnershipWhenRestoreFails(t *testing.T) {
	executor := &fakeExecutor{restoreErr: errors.New("restore")}
	activation := &ManagedActivation{executor: executor, snapshot: StateSnapshot{ID: "snapshot"}, active: true}
	if err := activation.Revert(context.Background()); err == nil {
		t.Fatal("restore failure was reported as success")
	}
	if !activation.active {
		t.Fatal("restore failure cleared managed ownership")
	}
}

package autotunevnext

import (
	"context"
	"slices"
	"sync"
	"testing"

	"unbound/engine/observatory"
)

func managedServiceScopeRequest(t *testing.T, id string) (ManagedRequest, ServiceScopeSnapshot, *fakeObserver) {
	t.Helper()
	request, _ := managedLifetimeRequest(t, id)
	scope := ServiceScopeSnapshot{
		Target: request.Target,
		Edges:  []ServiceScopeEdge{{IP: []byte{192, 0, 2, 1}, Family: observatory.AddressFamilyIPv4}},
	}
	request.ValidatedScope = scope
	return request, scope, &fakeObserver{results: []observatory.ObservationResult{
		observation("before", false, "192.0.2.1", request.Target.URL),
		observation("active", true, "192.0.2.1", request.Target.URL),
	}}
}

func TestApplyVerifiedServiceScopeCommittedProcessSurvivesValidationReturn(t *testing.T) {
	request, scope, observer := managedServiceScopeRequest(t, "managed-service-scope-lifetime")
	key := managedLifetimeContextKey{}
	caller := context.WithValue(context.Background(), key, "retained")
	executor := &contextBoundExecutor{}
	activation, err := ApplyVerifiedServiceScope(caller, request, scope, observer, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
	if err != nil {
		t.Fatal(err)
	}
	if executor.validationCtx.Err() == nil {
		t.Fatal("service-scope validation context remained active after return")
	}
	if got := executor.processCtx.Value(key); got != "retained" {
		t.Fatalf("managed process context lost caller value: %v", got)
	}
	if _, hasDeadline := executor.processCtx.Deadline(); hasDeadline {
		t.Fatal("managed process inherited service-scope validation deadline")
	}
	if err := activation.VerifyActive(context.Background()); err != nil {
		t.Fatalf("service-scope managed process did not survive Apply return: %v", err)
	}
	if err := activation.Revert(context.Background()); err != nil {
		t.Fatal(err)
	}
	if executor.processAlive || !executor.restored {
		t.Fatalf("Revert processAlive=%t restored=%t", executor.processAlive, executor.restored)
	}
}

func TestApplyVerifiedServiceScopeCallerCancelAfterCommitDoesNotStopProcess(t *testing.T) {
	request, scope, observer := managedServiceScopeRequest(t, "managed-service-scope-caller-cancel")
	caller, cancelCaller := context.WithCancel(context.Background())
	defer cancelCaller()
	executor := &contextBoundExecutor{}
	activation, err := ApplyVerifiedServiceScope(caller, request, scope, observer, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
	if err != nil {
		t.Fatal(err)
	}
	cancelCaller()
	if err := activation.VerifyActive(context.Background()); err != nil {
		t.Fatalf("caller cancellation stopped committed service-scope process: %v", err)
	}
	if err := activation.Revert(context.Background()); err != nil {
		t.Fatal(err)
	}
	callsAfterRevert := append([]string(nil), executor.calls...)
	if err := activation.Revert(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(executor.calls, callsAfterRevert) {
		t.Fatalf("second Revert mutated lifecycle: %v", executor.calls)
	}
}

func TestApplyVerifiedServiceScopeFailureAfterActivateStopsProcess(t *testing.T) {
	request, scope, observer := managedServiceScopeRequest(t, "managed-service-scope-active-failure")
	observer.results[1] = observation("active", false, "192.0.2.1", request.Target.URL)
	executor := &contextBoundExecutor{}
	activation, err := ApplyVerifiedServiceScope(context.Background(), request, scope, observer, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
	if err == nil || activation != nil {
		t.Fatalf("activation=%#v err=%v", activation, err)
	}
	if executor.processAlive || !executor.restored {
		t.Fatalf("failed Apply processAlive=%t restored=%t", executor.processAlive, executor.restored)
	}
}

func TestApplyVerifiedServiceScopeCancellationAfterActivateStopsProcess(t *testing.T) {
	request, scope, observer := managedServiceScopeRequest(t, "managed-service-scope-cancel")
	caller, cancelCaller := context.WithCancel(context.Background())
	defer cancelCaller()
	observer.cancel = cancelCaller
	observer.errAt = 1
	executor := &contextBoundExecutor{}
	activation, err := ApplyVerifiedServiceScope(caller, request, scope, observer, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
	if err == nil || activation != nil {
		t.Fatalf("activation=%#v err=%v", activation, err)
	}
	if executor.processAlive || !executor.restored {
		t.Fatalf("canceled Apply processAlive=%t restored=%t", executor.processAlive, executor.restored)
	}
}

type cancelActiveScopeObserver struct {
	executor *contextBoundExecutor
	cancel   context.CancelFunc
	once     sync.Once
}

func (o *cancelActiveScopeObserver) Observe(ctx context.Context, rawURL string, options observatory.Options) (observatory.ObservationResult, error) {
	if o.executor.processAlive {
		o.once.Do(o.cancel)
		<-ctx.Done()
		return observatory.ObservationResult{}, ctx.Err()
	}
	return observation("direct", false, options.ResolvedIP.String(), rawURL), nil
}

func TestApplyVerifiedServiceScopeMultiEdgeCancellationCleansUp(t *testing.T) {
	request, _ := managedLifetimeRequest(t, "managed-service-scope-multi-edge-cancel")
	scope := ServiceScopeSnapshot{
		Target: request.Target,
		Edges: []ServiceScopeEdge{
			{IP: []byte{192, 0, 2, 1}, Family: observatory.AddressFamilyIPv4},
			{IP: []byte{192, 0, 2, 2}, Family: observatory.AddressFamilyIPv4},
		},
	}
	request.ValidatedScope = scope
	caller, cancelCaller := context.WithCancel(context.Background())
	defer cancelCaller()
	executor := &contextBoundExecutor{}
	observer := &cancelActiveScopeObserver{executor: executor, cancel: cancelCaller}
	activation, err := ApplyVerifiedServiceScope(caller, request, scope, observer, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
	if err == nil || activation != nil {
		t.Fatalf("activation=%#v err=%v", activation, err)
	}
	if executor.processAlive || !executor.restored {
		t.Fatalf("multi-edge cancellation processAlive=%t restored=%t", executor.processAlive, executor.restored)
	}
	if !slices.Contains(executor.calls, "deactivate") || !slices.Contains(executor.calls, "verify-restored") {
		t.Fatalf("multi-edge cancellation cleanup calls=%v", executor.calls)
	}
}

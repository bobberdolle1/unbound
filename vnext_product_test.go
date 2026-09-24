package main

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"unbound/engine"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/planner"
	"unbound/engine/providers"
)

type productVNextTestProvider struct {
	mu      sync.Mutex
	name    string
	profile string
	starts  []string
	stops   int
}

func (p *productVNextTestProvider) Name() string                             { return p.name }
func (p *productVNextTestProvider) CheckPrivileges() (bool, error)           { return true, nil }
func (p *productVNextTestProvider) GetProfiles() []string                    { return []string{"profile-X"} }
func (p *productVNextTestProvider) GetLogs() []string                        { return nil }
func (p *productVNextTestProvider) SetStatusCallback(func(providers.Status)) {}
func (p *productVNextTestProvider) SetLogCallback(func(string))              {}
func (p *productVNextTestProvider) RegisterProfile(string, []string)         {}
func (p *productVNextTestProvider) CurrentProfile() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.profile
}
func (p *productVNextTestProvider) GetStatus() providers.Status {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.profile == "" {
		return providers.StatusStopped
	}
	return providers.StatusRunning
}
func (p *productVNextTestProvider) Start(_ context.Context, profile string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.profile = profile
	p.starts = append(p.starts, profile)
	return nil
}
func (p *productVNextTestProvider) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.profile = ""
	p.stops++
	return nil
}

func productVNextTestManager(t *testing.T) (*providers.ProviderManager, *productVNextTestProvider) {
	t.Helper()
	manager := providers.NewProviderManager()
	provider := &productVNextTestProvider{name: "engine-A"}
	manager.Register(provider)
	if err := manager.Start(context.Background(), provider.Name(), "profile-X"); err != nil {
		t.Fatal(err)
	}
	return manager, provider
}

func TestProductRuntimeProviderRestoresActualManagerProfile(t *testing.T) {
	manager, provider := productVNextTestManager(t)
	adapter := newProductRuntimeProvider(manager)
	if got := adapter.GetStatus(); got != providers.StatusRunning {
		t.Fatalf("status=%s", got)
	}
	if got := adapter.CurrentProfile(); got != "profile-X" {
		t.Fatalf("profile=%q", got)
	}
	if err := adapter.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := adapter.Start(context.Background(), "profile-X"); err != nil {
		t.Fatal(err)
	}
	if got := manager.ActiveEngineName(); got != "engine-A" {
		t.Fatalf("manager engine=%q", got)
	}
	if got := manager.ActiveProfileName(); got != "profile-X" {
		t.Fatalf("manager profile=%q", got)
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if provider.stops != 1 || !slices.Equal(provider.starts, []string{"profile-X", "profile-X"}) {
		t.Fatalf("provider lifecycle stops=%d starts=%v", provider.stops, provider.starts)
	}
}

func TestProductRuntimeProviderKeepsStoppedProductState(t *testing.T) {
	manager := providers.NewProviderManager()
	provider := &productVNextTestProvider{name: "engine-A"}
	manager.Register(provider)
	adapter := newProductRuntimeProvider(manager)
	if got := adapter.GetStatus(); got != providers.StatusStopped {
		t.Fatalf("status=%s", got)
	}
	if got := adapter.CurrentProfile(); got != "" {
		t.Fatalf("profile=%q", got)
	}
	if err := adapter.Stop(); err != nil {
		t.Fatal(err)
	}
	if got := manager.GetStatus(); got != providers.StatusStopped {
		t.Fatalf("restored status=%s", got)
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if len(provider.starts) != 0 {
		t.Fatalf("stopped state invented a profile: starts=%v", provider.starts)
	}
}

func TestNormalizeVNextTargetRejectsUnsafeInputs(t *testing.T) {
	for _, raw := range []string{
		"http://example.test/",
		"https://user:password@example.test/",
		"https://-bad.example/",
		"https:///missing-host",
	} {
		if _, _, err := normalizeVNextTarget(raw); err == nil {
			t.Fatalf("accepted unsafe target %q", raw)
		}
	}
	target, public, err := normalizeVNextTarget("https://Example.test/path?token=secret")
	if err != nil {
		t.Fatal(err)
	}
	if target.URL != "https://example.test/path?token=secret" || public != "https://example.test/path" {
		t.Fatalf("target=%q public=%q", target.URL, public)
	}
}

func TestProductionVNextCatalogExcludesAcceptanceStrategy(t *testing.T) {
	catalog, err := productionVNextStrategyCatalog("target.test")
	if err != nil {
		t.Fatal(err)
	}
	for _, strategy := range catalog {
		if strategy.ID == "autotune-acceptance-tls-split-v1" {
			t.Fatal("acceptance strategy leaked into production catalog")
		}
		if strategy.ID == "alternative-fake-tls" {
			t.Fatal("high-risk fake TLS fixture leaked into production catalog")
		}
	}
	if len(catalog) == 0 {
		t.Fatal("production catalog is empty")
	}
}

func TestAutoTuneVNextResultMappingPreservesStatuses(t *testing.T) {
	for _, status := range []autotunevnext.Status{
		autotunevnext.StatusCompletedNoActionNeeded,
		autotunevnext.StatusCompletedNoVerifiedCandidate,
		autotunevnext.StatusStateRestoreFailed,
	} {
		mapped := mapAutoTuneVNextResult(autotunevnext.Result{Status: status, Backend: backendcap.Zapret2Windows}, "https://target.test/")
		if mapped.Status != string(status) || mapped.CatalogStatus != productVNextCatalogStatus {
			t.Fatalf("mapped=%+v, want status=%q catalog=%q", mapped, status, productVNextCatalogStatus)
		}
	}
}

func TestAutoTuneVNextResultSerializationIsRedactedAndStable(t *testing.T) {
	result := autotunevnext.Result{
		Status:        autotunevnext.StatusCompletedNoEligibleCandidates,
		StateRestored: true,
		Backend:       backendcap.Zapret2Windows,
		Limitations:   []string{"TEST_LIMITATION"},
		Experiments: []autotunevnext.CandidateExperiment{{
			StrategyID: "candidate", PlannerStatus: planner.StatusEligible, Outcome: autotunevnext.OutcomeNotRunPolicy,
		}},
		Lifecycle: autotunevnext.Lifecycle{Errors: []autotunevnext.Reason{{Code: "RESTORE_FAILED", Detail: "contains secret query"}}},
	}
	data, err := json.Marshal(mapAutoTuneVNextResult(result, "https://target.test/path"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !containsAll(text, `"status":"COMPLETED_NO_ELIGIBLE_CANDIDATES"`, `"state_restored":true`, `"planner_status":"ELIGIBLE"`, `"code":"RESTORE_FAILED"`) || strings.Contains(text, "secret query") || strings.Contains(text, `"planner_disposition":"ELIGIBLE"`) {
		t.Fatalf("unexpected DTO JSON: %s", text)
	}
}

func containsAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
}

type productVNextNoopExecutor struct{ calls []string }

func (e *productVNextNoopExecutor) Snapshot(context.Context) (autotunevnext.StateSnapshot, error) {
	e.calls = append(e.calls, "snapshot")
	return autotunevnext.StateSnapshot{ID: "test"}, nil
}
func (e *productVNextNoopExecutor) EstablishDirect(context.Context, autotunevnext.StateSnapshot) error {
	e.calls = append(e.calls, "direct")
	return nil
}
func (e *productVNextNoopExecutor) Activate(context.Context, autotunevnext.ExecutableCandidate) error {
	e.calls = append(e.calls, "activate")
	return nil
}
func (e *productVNextNoopExecutor) VerifyActive(context.Context, autotunevnext.ExecutableCandidate) error {
	return nil
}
func (e *productVNextNoopExecutor) Deactivate(context.Context) error {
	e.calls = append(e.calls, "deactivate")
	return nil
}
func (e *productVNextNoopExecutor) Restore(context.Context, autotunevnext.StateSnapshot) error {
	e.calls = append(e.calls, "restore")
	return nil
}
func (e *productVNextNoopExecutor) VerifyRestored(context.Context, autotunevnext.StateSnapshot) error {
	e.calls = append(e.calls, "verify-restored")
	return nil
}

type productVNextSupportedPreflight struct{}

func (productVNextSupportedPreflight) Check(context.Context, autotunevnext.HostPreflightRequest) autotunevnext.HostPreflightResult {
	return autotunevnext.HostPreflightResult{Status: autotunevnext.PreflightSupported}
}

type productVNextAssets struct{}

func (productVNextAssets) Resolve(context.Context, backendcap.Backend, []string) ([]autotunevnext.ResolvedAsset, error) {
	return nil, nil
}

type productVNextObserver struct{}

func (productVNextObserver) Observe(context.Context, string, observatory.Options) (observatory.ObservationResult, error) {
	return observatory.ObservationResult{}, errors.New("must not observe while coordinator is held")
}

type productVNextBlockingObserver struct {
	started chan struct{}
}

func (o productVNextBlockingObserver) Observe(ctx context.Context, _ string, _ observatory.Options) (observatory.ObservationResult, error) {
	o.started <- struct{}{}
	<-ctx.Done()
	return observatory.ObservationResult{}, ctx.Err()
}

func productVNextTestApp(t *testing.T) (*App, *productVNextNoopExecutor, <-chan struct{}) {
	t.Helper()
	manager, _ := productVNextTestManager(t)
	executor := &productVNextNoopExecutor{}
	observer := productVNextBlockingObserver{started: make(chan struct{}, 1)}
	app := NewApp()
	app.ctx = context.Background()
	app.manager = manager
	app.assets = &engine.AssetPaths{}
	app.newVNextService = func(manager *providers.ProviderManager, assets *engine.AssetPaths) *productVNextService {
		return newProductVNextServiceWith(manager, assets, productVNextDependencies{
			newRuntime: func(autotunevnext.RuntimeProvider, *engine.AssetPaths, func(autotunevnext.PhysicalLog)) (productVNextRuntime, error) {
				return productVNextRuntime{executor: executor, preflight: productVNextSupportedPreflight{}, backend: backendcap.Zapret2Windows}, nil
			},
			newResolver: func(*engine.AssetPaths) (autotunevnext.AssetResolver, error) { return productVNextAssets{}, nil },
			observer:    observer,
			run:         autotunevnext.RunCoordinated,
		})
	}
	return app, executor, observer.started
}

func waitForProductVNextStart(t *testing.T, started <-chan struct{}) {
	t.Helper()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("AutoTune vNext did not reach its cancellable observation")
	}
}

func assertProductVNextRestored(t *testing.T, executor *productVNextNoopExecutor) {
	t.Helper()
	if !slices.Equal(executor.calls, []string{"snapshot", "direct", "restore", "verify-restored"}) {
		t.Fatalf("executor lifecycle=%v", executor.calls)
	}
}

func TestAppAutoTuneVNextCancelRestoresAndClearsState(t *testing.T) {
	app, executor, started := productVNextTestApp(t)
	results := make(chan AutoTuneVNextResult, 1)
	go func() {
		results <- app.AutoTuneVNext("https://target.test/", nil)
	}()
	waitForProductVNextStart(t, started)
	app.CancelAutoTune()
	result := <-results
	if result.Status != string(autotunevnext.StatusCancelled) || !result.StateRestored {
		t.Fatalf("result=%+v", result)
	}
	app.autoTuneWG.Wait()
	app.mu.Lock()
	cancel := app.autoTuneCancel
	app.mu.Unlock()
	if cancel != nil {
		t.Fatal("AutoTune vNext cancellation state leaked")
	}
	if op, owner := engine.GetCoordinator().CurrentOperation(); op != engine.OpIdle || owner != "" {
		t.Fatalf("coordinator leaked operation=%s owner=%q", op, owner)
	}
	assertProductVNextRestored(t, executor)
}

func TestAppShutdownCancelsAutoTuneVNextAfterRestoration(t *testing.T) {
	app, executor, started := productVNextTestApp(t)
	results := make(chan AutoTuneVNextResult, 1)
	go func() {
		results <- app.AutoTuneVNext("https://target.test/", nil)
	}()
	waitForProductVNextStart(t, started)
	shutdownDone := make(chan struct{})
	go func() {
		app.shutdown(context.Background())
		close(shutdownDone)
	}()
	select {
	case <-shutdownDone:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown did not wait for AutoTune vNext restoration")
	}
	result := <-results
	if result.Status != string(autotunevnext.StatusCancelled) || !result.StateRestored {
		t.Fatalf("result=%+v", result)
	}
	assertProductVNextRestored(t, executor)
}

func TestProductVNextUsesCoordinatorWithoutCallerDoubleLock(t *testing.T) {
	manager, _ := productVNextTestManager(t)
	executor := &productVNextNoopExecutor{}
	service := newProductVNextServiceWith(manager, &engine.AssetPaths{}, productVNextDependencies{
		newRuntime: func(autotunevnext.RuntimeProvider, *engine.AssetPaths, func(autotunevnext.PhysicalLog)) (productVNextRuntime, error) {
			return productVNextRuntime{executor: executor, preflight: productVNextSupportedPreflight{}, backend: backendcap.Zapret2Windows}, nil
		},
		newResolver: func(*engine.AssetPaths) (autotunevnext.AssetResolver, error) { return productVNextAssets{}, nil },
		observer:    productVNextObserver{},
		run:         autotunevnext.RunCoordinated,
	})
	release, err := engine.GetCoordinator().Acquire(engine.OpAutoTune, "legacy-autotune", false)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	result := service.Run(context.Background(), AutoTuneVNextRequest{Target: "https://target.test/"})
	if result.Status != string(autotunevnext.StatusInconclusive) || !slices.Contains(result.Limitations, "OPERATION_CONFLICT") {
		t.Fatalf("result=%+v", result)
	}
	if len(executor.calls) != 0 {
		t.Fatalf("conflict mutated executor: %v", executor.calls)
	}
}

func TestPlatformVNextRuntimeFactory(t *testing.T) {
	manager, _ := productVNextTestManager(t)
	binding, err := newPlatformVNextRuntime(newProductRuntimeProvider(manager), &engine.AssetPaths{}, nil)
	if runtime.GOOS == "darwin" {
		if !errors.Is(err, errVNextMeasurementPathUnsupported) {
			t.Fatalf("macOS factory err=%v", err)
		}
		return
	}
	if err != nil || binding.executor == nil || binding.preflight == nil {
		t.Fatalf("factory binding=%+v err=%v", binding, err)
	}
	want := backendcap.Zapret2Windows
	if runtime.GOOS == "linux" {
		want = backendcap.Zapret2Linux
	}
	if binding.backend != want {
		t.Fatalf("backend=%s, want %s", binding.backend, want)
	}
}

package main

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"unbound/engine"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/strategyir"
)

type managedVNextSequenceObserver struct {
	results []observatory.ObservationResult
}

func (o *managedVNextSequenceObserver) Observe(context.Context, string, observatory.Options) (observatory.ObservationResult, error) {
	if len(o.results) == 0 {
		return observatory.ObservationResult{}, errors.New("unexpected observation")
	}
	result := o.results[0]
	o.results = o.results[1:]
	return result, nil
}

func managedVNextObservation(success bool) observatory.ObservationResult {
	now := time.Unix(0, 0).UTC()
	stages := []observatory.StageEvidence{
		{Stage: observatory.StageResolve, Status: observatory.StatusPass, StartedAt: now},
		{Stage: observatory.StageConnect, Status: observatory.StatusPass, StartedAt: now},
		{Stage: observatory.StageHello, Status: observatory.StatusPass, StartedAt: now},
	}
	classification, boundary := observatory.ClassTLSHandshakeTimeout, observatory.StageHandshake
	if success {
		stages = append(stages,
			observatory.StageEvidence{Stage: observatory.StageHandshake, Status: observatory.StatusPass, StartedAt: now},
			observatory.StageEvidence{Stage: observatory.StageHTTP, Status: observatory.StatusPass, StartedAt: now, HTTPStatus: 200, PathComplete: true},
		)
		classification, boundary = observatory.ClassSuccess, observatory.StageHTTP
	} else {
		stages = append(stages, observatory.StageEvidence{Stage: observatory.StageHandshake, Status: observatory.StatusTimeout, StartedAt: now, Class: observatory.ClassTLSHandshakeTimeout})
	}
	primary := 0
	return observatory.ObservationResult{
		SchemaVersion: observatory.SchemaVersion,
		StartedAt:     now,
		FinishedAt:    now.Add(time.Second),
		NetworkContext: observatory.NetworkContext{
			AddressFamily: observatory.AddressFamilyIPv4,
		},
		Target:              observatory.Target{URL: "https://target.test/", Hostname: "target.test", Port: "443", RequestedProtocol: observatory.TransportTCP},
		Attempts:            []observatory.ConnectionAttempt{{ResolvedIP: "192.0.2.1", AddressFamily: observatory.AddressFamilyIPv4, Transport: observatory.TransportTCP, Stages: stages}},
		PrimaryAttemptIndex: &primary,
		FinalBoundary:       boundary,
		Classification:      classification,
	}
}

type managedVNextRequestedAssets struct{}

func (managedVNextRequestedAssets) Resolve(_ context.Context, _ backendcap.Backend, ids []string) ([]autotunevnext.ResolvedAsset, error) {
	assets := make([]autotunevnext.ResolvedAsset, 0, len(ids))
	for _, id := range ids {
		assets = append(assets, autotunevnext.ResolvedAsset{ID: id, EngineValue: "test-" + id})
	}
	return assets, nil
}

func appliedManagedVNextService(t *testing.T) (*productVNextService, *productVNextNoopExecutor, *productVNextTestProvider) {
	t.Helper()
	manager, provider := productVNextTestManager(t)
	target, publicTarget, err := normalizeVNextTarget("https://target.test/")
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := productionVNextStrategyCatalog("target.test")
	if err != nil || len(catalog) == 0 {
		t.Fatalf("catalog=%v err=%v", catalog, err)
	}
	strategy := catalog[0]
	fingerprint, err := strategyir.Fingerprint(strategy)
	if err != nil {
		t.Fatal(err)
	}
	executor := &productVNextNoopExecutor{}
	observer := &managedVNextSequenceObserver{results: []observatory.ObservationResult{
		managedVNextObservation(false),
		managedVNextObservation(false),
		managedVNextObservation(true),
	}}
	service := newProductVNextServiceWith(manager, &engine.AssetPaths{}, productVNextDependencies{
		newRuntime: func(autotunevnext.RuntimeProvider, *engine.AssetPaths, func(autotunevnext.PhysicalLog)) (productVNextRuntime, error) {
			return productVNextRuntime{executor: executor, preflight: productVNextSupportedPreflight{}, backend: backendcap.Zapret2Windows}, nil
		},
		newResolver: func(*engine.AssetPaths) (autotunevnext.AssetResolver, error) {
			return managedVNextRequestedAssets{}, nil
		},
		observer: observer,
		run:      autotunevnext.RunCoordinated,
	})
	grant := verifiedSelectionGrant{token: "test-grant", target: target, publicTarget: publicTarget, strategyID: strategy.ID, fingerprint: fingerprint, backend: backendcap.Zapret2Windows, createdAt: time.Now(), expiresAt: time.Now().Add(time.Minute)}
	service.grants[grant.token] = grant
	if status := service.Apply(context.Background(), grant.token); status.State != "APPLIED" || !status.Active {
		t.Fatalf("apply status=%+v", status)
	}
	return service, executor, provider
}

func stubStartEngineRuntime(t *testing.T) {
	t.Helper()
	previousEventsEmit := appRuntimeEventsEmit
	previousLogError := appRuntimeLogError
	previousLogErrorf := appRuntimeLogErrorf
	previousLogInfo := appRuntimeLogInfo
	previousLogInfof := appRuntimeLogInfof
	appRuntimeEventsEmit = func(context.Context, string, ...interface{}) {}
	appRuntimeLogError = func(context.Context, string) {}
	appRuntimeLogErrorf = func(context.Context, string, ...interface{}) {}
	appRuntimeLogInfo = func(context.Context, string) {}
	appRuntimeLogInfof = func(context.Context, string, ...interface{}) {}
	t.Cleanup(func() {
		appRuntimeEventsEmit = previousEventsEmit
		appRuntimeLogError = previousLogError
		appRuntimeLogErrorf = previousLogErrorf
		appRuntimeLogInfo = previousLogInfo
		appRuntimeLogInfof = previousLogInfof
	})
}

func TestVerifiedGrantRequiresSelectedRestoredVerifiedOutcome(t *testing.T) {
	service := newProductVNextService(nil, nil)
	target, public, err := normalizeVNextTarget("https://target.test/path?secret=value")
	if err != nil {
		t.Fatal(err)
	}
	result := autotunevnext.Result{Status: autotunevnext.StatusCompletedSelected, StateRestored: true, Backend: backendcap.Zapret2Windows, SelectedStrategyID: "strategy", SelectedFingerprint: "fingerprint", Experiments: []autotunevnext.CandidateExperiment{{StrategyID: "strategy", Fingerprint: "fingerprint", Outcome: autotunevnext.OutcomeVerifiedFixed}}}
	token := service.issueVerifiedGrant(result, target, public, nil)
	if token == "" {
		t.Fatal("verified result did not receive a grant")
	}
	grant, err := service.resolveGrant(token)
	if err != nil || grant.strategyID != "strategy" || grant.publicTarget != "https://target.test/path" {
		t.Fatalf("grant=%#v err=%v", grant, err)
	}
	result.StateRestored = false
	if token := service.issueVerifiedGrant(result, target, public, nil); token != "" {
		t.Fatal("unrestored result received a grant")
	}
	result.StateRestored = true
	result.Experiments[0].Outcome = autotunevnext.OutcomeRegressionObserved
	if token := service.issueVerifiedGrant(result, target, public, nil); token != "" {
		t.Fatal("non-verified result received a grant")
	}
}

func TestExpiredAndConsumedGrantFailClosed(t *testing.T) {
	service := newProductVNextService(nil, nil)
	service.grants["expired"] = verifiedSelectionGrant{token: "expired", expiresAt: time.Now().Add(-time.Second)}
	if _, err := service.resolveGrant("expired"); err == nil {
		t.Fatal("expired grant was accepted")
	}
	service.grants["used"] = verifiedSelectionGrant{token: "used", expiresAt: time.Now().Add(time.Minute), consumed: true}
	if _, err := service.resolveGrant("used"); err == nil {
		t.Fatal("consumed grant was accepted")
	}
}

func TestManagedStateRoundTripAndCorruptionFailClosed(t *testing.T) {
	dir := t.TempDir()
	restore := engine.SetConfigDirForTest(dir)
	defer restore()
	state := persistedVNextState{SchemaVersion: vNextManagedSchema, Enabled: true, Target: "https://target.test/path", StrategyID: "strategy", Fingerprint: "fingerprint", Backend: string(backendcap.Zapret2Windows), SavedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if err := saveVNextManagedState(state); err != nil {
		t.Fatal(err)
	}
	loaded, present, err := loadVNextManagedState()
	if err != nil || !present || loaded != state {
		t.Fatalf("loaded=%#v present=%t err=%v", loaded, present, err)
	}
	path, err := getVNextManagedStatePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadVNextManagedState(); err == nil {
		t.Fatal("corrupt state was accepted")
	}
	if err := clearVNextManagedState(); err != nil {
		t.Fatal(err)
	}
	if _, present, err := loadVNextManagedState(); err != nil || present {
		t.Fatalf("present=%t err=%v", present, err)
	}
}

func TestStartupRejectsCatalogFingerprintDrift(t *testing.T) {
	restore := engine.SetConfigDirForTest(t.TempDir())
	defer restore()
	state := persistedVNextState{SchemaVersion: vNextManagedSchema, Enabled: true, Target: "https://target.test/", StrategyID: "prod-tls-multisplit-1-v1", Fingerprint: "stale-fingerprint", Backend: string(backendcap.Zapret2Windows), SavedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if err := saveVNextManagedState(state); err != nil {
		t.Fatal(err)
	}
	if got := newProductVNextService(nil, nil).RevalidateSaved(context.Background()); got.State != "SAVED_STRATEGY_STALE" {
		t.Fatalf("state=%#v", got)
	}
}

func TestShutdownPreservesDormantManagedIntentAndLegacyTakeoverClearsIt(t *testing.T) {
	restore := engine.SetConfigDirForTest(t.TempDir())
	defer restore()
	state := persistedVNextState{SchemaVersion: vNextManagedSchema, Enabled: true, Target: "https://target.test/", StrategyID: "strategy", Fingerprint: "fingerprint", Backend: string(backendcap.Zapret2Windows), SavedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if err := saveVNextManagedState(state); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	app.ctx = context.Background()
	app.shutdown(context.Background())
	if _, present, err := loadVNextManagedState(); err != nil || !present {
		t.Fatalf("shutdown lost managed intent present=%t err=%v", present, err)
	}
	if err := app.disableManagedVNextIntent(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, present, err := loadVNextManagedState(); err != nil || present {
		t.Fatalf("legacy takeover retained managed intent present=%t err=%v", present, err)
	}
}

func TestManagedStatusReportsDormantIntent(t *testing.T) {
	restore := engine.SetConfigDirForTest(t.TempDir())
	defer restore()
	state := persistedVNextState{SchemaVersion: vNextManagedSchema, Enabled: true, Target: "https://target.test/", StrategyID: "strategy", Fingerprint: "fingerprint", Backend: string(backendcap.Zapret2Windows), SavedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if err := saveVNextManagedState(state); err != nil {
		t.Fatal(err)
	}
	if got := newProductVNextService(nil, nil).Status(); got.State != "SAVED_REVALIDATION_PENDING" || got.Active || !got.NeedsRevalidation {
		t.Fatalf("status=%+v", got)
	}
}

func TestManualMutationCancelsStartupRevalidationBeforeLegacyTakeover(t *testing.T) {
	restore := engine.SetConfigDirForTest(t.TempDir())
	defer restore()
	state := persistedVNextState{SchemaVersion: vNextManagedSchema, Enabled: true, Target: "https://target.test/", StrategyID: "strategy", Fingerprint: "fingerprint", Backend: string(backendcap.Zapret2Windows), SavedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if err := saveVNextManagedState(state); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	app.ctx = context.Background()
	app.assets = &engine.AssetPaths{}
	app.startManagedVNextRevalidation(context.Background())
	if !app.beginManualProfileChange() {
		t.Fatal("manual mutation was rejected")
	}
	app.endManualProfileChange()
	app.startupWG.Wait()
}

func TestExperimentWhileManagedActiveIsRejected(t *testing.T) {
	service := newProductVNextService(nil, nil)
	service.active = &managedVNextActivation{}
	result := service.Run(context.Background(), AutoTuneVNextRequest{Target: "https://target.test/"})
	if result.Status != string(autotunevnext.StatusInconclusive) || len(result.Limitations) != 1 || result.Limitations[0] != "MANAGED_ACTIVATION_ACTIVE" {
		t.Fatalf("result=%+v", result)
	}
}

func TestStartEngineTakesOverActiveManagedVNext(t *testing.T) {
	stubStartEngineRuntime(t)
	engine.GetLogger()
	restore := engine.SetConfigDirForTest(t.TempDir())
	defer restore()
	service, executor, provider := appliedManagedVNextService(t)
	app := NewApp()
	app.ctx = context.Background()
	app.manager = service.manager
	app.vNextService = service
	previousCheck := appCheckAdminPrivileges
	appCheckAdminPrivileges = func() (bool, error) { return true, nil }
	defer func() { appCheckAdminPrivileges = previousCheck }()
	if err := app.StartEngine(provider.Name(), "profile-X"); err != nil {
		t.Fatalf("StartEngine: %v", err)
	}
	if status := service.Status(); status.State != "DIRECT" || status.Active {
		t.Fatalf("managed status=%+v", status)
	}
	if _, present, err := loadVNextManagedState(); err != nil || present {
		t.Fatalf("persisted state present=%t err=%v", present, err)
	}
	if len(executor.calls) < 6 || executor.calls[len(executor.calls)-2] != "restore" || executor.calls[len(executor.calls)-1] != "verify-restored" {
		t.Fatalf("executor calls=%v", executor.calls)
	}
}

func TestStartEngineClearsDormantManagedVNextIntent(t *testing.T) {
	stubStartEngineRuntime(t)
	engine.GetLogger()
	restore := engine.SetConfigDirForTest(t.TempDir())
	defer restore()
	manager, provider := productVNextTestManager(t)
	state := persistedVNextState{SchemaVersion: vNextManagedSchema, Enabled: true, Target: "https://target.test/", StrategyID: "strategy", Fingerprint: "fingerprint", Backend: string(backendcap.Zapret2Windows), SavedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	if err := saveVNextManagedState(state); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	app.ctx = context.Background()
	app.manager = manager
	previousCheck := appCheckAdminPrivileges
	appCheckAdminPrivileges = func() (bool, error) { return true, nil }
	defer func() { appCheckAdminPrivileges = previousCheck }()
	if err := app.StartEngine(provider.Name(), "profile-X"); err != nil {
		t.Fatalf("StartEngine: %v", err)
	}
	if _, present, err := loadVNextManagedState(); err != nil || present {
		t.Fatalf("persisted state present=%t err=%v", present, err)
	}
}

func TestStartEngineAbortsWhenManagedRestoreFails(t *testing.T) {
	stubStartEngineRuntime(t)
	engine.GetLogger()
	restore := engine.SetConfigDirForTest(t.TempDir())
	defer restore()
	service, executor, provider := appliedManagedVNextService(t)
	executor.restoreErr = errors.New("restore")
	app := NewApp()
	app.ctx = context.Background()
	app.manager = service.manager
	app.vNextService = service
	previousCheck := appCheckAdminPrivileges
	appCheckAdminPrivileges = func() (bool, error) { return true, nil }
	defer func() { appCheckAdminPrivileges = previousCheck }()
	if err := app.StartEngine(provider.Name(), "profile-X"); err == nil {
		t.Fatal("StartEngine started legacy profile after failed managed restoration")
	}
	if status := service.Status(); status.State != "STATE_RESTORE_FAILED" || status.Active {
		t.Fatalf("managed status=%+v", status)
	}
	provider.mu.Lock()
	starts := append([]string(nil), provider.starts...)
	provider.mu.Unlock()
	if len(starts) != 1 {
		t.Fatalf("legacy start ran after failed restoration: starts=%v", starts)
	}
}

func TestRestoreFailureStatusStaysFactualUntilRetrySucceeds(t *testing.T) {
	restore := engine.SetConfigDirForTest(t.TempDir())
	defer restore()
	service, executor, _ := appliedManagedVNextService(t)
	executor.verifyRestoredErr = errors.New("verify restored")
	if status := service.Revert(context.Background()); status.State != "STATE_RESTORE_FAILED" || status.Active {
		t.Fatalf("initial restore status=%+v", status)
	}
	if status := service.Status(); status.State != "STATE_RESTORE_FAILED" || status.Active {
		t.Fatalf("sticky status=%+v", status)
	}
	executor.verifyRestoredErr = nil
	if status := service.Revert(context.Background()); status.State != "REVERTED" || status.Active {
		t.Fatalf("retry status=%+v", status)
	}
	if status := service.Status(); status.State != "DIRECT" || status.Active {
		t.Fatalf("final status=%+v", status)
	}
}

func TestActiveManagedShutdownPreservesIntent(t *testing.T) {
	restore := engine.SetConfigDirForTest(t.TempDir())
	defer restore()
	service, _, _ := appliedManagedVNextService(t)
	app := NewApp()
	app.ctx = context.Background()
	app.manager = service.manager
	app.vNextService = service
	app.shutdown(context.Background())
	if _, present, err := loadVNextManagedState(); err != nil || !present {
		t.Fatalf("shutdown lost saved intent present=%t err=%v", present, err)
	}
	if status := service.Status(); status.State != "SAVED_REVALIDATION_PENDING" || status.Active || !status.NeedsRevalidation {
		t.Fatalf("shutdown status=%+v", status)
	}
}

func TestHealthFailureSuspendsAndPreservesIntent(t *testing.T) {
	restore := engine.SetConfigDirForTest(t.TempDir())
	defer restore()
	service, _, _ := appliedManagedVNextService(t)
	service.deps.run = func(context.Context, autotunevnext.Request, autotunevnext.Observer, autotunevnext.Executor, autotunevnext.HostPreflight, autotunevnext.AssetResolver) (autotunevnext.Result, error) {
		return autotunevnext.Result{Status: autotunevnext.StatusCompletedNoActionNeeded, StateRestored: true}, nil
	}
	if status := service.RevalidateActive(context.Background()); status.State != "SAVED_NOT_CURRENTLY_NEEDED" || status.Active {
		t.Fatalf("health recovery status=%+v", status)
	}
	if _, present, err := loadVNextManagedState(); err != nil || !present {
		t.Fatalf("health recovery lost intent present=%t err=%v", present, err)
	}
}

func TestHealthRevalidationReactivatesExactSavedStrategy(t *testing.T) {
	restore := engine.SetConfigDirForTest(t.TempDir())
	defer restore()
	service, _, _ := appliedManagedVNextService(t)
	state, present, err := loadVNextManagedState()
	if err != nil || !present {
		t.Fatalf("saved state present=%t err=%v", present, err)
	}
	observer := service.deps.observer.(*managedVNextSequenceObserver)
	observer.results = []observatory.ObservationResult{
		managedVNextObservation(false),
		managedVNextObservation(false),
		managedVNextObservation(false),
		managedVNextObservation(true),
		managedVNextObservation(true),
	}
	service.deps.run = func(_ context.Context, request autotunevnext.Request, _ autotunevnext.Observer, _ autotunevnext.Executor, _ autotunevnext.HostPreflight, _ autotunevnext.AssetResolver) (autotunevnext.Result, error) {
		return autotunevnext.Result{
			Status:              autotunevnext.StatusCompletedSelected,
			StateRestored:       true,
			Backend:             backendcap.Zapret2Windows,
			SelectedStrategyID:  state.StrategyID,
			SelectedFingerprint: state.Fingerprint,
			Experiments:         []autotunevnext.CandidateExperiment{{StrategyID: state.StrategyID, Fingerprint: state.Fingerprint, Outcome: autotunevnext.OutcomeVerifiedFixed}},
		}, nil
	}
	if status := service.RevalidateActive(context.Background()); status.State != "APPLIED" || !status.Active || status.StrategyID != state.StrategyID {
		t.Fatalf("health revalidation status=%+v", status)
	}
}

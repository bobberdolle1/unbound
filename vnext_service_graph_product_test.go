package main

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"unbound/engine"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
	"unbound/engine/historyid"
	"unbound/engine/observatory"
	"unbound/engine/outcomeledger"
	"unbound/engine/providers"
)

func TestSaveSettingsDoesNotClobberOSDNSWhenUnchanged(t *testing.T) {
	restoreDir := engine.SetConfigDirForTest(t.TempDir())
	defer restoreDir()

	prevSetDNS := appSetSecureDNS
	var dnsCalls []bool
	appSetSecureDNS = func(enabled bool) error {
		dnsCalls = append(dnsCalls, enabled)
		return nil
	}
	defer func() { appSetSecureDNS = prevSetDNS }()

	app := NewApp()
	app.ctx = context.Background()

	// 1. Fresh config dir: saving logs/mode with SecureDNS=false MUST NOT call SetSecureDNS(false)
	s, err := app.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	s.DiagnosticsMode = "extended"
	s.ShowLogs = true
	s.SecureDNS = false
	if err := app.SaveSettings(s); err != nil {
		t.Fatalf("SaveSettings (logs/mode): %v", err)
	}
	if len(dnsCalls) != 0 {
		t.Fatalf("SaveSettings called SetSecureDNS on unrelated change: %v", dnsCalls)
	}

	// 2. Another unrelated toggle (AutoStartProfile / GameFilter) still makes 0 DNS calls
	s.GameFilter = true
	if err := app.SaveSettings(s); err != nil {
		t.Fatalf("SaveSettings (GameFilter): %v", err)
	}
	if len(dnsCalls) != 0 {
		t.Fatalf("SaveSettings called SetSecureDNS when SecureDNS remained false: %v", dnsCalls)
	}

	// 3. Enabling SecureDNS=true invokes SetSecureDNS(true) exactly once
	s.SecureDNS = true
	if err := app.SaveSettings(s); err != nil {
		t.Fatalf("SaveSettings (enable SecureDNS): %v", err)
	}
	if !slices.Equal(dnsCalls, []bool{true}) {
		t.Fatalf("expected [true], got %v", dnsCalls)
	}

	// 4. Changing DefaultProfile while SecureDNS remains true does NOT re-run SetSecureDNS
	s.DefaultProfile = "Alternative 1"
	if err := app.SaveSettings(s); err != nil {
		t.Fatalf("SaveSettings (profile while SecureDNS=true): %v", err)
	}
	if !slices.Equal(dnsCalls, []bool{true}) {
		t.Fatalf("expected no duplicate SetSecureDNS(true), got %v", dnsCalls)
	}

	// 5. Disabling SecureDNS=false invokes SetSecureDNS(false)
	s.SecureDNS = false
	if err := app.SaveSettings(s); err != nil {
		t.Fatalf("SaveSettings (disable SecureDNS): %v", err)
	}
	if !slices.Equal(dnsCalls, []bool{true, false}) {
		t.Fatalf("expected [true, false], got %v", dnsCalls)
	}
}

func TestKillConflictsReportsSkippedExternalVPNsHonestly(t *testing.T) {
	prevResolve := appResolveConflicts
	defer func() { appResolveConflicts = prevResolve }()

	var emitted []struct {
		event string
		data  []interface{}
	}
	notifMgr := engine.GetNotificationManager()
	notifMgr.Initialize(context.Background(), func(_ context.Context, name string, data ...interface{}) {
		emitted = append(emitted, struct {
			event string
			data  []interface{}
		}{event: name, data: data})
	})
	defer notifMgr.Initialize(nil, nil)

	app := NewApp()
	app.ctx = context.Background()

	// Case 1: Non-killable external VPNs remain active -> Warning notification naming skipped VPNs
	appResolveConflicts = func() ([]string, error) {
		return []string{"Sing-Box Proxy", "Cloudflare WARP"}, nil
	}
	if err := app.KillConflicts(); err != nil {
		t.Fatalf("KillConflicts with skipped VPNs: %v", err)
	}
	if len(emitted) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(emitted))
	}
	payloadJSON, _ := json.Marshal(emitted[0].data)
	text := string(payloadJSON)
	if !strings.Contains(text, "Sing-Box Proxy") || !strings.Contains(text, "Cloudflare WARP") || strings.Contains(text, "Все конфликтующие процессы") {
		t.Fatalf("notification lied or omitted skipped VPNs: %s", text)
	}

	// Case 2: Zero skipped VPNs -> Success notification
	emitted = nil
	appResolveConflicts = func() ([]string, error) {
		return nil, nil
	}
	if err := app.KillConflicts(); err != nil {
		t.Fatalf("KillConflicts clean: %v", err)
	}
	if len(emitted) != 1 {
		t.Fatalf("expected 1 success notification, got %d", len(emitted))
	}
	payloadJSON, _ = json.Marshal(emitted[0].data)
	if !strings.Contains(string(payloadJSON), "Конфликты устранены") {
		t.Fatalf("unexpected success notification: %s", payloadJSON)
	}
}

func TestFormatTrayStatusLabelCrossOSVNextParity(t *testing.T) {
	label, running := formatTrayStatusLabel(providers.StatusStopped, "", false, AutoTuneVNextManagedStatus{
		State:      "VNEXT_MANAGED_ACTIVE",
		Active:     true,
		StrategyID: "prod-tls-multisplit-1-v1",
	})
	if !running || !strings.Contains(label, "Управляемая стратегия") || !strings.Contains(label, "prod-tls-multisplit-1-v1") {
		t.Fatalf("vNext single-host tray label = (%q, %v)", label, running)
	}

	label, running = formatTrayStatusLabel(providers.StatusStopped, "", false, AutoTuneVNextManagedStatus{
		State:      string(GraphManagedActive),
		Active:     true,
		StrategyID: "prod-tls-sniext-split-v1",
	})
	if !running || !strings.Contains(label, "prod-tls-sniext-split-v1") {
		t.Fatalf("vNext graph tray label = (%q, %v)", label, running)
	}

	label, running = formatTrayStatusLabel(providers.StatusStopped, "", false, AutoTuneVNextManagedStatus{
		State: "STATE_RESTORE_FAILED",
	})
	if !running || !strings.Contains(label, "восстановление") {
		t.Fatalf("restore-failed tray label = (%q, %v)", label, running)
	}

	label, running = formatTrayStatusLabel(providers.StatusStopped, "", true, AutoTuneVNextManagedStatus{State: "DIRECT"})
	if running || label != "Статус: Автоподбор..." {
		t.Fatalf("autotuning tray label = (%q, %v)", label, running)
	}
}

func TestVNextHistorySummaryAndClearAreSafeAndAccurate(t *testing.T) {
	tempDir := t.TempDir()
	restoreDir := engine.SetConfigDirForTest(tempDir)
	defer restoreDir()
	historyid.ForgetCachedContextKey()
	defer historyid.ForgetCachedContextKey()

	app := NewApp()

	// 1. Read-only check on empty directory: must NOT create outcomes.json or context.key!
	emptySummary := app.GetAutoTuneVNextHistorySummary()
	if emptySummary.RecordCount != 0 || emptySummary.VerifiedCount != 0 || emptySummary.NegativeCount != 0 || emptySummary.UpdatedAt != "" || emptySummary.ContextReady {
		t.Fatalf("unexpected empty summary: %+v", emptySummary)
	}
	if _, err := os.Stat(filepath.Join(tempDir, historyid.ContextKeyFileName)); !os.IsNotExist(err) {
		t.Fatalf("GetAutoTuneVNextHistorySummary mutated disk by creating context key: %v", err)
	}
	if _, err := os.Stat(filepath.Join(tempDir, outcomeledger.FileName)); !os.IsNotExist(err) {
		t.Fatalf("GetAutoTuneVNextHistorySummary mutated disk by creating outcome ledger: %v", err)
	}

	// Verify exact JSON field names required by UI contract
	rawJSON, err := json.Marshal(emptySummary)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"record_count":0`, `"verified_count":0`, `"negative_count":0`, `"updated_at":""`, `"context_ready":false`} {
		if !strings.Contains(string(rawJSON), key) {
			t.Fatalf("history summary JSON missing %s: %s", key, rawJSON)
		}
	}

	// 2. Create real context key and persist one VERIFIED_FIXED and one STILL_FAILING entry
	if _, err := historyid.LoadOrCreateContextKey(tempDir); err != nil {
		t.Fatalf("LoadOrCreateContextKey: %v", err)
	}
	target, _, err := normalizeVNextTarget("https://target.test/")
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := productionVNextStrategyCatalog("target.test")
	if err != nil {
		t.Fatal(err)
	}
	probe, controls, err := productProbeContracts(target.URL, target, nil)
	if err != nil {
		t.Fatal(err)
	}

	verifiedRun := autotunevnext.Run(
		context.Background(),
		autotunevnext.Request{
			Target: target, Strategies: catalog[:1], Backend: backendcap.Zapret2Windows,
			TargetProbe: &probe, ControlProbes: controls,
			Policy: autotunevnext.Policy{MaxCandidates: 1, MaxDuration: time.Minute, PerObservationTimeout: time.Second},
		},
		&productHistorySequenceObserver{results: []observatory.ObservationResult{
			historyIdentityObservation("b1", false),
			historyIdentityObservation("pre1", false),
			historyIdentityObservation("act1", true),
			historyIdentityObservation("post1", false),
		}},
		&productVNextNoopExecutor{},
		productVNextSupportedPreflight{},
		productVNextAssets{},
	)
	history := loadProductOutcomeLedger(historyid.Bundle{})
	if lim := persistProductOutcomes(verifiedRun, history, historyid.Bundle{}); lim != "" {
		t.Fatalf("persist verified outcome: %s", lim)
	}

	failingObs := func(id string) observatory.ObservationResult {
		obs := historyIdentityObservation(id, false)
		obs.StartedAt = obs.StartedAt.Add(5 * time.Minute)
		obs.FinishedAt = obs.FinishedAt.Add(5 * time.Minute)
		return obs
	}
	negativeRun := autotunevnext.Run(
		context.Background(),
		autotunevnext.Request{
			Target: target, Strategies: catalog[1:2], Backend: backendcap.Zapret2Windows,
			TargetProbe: &probe, ControlProbes: controls,
			Policy: autotunevnext.Policy{MaxCandidates: 1, MaxDuration: time.Minute, PerObservationTimeout: time.Second},
		},
		&productHistorySequenceObserver{results: []observatory.ObservationResult{
			failingObs("b2"),
			failingObs("pre2"),
			failingObs("act2"),
			failingObs("post2"),
		}},
		&productVNextNoopExecutor{},
		productVNextSupportedPreflight{},
		productVNextAssets{},
	)
	history = loadProductOutcomeLedger(historyid.Bundle{})
	if lim := persistProductOutcomes(negativeRun, history, historyid.Bundle{}); lim != "" {
		t.Fatalf("persist negative outcome: %s", lim)
	}

	populated := app.GetAutoTuneVNextHistorySummary()
	if populated.RecordCount != 2 || populated.VerifiedCount != 1 || populated.NegativeCount != 1 || populated.UpdatedAt == "" || !populated.ContextReady {
		t.Fatalf("unexpected populated summary: %+v", populated)
	}

	// 3. ClearAutoTuneVNextHistory removes outcomes.json while keeping context_ready=true
	if err := app.ClearAutoTuneVNextHistory(); err != nil {
		t.Fatalf("ClearAutoTuneVNextHistory: %v", err)
	}
	cleared := app.GetAutoTuneVNextHistorySummary()
	if cleared.RecordCount != 0 || cleared.VerifiedCount != 0 || cleared.NegativeCount != 0 || cleared.UpdatedAt != "" || !cleared.ContextReady {
		t.Fatalf("unexpected cleared summary: %+v", cleared)
	}
}

type multiHostScopeResolver struct {
	mu      sync.Mutex
	byHost  map[string][]string
	resolve func(context.Context, autotunevnext.Target) (autotunevnext.ServiceScopeSnapshot, error)
}

func (r *multiHostScopeResolver) ResolveServiceScope(ctx context.Context, target autotunevnext.Target) (autotunevnext.ServiceScopeSnapshot, error) {
	if r.resolve != nil {
		return r.resolve(ctx, target)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	host := hostnameForVNextTarget(target)
	ips := r.byHost[host]
	scope := autotunevnext.ServiceScopeSnapshot{Target: target}
	for _, ip := range ips {
		scope.Edges = append(scope.Edges, autotunevnext.ServiceScopeEdge{
			IP:     net.ParseIP(ip),
			Family: observatory.AddressFamilyIPv4,
		})
	}
	return scope, nil
}

type serviceGraphDynamicObserver struct {
	graphExec     *fakeGraphExecutor
	alwaysHealthy bool
}

func (o *serviceGraphDynamicObserver) Observe(_ context.Context, rawURL string, opts observatory.Options) (observatory.ObservationResult, error) {
	ip := "203.0.113.10"
	if len(opts.ResolvedIP) > 0 {
		ip = opts.ResolvedIP.String()
	}
	if strings.Contains(rawURL, "cloudflare.com") || o.alwaysHealthy {
		res := managedVNextObservationAt(true, ip)
		res.Target.URL = rawURL
		return res, nil
	}
	res := managedVNextObservationAt(o.graphExec.alive, ip)
	res.Target.URL = rawURL
	return res, nil
}

func newServiceGraphTestApp(t *testing.T, alwaysHealthy bool) (*App, *fakeGraphExecutor) {
	t.Helper()
	manager, _ := productVNextTestManager(t)
	graphExec := &fakeGraphExecutor{}
	resolver := &multiHostScopeResolver{
		byHost: map[string][]string{
			"www.youtube.com":         {"198.51.100.1"},
			"youtubei.googleapis.com": {"198.51.100.2"},
			"i.ytimg.com":             {"198.51.100.3"},
			"discord.com":             {"198.51.100.11"},
			"gateway.discord.gg":      {"198.51.100.12"},
			"cdn.discordapp.com":      {"198.51.100.13"},
		},
	}
	obs := &serviceGraphDynamicObserver{graphExec: graphExec, alwaysHealthy: alwaysHealthy}

	app := NewApp()
	app.ctx = context.Background()
	app.manager = manager
	app.assets = &engine.AssetPaths{}
	app.newVNextService = func(m *providers.ProviderManager, a *engine.AssetPaths) *productVNextService {
		return newProductVNextServiceWith(m, a, productVNextDependencies{
			newRuntime: func(autotunevnext.RuntimeProvider, *engine.AssetPaths, func(autotunevnext.PhysicalLog)) (productVNextRuntime, error) {
				return productVNextRuntime{executor: &productVNextNoopExecutor{}, preflight: productVNextSupportedPreflight{}, backend: backendcap.Zapret2Windows}, nil
			},
			newGraphRuntime: func(autotunevnext.RuntimeProvider, *engine.AssetPaths, func(autotunevnext.PhysicalLog)) (productVNextGraphRuntime, error) {
				return productVNextGraphRuntime{
					executor:  &productGraphExecutorAdapter{inner: graphExec, backend: backendcap.Zapret2Windows},
					preflight: productVNextSupportedPreflight{},
					backend:   backendcap.Zapret2Windows,
				}, nil
			},
			newResolver: func(*engine.AssetPaths) (autotunevnext.AssetResolver, error) {
				return managedVNextRequestedAssets{}, nil
			},
			scopeResolver: resolver,
			observer:      obs,
			run:           autotunevnext.RunCoordinated,
		})
	}
	return app, graphExec
}

func TestAutoTuneVNextServicePresetsAndExecutionLifecycle(t *testing.T) {
	restoreDir := engine.SetConfigDirForTest(t.TempDir())
	defer restoreDir()

	app, graphExec := newServiceGraphTestApp(t, false)

	// 1. Verify presets contract
	presets := app.GetAutoTuneVNextServicePresets()
	if len(presets) != 2 {
		t.Fatalf("presets len=%d, want 2", len(presets))
	}
	if presets[0].ID != "youtube" || !slices.Equal(presets[0].Hosts, []string{"www.youtube.com", "youtubei.googleapis.com", "i.ytimg.com"}) {
		t.Fatalf("youtube preset=%+v", presets[0])
	}
	if presets[1].ID != "discord" || !slices.Equal(presets[1].Hosts, []string{"discord.com", "gateway.discord.gg", "cdn.discordapp.com"}) {
		t.Fatalf("discord preset=%+v", presets[1])
	}
	for _, p := range presets {
		if len(p.Hosts) > autotunevnext.MaxServiceGraphHosts || p.Label == "" || p.Description == "" {
			t.Fatalf("invalid preset %+v", p)
		}
	}

	// 2. Invalid preset rejected cleanly
	badRes, err := app.AutoTuneVNextServiceGraph("unknown-service")
	if err == nil || badRes.Status != string(autotunevnext.StatusPreflightFailed) || badRes.Applied {
		t.Fatalf("expected invalid preset error, got res=%+v err=%v", badRes, err)
	}

	// 3. Capture live progress events during AutoTuneVNextServiceGraph("youtube")
	prevEmit := appRuntimeEventsEmit
	var progressEvents []VNextProgressEvent
	var emitMu sync.Mutex
	appRuntimeEventsEmit = func(_ context.Context, name string, data ...interface{}) {
		if name == "autotune_vnext_progress" && len(data) == 1 {
			if ev, ok := data[0].(VNextProgressEvent); ok {
				emitMu.Lock()
				progressEvents = append(progressEvents, ev)
				emitMu.Unlock()
			}
		}
	}
	defer func() { appRuntimeEventsEmit = prevEmit }()

	res, err := app.AutoTuneVNextServiceGraph("youtube")
	if err != nil {
		t.Fatalf("AutoTuneVNextServiceGraph(youtube): %v", err)
	}
	if !res.Applied || res.Status != string(autotunevnext.ServiceGraphVerifiedFixed) || res.PresetID != "youtube" {
		t.Fatalf("unexpected graph result: %+v", res)
	}
	if !slices.Equal(res.VerifiedHosts, []string{"www.youtube.com", "youtubei.googleapis.com", "i.ytimg.com"}) {
		t.Fatalf("verified_hosts=%v", res.VerifiedHosts)
	}
	if !graphExec.alive {
		t.Fatal("expected committed graph executor to remain alive after ApplyGraph")
	}

	// Verify exact JSON schema keys
	blob, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"status":`, `"preset_id":"youtube"`, `"hosts":[`, `"verified_hosts":[`, `"applied":true`, `"summary":`, `"errors":[]`} {
		if !strings.Contains(string(blob), key) {
			t.Fatalf("ServiceGraph JSON missing %s: %s", key, blob)
		}
	}

	// Verify all 6 progress stages were emitted in monotonic order
	emitMu.Lock()
	stages := make([]string, len(progressEvents))
	percents := make([]int, len(progressEvents))
	for i, ev := range progressEvents {
		stages[i] = ev.Stage
		percents[i] = ev.Percent
	}
	emitMu.Unlock()
	wantStages := []string{"preflight", "baseline", "diagnosis", "planning", "applying", "complete"}
	wantPercents := []int{10, 25, 40, 55, 80, 100}
	if !slices.Equal(stages, wantStages) || !slices.Equal(percents, wantPercents) {
		t.Fatalf("progress events = %v (%v), want %v (%v)", stages, percents, wantStages, wantPercents)
	}

	// Managed status reflects GRAPH_MANAGED and health monitor evaluates GRAPH_MANAGED as healthy
	managed := app.GetAutoTuneVNextManagedStatus()
	if !managed.Active || managed.State != string(GraphManagedActive) {
		t.Fatalf("managed status=%+v, want GRAPH_MANAGED active", managed)
	}
	if !app.vNextService.ManagedHealthy(context.Background()) {
		t.Fatal("expected active ServiceGraph to report ManagedHealthy=true")
	}

	// 4. RevertAutoTuneVNextServiceGraph stops runtime and clears persisted graph intent
	if err := app.RevertAutoTuneVNextServiceGraph(); err != nil {
		t.Fatalf("RevertAutoTuneVNextServiceGraph: %v", err)
	}
	app.shutdown(context.Background())
	if graphExec.alive {
		t.Fatal("graph executor still alive after RevertAutoTuneVNextServiceGraph")
	}
	if managed := app.GetAutoTuneVNextManagedStatus(); managed.Active || managed.State != "DIRECT" {
		t.Fatalf("managed status after revert=%+v", managed)
	}
	if _, ok, err := loadVNextGraphState(); err != nil || ok {
		t.Fatalf("persisted graph state remained after revert: ok=%v err=%v", ok, err)
	}
}

func TestAutoTuneVNextServiceGraphNoActionNeededWhenDirectlyReachable(t *testing.T) {
	restoreDir := engine.SetConfigDirForTest(t.TempDir())
	defer restoreDir()

	app, graphExec := newServiceGraphTestApp(t, true)
	res, err := app.AutoTuneVNextServiceGraph("discord")
	if err != nil {
		t.Fatalf("AutoTuneVNextServiceGraph(discord): %v", err)
	}
	if res.Status != string(autotunevnext.StatusCompletedNoActionNeeded) || res.Applied || len(res.VerifiedHosts) != 3 {
		t.Fatalf("unexpected direct-healthy graph result: %+v", res)
	}
	if graphExec.alive {
		t.Fatal("direct-healthy graph should not leave executor active")
	}
}

func TestCancelAutoTuneVNextReturnsBooleanAndCancelsSingleAndGraphRuns(t *testing.T) {
	restoreDir := engine.SetConfigDirForTest(t.TempDir())
	defer restoreDir()

	// 1. Idle returns false
	idleApp := NewApp()
	if idleApp.CancelAutoTuneVNext() {
		t.Fatal("CancelAutoTuneVNext returned true while idle")
	}

	// 2. Cancels in-flight single-host AutoTuneVNext and returns true
	singleApp, executor, started := productVNextTestApp(t)
	singleDone := make(chan AutoTuneVNextResult, 1)
	go func() {
		singleDone <- singleApp.AutoTuneVNext("https://target.test/", nil)
	}()
	waitForProductVNextStart(t, started)
	if !singleApp.CancelAutoTuneVNext() {
		t.Fatal("CancelAutoTuneVNext returned false during active single-host vNext run")
	}
	if singleApp.CancelAutoTuneVNext() {
		t.Fatal("second CancelAutoTuneVNext returned true after already cancelling")
	}
	singleRes := <-singleDone
	if singleRes.Status != string(autotunevnext.StatusCancelled) || !singleRes.StateRestored {
		t.Fatalf("cancelled single-host result=%+v", singleRes)
	}
	assertProductVNextRestored(t, executor)

	// 3. Cancels in-flight ServiceGraph run and returns true
	graphStarted := make(chan struct{}, 1)
	graphApp, _ := newServiceGraphTestApp(t, false)
	origFactory := graphApp.newVNextService
	graphApp.newVNextService = func(m *providers.ProviderManager, a *engine.AssetPaths) *productVNextService {
		svc := origFactory(m, a)
		svc.deps.scopeResolver = &multiHostScopeResolver{
			resolve: func(ctx context.Context, _ autotunevnext.Target) (autotunevnext.ServiceScopeSnapshot, error) {
				select {
				case graphStarted <- struct{}{}:
				default:
				}
				<-ctx.Done()
				return autotunevnext.ServiceScopeSnapshot{}, ctx.Err()
			},
		}
		return svc
	}

	graphDone := make(chan VNextServiceGraphResult, 1)
	go func() {
		res, _ := graphApp.AutoTuneVNextServiceGraph("youtube")
		graphDone <- res
	}()
	waitForProductVNextStart(t, graphStarted)
	if !graphApp.CancelAutoTuneVNext() {
		t.Fatal("CancelAutoTuneVNext returned false during active ServiceGraph run")
	}
	graphRes := <-graphDone
	if graphRes.Status != string(autotunevnext.StatusCancelled) || graphRes.Applied {
		t.Fatalf("cancelled graph result=%+v", graphRes)
	}
}

func TestAutoTuneVNextProgressNilContextNeverPanicsAndEmitsStagesWhenPresent(t *testing.T) {
	restoreDir := engine.SetConfigDirForTest(t.TempDir())
	defer restoreDir()

	// 1. Nil ctx safety: must never panic
	nilCtxApp := NewApp()
	nilCtxApp.ctx = nil
	res := nilCtxApp.AutoTuneVNext("https://target.test/", nil)
	if res.Status != string(autotunevnext.StatusPreflightFailed) {
		t.Fatalf("expected preflight failure without assets, got %+v", res)
	}

	// 2. Single-host full run emits preflight -> baseline -> diagnosis -> planning -> applying -> complete
	prevEmit := appRuntimeEventsEmit
	var events []VNextProgressEvent
	appRuntimeEventsEmit = func(_ context.Context, name string, data ...interface{}) {
		if name == "autotune_vnext_progress" && len(data) == 1 {
			if ev, ok := data[0].(VNextProgressEvent); ok {
				events = append(events, ev)
			}
		}
	}
	defer func() { appRuntimeEventsEmit = prevEmit }()

	manager, _ := productVNextTestManager(t)
	exec := &productVNextNoopExecutor{}
	obs := &serviceScopeLifecycleObserver{executor: exec, target: "https://target.test/"}
	app := NewApp()
	app.ctx = context.Background()
	app.manager = manager
	app.assets = &engine.AssetPaths{}
	app.newVNextService = func(m *providers.ProviderManager, a *engine.AssetPaths) *productVNextService {
		return newProductVNextServiceWith(m, a, productVNextDependencies{
			newRuntime: func(autotunevnext.RuntimeProvider, *engine.AssetPaths, func(autotunevnext.PhysicalLog)) (productVNextRuntime, error) {
				return productVNextRuntime{executor: exec, preflight: productVNextSupportedPreflight{}, backend: backendcap.Zapret2Windows}, nil
			},
			newResolver: func(*engine.AssetPaths) (autotunevnext.AssetResolver, error) {
				return managedVNextRequestedAssets{}, nil
			},
			scopeResolver: &managedVNextScopeResolver{
				scope: serviceScopeForManagedTarget(autotunevnext.Target{URL: "https://target.test/", Transport: observatory.TransportTCP, AddressFamily: observatory.AddressFamilyAny}, "192.0.2.1"),
			},
			observer: obs,
			run:      autotunevnext.RunCoordinated,
		})
	}

	runRes := app.AutoTuneVNext("https://target.test/", []string{"https://cloudflare.com/cdn-cgi/trace"})
	if runRes.Status != string(autotunevnext.StatusCompletedSelected) {
		t.Fatalf("expected COMPLETED_SELECTED, got %+v", runRes)
	}
	stages := make([]string, len(events))
	for i, ev := range events {
		stages[i] = ev.Stage
	}
	wantStages := []string{"preflight", "baseline", "diagnosis", "planning", "applying", "complete"}
	if !slices.Equal(stages, wantStages) {
		t.Fatalf("single-host progress stages = %v, want %v", stages, wantStages)
	}
}

func TestValidateAutoTuneVNextCLIOptions(t *testing.T) {
	if err := validateAutoTuneVNextCLIOptions(false, true, "", "youtube"); err != nil {
		t.Fatalf("youtube preset rejected: %v", err)
	}
	if err := validateAutoTuneVNextCLIOptions(false, true, "", "discord"); err != nil {
		t.Fatalf("discord preset rejected: %v", err)
	}
	if err := validateAutoTuneVNextCLIOptions(false, true, "https://example.com", ""); err != nil {
		t.Fatalf("explicit target rejected: %v", err)
	}
	for _, tc := range []struct {
		name            string
		autoTune, vNext bool
		target, preset  string
	}{
		{"both autotune modes", true, true, "https://example.com", ""},
		{"neither target nor preset", false, true, "", ""},
		{"both target and preset", false, true, "https://example.com", "youtube"},
		{"preset without vnext flag", false, false, "", "youtube"},
		{"unknown preset", false, true, "", "netflix"},
	} {
		if err := validateAutoTuneVNextCLIOptions(tc.autoTune, tc.vNext, tc.target, tc.preset); err == nil {
			t.Errorf("%s: expected validation error, got nil", tc.name)
		}
	}
}

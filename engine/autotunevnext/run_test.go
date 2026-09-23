package autotunevnext

import (
	"context"
	"errors"
	"net"
	"slices"
	"testing"
	"time"

	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/planner"
	"unbound/engine/strategyir"
)

var testStart = time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

func tlsStrategy(id string) strategyir.Strategy {
	position := 1
	for _, runeValue := range id {
		position += int(runeValue)
	}
	return strategyir.Strategy{SchemaVersion: strategyir.SchemaVersion, ID: id, Name: id, Transport: []strategyir.Transport{strategyir.TransportTCP}, Selector: strategyir.TrafficSelector{ApplicationProtocols: []strategyir.ApplicationProtocol{strategyir.ApplicationTLS}, IPFamilies: []strategyir.IPFamily{strategyir.IPFamilyV4}, Direction: strategyir.DirectionOutbound, TCPPorts: []strategyir.PortRange{{Start: 443, End: 443}}, Scope: strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeExplicit, Hosts: []string{"blocked.test"}}}}, Operations: []strategyir.Operation{{Type: strategyir.OperationMultiSplit, Positions: []strategyir.PositionExpr{{Absolute: &position}}}}, Safety: strategyir.SafetyPolicy{Aggressiveness: "LOW", TargetOnly: true}}
}

func observation(id string, ok bool, ip string, target string) observatory.ObservationResult {
	family := observatory.AddressFamilyIPv4
	stages := []observatory.StageEvidence{{Stage: observatory.StageResolve, Status: observatory.StatusPass, StartedAt: testStart}, {Stage: observatory.StageConnect, Status: observatory.StatusPass, StartedAt: testStart}, {Stage: observatory.StageHello, Status: observatory.StatusPass, StartedAt: testStart}}
	boundary, class := observatory.StageHandshake, observatory.ClassTLSHandshakeTimeout
	if ok {
		stages = append(stages, observatory.StageEvidence{Stage: observatory.StageHandshake, Status: observatory.StatusPass, StartedAt: testStart}, observatory.StageEvidence{Stage: observatory.StageHTTP, Status: observatory.StatusPass, StartedAt: testStart, HTTPStatus: 200, PathComplete: true})
		boundary, class = observatory.StageHTTP, observatory.ClassSuccess
	} else {
		stages = append(stages, observatory.StageEvidence{Stage: observatory.StageHandshake, Status: observatory.StatusTimeout, StartedAt: testStart, Class: class})
	}
	primary := 0
	return observatory.ObservationResult{SchemaVersion: observatory.SchemaVersion, RunID: id, StartedAt: testStart, FinishedAt: testStart.Add(time.Second), Platform: "test/amd64", NetworkContext: observatory.NetworkContext{NetworkLabel: "test-network", AddressFamily: family}, Target: observatory.Target{URL: target, Hostname: "blocked.test", Port: "443", RequestedProtocol: observatory.TransportTCP}, Attempts: []observatory.ConnectionAttempt{{ResolvedIP: ip, AddressFamily: family, Transport: observatory.TransportTCP, Stages: stages}}, PrimaryAttemptIndex: &primary, FinalBoundary: boundary, Classification: class}
}

func httpFailure(id string) observatory.ObservationResult {
	result := observation(id, true, "192.0.2.1", "https://blocked.test/")
	result.Attempts[0].Stages[len(result.Attempts[0].Stages)-1].HTTPStatus = 451
	result.Classification = observatory.ClassHTTPStatus
	return result
}

type fakeObserver struct {
	results []observatory.ObservationResult
	calls   []observatory.Options
	errAt   int
	cancel  context.CancelFunc
	block   bool
}

func (f *fakeObserver) Observe(ctx context.Context, _ string, options observatory.Options) (observatory.ObservationResult, error) {
	f.calls = append(f.calls, options)
	index := len(f.calls) - 1
	if f.cancel != nil && f.errAt == index {
		f.cancel()
		return observatory.ObservationResult{}, context.Canceled
	}
	if f.block && f.errAt == index {
		<-ctx.Done()
		return observatory.ObservationResult{}, ctx.Err()
	}
	if index >= len(f.results) {
		return observatory.ObservationResult{}, errors.New("unexpected observation")
	}
	return f.results[index], nil
}

type fakeExecutor struct {
	calls                                                   []string
	activateErr, verifyActiveErr, deactivateErr, restoreErr error
	snapshot                                                StateSnapshot
	directErrAt                                             int
	directCalls                                             int
	deactivateContextCancelled                              bool
}

func (f *fakeExecutor) Snapshot(context.Context) (StateSnapshot, error) {
	f.calls = append(f.calls, "snapshot")
	return f.snapshot, nil
}
func (f *fakeExecutor) EstablishDirect(context.Context, StateSnapshot) error {
	f.calls = append(f.calls, "direct")
	f.directCalls++
	if f.directErrAt == f.directCalls {
		return errors.New("establish direct")
	}
	return nil
}
func (f *fakeExecutor) Activate(context.Context, ExecutableCandidate) error {
	f.calls = append(f.calls, "activate")
	return f.activateErr
}
func (f *fakeExecutor) VerifyActive(context.Context, ExecutableCandidate) error {
	f.calls = append(f.calls, "verify-active")
	return f.verifyActiveErr
}
func (f *fakeExecutor) Deactivate(ctx context.Context) error {
	f.calls = append(f.calls, "deactivate")
	f.deactivateContextCancelled = ctx.Err() != nil
	return f.deactivateErr
}
func (f *fakeExecutor) Restore(context.Context, StateSnapshot) error {
	f.calls = append(f.calls, "restore")
	return f.restoreErr
}
func (f *fakeExecutor) VerifyRestored(context.Context, StateSnapshot) error {
	f.calls = append(f.calls, "verify-restored")
	return nil
}

type supportedPreflight struct{ status PreflightStatus }

func (p supportedPreflight) Check(context.Context, HostPreflightRequest) HostPreflightResult {
	return HostPreflightResult{Status: p.status}
}

type fakeAssets struct {
	err    error
	assets []ResolvedAsset
}

func (a fakeAssets) Resolve(context.Context, backendcap.Backend, []string) ([]ResolvedAsset, error) {
	return a.assets, a.err
}

func request(strategies ...strategyir.Strategy) Request {
	return Request{Target: Target{URL: "https://blocked.test/", Transport: observatory.TransportTCP, AddressFamily: observatory.AddressFamilyIPv4}, Strategies: strategies, Backend: backendcap.Zapret2Windows, NetworkLabel: "test-network", Policy: Policy{MaxCandidates: 3, MaxDuration: time.Minute, PerObservationTimeout: time.Second}}
}

func TestBaselineGatesDoNotActivate(t *testing.T) {
	for _, tc := range []struct {
		name     string
		baseline observatory.ObservationResult
		want     Status
	}{
		{"no anomaly", observation("baseline", true, "192.0.2.1", "https://blocked.test/"), StatusCompletedNoActionNeeded},
		{"http application failure", httpFailure("baseline"), StatusInconclusive},
	} {
		t.Run(tc.name, func(t *testing.T) {
			executor := &fakeExecutor{}
			result := Run(context.Background(), request(tlsStrategy("tls")), &fakeObserver{results: []observatory.ObservationResult{tc.baseline}}, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
			if result.Status != tc.want {
				t.Fatalf("status=%s", result.Status)
			}
			if slices.Contains(executor.calls, "activate") {
				t.Fatalf("unexpected activation: %v", executor.calls)
			}
			if !result.StateRestored {
				t.Fatal("state was not restored")
			}
		})
	}
}

func TestControlledSameEdgeAttributionAndSelection(t *testing.T) {
	executor := &fakeExecutor{snapshot: StateSnapshot{ID: "running-profile-x"}}
	observer := &fakeObserver{results: []observatory.ObservationResult{observation("baseline", false, "192.0.2.1", "https://blocked.test/"), observation("before", false, "192.0.2.1", "https://blocked.test/"), observation("active", true, "192.0.2.1", "https://blocked.test/"), observation("after", false, "192.0.2.1", "https://blocked.test/")}}
	result := Run(context.Background(), request(tlsStrategy("tls")), observer, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
	if result.Status != StatusCompletedSelected || result.SelectedStrategyID != "tls" || result.Experiments[0].Outcome != OutcomeVerifiedFixed {
		t.Fatalf("result=%#v", result)
	}
	if len(observer.calls) != 4 || observer.calls[1].ResolvedIP.String() != "192.0.2.1" || observer.calls[2].ResolvedIP.String() != "192.0.2.1" || observer.calls[3].ResolvedIP.String() != "192.0.2.1" {
		t.Fatalf("same-edge pinning failed: %#v", observer.calls)
	}
	if !slices.Contains(executor.calls, "restore") || !slices.Contains(executor.calls, "verify-restored") {
		t.Fatalf("missing restoration: %v", executor.calls)
	}
}

func TestNoCandidateCreditForRecoveryRegressionOrDifferentEdge(t *testing.T) {
	cases := []struct {
		name                  string
		before, active, after bool
		activeIP              string
		want                  Outcome
	}{
		{"still failing", false, false, false, "192.0.2.1", OutcomeStillFailing},
		{"direct recovered", false, true, true, "192.0.2.1", OutcomeDirectBecameReachable},
		{"active regression", true, false, false, "192.0.2.1", OutcomeRegressionObserved},
		{"different edge", false, true, false, "192.0.2.2", OutcomeInconclusive},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			observer := &fakeObserver{results: []observatory.ObservationResult{observation("base", false, "192.0.2.1", "https://blocked.test/"), observation("before", tc.before, "192.0.2.1", "https://blocked.test/"), observation("active", tc.active, tc.activeIP, "https://blocked.test/"), observation("after", tc.after, "192.0.2.1", "https://blocked.test/")}}
			result := Run(context.Background(), request(tlsStrategy("tls")), observer, &fakeExecutor{}, supportedPreflight{PreflightSupported}, fakeAssets{})
			if result.Experiments[0].Outcome != tc.want {
				t.Fatalf("outcome=%s result=%#v", result.Experiments[0].Outcome, result)
			}
			if result.SelectedStrategyID != "" {
				t.Fatalf("improper selection: %#v", result)
			}
		})
	}
}

func TestPlannerAndPreflightGatesPreventActivation(t *testing.T) {
	inapplicable := tlsStrategy("http")
	inapplicable.Selector.ApplicationProtocols = []strategyir.ApplicationProtocol{strategyir.ApplicationHTTP}
	inapplicable.Operations = []strategyir.Operation{{Type: strategyir.OperationHTTPHostCase}}
	for _, tc := range []struct {
		name      string
		strategy  strategyir.Strategy
		backend   backendcap.Backend
		preflight PreflightStatus
		assets    error
	}{
		{"structurally inapplicable", inapplicable, backendcap.Zapret2Windows, PreflightSupported, nil},
		{"backend unsupported", tlsStrategy("tls"), backendcap.NativeWindows, PreflightSupported, nil},
		{"host unsupported", tlsStrategy("tls"), backendcap.Zapret2Windows, PreflightUnsupported, nil},
		{"missing logical asset", tlsStrategy("tls"), backendcap.Zapret2Windows, PreflightSupported, ErrMissingAsset},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := request(tc.strategy)
			req.Backend = tc.backend
			executor := &fakeExecutor{}
			result := Run(context.Background(), req, &fakeObserver{results: []observatory.ObservationResult{observation("base", false, "192.0.2.1", "https://blocked.test/")}}, executor, supportedPreflight{tc.preflight}, fakeAssets{err: tc.assets})
			if slices.Contains(executor.calls, "activate") {
				t.Fatalf("activated despite gate: %#v", result)
			}
		})
	}
}

func TestControlRegressionBudgetDuplicateAndSafetyOrdering(t *testing.T) {
	low := tlsStrategy("low")
	broad := tlsStrategy("broad")
	broad.Safety.TargetOnly = false
	high := tlsStrategy("high")
	high.Safety.Aggressiveness = "HIGH"
	control := Target{URL: "https://control.test/", Transport: observatory.TransportTCP, AddressFamily: observatory.AddressFamilyIPv4}
	controlOK := observation("control-base", true, "192.0.2.9", "https://control.test/")
	controlOK.Target.Hostname = "control.test"
	controlBroken := observation("control-active", false, "192.0.2.9", "https://control.test/")
	controlBroken.Target.Hostname = "control.test"
	observer := &fakeObserver{results: []observatory.ObservationResult{observation("base", false, "192.0.2.1", "https://blocked.test/"), controlOK, observation("before", false, "192.0.2.1", "https://blocked.test/"), observation("active", true, "192.0.2.1", "https://blocked.test/"), controlBroken, observation("after", false, "192.0.2.1", "https://blocked.test/")}}
	req := request(low, broad, high, low)
	req.Controls = []Target{control}
	req.Policy.MaxCandidates = 1
	result := Run(context.Background(), req, observer, &fakeExecutor{}, supportedPreflight{PreflightSupported}, fakeAssets{})
	if result.Experiments[0].Outcome != OutcomeRegressionObserved || result.SelectedStrategyID != "" {
		t.Fatalf("control regression was selected: %#v", result)
	}
	if len(result.Experiments) != 3 {
		t.Fatalf("duplicate fingerprint produced experiments: %#v", result.Experiments)
	}
	for _, experiment := range result.Experiments[1:] {
		if experiment.Outcome != OutcomeNotRunBudget {
			t.Fatalf("budget not enforced: %#v", result.Experiments)
		}
	}
}

func TestLifecycleCancellationAndRestoreFailurePrecedence(t *testing.T) {
	t.Run("activation cleanup", func(t *testing.T) {
		executor := &fakeExecutor{activateErr: errors.New("activate")}
		result := Run(context.Background(), request(tlsStrategy("tls")), &fakeObserver{results: []observatory.ObservationResult{observation("base", false, "192.0.2.1", "https://blocked.test/"), observation("before", false, "192.0.2.1", "https://blocked.test/")}}, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
		if result.Status != StatusLifecycleFailed || !slices.Contains(executor.calls, "deactivate") || !result.StateRestored {
			t.Fatalf("lifecycle cleanup=%#v calls=%v", result, executor.calls)
		}
	})
	t.Run("restore failure wins", func(t *testing.T) {
		executor := &fakeExecutor{restoreErr: errors.New("restore")}
		observer := &fakeObserver{results: []observatory.ObservationResult{observation("base", false, "192.0.2.1", "https://blocked.test/"), observation("before", false, "192.0.2.1", "https://blocked.test/"), observation("active", true, "192.0.2.1", "https://blocked.test/"), observation("after", false, "192.0.2.1", "https://blocked.test/")}}
		result := Run(context.Background(), request(tlsStrategy("tls")), observer, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
		if result.Status != StatusStateRestoreFailed || result.SelectedStrategyID != "" {
			t.Fatalf("restore failure left a selected strategy: %#v", result)
		}
	})
	t.Run("cancelled active restores", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		executor := &fakeExecutor{}
		observer := &fakeObserver{results: []observatory.ObservationResult{observation("base", false, "192.0.2.1", "https://blocked.test/"), observation("before", false, "192.0.2.1", "https://blocked.test/")}, errAt: 2, cancel: cancel}
		result := Run(ctx, request(tlsStrategy("tls")), observer, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
		if result.Status != StatusCancelled || !result.StateRestored || !slices.Contains(executor.calls, "deactivate") {
			t.Fatalf("cancellation=%#v calls=%v", result, executor.calls)
		}
	})
}

func TestExactPlatformExecutionBoundaries(t *testing.T) {
	strategy := tlsStrategy("tls")
	linux := backendcap.Compile(strategy, backendcap.Zapret2Linux)
	windows := backendcap.Compile(strategy, backendcap.Zapret2Windows)
	linuxPlan, err := NewLinuxExactPlan(ExecutableCandidate{Plan: linux.Plan})
	if err != nil || len(linuxPlan.EngineArgv) == 0 {
		t.Fatalf("linux exact plan: %v %#v", err, linuxPlan)
	}
	for _, arg := range linuxPlan.EngineArgv {
		if len(arg) >= 5 && arg[:5] == "--wf-" {
			t.Fatal("Linux argv leaked capture")
		}
	}
	windowsPlan, err := NewWindowsExactPlan(ExecutableCandidate{Plan: windows.Plan})
	if err != nil || len(windowsPlan.CaptureArgv) == 0 {
		t.Fatalf("windows exact plan: %v %#v", err, windowsPlan)
	}
	if slices.Contains(windowsPlan.CaptureArgv, "--wf-udp-out=443,50000-65535") {
		t.Fatalf("legacy default capture inserted: %v", windowsPlan.CaptureArgv)
	}
	if got := MacOSMeasurementPath(backendcap.Zapret1TPWSDarwin); got.Status != PreflightUnsupported || got.Reasons[0].Code != "MEASUREMENT_PATH_UNSUPPORTED" {
		t.Fatalf("mac measurement=%#v", got)
	}
}

func TestCompilerPlannerMismatchFailsClosed(t *testing.T) {
	strategy := tlsStrategy("tls")
	compiled := backendcap.Compile(strategy, backendcap.Zapret2Windows)
	assessment := planner.CandidateAssessment{StrategyID: strategy.ID, StrategyFingerprint: "different", Status: planner.StatusEligible, Safety: strategy.Safety, CompileStatus: backendcap.StatusCompiled}
	experiment := runCandidate(context.Background(), request(strategy), DefaultPolicy(), &fakeObserver{}, &fakeExecutor{}, supportedPreflight{PreflightSupported}, fakeAssets{}, StateSnapshot{}, net.ParseIP("192.0.2.1"), nil, candidateInput{assessment: assessment, strategy: strategy})
	if compiled.Status != backendcap.StatusCompiled || experiment.RejectionReasons[0].Code != "COMPILER_PLANNER_MISMATCH" {
		t.Fatalf("mismatch=%#v", experiment)
	}
}

func TestLifecycleFailureStopsLaterCandidatesAndClosesOnce(t *testing.T) {
	executor := &fakeExecutor{verifyActiveErr: errors.New("verify")}
	result := Run(context.Background(), request(tlsStrategy("first"), tlsStrategy("second")), &fakeObserver{results: []observatory.ObservationResult{
		observation("base", false, "192.0.2.1", "https://blocked.test/"),
		observation("before-first", false, "192.0.2.1", "https://blocked.test/"),
	}}, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
	if result.Status != StatusLifecycleFailed || countCalls(executor.calls, "activate") != 1 || countCalls(executor.calls, "deactivate") != 1 {
		t.Fatalf("lifecycle failure continued or duplicated teardown: result=%#v calls=%v", result, executor.calls)
	}
	if len(result.Experiments) != 1 || !result.StateRestored {
		t.Fatalf("terminal lifecycle restoration=%#v", result)
	}
}

func TestDirectReestablishmentFailureIsTerminal(t *testing.T) {
	executor := &fakeExecutor{directErrAt: 2}
	result := Run(context.Background(), request(tlsStrategy("first"), tlsStrategy("second")), &fakeObserver{results: []observatory.ObservationResult{
		observation("base", false, "192.0.2.1", "https://blocked.test/"),
		observation("before-first", false, "192.0.2.1", "https://blocked.test/"),
		observation("active-first", true, "192.0.2.1", "https://blocked.test/"),
	}}, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
	if result.Status != StatusLifecycleFailed || countCalls(executor.calls, "activate") != 1 || !result.StateRestored {
		t.Fatalf("direct re-establishment failure was not terminal: result=%#v calls=%v", result, executor.calls)
	}
}

func TestCandidateCleanupFailureIsTerminal(t *testing.T) {
	cases := []struct {
		name     string
		executor *fakeExecutor
		results  []observatory.ObservationResult
	}{
		{
			name:     "active observation failure",
			executor: &fakeExecutor{deactivateErr: errors.New("deactivate")},
			results: []observatory.ObservationResult{
				observation("base", false, "192.0.2.1", "https://blocked.test/"),
				observation("before", false, "192.0.2.1", "https://blocked.test/"),
			},
		},
		{
			name:     "active verification failure",
			executor: &fakeExecutor{verifyActiveErr: errors.New("verify"), deactivateErr: errors.New("deactivate")},
			results: []observatory.ObservationResult{
				observation("base", false, "192.0.2.1", "https://blocked.test/"),
				observation("before", false, "192.0.2.1", "https://blocked.test/"),
			},
		},
		{
			name:     "pinned edge mismatch",
			executor: &fakeExecutor{deactivateErr: errors.New("deactivate")},
			results: []observatory.ObservationResult{
				observation("base", false, "192.0.2.1", "https://blocked.test/"),
				observation("before", false, "192.0.2.1", "https://blocked.test/"),
				observation("active", true, "192.0.2.2", "https://blocked.test/"),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := Run(context.Background(), request(tlsStrategy("tls")), &fakeObserver{results: tc.results}, tc.executor, supportedPreflight{PreflightSupported}, fakeAssets{})
			if result.Status != StatusLifecycleFailed || countCalls(tc.executor.calls, "deactivate") != 1 || !result.StateRestored {
				t.Fatalf("cleanup failure not terminal: result=%#v calls=%v", result, tc.executor.calls)
			}
		})
	}
}

func TestCancellationUsesUncancelledCleanupAndRestore(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	executor := &fakeExecutor{}
	observer := &fakeObserver{results: []observatory.ObservationResult{
		observation("base", false, "192.0.2.1", "https://blocked.test/"),
		observation("before", false, "192.0.2.1", "https://blocked.test/"),
	}, errAt: 2, cancel: cancel}
	result := Run(ctx, request(tlsStrategy("tls")), observer, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
	if result.Status != StatusCancelled || !result.StateRestored || executor.deactivateContextCancelled || !slices.Contains(executor.calls, "verify-restored") {
		t.Fatalf("cancelled cleanup did not use independent context: result=%#v calls=%v", result, executor.calls)
	}
}

func TestRestoreFailureOverridesLifecycleFailure(t *testing.T) {
	executor := &fakeExecutor{verifyActiveErr: errors.New("verify"), restoreErr: errors.New("restore")}
	result := Run(context.Background(), request(tlsStrategy("tls")), &fakeObserver{results: []observatory.ObservationResult{
		observation("base", false, "192.0.2.1", "https://blocked.test/"),
		observation("before", false, "192.0.2.1", "https://blocked.test/"),
	}}, executor, supportedPreflight{PreflightSupported}, fakeAssets{})
	if result.Status != StatusStateRestoreFailed || result.SelectedStrategyID != "" || result.SelectedFingerprint != "" || result.SelectionReason != "" {
		t.Fatalf("restore did not override lifecycle failure: %#v", result)
	}
}

func TestMaterializeEngineArgvAssetsAndRejectInvalidValues(t *testing.T) {
	assets := []ResolvedAsset{
		{ID: "blob", Kind: AssetKindBlobSymbol, EngineValue: "bundle:tls"},
		{ID: "hosts", Kind: AssetKindHostlistFile, EngineValue: "C:/managed/hosts.txt"},
		{ID: "ips", Kind: AssetKindIPSetFile, EngineValue: "C:/managed/ips.txt"},
	}
	argv, err := MaterializeEngineArgv([]string{
		"--lua-desync=fake:blob=${asset:blob}",
		"--hostlist=${asset:hosts}",
		"--ipset=${asset:ips}",
	}, assets)
	if err != nil || !slices.Equal(argv, []string{"--lua-desync=fake:blob=bundle:tls", "--hostlist=C:/managed/hosts.txt", "--ipset=C:/managed/ips.txt"}) {
		t.Fatalf("materialized argv=%v err=%v", argv, err)
	}
	for _, tc := range []struct {
		name string
		argv []string
		set  []ResolvedAsset
	}{
		{"missing", []string{"--hostlist=${asset:missing}"}, assets},
		{"wrong kind", []string{"--hostlist=${asset:blob}"}, assets},
		{"malformed", []string{"--hostlist=${asset:hosts"}, assets},
		{"conflicting duplicate", []string{"--hostlist=${asset:hosts}"}, []ResolvedAsset{{ID: "hosts", Kind: AssetKindHostlistFile, EngineValue: "a"}, {ID: "hosts", Kind: AssetKindHostlistFile, EngineValue: "b"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := MaterializeEngineArgv(tc.argv, tc.set); err == nil {
				t.Fatal("invalid asset materialization succeeded")
			}
		})
	}
}

func TestExactPlansRejectUnresolvedAssetsAndLinuxCaptureFlags(t *testing.T) {
	windows := backendcap.Compile(tlsStrategy("tls"), backendcap.Zapret2Windows)
	linux := backendcap.Compile(tlsStrategy("tls"), backendcap.Zapret2Linux)
	if _, err := NewWindowsExactPlan(ExecutableCandidate{Plan: backendcap.Plan{EngineArgv: []string{"--hostlist=${asset:hosts}"}, Capture: windows.Plan.Capture}}); err == nil {
		t.Fatal("Windows exact plan accepted unresolved asset")
	}
	if _, err := NewLinuxExactPlan(ExecutableCandidate{Plan: backendcap.Plan{EngineArgv: []string{"--hostlist=${asset:hosts}"}, Capture: linux.Plan.Capture}}); err == nil {
		t.Fatal("Linux exact plan accepted unresolved asset")
	}
	if _, err := NewLinuxExactPlan(ExecutableCandidate{Plan: backendcap.Plan{EngineArgv: []string{"--wf-tcp-out=443"}, Capture: linux.Plan.Capture}}); err == nil {
		t.Fatal("Linux exact plan accepted capture argv")
	}
	wantCapture, err := backendcap.RenderWindowsCaptureArgv(windows.Plan.Capture)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := NewWindowsExactPlan(ExecutableCandidate{Plan: windows.Plan})
	if err != nil || !slices.Equal(plan.CaptureArgv, wantCapture) {
		t.Fatalf("Windows capture changed: plan=%#v want=%v err=%v", plan, wantCapture, err)
	}
}

func TestWrongResolvedAssetKindFailsPreflight(t *testing.T) {
	strategy := tlsStrategy("asset-kind")
	strategy.Operations = []strategyir.Operation{{Type: strategyir.OperationFakeInjection, PayloadRef: "tls-clienthello-default"}}
	result := Run(context.Background(), request(strategy), &fakeObserver{results: []observatory.ObservationResult{
		observation("base", false, "192.0.2.1", "https://blocked.test/"),
	}}, &fakeExecutor{}, supportedPreflight{PreflightSupported}, fakeAssets{assets: []ResolvedAsset{{ID: "tls-clienthello-default", Kind: AssetKindHostlistFile, EngineValue: "C:/managed/hosts.txt"}}})
	if result.Status != StatusPreflightFailed || len(result.Experiments) != 1 || result.Experiments[0].RejectionReasons[0].Code != "INVALID_RESOLVED_ASSET" {
		t.Fatalf("wrong resolved kind did not fail closed: %#v", result)
	}
}

func TestBaselineEdgeIsPlannerScopeAndPinnedPrimaryIsStrict(t *testing.T) {
	ipset := tlsStrategy("ipset")
	ipset.Selector.Scope.Host = strategyir.HostScope{Mode: strategyir.HostScopeIPSetReference, ID: "target-set"}
	req := request(ipset)
	req.ScopeSnapshot.IPSetMembers = map[string][]string{"target-set": {"192.0.2.0/24"}}
	result := Run(context.Background(), req, &fakeObserver{results: []observatory.ObservationResult{
		observation("base", false, "192.0.2.1", "https://blocked.test/"),
	}}, &fakeExecutor{}, supportedPreflight{PreflightSupported}, fakeAssets{})
	if got := result.PlannerReport.Candidates[0].Status; got != planner.StatusEligible {
		t.Fatalf("actual baseline edge did not match IP-set scope: %#v", result.PlannerReport)
	}

	active := observation("active", true, "192.0.2.1", "https://blocked.test/")
	secondary := active.Attempts[0]
	secondary.ResolvedIP = "192.0.2.2"
	active.Attempts = append(active.Attempts, secondary)
	strict := Run(context.Background(), request(tlsStrategy("strict")), &fakeObserver{results: []observatory.ObservationResult{
		observation("base", false, "192.0.2.2", "https://blocked.test/"),
		observation("before", false, "192.0.2.2", "https://blocked.test/"),
		active,
	}}, &fakeExecutor{}, supportedPreflight{PreflightSupported}, fakeAssets{})
	if strict.Experiments[0].Outcome != OutcomeInconclusive {
		t.Fatalf("secondary non-primary edge incorrectly satisfied pin: %#v", strict.Experiments[0])
	}
}

type sequencePreflight struct{ calls int }

func (p *sequencePreflight) Check(context.Context, HostPreflightRequest) HostPreflightResult {
	p.calls++
	if p.calls == 1 {
		return HostPreflightResult{Status: PreflightUnsupported, Reasons: []Reason{{Code: "HOST_UNSUPPORTED"}}}
	}
	return HostPreflightResult{Status: PreflightSupported}
}

func TestPreflightRejectedCandidateDoesNotConsumeExecutionBudget(t *testing.T) {
	preflight := &sequencePreflight{}
	req := request(tlsStrategy("first"), tlsStrategy("second"))
	req.Policy.MaxCandidates = 1
	result := Run(context.Background(), req, &fakeObserver{results: []observatory.ObservationResult{
		observation("base", false, "192.0.2.1", "https://blocked.test/"),
		observation("before-second", false, "192.0.2.1", "https://blocked.test/"),
		observation("active-second", true, "192.0.2.1", "https://blocked.test/"),
		observation("after-second", false, "192.0.2.1", "https://blocked.test/"),
	}}, &fakeExecutor{}, preflight, fakeAssets{})
	if len(result.Experiments) != 2 || result.Experiments[0].ExperimentExecuted || !result.Experiments[1].ExperimentExecuted || result.Status != StatusCompletedSelected {
		t.Fatalf("preflight rejection consumed network budget: %#v", result)
	}
}

func countCalls(calls []string, wanted string) int {
	count := 0
	for _, call := range calls {
		if call == wanted {
			count++
		}
	}
	return count
}

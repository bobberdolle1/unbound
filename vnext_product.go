package main

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
	"unbound/engine"
	"unbound/engine/attribution"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
	"unbound/engine/historyid"
	"unbound/engine/observatory"
	"unbound/engine/providers"
	"unbound/engine/strategyir"
)

const (
	productVNextCatalogStatus            = "READY"
	productVNextCatalogStatusUnavailable = "UNAVAILABLE"
	maxProductionVNextCatalogCandidates  = 6
	productVNextCatalogIdentity          = "production-vnext-catalog-v2"
)

var errVNextMeasurementPathUnsupported = errors.New("MEASUREMENT_PATH_UNSUPPORTED")

// VNextProgressEvent is the live stage payload emitted on Wails event
// "autotune_vnext_progress".
type VNextProgressEvent struct {
	Stage   string `json:"stage"`
	Message string `json:"message"`
	Percent int    `json:"percent"`
}

type vNextProgressTracker struct {
	mu      sync.Mutex
	onEvent func(VNextProgressEvent)
	highest int
}

func newVNextProgressTracker(onEvent func(VNextProgressEvent)) *vNextProgressTracker {
	return &vNextProgressTracker{onEvent: onEvent}
}

func (t *vNextProgressTracker) emit(stage, message string, percent int) {
	if t == nil || t.onEvent == nil {
		return
	}
	t.mu.Lock()
	if percent < t.highest || (percent == t.highest && stage != "complete") {
		t.mu.Unlock()
		return
	}
	if percent == 100 && t.highest == 100 {
		t.mu.Unlock()
		return
	}
	t.highest = percent
	cb := t.onEvent
	t.mu.Unlock()
	cb(VNextProgressEvent{Stage: stage, Message: message, Percent: percent})
}

type vNextProgressObserver struct {
	inner            autotunevnext.Observer
	tracker          *vNextProgressTracker
	expectedBaseline int
	mu               sync.Mutex
	observed         int
}

func (o *vNextProgressObserver) Observe(ctx context.Context, rawURL string, opts observatory.Options) (observatory.ObservationResult, error) {
	res, err := o.inner.Observe(ctx, rawURL, opts)
	if err == nil {
		o.mu.Lock()
		o.observed++
		count := o.observed
		o.mu.Unlock()
		if count >= o.expectedBaseline {
			o.tracker.emit("diagnosis", "Анализ сбоя и диагностика маршрута...", 40)
		}
	}
	return res, err
}

type vNextProgressPreflight struct {
	inner   autotunevnext.HostPreflight
	tracker *vNextProgressTracker
}

func (p vNextProgressPreflight) Check(ctx context.Context, req autotunevnext.HostPreflightRequest) autotunevnext.HostPreflightResult {
	p.tracker.emit("planning", "Подбор и проверка совместимости стратегий...", 55)
	return p.inner.Check(ctx, req)
}

type vNextProgressExecutor struct {
	inner   autotunevnext.Executor
	tracker *vNextProgressTracker
}

func (e vNextProgressExecutor) Snapshot(ctx context.Context) (autotunevnext.StateSnapshot, error) {
	return e.inner.Snapshot(ctx)
}

func (e vNextProgressExecutor) EstablishDirect(ctx context.Context, snapshot autotunevnext.StateSnapshot) error {
	return e.inner.EstablishDirect(ctx, snapshot)
}

func (e vNextProgressExecutor) Activate(ctx context.Context, candidate autotunevnext.ExecutableCandidate) error {
	e.tracker.emit("applying", "Проверка и валидация стратегии: "+candidate.Strategy.ID, 80)
	return e.inner.Activate(ctx, candidate)
}

func (e vNextProgressExecutor) VerifyActive(ctx context.Context, candidate autotunevnext.ExecutableCandidate) error {
	return e.inner.VerifyActive(ctx, candidate)
}

func (e vNextProgressExecutor) Deactivate(ctx context.Context) error {
	return e.inner.Deactivate(ctx)
}

func (e vNextProgressExecutor) Restore(ctx context.Context, snapshot autotunevnext.StateSnapshot) error {
	return e.inner.Restore(ctx, snapshot)
}

func (e vNextProgressExecutor) VerifyRestored(ctx context.Context, snapshot autotunevnext.StateSnapshot) error {
	return e.inner.VerifyRestored(ctx, snapshot)
}

type AutoTuneVNextRequest struct {
	Target   string   `json:"target"`
	Controls []string `json:"controls,omitempty"`
}

// AutoTuneVNextTargetPreset is a product-owned HTTPS diagnostic target shown
// by the primary vNext flow.
type AutoTuneVNextTargetPreset struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Target string `json:"target"`
}

// AutoTuneVNextExperimentConfig keeps the target and protected-control policy
// in the product layer instead of asking the frontend to construct URLs.
type AutoTuneVNextExperimentConfig struct {
	Targets        []AutoTuneVNextTargetPreset `json:"targets"`
	DefaultControl AutoTuneVNextTargetPreset   `json:"default_control"`
}

var productVNextExperimentTargets = []AutoTuneVNextTargetPreset{
	{ID: "youtube-web", Label: "YouTube", Target: "https://www.youtube.com/generate_204"},
	{ID: "discord-api", Label: "Discord", Target: "https://discord.com/api/v10/gateway"},
	{ID: "steam-store", Label: "Steam", Target: "https://store.steampowered.com/"},
}

var productVNextDefaultControl = AutoTuneVNextTargetPreset{
	ID: "cloudflare-control", Label: "Cloudflare", Target: "https://cloudflare.com/cdn-cgi/trace",
}

func productVNextExperimentConfig() AutoTuneVNextExperimentConfig {
	return AutoTuneVNextExperimentConfig{
		Targets:        append([]AutoTuneVNextTargetPreset(nil), productVNextExperimentTargets...),
		DefaultControl: productVNextDefaultControl,
	}
}

func resolveProductVNextExperimentTarget(presetID, customTarget string) (string, []string, string, error) {
	config := productVNextExperimentConfig()
	control, _, err := normalizeVNextTarget(config.DefaultControl.Target)
	if err != nil {
		return "", nil, "", err
	}

	presetID = strings.TrimSpace(presetID)
	var rawTarget string
	switch presetID {
	case "custom":
		rawTarget = customTarget
	default:
		if strings.TrimSpace(customTarget) != "" {
			return "", nil, "", errors.New("custom target requires custom preset")
		}
		for _, preset := range config.Targets {
			if preset.ID == presetID {
				rawTarget = preset.Target
				break
			}
		}
		if rawTarget == "" {
			return "", nil, "", errors.New("unknown experimental target preset")
		}
	}
	target, publicTarget, err := normalizeVNextTarget(rawTarget)
	if err != nil {
		return "", nil, "", err
	}
	return target.URL, []string{control.URL}, publicTarget, nil
}

type AutoTuneVNextAttribution struct {
	PrimaryFinding string `json:"primary_finding,omitempty"`
	Confidence     string `json:"confidence,omitempty"`
}

type AutoTuneVNextCandidateOutcome struct {
	StrategyID           string   `json:"strategy_id"`
	Fingerprint          string   `json:"fingerprint,omitempty"`
	PlannerStatus        string   `json:"planner_status,omitempty"`
	Outcome              string   `json:"outcome"`
	RecommendationReason []string `json:"recommendation_reason,omitempty"`
}

type AutoTuneVNextDiagnosis struct {
	Kind       string `json:"kind,omitempty"`
	Confidence string `json:"confidence,omitempty"`
}

type AutoTuneVNextRecommendation struct {
	HistoryUsed bool     `json:"history_used"`
	ReasonCodes []string `json:"reason_codes,omitempty"`
}

type AutoTuneVNextLifecycleError struct {
	Code string `json:"code"`
}

// AutoTuneVNextResult is the stable, redacted product-facing result contract.
// It intentionally omits raw observations, headers, query values, and process data.
type AutoTuneVNextResult struct {
	Status              string                          `json:"status"`
	StateRestored       bool                            `json:"state_restored"`
	Target              string                          `json:"target,omitempty"`
	Backend             string                          `json:"backend,omitempty"`
	BaselineAttribution AutoTuneVNextAttribution        `json:"baseline_attribution,omitempty"`
	Diagnosis           AutoTuneVNextDiagnosis          `json:"diagnosis,omitempty"`
	Recommendation      AutoTuneVNextRecommendation     `json:"recommendation,omitempty"`
	PlannerDisposition  string                          `json:"planner_disposition,omitempty"`
	CatalogStatus       string                          `json:"catalog_status"`
	CandidateOutcomes   []AutoTuneVNextCandidateOutcome `json:"candidate_outcomes,omitempty"`
	SelectedStrategyID  string                          `json:"selected_strategy_id,omitempty"`
	SelectedFingerprint string                          `json:"selected_fingerprint,omitempty"`
	ApplyAvailable      bool                            `json:"apply_available"`
	ApplyToken          string                          `json:"apply_token,omitempty"`
	ScopeEdgeCount      int                             `json:"scope_edge_count,omitempty"`
	ScopeStatus         string                          `json:"scope_status,omitempty"`
	ScopeChanged        bool                            `json:"scope_changed,omitempty"`
	Limitations         []string                        `json:"limitations,omitempty"`
	LifecycleErrors     []AutoTuneVNextLifecycleError   `json:"lifecycle_errors,omitempty"`
}

type productVNextRuntime struct {
	executor  autotunevnext.Executor
	preflight autotunevnext.HostPreflight
	backend   backendcap.Backend
}

type productVNextGraphRuntime struct {
	executor  autotunevnext.ServiceGraphExecutor
	preflight autotunevnext.HostPreflight
	backend   backendcap.Backend
}

type productVNextDependencies struct {
	newRuntime      func(autotunevnext.RuntimeProvider, *engine.AssetPaths, func(autotunevnext.PhysicalLog)) (productVNextRuntime, error)
	newGraphRuntime func(autotunevnext.RuntimeProvider, *engine.AssetPaths, func(autotunevnext.PhysicalLog)) (productVNextGraphRuntime, error)
	newResolver     func(*engine.AssetPaths) (autotunevnext.AssetResolver, error)
	scopeResolver   autotunevnext.ScopeResolver
	observer        autotunevnext.Observer
	run             func(context.Context, autotunevnext.Request, autotunevnext.Observer, autotunevnext.Executor, autotunevnext.HostPreflight, autotunevnext.AssetResolver) (autotunevnext.Result, error)
}

type productVNextService struct {
	manager *providers.ProviderManager
	assets  *engine.AssetPaths
	deps    productVNextDependencies

	mu              sync.Mutex
	grants          map[string]verifiedSelectionGrant
	active          *managedVNextActivation
	graphManaged    *graphManagedService
	graphActivation *autotunevnext.ServiceGraphActivation
	health          ManagedHealthState
	fault           AutoTuneVNextManagedStatus
	dormant         AutoTuneVNextManagedStatus
}

func newProductVNextService(manager *providers.ProviderManager, assets *engine.AssetPaths) *productVNextService {
	return newProductVNextServiceWith(manager, assets, productVNextDependencies{
		newRuntime:      newPlatformVNextRuntime,
		newGraphRuntime: newPlatformVNextGraphRuntime,
		newResolver: func(paths *engine.AssetPaths) (autotunevnext.AssetResolver, error) {
			return autotunevnext.NewProductAssetResolver(paths)
		},
		scopeResolver: autotunevnext.DefaultScopeResolver{},
		observer:      observatory.NewDirectTCPHTTPSObserver(),
		run:           autotunevnext.RunCoordinated,
	})
}

func newProductVNextServiceWith(manager *providers.ProviderManager, assets *engine.AssetPaths, deps productVNextDependencies) *productVNextService {
	if deps.newGraphRuntime == nil {
		deps.newGraphRuntime = newPlatformVNextGraphRuntime
	}
	return &productVNextService{manager: manager, assets: assets, deps: deps, grants: make(map[string]verifiedSelectionGrant)}
}

func (s *productVNextService) Run(ctx context.Context, input AutoTuneVNextRequest) AutoTuneVNextResult {
	return s.RunWithProgress(ctx, input, nil)
}

func (s *productVNextService) RunWithProgress(ctx context.Context, input AutoTuneVNextRequest, onProgress func(VNextProgressEvent)) AutoTuneVNextResult {
	tracker := newVNextProgressTracker(onProgress)
	defer tracker.emit("complete", "Проверка завершена.", 100)
	tracker.emit("preflight", "Проверка среды и подготовка ресурсов...", 10)

	s.mu.Lock()
	managedActive := s.active != nil || (s.graphManaged != nil && s.graphManaged.isActiveOrDirty())
	s.mu.Unlock()
	if managedActive {
		return productVNextFailure(autotunevnext.StatusInconclusive, "", "", "MANAGED_ACTIVATION_ACTIVE")
	}
	s.InvalidateGrants()
	target, publicTarget, err := normalizeVNextTarget(input.Target)
	if err != nil {
		return productVNextFailure(autotunevnext.StatusPreflightFailed, "", "", "INVALID_TARGET")
	}
	controls := make([]autotunevnext.Target, 0, len(input.Controls))
	for _, raw := range input.Controls {
		control, _, err := normalizeVNextTarget(raw)
		if err != nil {
			return productVNextFailure(autotunevnext.StatusPreflightFailed, publicTarget, "", "INVALID_CONTROL")
		}
		controls = append(controls, control)
	}
	catalogURL, err := url.Parse(target.URL)
	if err != nil {
		return productVNextFailure(autotunevnext.StatusPreflightFailed, publicTarget, "", "PRODUCT_CATALOG_INVALID")
	}
	catalog, err := productionVNextStrategyCatalog(catalogURL.Hostname())
	if err != nil {
		return productVNextFailure(autotunevnext.StatusPreflightFailed, publicTarget, "", "PRODUCT_CATALOG_INVALID")
	}
	adapter := newProductRuntimeProvider(s.manager)
	if err := adapter.validateRestorableState(); err != nil {
		return productVNextFailureWithCatalog(autotunevnext.StatusPreflightFailed, publicTarget, "", "PRODUCT_STATE_UNCERTAIN", productVNextCatalogStatus)
	}
	runtimeBinding, err := s.deps.newRuntime(adapter, s.assets, logProductVNextPhysicalEvent)
	if err != nil {
		if errors.Is(err, errVNextMeasurementPathUnsupported) {
			return productVNextFailureWithCatalog(autotunevnext.StatusPreflightFailed, publicTarget, "", "MEASUREMENT_PATH_UNSUPPORTED", productVNextCatalogStatus)
		}
		return productVNextFailureWithCatalog(autotunevnext.StatusPreflightFailed, publicTarget, "", "PRODUCT_RUNTIME_UNAVAILABLE", productVNextCatalogStatus)
	}
	resolver, err := s.deps.newResolver(s.assets)
	if err != nil {
		return productVNextFailureWithCatalog(autotunevnext.StatusPreflightFailed, publicTarget, string(runtimeBinding.backend), "PRODUCT_ASSETS_UNAVAILABLE", productVNextCatalogStatus)
	}
	targetProbe, controlProbes, err := productProbeContracts(input.Target, target, controls)
	if err != nil {
		return productVNextFailureWithCatalog(autotunevnext.StatusPreflightFailed, publicTarget, string(runtimeBinding.backend), "PRODUCT_PROBE_CONTRACT_INVALID", productVNextCatalogStatus)
	}
	// One run has ONE coherent history identity cohort. The same bundle is used
	// for the history query below AND for the outcome persistence at the end, so
	// entries are never written under a namespace the query never consulted.
	configDir, _ := engine.GetConfigDir()
	identities := historyid.ForRun(ctx, runtimeBinding.backend, s.assets, configDir)

	history := loadProductOutcomeLedger(identities)
	advisor := productHistoryAdvisor{
		ledger:     history.ledger,
		probe:      targetProbe,
		backend:    runtimeBinding.backend,
		identities: identities,
		now:        time.Now,
		onAdvise: func() {
			tracker.emit("planning", "Подбор и ранжирование стратегий...", 55)
		},
	}
	request := autotunevnext.Request{
		Target:        target,
		Controls:      controls,
		Strategies:    catalog,
		Backend:       runtimeBinding.backend,
		NetworkLabel:  "product-autotune-vnext",
		Policy:        autotunevnext.Policy{MaxCandidates: len(catalog)},
		TargetProbe:   &targetProbe,
		ControlProbes: controlProbes,
		Advisor:       advisor,
	}

	observer := s.deps.observer
	executor := runtimeBinding.executor
	preflight := runtimeBinding.preflight
	if onProgress != nil {
		if observer != nil {
			observer = &vNextProgressObserver{
				inner:            observer,
				tracker:          tracker,
				expectedBaseline: 1 + len(controls),
			}
		}
		if executor != nil {
			executor = vNextProgressExecutor{inner: executor, tracker: tracker}
		}
		if preflight != nil {
			preflight = vNextProgressPreflight{inner: preflight, tracker: tracker}
		}
	}

	tracker.emit("baseline", "Измерение прямого соединения и контрольного узла...", 25)
	var result autotunevnext.Result
	if s.deps.scopeResolver != nil {
		scope, scopeErr := s.deps.scopeResolver.ResolveServiceScope(ctx, target)
		if scopeErr != nil {
			return productVNextFailureWithCatalog(autotunevnext.StatusPreflightFailed, publicTarget, string(runtimeBinding.backend), serviceScopeLimitation(scopeErr), productVNextCatalogStatus)
		}
		result, err = autotunevnext.RunServiceScopeCoordinated(ctx, request, scope, observer, executor, preflight, resolver)
	} else {
		result, err = s.deps.run(ctx, request, observer, executor, preflight, resolver)
	}
	if err != nil {
		return productVNextFailureWithCatalog(autotunevnext.StatusInconclusive, publicTarget, string(runtimeBinding.backend), "OPERATION_CONFLICT", productVNextCatalogStatus)
	}
	mapped := mapAutoTuneVNextResult(result, publicTarget)
	if persistenceLimitation := persistProductOutcomes(result, history, identities); persistenceLimitation != "" {
		mapped.Limitations = append(mapped.Limitations, persistenceLimitation)
	}
	mapped.Limitations = append(mapped.Limitations, history.limitations...)
	if token := s.issueVerifiedGrant(result, target, publicTarget, controls); token != "" {
		mapped.ApplyAvailable = true
		mapped.ApplyToken = token
	}
	return mapped
}

func productVNextFailure(status autotunevnext.Status, target, backend, limitation string) AutoTuneVNextResult {
	return productVNextFailureWithCatalog(status, target, backend, limitation, productVNextCatalogStatusUnavailable)
}

func productVNextFailureWithCatalog(status autotunevnext.Status, target, backend, limitation, catalogStatus string) AutoTuneVNextResult {
	return AutoTuneVNextResult{Status: string(status), Target: target, Backend: backend, CatalogStatus: catalogStatus, Limitations: []string{limitation}}
}

func mapAutoTuneVNextResult(result autotunevnext.Result, publicTarget string) AutoTuneVNextResult {
	mapped := AutoTuneVNextResult{
		Status:              string(result.Status),
		StateRestored:       result.StateRestored,
		Target:              publicTarget,
		Backend:             string(result.Backend),
		BaselineAttribution: mapAttribution(result.BaselineAttribution),
		Diagnosis:           AutoTuneVNextDiagnosis{Kind: string(result.DiagnosisReport.Kind), Confidence: string(result.DiagnosisReport.Confidence)},
		PlannerDisposition:  string(result.PlannerReport.Disposition),
		CatalogStatus:       productVNextCatalogStatus,
		SelectedStrategyID:  result.SelectedStrategyID,
		SelectedFingerprint: result.SelectedFingerprint,
		ScopeEdgeCount:      result.ServiceScope.EdgeCount,
		ScopeStatus:         string(result.ServiceScope.Status),
		ScopeChanged:        result.ServiceScope.Status == autotunevnext.ServiceScopeChanged,
		Limitations:         append([]string(nil), result.Limitations...),
	}
	for _, candidate := range result.Recommendation.Candidates {
		if candidate.HistoryUsed {
			mapped.Recommendation.HistoryUsed = true
		}
		mapped.Recommendation.ReasonCodes = append(mapped.Recommendation.ReasonCodes, candidate.ReasonCodes...)
	}
	for _, experiment := range result.Experiments {
		mapped.CandidateOutcomes = append(mapped.CandidateOutcomes, AutoTuneVNextCandidateOutcome{
			StrategyID: experiment.StrategyID, Fingerprint: experiment.Fingerprint,
			PlannerStatus: string(experiment.PlannerStatus), Outcome: string(experiment.Outcome),
			RecommendationReason: append([]string(nil), experiment.RecommendationReason...),
		})
	}
	for _, lifecycle := range result.Lifecycle.Errors {
		mapped.LifecycleErrors = append(mapped.LifecycleErrors, AutoTuneVNextLifecycleError{Code: lifecycle.Code})
	}
	return mapped
}

func mapAttribution(report attribution.AttributionReport) AutoTuneVNextAttribution {
	return AutoTuneVNextAttribution{PrimaryFinding: string(report.PrimaryFinding.Code), Confidence: string(report.Confidence)}
}

func serviceScopeLimitation(err error) string {
	switch {
	case errors.Is(err, autotunevnext.ErrServiceScopeTooLarge):
		return "SERVICE_SCOPE_TOO_LARGE"
	case errors.Is(err, autotunevnext.ErrServiceScopeEmpty):
		return "SERVICE_SCOPE_EMPTY"
	default:
		return "SERVICE_SCOPE_INVALID"
	}
}

// productionVNextStrategyCatalog is a closed audited catalog. The caller may
// choose only the normalized explicit hostname; all packet semantics remain
// static reviewed product code.
func productionVNextStrategyCatalog(hostname string) ([]strategyir.Strategy, error) {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if err := validateVNextHostname(hostname); err != nil {
		return nil, err
	}
	one, two, overlap := 1, 2, 652
	outboundCutoff := func() *strategyir.Cutoff {
		return &strategyir.Cutoff{
			Direction: strategyir.RangeDirectionOut,
			Counter:   strategyir.RangeCounterDataPacketNumber,
			Limit:     8,
		}
	}
	selector := func() strategyir.TrafficSelector {
		return strategyir.TrafficSelector{
			ApplicationProtocols: []strategyir.ApplicationProtocol{strategyir.ApplicationTLS},
			IPFamilies:           []strategyir.IPFamily{strategyir.IPFamilyAny},
			Direction:            strategyir.DirectionOutbound,
			TCPPorts:             []strategyir.PortRange{{Start: 443, End: 443}},
			Scope:                strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeExplicit, Hosts: []string{hostname}}},
		}
	}
	catalog := []strategyir.Strategy{
		{
			SchemaVersion: strategyir.SchemaVersion, ID: "prod-tls-multisplit-1-v1", Name: "Production TLS multisplit at byte 1",
			Transport: []strategyir.Transport{strategyir.TransportTCP}, Selector: selector(),
			Operations: []strategyir.Operation{{Type: strategyir.OperationMultiSplit, Positions: []strategyir.PositionExpr{{Absolute: &one}}}},
			Safety:     strategyir.SafetyPolicy{Aggressiveness: "LOW", TargetOnly: true},
			Metadata:   strategyir.Metadata{Description: "LOW: one target-local TLS segmentation point without fake payload or overlap.", Source: "Production vNext V1; reviewed target-local TLS multisplit semantic"},
		},
		{
			SchemaVersion: strategyir.SchemaVersion, ID: "prod-tls-sniext-split-v1", Name: "Production TLS SNI record-extension split",
			Transport: []strategyir.Transport{strategyir.TransportTCP}, Selector: selector(),
			Range:      outboundCutoff(),
			Operations: []strategyir.Operation{{Type: strategyir.OperationMultiSplit, Positions: []strategyir.PositionExpr{{Absolute: &one}, {Anchor: strategyir.AnchorSNIExt, Offset: 1}}}},
			Safety:     strategyir.SafetyPolicy{Aggressiveness: "LOW", TargetOnly: true},
			Metadata:   strategyir.Metadata{Description: "LOW: target-local TLS record and SNI-extension boundary segmentation without fake packets.", Source: "Production vNext V2; reviewed target-local TLS SNI-extension split semantic"},
		},
		{
			SchemaVersion: strategyir.SchemaVersion, ID: "prod-tls-multisplit-overlap-v1", Name: "Production TLS overlap multisplit",
			Transport: []strategyir.Transport{strategyir.TransportTCP}, Selector: selector(),
			Range:      outboundCutoff(),
			Operations: []strategyir.Operation{{Type: strategyir.OperationMultiSplit, Positions: []strategyir.PositionExpr{{Absolute: &two}}, SequenceOverlap: &overlap, OverlapPatternRef: "tls-google"}},
			Safety:     strategyir.SafetyPolicy{Aggressiveness: "MEDIUM", TargetOnly: true},
			Metadata:   strategyir.Metadata{Description: "MEDIUM: target-local TLS segmentation with sequence overlap.", Source: "Production vNext V1; reviewed from RepresentativeFixtures alternative-multisplit packet-operation semantics"},
		},
		{
			SchemaVersion: strategyir.SchemaVersion, ID: "prod-tls-hostfakesplit-v1", Name: "Production TLS host fake split",
			Transport: []strategyir.Transport{strategyir.TransportTCP}, Selector: selector(),
			Range:      outboundCutoff(),
			Operations: []strategyir.Operation{{Type: strategyir.OperationHostFakeSplit, Positions: []strategyir.PositionExpr{{Anchor: strategyir.AnchorMidSLD}}, HostTemplate: "ozon.ru", Fake: &strategyir.FakeModifiers{Repeat: 4, TCPMD5: true, TCPTimestamp: true}}},
			Safety:     strategyir.SafetyPolicy{Aggressiveness: "MEDIUM", TargetOnly: true},
			Metadata:   strategyir.Metadata{Description: "MEDIUM: repeated target-local host fake split with TCP modifiers.", Source: "Production vNext V1; reviewed from RepresentativeFixtures recommended-hostfakesplit packet-operation semantics"},
		},
		{
			SchemaVersion: strategyir.SchemaVersion, ID: "prod-tls-multidisorder-midsld-v1", Name: "Production TLS multidisorder at byte 1 and mid-SLD",
			Transport: []strategyir.Transport{strategyir.TransportTCP}, Selector: selector(),
			Range:      outboundCutoff(),
			Operations: []strategyir.Operation{{Type: strategyir.OperationMultiDisorder, Positions: []strategyir.PositionExpr{{Absolute: &one}, {Anchor: strategyir.AnchorMidSLD}}}},
			Safety:     strategyir.SafetyPolicy{Aggressiveness: "MEDIUM", TargetOnly: true},
			Metadata:   strategyir.Metadata{Description: "MEDIUM: target-local out-of-order TLS ClientHello delivery across record header and mid-SLD.", Source: "Production vNext V2; reviewed target-local multidisorder semantic"},
		},
		{
			SchemaVersion: strategyir.SchemaVersion, ID: "prod-tls-fake-multisplit-sni-v1", Name: "Production TLS fake ClientHello + mid-SLD split",
			Transport: []strategyir.Transport{strategyir.TransportTCP}, Selector: selector(),
			Range: outboundCutoff(),
			Operations: []strategyir.Operation{
				{Type: strategyir.OperationFakeInjection, PayloadRef: "tls-clienthello-default", Fake: &strategyir.FakeModifiers{Repeat: 6, TCPMD5: true, TCPTimestamp: true}},
				{Type: strategyir.OperationMultiSplit, Positions: []strategyir.PositionExpr{{Absolute: &one}, {Anchor: strategyir.AnchorMidSLD}}},
			},
			Safety:   strategyir.SafetyPolicy{Aggressiveness: "MEDIUM", TargetOnly: true},
			Metadata: strategyir.Metadata{Description: "MEDIUM: target-local TCP-timestamp/MD5 fake ClientHello paired with mid-SLD segmentation.", Source: "Production vNext V2; reviewed target-local fake+multisplit semantic"},
		},
	}
	if len(catalog) == 0 || len(catalog) > maxProductionVNextCatalogCandidates {
		return nil, errors.New("invalid production catalog size")
	}
	ids := make(map[string]struct{}, len(catalog))
	fingerprints := make(map[string]struct{}, len(catalog))
	for index, strategy := range catalog {
		normalized, err := strategyir.Canonicalize(strategy)
		if err != nil {
			return nil, err
		}
		fingerprint, err := strategyir.Fingerprint(normalized)
		if err != nil {
			return nil, err
		}
		if _, exists := ids[normalized.ID]; exists {
			return nil, errors.New("duplicate production strategy id")
		}
		if _, exists := fingerprints[fingerprint]; exists {
			return nil, errors.New("duplicate production strategy fingerprint")
		}
		ids[normalized.ID] = struct{}{}
		fingerprints[fingerprint] = struct{}{}
		catalog[index] = normalized
	}
	return catalog, nil
}

func normalizeVNextTarget(raw string) (autotunevnext.Target, string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Hostname() == "" {
		return autotunevnext.Target{}, "", errors.New("invalid HTTPS target")
	}
	if err := validateVNextHostname(parsed.Hostname()); err != nil {
		return autotunevnext.Target{}, "", err
	}
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.RawFragment = ""
	parsed.Scheme = "https"
	parsed.Host = strings.ToLower(parsed.Host)
	public := url.URL{Scheme: parsed.Scheme, Host: parsed.Host, Path: parsed.Path}
	if public.Path == "" {
		public.Path = "/"
	}
	return autotunevnext.Target{URL: parsed.String(), Transport: observatory.TransportTCP, AddressFamily: observatory.AddressFamilyAny}, public.String(), nil
}

func validateVNextHostname(host string) error {
	if net.ParseIP(host) != nil {
		return nil
	}
	if len(host) > 253 {
		return errors.New("hostname is too long")
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return errors.New("malformed hostname")
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
				return errors.New("malformed hostname")
			}
		}
	}
	return nil
}

type productRuntimeProvider struct {
	manager *providers.ProviderManager
	mu      sync.Mutex
	engine  string
}

func newProductRuntimeProvider(manager *providers.ProviderManager) *productRuntimeProvider {
	return &productRuntimeProvider{manager: manager}
}

func (p *productRuntimeProvider) CheckPrivileges() (bool, error) { return p.manager.CheckPrivileges() }
func (p *productRuntimeProvider) Stop() error                    { return p.manager.Stop() }
func (p *productRuntimeProvider) GetStatus() providers.Status    { return p.manager.GetStatus() }
func (p *productRuntimeProvider) Name() string {
	if engineName := p.manager.ActiveEngineName(); engineName != "" {
		return engineName
	}
	return "product-manager"
}
func (p *productRuntimeProvider) CurrentProfile() string {
	engineName := p.manager.ActiveEngineName()
	profile := p.manager.ActiveProfileName()
	p.mu.Lock()
	p.engine = engineName
	p.mu.Unlock()
	return profile
}
func (p *productRuntimeProvider) Start(ctx context.Context, profile string) error {
	p.mu.Lock()
	engineName := p.engine
	p.mu.Unlock()
	if engineName == "" || profile == "" {
		return errors.New("original product engine/profile is unavailable")
	}
	return p.manager.Start(ctx, engineName, profile)
}
func (p *productRuntimeProvider) validateRestorableState() error {
	if p.manager == nil {
		return errors.New("product manager is unavailable")
	}
	if p.manager.GetStatus() != providers.StatusRunning {
		return nil
	}
	if p.manager.ActiveEngineName() == "" || p.manager.ActiveProfileName() == "" {
		return errors.New("running product engine has no restorable profile")
	}
	return nil
}

func logProductVNextPhysicalEvent(event autotunevnext.PhysicalLog) {
	engine.GetLogger().Infof("AutoTuneVNext", "backend=%s phase=%s strategy=%s fingerprint=%s edge=%s pid=%d outcome=%s", event.Backend, event.Phase, event.StrategyID, event.Fingerprint, event.Edge, event.PID, event.Outcome)
}

var _ autotunevnext.RuntimeProvider = (*productRuntimeProvider)(nil)

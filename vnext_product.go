package main

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"sync"

	"unbound/engine"
	"unbound/engine/attribution"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/providers"
	"unbound/engine/strategyir"
)

const (
	productVNextCatalogStatus            = "READY"
	productVNextCatalogStatusUnavailable = "UNAVAILABLE"
)

var errVNextMeasurementPathUnsupported = errors.New("MEASUREMENT_PATH_UNSUPPORTED")

type AutoTuneVNextRequest struct {
	Target   string   `json:"target"`
	Controls []string `json:"controls,omitempty"`
}

type AutoTuneVNextAttribution struct {
	PrimaryFinding string `json:"primary_finding,omitempty"`
	Confidence     string `json:"confidence,omitempty"`
}

type AutoTuneVNextCandidateOutcome struct {
	StrategyID    string `json:"strategy_id"`
	Fingerprint   string `json:"fingerprint,omitempty"`
	PlannerStatus string `json:"planner_status,omitempty"`
	Outcome       string `json:"outcome"`
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
	PlannerDisposition  string                          `json:"planner_disposition,omitempty"`
	CatalogStatus       string                          `json:"catalog_status"`
	CandidateOutcomes   []AutoTuneVNextCandidateOutcome `json:"candidate_outcomes,omitempty"`
	SelectedStrategyID  string                          `json:"selected_strategy_id,omitempty"`
	SelectedFingerprint string                          `json:"selected_fingerprint,omitempty"`
	Limitations         []string                        `json:"limitations,omitempty"`
	LifecycleErrors     []AutoTuneVNextLifecycleError   `json:"lifecycle_errors,omitempty"`
}

type productVNextRuntime struct {
	executor  autotunevnext.Executor
	preflight autotunevnext.HostPreflight
	backend   backendcap.Backend
}

type productVNextDependencies struct {
	newRuntime  func(autotunevnext.RuntimeProvider, *engine.AssetPaths, func(autotunevnext.PhysicalLog)) (productVNextRuntime, error)
	newResolver func(*engine.AssetPaths) (autotunevnext.AssetResolver, error)
	observer    autotunevnext.Observer
	run         func(context.Context, autotunevnext.Request, autotunevnext.Observer, autotunevnext.Executor, autotunevnext.HostPreflight, autotunevnext.AssetResolver) (autotunevnext.Result, error)
}

type productVNextService struct {
	manager *providers.ProviderManager
	assets  *engine.AssetPaths
	deps    productVNextDependencies
}

func newProductVNextService(manager *providers.ProviderManager, assets *engine.AssetPaths) *productVNextService {
	return newProductVNextServiceWith(manager, assets, productVNextDependencies{
		newRuntime: newPlatformVNextRuntime,
		newResolver: func(paths *engine.AssetPaths) (autotunevnext.AssetResolver, error) {
			return autotunevnext.NewProductAssetResolver(paths)
		},
		observer: observatory.NewDirectTCPHTTPSObserver(),
		run:      autotunevnext.RunCoordinated,
	})
}

func newProductVNextServiceWith(manager *providers.ProviderManager, assets *engine.AssetPaths, deps productVNextDependencies) *productVNextService {
	return &productVNextService{manager: manager, assets: assets, deps: deps}
}

func (s *productVNextService) Run(ctx context.Context, input AutoTuneVNextRequest) AutoTuneVNextResult {
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
	request := autotunevnext.Request{
		Target:       target,
		Controls:     controls,
		Strategies:   catalog,
		Backend:      runtimeBinding.backend,
		NetworkLabel: "product-autotune-vnext",
	}
	result, err := s.deps.run(ctx, request, s.deps.observer, runtimeBinding.executor, runtimeBinding.preflight, resolver)
	if err != nil {
		return productVNextFailureWithCatalog(autotunevnext.StatusInconclusive, publicTarget, string(runtimeBinding.backend), "OPERATION_CONFLICT", productVNextCatalogStatus)
	}
	return mapAutoTuneVNextResult(result, publicTarget)
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
		PlannerDisposition:  string(result.PlannerReport.Disposition),
		CatalogStatus:       productVNextCatalogStatus,
		SelectedStrategyID:  result.SelectedStrategyID,
		SelectedFingerprint: result.SelectedFingerprint,
		Limitations:         append([]string(nil), result.Limitations...),
	}
	for _, experiment := range result.Experiments {
		mapped.CandidateOutcomes = append(mapped.CandidateOutcomes, AutoTuneVNextCandidateOutcome{
			StrategyID: experiment.StrategyID, Fingerprint: experiment.Fingerprint,
			PlannerStatus: string(experiment.PlannerStatus), Outcome: string(experiment.Outcome),
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

// productionVNextStrategyCatalog is a closed audited catalog. The caller may
// choose only the normalized explicit hostname; all packet semantics remain
// static reviewed product code.
func productionVNextStrategyCatalog(hostname string) ([]strategyir.Strategy, error) {
	hostname = strings.ToLower(strings.TrimSpace(hostname))
	if err := validateVNextHostname(hostname); err != nil {
		return nil, err
	}
	one, two, overlap := 1, 2, 652
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
			SchemaVersion: strategyir.SchemaVersion, ID: "prod-tls-multisplit-overlap-v1", Name: "Production TLS overlap multisplit",
			Transport: []strategyir.Transport{strategyir.TransportTCP}, Selector: selector(),
			Range:      &strategyir.Cutoff{Direction: strategyir.RangeDirectionOut, Counter: strategyir.RangeCounterDataPacketNumber, Limit: 8},
			Operations: []strategyir.Operation{{Type: strategyir.OperationMultiSplit, Positions: []strategyir.PositionExpr{{Absolute: &two}}, SequenceOverlap: &overlap, OverlapPatternRef: "tls-google"}},
			Safety:     strategyir.SafetyPolicy{Aggressiveness: "MEDIUM", TargetOnly: true},
			Metadata:   strategyir.Metadata{Description: "MEDIUM: target-local TLS segmentation with sequence overlap.", Source: "Production vNext V1; reviewed from RepresentativeFixtures alternative-multisplit packet-operation semantics"},
		},
		{
			SchemaVersion: strategyir.SchemaVersion, ID: "prod-tls-hostfakesplit-v1", Name: "Production TLS host fake split",
			Transport: []strategyir.Transport{strategyir.TransportTCP}, Selector: selector(),
			Range:      &strategyir.Cutoff{Direction: strategyir.RangeDirectionOut, Counter: strategyir.RangeCounterDataPacketNumber, Limit: 8},
			Operations: []strategyir.Operation{{Type: strategyir.OperationHostFakeSplit, Positions: []strategyir.PositionExpr{{Anchor: strategyir.AnchorMidSLD}}, HostTemplate: "ozon.ru", Fake: &strategyir.FakeModifiers{Repeat: 4, TCPMD5: true, TCPTimestamp: true}}},
			Safety:     strategyir.SafetyPolicy{Aggressiveness: "MEDIUM", TargetOnly: true},
			Metadata:   strategyir.Metadata{Description: "MEDIUM: repeated target-local host fake split with TCP modifiers.", Source: "Production vNext V1; reviewed from RepresentativeFixtures recommended-hostfakesplit packet-operation semantics"},
		},
	}
	if len(catalog) == 0 || len(catalog) > autotunevnext.DefaultPolicy().MaxCandidates {
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
	parsed, err := url.ParseRequestURI(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Hostname() == "" {
		return autotunevnext.Target{}, "", errors.New("invalid HTTPS target")
	}
	if err := validateVNextHostname(parsed.Hostname()); err != nil {
		return autotunevnext.Target{}, "", err
	}
	parsed.Fragment = ""
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

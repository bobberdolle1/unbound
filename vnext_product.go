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

const productVNextCatalogStatus = "CATALOG_REQUIRED"

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
	adapter := newProductRuntimeProvider(s.manager)
	if err := adapter.validateRestorableState(); err != nil {
		return productVNextFailure(autotunevnext.StatusPreflightFailed, publicTarget, "", "PRODUCT_STATE_UNCERTAIN")
	}
	runtimeBinding, err := s.deps.newRuntime(adapter, s.assets, logProductVNextPhysicalEvent)
	if err != nil {
		if errors.Is(err, errVNextMeasurementPathUnsupported) {
			return productVNextFailure(autotunevnext.StatusPreflightFailed, publicTarget, "", "MEASUREMENT_PATH_UNSUPPORTED")
		}
		return productVNextFailure(autotunevnext.StatusPreflightFailed, publicTarget, "", "PRODUCT_RUNTIME_UNAVAILABLE")
	}
	resolver, err := s.deps.newResolver(s.assets)
	if err != nil {
		return productVNextFailure(autotunevnext.StatusPreflightFailed, publicTarget, string(runtimeBinding.backend), "PRODUCT_ASSETS_UNAVAILABLE")
	}
	request := autotunevnext.Request{
		Target:       target,
		Controls:     controls,
		Strategies:   productionVNextStrategyCatalog(),
		Backend:      runtimeBinding.backend,
		NetworkLabel: "product-autotune-vnext",
	}
	result, err := s.deps.run(ctx, request, s.deps.observer, runtimeBinding.executor, runtimeBinding.preflight, resolver)
	if err != nil {
		return productVNextFailure(autotunevnext.StatusInconclusive, publicTarget, string(runtimeBinding.backend), "OPERATION_CONFLICT")
	}
	mapped := mapAutoTuneVNextResult(result, publicTarget)
	mapped.Limitations = append(mapped.Limitations, productVNextCatalogStatus)
	return mapped
}

func productVNextFailure(status autotunevnext.Status, target, backend, limitation string) AutoTuneVNextResult {
	return AutoTuneVNextResult{Status: string(status), Target: target, Backend: backend, CatalogStatus: productVNextCatalogStatus, Limitations: []string{limitation}}
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

func productionVNextStrategyCatalog() []strategyir.Strategy {
	// No audited product StrategyIR candidates exist yet. In particular, this
	// boundary must never derive candidates from legacy profile names or expose
	// AcceptanceTLSStrategy, which remains developer-only.
	return nil
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

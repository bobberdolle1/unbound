package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"unbound/engine"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
	"unbound/engine/strategyir"
)

// VNextServicePreset describes a bounded multi-host service preset (<= 4 hosts)
// exposed to the Wails UI and CLI.
type VNextServicePreset struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Hosts       []string `json:"hosts"`
}

// VNextServiceGraphResult is the product-facing result returned by
// AutoTuneVNextServiceGraph.
type VNextServiceGraphResult struct {
	Status        string   `json:"status"`
	PresetID      string   `json:"preset_id"`
	Hosts         []string `json:"hosts"`
	VerifiedHosts []string `json:"verified_hosts"`
	Applied       bool     `json:"applied"`
	Summary       string   `json:"summary"`
	Errors        []string `json:"errors"`
}

type productServiceGraphNodeSpec struct {
	NodeID    string
	Role      autotunevnext.ServiceNodeRole
	Required  bool
	Host      string
	TargetURL string
}

type productServiceGraphPresetSpec struct {
	ID          string
	Label       string
	Description string
	Nodes       []productServiceGraphNodeSpec
}

var productVNextServicePresetSpecs = []productServiceGraphPresetSpec{
	{
		ID:          "youtube",
		Label:       "YouTube",
		Description: "Веб-интерфейс, внутренний API и статические ресурсы YouTube",
		Nodes: []productServiceGraphNodeSpec{
			{NodeID: "ENTRY", Role: autotunevnext.ServiceNodeRoleEntry, Required: true, Host: "www.youtube.com", TargetURL: "https://www.youtube.com/generate_204"},
			{NodeID: "API", Role: autotunevnext.ServiceNodeRoleAPI, Required: true, Host: "youtubei.googleapis.com", TargetURL: "https://youtubei.googleapis.com/generate_204"},
			{NodeID: "MEDIA", Role: autotunevnext.ServiceNodeRoleMedia, Required: true, Host: "i.ytimg.com", TargetURL: "https://i.ytimg.com/generate_204"},
		},
	},
	{
		ID:          "discord",
		Label:       "Discord",
		Description: "Основной API, шлюз реального времени и CDN медиа-ресурсов Discord",
		Nodes: []productServiceGraphNodeSpec{
			{NodeID: "ENTRY", Role: autotunevnext.ServiceNodeRoleEntry, Required: true, Host: "discord.com", TargetURL: "https://discord.com/api/v10/gateway"},
			{NodeID: "API", Role: autotunevnext.ServiceNodeRoleAPI, Required: true, Host: "gateway.discord.gg", TargetURL: "https://gateway.discord.gg/cdn-cgi/trace"},
			{NodeID: "MEDIA", Role: autotunevnext.ServiceNodeRoleMedia, Required: true, Host: "cdn.discordapp.com", TargetURL: "https://cdn.discordapp.com/embed/avatars/0.png"},
		},
	},
}

func productVNextServicePresets() []VNextServicePreset {
	out := make([]VNextServicePreset, 0, len(productVNextServicePresetSpecs))
	for _, spec := range productVNextServicePresetSpecs {
		hosts := make([]string, 0, len(spec.Nodes))
		for _, node := range spec.Nodes {
			hosts = append(hosts, node.Host)
		}
		out = append(out, VNextServicePreset{
			ID:          spec.ID,
			Label:       spec.Label,
			Description: spec.Description,
			Hosts:       hosts,
		})
	}
	return out
}

func lookupProductServiceGraphPreset(presetID string) (productServiceGraphPresetSpec, []string, error) {
	normalized := strings.ToLower(strings.TrimSpace(presetID))
	if normalized == "" {
		return productServiceGraphPresetSpec{}, []string{}, errors.New("service preset ID is required")
	}
	for _, spec := range productVNextServicePresetSpecs {
		if spec.ID == normalized {
			if len(spec.Nodes) == 0 || len(spec.Nodes) > autotunevnext.MaxServiceGraphHosts {
				return productServiceGraphPresetSpec{}, []string{}, autotunevnext.ErrServiceGraphTooLarge
			}
			hosts := make([]string, len(spec.Nodes))
			for i, node := range spec.Nodes {
				hosts[i] = node.Host
			}
			return spec, hosts, nil
		}
	}
	return productServiceGraphPresetSpec{}, []string{}, fmt.Errorf("unknown service preset %q", presetID)
}

func (s *graphManagedService) isActiveOrDirty() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state == GraphManagedActive || s.dirty
}

func (s *graphManagedService) currentStatus() AutoTuneVNextGraphManagedStatus {
	if s == nil {
		return AutoTuneVNextGraphManagedStatus{State: GraphManagedDirect}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status.State == "" {
		return AutoTuneVNextGraphManagedStatus{State: s.state}
	}
	return s.status
}

// productGraphExecutorAdapter populates ServiceGraphActivation.Plan from the
// compiled Capture and per-node Sections so both WindowsGraphExecutor and
// LinuxGraphExecutor receive complete exact plans during RunServiceGraphExperiment
// and ApplyGraph.
type productGraphExecutorAdapter struct {
	inner   autotunevnext.ServiceGraphExecutor
	backend backendcap.Backend
}

func (a *productGraphExecutorAdapter) prepareActivation(activation autotunevnext.ServiceGraphActivation) autotunevnext.ServiceGraphActivation {
	if activation.Backend == "" {
		activation.Backend = a.backend
	}
	if activation.Plan.Capture.BackendKind == "" {
		activation.Plan.Capture = activation.Capture
	}
	if activation.Backend == backendcap.Zapret2Linux && len(activation.Plan.EngineArgv) == 0 && len(activation.Sections) > 0 {
		ordered := append([]autotunevnext.ServiceGraphSection(nil), activation.Sections...)
		sort.Slice(ordered, func(i, j int) bool { return ordered[i].NodeID < ordered[j].NodeID })
		var joined []string
		for i, section := range ordered {
			if i > 0 {
				joined = append(joined, "--new")
			}
			joined = append(joined, section.Argv...)
		}
		activation.Plan.EngineArgv = joined
	}
	return activation
}

func (a *productGraphExecutorAdapter) Snapshot(ctx context.Context) (autotunevnext.StateSnapshot, error) {
	return a.inner.Snapshot(ctx)
}

func (a *productGraphExecutorAdapter) EstablishDirect(ctx context.Context, snapshot autotunevnext.StateSnapshot) error {
	return a.inner.EstablishDirect(ctx, snapshot)
}

func (a *productGraphExecutorAdapter) ActivateGraph(ctx context.Context, activation autotunevnext.ServiceGraphActivation) error {
	return a.inner.ActivateGraph(ctx, a.prepareActivation(activation))
}

func (a *productGraphExecutorAdapter) VerifyActiveGraph(ctx context.Context, activation autotunevnext.ServiceGraphActivation) error {
	return a.inner.VerifyActiveGraph(ctx, a.prepareActivation(activation))
}

func (a *productGraphExecutorAdapter) Deactivate(ctx context.Context) error {
	return a.inner.Deactivate(ctx)
}

func (a *productGraphExecutorAdapter) CommitGraph(activation autotunevnext.ServiceGraphActivation) error {
	return a.inner.CommitGraph(a.prepareActivation(activation))
}

func (a *productGraphExecutorAdapter) Restore(ctx context.Context, snapshot autotunevnext.StateSnapshot) error {
	return a.inner.Restore(ctx, snapshot)
}

func (a *productGraphExecutorAdapter) VerifyRestored(ctx context.Context, snapshot autotunevnext.StateSnapshot) error {
	return a.inner.VerifyRestored(ctx, snapshot)
}

func exactGraphSectionArgv(compiledArgv []string, host string) []string {
	plain := "--hostlist-domains=" + host
	anchored := "--hostlist-domains=^" + host
	out := make([]string, len(compiledArgv))
	for i, arg := range compiledArgv {
		if arg == plain {
			out[i] = anchored
		} else {
			out[i] = arg
		}
	}
	return out
}

type resolvedGraphNodeScope struct {
	spec   productServiceGraphNodeSpec
	target autotunevnext.Target
	scope  autotunevnext.ServiceScopeSnapshot
}

func (s *productVNextService) resolveGraphNodeScopes(ctx context.Context, spec productServiceGraphPresetSpec) ([]resolvedGraphNodeScope, error) {
	resolver := s.deps.scopeResolver
	if resolver == nil {
		resolver = autotunevnext.DefaultScopeResolver{}
	}
	resolved := make([]resolvedGraphNodeScope, 0, len(spec.Nodes))
	for _, nodeSpec := range spec.Nodes {
		target, _, err := normalizeVNextTarget(nodeSpec.TargetURL)
		if err != nil {
			return nil, err
		}
		scope, err := resolver.ResolveServiceScope(ctx, target)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, resolvedGraphNodeScope{
			spec:   nodeSpec,
			target: target,
			scope:  scope,
		})
	}
	return resolved, nil
}

type compiledServiceGraphCandidate struct {
	strategyID string
	graph      autotunevnext.ServiceGraph
	sections   []autotunevnext.ServiceGraphSection
	capture    backendcap.CapturePlan
	intent     persistedVNextGraphState
}

func buildServiceGraphCandidate(
	ctx context.Context,
	serviceID string,
	nodes []resolvedGraphNodeScope,
	templateIndex int,
	backend backendcap.Backend,
	preflight autotunevnext.HostPreflight,
	resolver autotunevnext.AssetResolver,
) (compiledServiceGraphCandidate, error) {
	graphNodes := make([]autotunevnext.ServiceNode, 0, len(nodes))
	sections := make([]autotunevnext.ServiceGraphSection, 0, len(nodes))
	persistedNodes := make([]persistedGraphNode, 0, len(nodes))
	var capture backendcap.CapturePlan
	var strategyID string

	for i, item := range nodes {
		catalog, err := productionVNextStrategyCatalog(item.spec.Host)
		if err != nil {
			return compiledServiceGraphCandidate{}, err
		}
		if templateIndex < 0 || templateIndex >= len(catalog) {
			return compiledServiceGraphCandidate{}, errors.New("template index out of range")
		}
		strategy := catalog[templateIndex]
		if i == 0 {
			strategyID = strategy.ID
		}
		templateID, err := strategyir.TemplateIdentity(strategy)
		if err != nil {
			return compiledServiceGraphCandidate{}, err
		}
		fingerprint, err := strategyir.Fingerprint(strategy)
		if err != nil {
			return compiledServiceGraphCandidate{}, err
		}
		compiled := backendcap.Compile(strategy, backend)
		if compiled.Status != backendcap.StatusCompiled {
			return compiledServiceGraphCandidate{}, errors.New("COMPILE_NOT_COMPILED")
		}
		resolvedAssets, err := resolver.Resolve(ctx, backend, compiled.RequiredAssets)
		if err != nil {
			return compiledServiceGraphCandidate{}, err
		}
		argv, err := autotunevnext.MaterializeEngineArgv(compiled.Plan.EngineArgv, resolvedAssets)
		if err != nil {
			return compiledServiceGraphCandidate{}, err
		}
		check := preflight.Check(ctx, autotunevnext.HostPreflightRequest{
			Backend:        backend,
			Requirements:   compiled.DerivedRequirements,
			RequiredAssets: compiled.RequiredAssets,
			Capture:        compiled.Plan.Capture,
		})
		if check.Status != autotunevnext.PreflightSupported {
			code := "PREFLIGHT_UNSUPPORTED"
			if len(check.Reasons) > 0 && check.Reasons[0].Code != "" {
				code = check.Reasons[0].Code
			}
			return compiledServiceGraphCandidate{}, errors.New(code)
		}
		if i == 0 {
			capture = compiled.Plan.Capture
		}
		graphNodes = append(graphNodes, autotunevnext.ServiceNode{
			ID:                  item.spec.NodeID,
			Role:                item.spec.Role,
			Required:            item.spec.Required,
			Target:              item.target,
			Strategy:            strategy.ID,
			TemplateIdentity:    templateID,
			StrategyFingerprint: fingerprint,
			Validation:          autotunevnext.ServiceNodeActive,
			Scope:               item.scope,
		})
		sections = append(sections, autotunevnext.ServiceGraphSection{
			NodeID: item.spec.NodeID,
			Host:   item.spec.Host,
			Argv:   exactGraphSectionArgv(argv, item.spec.Host),
		})
		persistedNodes = append(persistedNodes, persistedGraphNode{
			NodeID:           item.spec.NodeID,
			Role:             string(item.spec.Role),
			Required:         item.spec.Required,
			Target:           item.target.URL,
			StrategyID:       strategy.ID,
			TemplateIdentity: templateID,
			Fingerprint:      fingerprint,
		})
	}

	graph, err := autotunevnext.NewServiceGraph(graphNodes)
	if err != nil {
		return compiledServiceGraphCandidate{}, err
	}
	intent := persistedVNextGraphState{
		SchemaVersion:    vNextGraphSchema,
		Enabled:          true,
		ServiceID:        serviceID,
		Backend:          string(backend),
		GraphFingerprint: graph.Fingerprint(),
		CatalogIdentity:  productVNextCatalogIdentity,
		Nodes:            persistedNodes,
		SavedAt:          time.Now().UTC().Format(time.RFC3339Nano),
	}
	return compiledServiceGraphCandidate{
		strategyID: strategyID,
		graph:      graph,
		sections:   sections,
		capture:    capture,
		intent:     intent,
	}, nil
}

func checkDirectGraphBaseline(
	ctx context.Context,
	executor autotunevnext.ServiceGraphExecutor,
	observer autotunevnext.Observer,
	nodes []resolvedGraphNodeScope,
) (allReachable bool, err error) {
	snapshot, err := executor.Snapshot(ctx)
	if err != nil {
		return false, fmt.Errorf("SNAPSHOT_FAILED: %w", err)
	}
	defer func() {
		rctx, rcancel := context.WithTimeout(context.WithoutCancel(ctx), autotunevnext.ManagedCleanupTimeout)
		defer rcancel()
		_ = executor.Deactivate(rctx)
		if rErr := executor.Restore(rctx, snapshot); rErr != nil && err == nil {
			err = fmt.Errorf("RESTORE_FAILED: %w", rErr)
		}
		if vErr := executor.VerifyRestored(rctx, snapshot); vErr != nil && err == nil {
			err = fmt.Errorf("RESTORE_VERIFY_FAILED: %w", vErr)
		}
	}()
	if err := executor.EstablishDirect(ctx, snapshot); err != nil {
		return false, fmt.Errorf("ESTABLISH_DIRECT_FAILED: %w", err)
	}
	allHealthy := true
	for _, node := range nodes {
		obs, obsErr := autotunevnext.ObserveServiceScope(ctx, observer, node.target, node.scope)
		if obsErr != nil {
			return false, obsErr
		}
		if !autotunevnext.ServiceScopeObservationsHealthy(obs) {
			allHealthy = false
		}
	}
	return allHealthy, nil
}

func graphFailureResult(presetID string, hosts []string, status string, summary string, errs ...string) VNextServiceGraphResult {
	cleanHosts := append([]string{}, hosts...)
	cleanErrs := make([]string, 0, len(errs))
	for _, e := range errs {
		if strings.TrimSpace(e) != "" {
			cleanErrs = append(cleanErrs, e)
		}
	}
	return VNextServiceGraphResult{
		Status:        status,
		PresetID:      presetID,
		Hosts:         cleanHosts,
		VerifiedHosts: []string{},
		Applied:       false,
		Summary:       summary,
		Errors:        cleanErrs,
	}
}

// RunServiceGraph executes bounded multi-host ServiceGraph discovery, live
// validation, and managed activation for a built-in preset ("youtube" | "discord").
func (s *productVNextService) RunServiceGraph(
	ctx context.Context,
	presetID string,
	onProgress func(VNextProgressEvent),
) (VNextServiceGraphResult, error) {
	tracker := newVNextProgressTracker(onProgress)
	defer tracker.emit("complete", "Автоподбор графа сервиса завершён.", 100)
	tracker.emit("preflight", "Проверка пресета и среды выполнения...", 10)

	preset, hosts, err := lookupProductServiceGraphPreset(presetID)
	if err != nil {
		res := graphFailureResult(strings.TrimSpace(presetID), []string{}, string(autotunevnext.StatusPreflightFailed), "Неизвестный пресет сервиса.", "INVALID_SERVICE_PRESET")
		return res, err
	}

	s.mu.Lock()
	busy := s.active != nil || (s.graphManaged != nil && s.graphManaged.isActiveOrDirty())
	s.mu.Unlock()
	if busy {
		return graphFailureResult(preset.ID, hosts, string(autotunevnext.StatusInconclusive), "Отключите активную управляемую стратегию перед новой проверкой.", "MANAGED_ACTIVATION_ACTIVE"), nil
	}

	release, err := engine.GetCoordinator().Acquire(engine.OpAutoTune, "autotune-vnext-graph", false)
	if err != nil {
		return graphFailureResult(preset.ID, hosts, string(autotunevnext.StatusInconclusive), "Другая операция автоподбора или диагностики уже выполняется.", "OPERATION_CONFLICT"), nil
	}
	defer release()
	opCtx, opCancel := context.WithCancel(ctx)
	engine.GetCoordinator().SetCancelFunc(opCancel)
	defer opCancel()

	s.InvalidateGrants()

	adapter := newProductRuntimeProvider(s.manager)
	if err := adapter.validateRestorableState(); err != nil {
		return graphFailureResult(preset.ID, hosts, string(autotunevnext.StatusPreflightFailed), "Не удалось подтвердить исходное состояние профиля.", "PRODUCT_STATE_UNCERTAIN"), nil
	}

	graphRuntime, err := s.deps.newGraphRuntime(adapter, s.assets, logProductVNextPhysicalEvent)
	if err != nil {
		if errors.Is(err, errVNextMeasurementPathUnsupported) {
			return graphFailureResult(preset.ID, hosts, string(autotunevnext.StatusPreflightFailed), "На macOS режим прозрачного захвата пакетов vNext не поддерживается (используйте Strategy Lab или Legacy AutoTune).", "MEASUREMENT_PATH_UNSUPPORTED"), nil
		}
		return graphFailureResult(preset.ID, hosts, string(autotunevnext.StatusPreflightFailed), "Среда выполнения графа недоступна.", "PRODUCT_RUNTIME_UNAVAILABLE"), nil
	}
	if s.deps.observer == nil || graphRuntime.executor == nil || graphRuntime.preflight == nil {
		return graphFailureResult(preset.ID, hosts, string(autotunevnext.StatusPreflightFailed), "Компоненты наблюдения графа недоступны.", "AUTOTUNE_VNEXT_DEPENDENCY_MISSING"), nil
	}
	resolver, err := s.deps.newResolver(s.assets)
	if err != nil {
		return graphFailureResult(preset.ID, hosts, string(autotunevnext.StatusPreflightFailed), "Ресурсы движка недоступны.", "PRODUCT_ASSETS_UNAVAILABLE"), nil
	}
	control, _, err := normalizeVNextTarget(productVNextDefaultControl.Target)
	if err != nil {
		return graphFailureResult(preset.ID, hosts, string(autotunevnext.StatusPreflightFailed), "Ошибка конфигурации контрольного узла.", "INVALID_CONTROL"), nil
	}

	tracker.emit("baseline", "Разрешение хостов сервиса и проверка прямого доступа...", 25)
	resolvedNodes, err := s.resolveGraphNodeScopes(opCtx, preset)
	if err != nil {
		if opCtx.Err() != nil {
			return graphFailureResult(preset.ID, hosts, string(autotunevnext.StatusCancelled), "Автоподбор графа сервиса отменён.", "CANCELLED"), nil
		}
		return graphFailureResult(preset.ID, hosts, string(autotunevnext.StatusPreflightFailed), "Не удалось разрешить DNS-адреса узлов сервиса.", serviceScopeLimitation(err)), nil
	}

	allDirectReachable, baselineErr := checkDirectGraphBaseline(opCtx, graphRuntime.executor, s.deps.observer, resolvedNodes)
	if baselineErr != nil {
		if opCtx.Err() != nil {
			return graphFailureResult(preset.ID, hosts, string(autotunevnext.StatusCancelled), "Автоподбор графа сервиса отменён.", "CANCELLED"), nil
		}
		return graphFailureResult(preset.ID, hosts, string(autotunevnext.StatusInconclusive), "Не удалось выполнить базовую проверку узлов сервиса.", baselineErr.Error()), nil
	}
	if allDirectReachable {
		return VNextServiceGraphResult{
			Status:        string(autotunevnext.StatusCompletedNoActionNeeded),
			PresetID:      preset.ID,
			Hosts:         append([]string{}, hosts...),
			VerifiedHosts: append([]string{}, hosts...),
			Applied:       false,
			Summary:       "Все хосты сервиса доступны напрямую. Изменения не требуются.",
			Errors:        []string{},
		}, nil
	}

	tracker.emit("diagnosis", "Обнаружена блокировка узлов сервиса; анализ маршрута...", 40)
	sampleCatalog, err := productionVNextStrategyCatalog(preset.Nodes[0].Host)
	if err != nil {
		return graphFailureResult(preset.ID, hosts, string(autotunevnext.StatusPreflightFailed), "Каталог стратегий недоступен.", "PRODUCT_CATALOG_INVALID"), nil
	}

	tracker.emit("planning", "Подбор и тестирование мульти-хост стратегий...", 55)
	hostByNodeID := make(map[string]string, len(preset.Nodes))
	for _, n := range preset.Nodes {
		hostByNodeID[n.NodeID] = n.Host
	}

	var bestVerifiedHosts []string
	lastStatus := string(autotunevnext.ServiceGraphStillFailing)
	var candidateErrors []string
	var winner *compiledServiceGraphCandidate

	for idx := range sampleCatalog {
		if opCtx.Err() != nil {
			return graphFailureResult(preset.ID, hosts, string(autotunevnext.StatusCancelled), "Автоподбор графа сервиса отменён.", "CANCELLED"), nil
		}
		cand, buildErr := buildServiceGraphCandidate(opCtx, preset.ID, resolvedNodes, idx, graphRuntime.backend, graphRuntime.preflight, resolver)
		if buildErr != nil {
			candidateErrors = append(candidateErrors, buildErr.Error())
			continue
		}
		expResult, expErr := autotunevnext.RunServiceGraphExperiment(opCtx, autotunevnext.ServiceGraphExperimentRequest{
			Graph:        cand.graph,
			Controls:     []autotunevnext.Target{control},
			Sections:     cand.sections,
			Capture:      cand.capture,
			Backend:      graphRuntime.backend,
			NetworkLabel: "product-autotune-vnext-graph",
		}, graphRuntime.executor, s.deps.observer)
		if expErr != nil {
			if opCtx.Err() != nil {
				return graphFailureResult(preset.ID, hosts, string(autotunevnext.StatusCancelled), "Автоподбор графа сервиса отменён.", "CANCELLED"), nil
			}
			if errors.Is(expErr, autotunevnext.ErrManagedStateRestoreFailed) {
				return graphFailureResult(preset.ID, hosts, string(autotunevnext.StatusStateRestoreFailed), "Критическая ошибка восстановления исходного состояния.", "STATE_RESTORE_FAILED"), nil
			}
			candidateErrors = append(candidateErrors, expErr.Error())
			continue
		}

		lastStatus = string(expResult.Status)
		verifiedNow := make([]string, 0, len(expResult.Nodes))
		for _, nodeRes := range expResult.Nodes {
			if nodeRes.Validation.Status == autotunevnext.ServiceScopeVerifiedFixed || nodeRes.Validation.Status == autotunevnext.ServiceScopeNoActionNeeded || (nodeRes.Validation.Reachable+nodeRes.Validation.Fixed == nodeRes.Validation.EdgeCount && nodeRes.Validation.EdgeCount > 0) {
				if h := hostByNodeID[nodeRes.NodeID]; h != "" {
					verifiedNow = append(verifiedNow, h)
				}
			}
		}
		if len(verifiedNow) > len(bestVerifiedHosts) {
			bestVerifiedHosts = verifiedNow
		}
		if expResult.Status == autotunevnext.ServiceGraphVerifiedFixed && expResult.StateRestored {
			chosen := cand
			winner = &chosen
			bestVerifiedHosts = append([]string{}, hosts...)
			break
		}
	}

	if winner == nil {
		if len(bestVerifiedHosts) == 0 && len(candidateErrors) > 0 && lastStatus == string(autotunevnext.ServiceGraphStillFailing) {
			return VNextServiceGraphResult{
				Status:        lastStatus,
				PresetID:      preset.ID,
				Hosts:         append([]string{}, hosts...),
				VerifiedHosts: []string{},
				Applied:       false,
				Summary:       "Ни одна из проверенных стратегий не восстановила все обязательные узлы сервиса.",
				Errors:        dedupeNonEmptyStrings(candidateErrors),
			}, nil
		}
		if bestVerifiedHosts == nil {
			bestVerifiedHosts = []string{}
		}
		return VNextServiceGraphResult{
			Status:        lastStatus,
			PresetID:      preset.ID,
			Hosts:         append([]string{}, hosts...),
			VerifiedHosts: bestVerifiedHosts,
			Applied:       false,
			Summary:       fmt.Sprintf("Проверено %d из %d узлов; полная верификация графа не достигнута.", len(bestVerifiedHosts), len(hosts)),
			Errors:        dedupeNonEmptyStrings(candidateErrors),
		}, nil
	}

	tracker.emit("applying", fmt.Sprintf("Применение проверенной стратегии (%s) для %s...", winner.strategyID, preset.Label), 80)

	freshNodes, err := s.resolveGraphNodeScopes(opCtx, preset)
	if err != nil {
		return VNextServiceGraphResult{
			Status:        string(autotunevnext.ServiceGraphNeedsRevalidate),
			PresetID:      preset.ID,
			Hosts:         append([]string{}, hosts...),
			VerifiedHosts: bestVerifiedHosts,
			Applied:       false,
			Summary:       "Адреса узлов сервиса изменились перед применением; требуется повторная проверка.",
			Errors:        []string{serviceScopeLimitation(err)},
		}, nil
	}

	currentCand, err := buildServiceGraphCandidate(opCtx, preset.ID, freshNodes, indexOfCatalogStrategy(sampleCatalog, winner.strategyID), graphRuntime.backend, graphRuntime.preflight, resolver)
	if err != nil {
		return VNextServiceGraphResult{
			Status:        "NOT_APPLIED",
			PresetID:      preset.ID,
			Hosts:         append([]string{}, hosts...),
			VerifiedHosts: bestVerifiedHosts,
			Applied:       false,
			Summary:       "Не удалось подготовить граф сервиса к применению.",
			Errors:        []string{err.Error()},
		}, nil
	}

	prepared, err := autotunevnext.PrepareServiceGraphApply(autotunevnext.ServiceGraphApplyRequest{
		Validated:    winner.graph,
		Current:      currentCand.graph,
		Controls:     []autotunevnext.Target{control},
		Sections:     currentCand.sections,
		Capture:      currentCand.capture,
		Backend:      graphRuntime.backend,
		NetworkLabel: "product-autotune-vnext-graph",
	})
	if err != nil {
		return VNextServiceGraphResult{
			Status:        string(autotunevnext.ServiceGraphNeedsRevalidate),
			PresetID:      preset.ID,
			Hosts:         append([]string{}, hosts...),
			VerifiedHosts: bestVerifiedHosts,
			Applied:       false,
			Summary:       "Сетевой скоуп изменился перед применением; требуется повторная проверка.",
			Errors:        []string{"SERVICE_GRAPH_CHANGED_REVALIDATION_REQUIRED"},
		}, nil
	}
	prepared.Plan = backendcap.Plan{Capture: currentCand.capture}

	// Clear any dormant single-host managed state before persisting graph state
	// so persisted ownership is never ambiguous across restarts.
	_ = clearVNextManagedState()

	managed := newGraphManagedService(graphRuntime.executor)
	if applyErr := managed.ApplyGraph(opCtx, prepared, currentCand.intent); applyErr != nil {
		state := "NOT_APPLIED"
		if errors.Is(applyErr, autotunevnext.ErrManagedStateRestoreFailed) {
			state = string(autotunevnext.StatusStateRestoreFailed)
			s.mu.Lock()
			s.graphManaged = managed
			s.mu.Unlock()
		}
		return VNextServiceGraphResult{
			Status:        state,
			PresetID:      preset.ID,
			Hosts:         append([]string{}, hosts...),
			VerifiedHosts: bestVerifiedHosts,
			Applied:       false,
			Summary:       "Не удалось активировать мульти-хост стратегию.",
			Errors:        []string{applyErr.Error()},
		}, nil
	}

	committed := prepared
	s.mu.Lock()
	s.graphManaged = managed
	s.graphActivation = &committed
	s.dormant = AutoTuneVNextManagedStatus{}
	s.fault = AutoTuneVNextManagedStatus{}
	s.mu.Unlock()

	return VNextServiceGraphResult{
		Status:        string(autotunevnext.ServiceGraphVerifiedFixed),
		PresetID:      preset.ID,
		Hosts:         append([]string{}, hosts...),
		VerifiedHosts: append([]string{}, hosts...),
		Applied:       true,
		Summary:       fmt.Sprintf("Мульти-хост стратегия (%s) проверена и активирована для %s (%d хоста).", winner.strategyID, preset.Label, len(hosts)),
		Errors:        []string{},
	}, nil
}

func (s *productVNextService) graphManagedHealth(
	ctx context.Context,
	managed *graphManagedService,
	activation *autotunevnext.ServiceGraphActivation,
) ManagedHealthState {
	status := managed.currentStatus()
	if status.State != GraphManagedActive {
		return ManagedHealthFault
	}
	if managed.executor == nil || managed.executor.VerifyActiveGraph(ctx, *activation) != nil {
		managed.EvaluateHealth(activation.Graph, activation.Graph, false, false)
		return ManagedHealthFault
	}
	preset, _, err := lookupProductServiceGraphPreset(status.ServiceID)
	if err != nil {
		return ManagedHealthFault
	}
	resolvedNodes, err := s.resolveGraphNodeScopes(ctx, preset)
	if err != nil {
		managed.EvaluateHealth(activation.Graph, activation.Graph, true, true)
		return ManagedHealthFault
	}
	scopeByNode := make(map[string]resolvedGraphNodeScope, len(resolvedNodes))
	for _, n := range resolvedNodes {
		scopeByNode[n.spec.NodeID] = n
	}
	currentNodes := make([]autotunevnext.ServiceNode, len(activation.Graph.Nodes))
	copy(currentNodes, activation.Graph.Nodes)
	for i := range currentNodes {
		if fresh, ok := scopeByNode[currentNodes[i].ID]; ok {
			currentNodes[i].Scope = fresh.scope
		}
	}
	currentGraph, err := autotunevnext.NewServiceGraph(currentNodes)
	if err != nil {
		return ManagedHealthFault
	}
	switch managed.EvaluateHealth(activation.Graph, currentGraph, true, false) {
	case GraphManagedFault:
		return ManagedHealthFault
	case GraphManagedRevalidation:
		return ManagedHealthNeedsRevalidation
	}
	for _, fresh := range resolvedNodes {
		obs, obsErr := autotunevnext.ObserveServiceScope(ctx, s.deps.observer, fresh.target, fresh.scope)
		if obsErr != nil || !autotunevnext.ServiceScopeObservationsHealthy(obs) {
			return ManagedHealthFault
		}
	}
	return ManagedHealthHealthy
}

func (s *productVNextService) RevertServiceGraph(ctx context.Context) error {
	if s == nil {
		return clearVNextGraphState()
	}
	s.mu.Lock()
	managed := s.graphManaged
	s.mu.Unlock()
	if managed == nil {
		return clearVNextGraphState()
	}
	err := managed.Revert(ctx)
	if err == nil {
		s.mu.Lock()
		if s.graphManaged == managed {
			s.graphManaged = nil
			s.graphActivation = nil
		}
		s.mu.Unlock()
	}
	return err
}

func (s *productVNextService) SuspendServiceGraph(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	managed := s.graphManaged
	s.mu.Unlock()
	if managed == nil {
		return nil
	}
	err := managed.Shutdown(ctx)
	if err == nil {
		s.mu.Lock()
		if s.graphManaged == managed {
			s.graphActivation = nil
		}
		s.mu.Unlock()
	}
	return err
}

// RevalidateSavedGraph evaluates persisted graph intent against fresh DNS
// resolution using PlanGraphRestart before allowing re-activation.
func (s *productVNextService) RevalidateSavedGraph(ctx context.Context) AutoTuneVNextManagedStatus {
	saved, present, err := loadVNextGraphState()
	if err != nil {
		return s.rememberDormant(AutoTuneVNextManagedStatus{State: "SAVED_STRATEGY_STALE", NeedsRevalidation: true})
	}
	if !present {
		return s.rememberDormant(AutoTuneVNextManagedStatus{State: "DIRECT"})
	}
	preset, _, presetErr := lookupProductServiceGraphPreset(saved.ServiceID)
	if presetErr != nil {
		return s.rememberDormant(AutoTuneVNextManagedStatus{State: "SAVED_STRATEGY_STALE", NeedsRevalidation: true})
	}
	resolvedNodes, resolveErr := s.resolveGraphNodeScopes(ctx, preset)
	if resolveErr != nil {
		return s.rememberDormant(AutoTuneVNextManagedStatus{
			State:             "SAVED_REVALIDATION_PENDING",
			NeedsRevalidation: true,
			Target:            preset.Nodes[0].TargetURL,
			StrategyID:        saved.Nodes[0].StrategyID,
			Fingerprint:       shortManagedFingerprint(saved.GraphFingerprint),
			Backend:           saved.Backend,
		})
	}
	fresh := make([]freshGraphNode, 0, len(resolvedNodes))
	for _, n := range resolvedNodes {
		fresh = append(fresh, freshGraphNode{
			NodeID: n.spec.NodeID,
			Role:   n.spec.Role,
			Target: n.target.URL,
			Edges:  n.scope.Edges,
		})
	}
	plan, planErr := PlanGraphRestart(saved, fresh)
	if planErr != nil || plan.State != GraphRestartReady {
		return s.rememberDormant(AutoTuneVNextManagedStatus{
			State:             "SAVED_STRATEGY_STALE",
			NeedsRevalidation: true,
			Target:            preset.Nodes[0].TargetURL,
			StrategyID:        saved.Nodes[0].StrategyID,
			Fingerprint:       shortManagedFingerprint(saved.GraphFingerprint),
			Backend:           saved.Backend,
		})
	}
	res, runErr := s.RunServiceGraph(ctx, saved.ServiceID, nil)
	if runErr == nil && res.Applied {
		return s.Status()
	}
	if res.Status == string(autotunevnext.StatusCompletedNoActionNeeded) {
		return s.rememberDormant(AutoTuneVNextManagedStatus{
			State:             "SAVED_NOT_CURRENTLY_NEEDED",
			NeedsRevalidation: true,
			Target:            preset.Nodes[0].TargetURL,
			StrategyID:        saved.Nodes[0].StrategyID,
			Fingerprint:       shortManagedFingerprint(saved.GraphFingerprint),
			Backend:           saved.Backend,
		})
	}
	return s.rememberDormant(AutoTuneVNextManagedStatus{
		State:             "NO_VERIFIED_STRATEGY",
		NeedsRevalidation: true,
		Target:            preset.Nodes[0].TargetURL,
		StrategyID:        saved.Nodes[0].StrategyID,
		Fingerprint:       shortManagedFingerprint(saved.GraphFingerprint),
		Backend:           saved.Backend,
	})
}

func indexOfCatalogStrategy(catalog []strategyir.Strategy, id string) int {
	for i, s := range catalog {
		if s.ID == id {
			return i
		}
	}
	return -1
}

func dedupeNonEmptyStrings(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, raw := range items {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func primaryURLForGraphPreset(serviceID string) string {
	preset, _, err := lookupProductServiceGraphPreset(serviceID)
	if err == nil && len(preset.Nodes) > 0 {
		return preset.Nodes[0].TargetURL
	}
	return ""
}

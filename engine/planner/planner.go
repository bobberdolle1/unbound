// Package planner derives deterministic StrategyIR eligibility assessments from
// attribution evidence. It does not execute engines or inspect the host.
package planner

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"

	"unbound/engine"
	"unbound/engine/attribution"
	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/strategyir"
)

const SchemaVersion = 1

type Disposition string

const (
	DispositionCandidatesAvailable       Disposition = "CANDIDATES_AVAILABLE"
	DispositionNoActionNeeded            Disposition = "NO_ACTION_NEEDED"
	DispositionNoPacketStrategyIndicated Disposition = "NO_PACKET_STRATEGY_INDICATED"
	DispositionInsufficientEvidence      Disposition = "INSUFFICIENT_EVIDENCE"
	DispositionNoCompatibleCandidates    Disposition = "NO_COMPATIBLE_CANDIDATES"
)

type CandidateStatus string

const (
	StatusEligible                 CandidateStatus = "ELIGIBLE"
	StatusStructurallyInapplicable CandidateStatus = "STRUCTURALLY_INAPPLICABLE"
	StatusTargetScopeMismatch      CandidateStatus = "TARGET_SCOPE_MISMATCH"
	StatusTargetScopeUnknown       CandidateStatus = "TARGET_SCOPE_UNKNOWN"
	StatusBackendUnsupported       CandidateStatus = "BACKEND_UNSUPPORTED"
	StatusInvalidStrategy          CandidateStatus = "INVALID_STRATEGY"
	StatusInsufficientEvidence     CandidateStatus = "INSUFFICIENT_EVIDENCE"
)

type Reason struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

// ScopeSnapshot contains caller-supplied, already resolved logical membership.
// It is data, not an interface to the filesystem or resolver.
type ScopeSnapshot struct {
	HostListMembers map[string][]string `json:"host_list_members,omitempty"`
	IPSetMembers    map[string][]string `json:"ip_set_members,omitempty"`
	TargetEdgeIPs   []string            `json:"target_edge_ips,omitempty"`
}

type Request struct {
	Attribution attribution.AttributionReport `json:"attribution"`
	Backend     backendcap.Backend            `json:"backend"`
	Strategies  []strategyir.Strategy         `json:"strategies"`
	Scope       ScopeSnapshot                 `json:"scope,omitempty"`
}

type CandidateAssessment struct {
	StrategyID          string                      `json:"strategy_id"`
	StrategyFingerprint string                      `json:"strategy_fingerprint,omitempty"`
	Status              CandidateStatus             `json:"status"`
	AffectedStages      []attribution.AffectedStage `json:"affected_stages,omitempty"`
	CompileStatus       backendcap.CompileStatus    `json:"compile_status,omitempty"`
	Backend             backendcap.Backend          `json:"backend"`
	DerivedRequirements engine.StrategyRequirements `json:"derived_requirements"`
	Reasons             []Reason                    `json:"reasons,omitempty"`
	CompilerReasons     []backendcap.Reason         `json:"compiler_reasons,omitempty"`
	Limitations         []string                    `json:"limitations,omitempty"`
	Safety              strategyir.SafetyPolicy     `json:"safety"`
}

type PlannerReport struct {
	SchemaVersion int                   `json:"schema_version"`
	AttributionID string                `json:"attribution_id"`
	Backend       backendcap.Backend    `json:"backend"`
	Target        attribution.TargetRef `json:"target"`
	Disposition   Disposition           `json:"disposition"`
	Candidates    []CandidateAssessment `json:"candidates"`
	Limitations   []string              `json:"limitations,omitempty"`
}

// Plan is deterministic and pure. Candidate order is serialization order only,
// never a preference, score, or recommendation.
func Plan(request Request) PlannerReport {
	report := PlannerReport{
		SchemaVersion: SchemaVersion, AttributionID: request.Attribution.AttributionID,
		Backend: request.Backend, Target: request.Attribution.Target,
		Limitations: append([]string(nil), request.Attribution.Limitations...),
	}
	if hasFinding(request.Attribution, attribution.FindingEdgeDependentFailure) || len(request.Attribution.Counterevidence) != 0 {
		report.Limitations = append(report.Limitations, "EDGE_DEPENDENT: successful or divergent target-edge evidence constrains eligibility and does not establish effectiveness.")
	}
	report.Limitations = uniqueStrings(report.Limitations)

	if hasFinding(request.Attribution, attribution.FindingNoAnomaly) || hasFinding(request.Attribution, attribution.FindingReachableDirectly) {
		report.Disposition = DispositionNoActionNeeded
		return report
	}
	if hasFinding(request.Attribution, attribution.FindingHTTPApplicationFailure) {
		report.Disposition = DispositionNoPacketStrategyIndicated
		return report
	}
	if hasQUICUnsupported(request.Attribution) {
		report.Disposition = DispositionInsufficientEvidence
		report.Limitations = uniqueStrings(append(report.Limitations, "QUIC_UNSUPPORTED: no real QUIC handshake boundary is available."))
		return report
	}
	boundary, transport, ok := concreteBoundary(request.Attribution)
	if !ok {
		report.Disposition = DispositionInsufficientEvidence
		return report
	}

	for _, strategy := range canonicalStrategies(request.Strategies) {
		candidate := assess(strategy, request, boundary, transport)
		candidate.Limitations = append([]string(nil), report.Limitations...)
		report.Candidates = append(report.Candidates, candidate)
	}
	if slices.ContainsFunc(report.Candidates, func(candidate CandidateAssessment) bool { return candidate.Status == StatusEligible }) {
		report.Disposition = DispositionCandidatesAvailable
	} else {
		report.Disposition = DispositionNoCompatibleCandidates
	}
	return report
}

func assess(strategy strategyir.Strategy, request Request, boundary observatory.Stage, transport observatory.Transport) CandidateAssessment {
	candidate := CandidateAssessment{StrategyID: strategy.ID, Backend: request.Backend, Safety: strategy.Safety}
	fingerprint, err := strategyir.Fingerprint(strategy)
	if err != nil {
		candidate.Status = StatusInvalidStrategy
		candidate.Reasons = []Reason{{Code: "INVALID_STRATEGY", Detail: err.Error()}}
		return candidate
	}
	candidate.StrategyFingerprint = fingerprint
	if strategy.Transport[0] == strategyir.TransportUDP {
		candidate.Status = StatusInsufficientEvidence
		candidate.Reasons = []Reason{{Code: "RAW_UDP_EVIDENCE_UNAVAILABLE", Detail: "Observatory v1 has no raw UDP evidence model."}}
		return candidate
	}
	capabilities, mapped := StrategyCapabilities(strategy)
	candidate.AffectedStages = capabilities.AffectedStages
	if !mapped {
		candidate.Status = StatusStructurallyInapplicable
		candidate.Reasons = []Reason{{Code: "UNKNOWN_EFFECT", Detail: "Strategy operations have no conservative mapping to the observed boundary."}}
		return candidate
	}
	if !attribution.CouldStrategyAffectFailure(capabilities, boundary, transport) {
		candidate.Status = StatusStructurallyInapplicable
		candidate.Reasons = []Reason{{Code: "EFFECT_STAGE_MISMATCH", Detail: "Strategy effect and evidence boundary or transport do not match."}}
		return candidate
	}
	match := MatchTargetScope(strategy.Selector.Scope, request.Attribution.Target.Hostname, request.Scope)
	if match == scopeUnknown {
		candidate.Status = StatusTargetScopeUnknown
		candidate.Reasons = []Reason{{Code: "TARGET_SCOPE_UNKNOWN", Detail: "Logical scope membership was not supplied by the caller."}}
		return candidate
	}
	if match == scopeMismatch {
		candidate.Status = StatusTargetScopeMismatch
		candidate.Reasons = []Reason{{Code: "TARGET_SCOPE_MISMATCH", Detail: "Strategy selector excludes the attributed target."}}
		return candidate
	}
	compiled := backendcap.Compile(strategy, request.Backend)
	candidate.CompileStatus = compiled.Status
	candidate.DerivedRequirements = compiled.DerivedRequirements
	candidate.CompilerReasons = append([]backendcap.Reason(nil), compiled.Unsupported...)
	switch compiled.Status {
	case backendcap.StatusCompiled:
		candidate.Status = StatusEligible
	case backendcap.StatusInvalid:
		candidate.Status = StatusInvalidStrategy
	default:
		candidate.Status = StatusBackendUnsupported
	}
	return candidate
}

// StrategyCapabilities is the conservative StrategyIR-to-attribution adapter.
// Unmapped operations deliberately confer no eligibility.
func StrategyCapabilities(strategy strategyir.Strategy) (attribution.StrategyCapabilities, bool) {
	if len(strategy.Transport) != 1 {
		return attribution.StrategyCapabilities{}, false
	}
	var transport observatory.Transport
	switch strategy.Transport[0] {
	case strategyir.TransportTCP:
		transport = observatory.TransportTCP
	case strategyir.TransportQUIC:
		transport = observatory.TransportQUIC
	default:
		return attribution.StrategyCapabilities{}, false
	}
	stages := map[attribution.AffectedStage]bool{}
	hasTLS, hasHTTP, hasQUIC := hasProtocol(strategy, strategyir.ApplicationTLS), hasProtocol(strategy, strategyir.ApplicationHTTP), hasProtocol(strategy, strategyir.ApplicationQUIC)
	for _, operation := range strategy.Operations {
		if transport == observatory.TransportTCP && hasTLS && affectsTLSHello(operation.Type) {
			stages[attribution.AffectedStageHello] = true
		}
		if transport == observatory.TransportTCP && hasHTTP && affectsHTTP(operation) {
			stages[attribution.AffectedStageHTTP] = true
		}
		if transport == observatory.TransportQUIC && hasQUIC && operation.Type == strategyir.OperationFakeInjection {
			stages[attribution.AffectedStageHello] = true
		}
	}
	if len(stages) == 0 {
		return attribution.StrategyCapabilities{}, false
	}
	out := make([]attribution.AffectedStage, 0, len(stages))
	for stage := range stages {
		out = append(out, stage)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return attribution.StrategyCapabilities{AffectedStages: out, Transports: []observatory.Transport{transport}}, true
}

func affectsTLSHello(kind strategyir.OperationKind) bool {
	switch kind {
	case strategyir.OperationSplit, strategyir.OperationMultiSplit, strategyir.OperationMultiDisorder, strategyir.OperationFakeInjection, strategyir.OperationHostFakeSplit, strategyir.OperationTLSRecordSplit:
		return true
	default:
		return false
	}
}
func affectsHTTP(operation strategyir.Operation) bool {
	if operation.Type == strategyir.OperationHTTPHostCase {
		return true
	}
	if operation.Type != strategyir.OperationSplit && operation.Type != strategyir.OperationMultiSplit {
		return false
	}
	return slices.ContainsFunc(operation.Positions, func(position strategyir.PositionExpr) bool { return position.Anchor == strategyir.AnchorMethod })
}
func hasProtocol(strategy strategyir.Strategy, wanted strategyir.ApplicationProtocol) bool {
	return slices.Contains(strategy.Selector.ApplicationProtocols, wanted)
}

type scopeMatch uint8

const (
	scopeMatchOK scopeMatch = iota
	scopeMismatch
	scopeUnknown
)

// MatchTargetScope uses exact hostname or subdomain membership: "example.com"
// covers example.com and a.example.com. Logical IDs are unknown unless present
// in ScopeSnapshot; the planner never infers membership from an ID's name.
func MatchTargetScope(scope strategyir.Scope, hostname string, snapshot ScopeSnapshot) scopeMatch {
	hostname = normalizeHost(hostname)
	if hostname == "" {
		return scopeUnknown
	}
	primary := scopeMatchOK
	switch scope.Host.Mode {
	case strategyir.HostScopeAll:
	case strategyir.HostScopeExplicit:
		if !containsHost(scope.Host.Hosts, hostname) {
			primary = scopeMismatch
		}
	case strategyir.HostScopeManagedList, strategyir.HostScopeAutoHostlist:
		members, ok := snapshot.HostListMembers[scope.Host.ID]
		if !ok {
			return scopeUnknown
		}
		if !containsHost(members, hostname) {
			primary = scopeMismatch
		}
	case strategyir.HostScopeIPSetReference:
		primary = matchesIPSets([]string{scope.Host.ID}, snapshot)
	default:
		return scopeUnknown
	}
	if primary != scopeMatchOK {
		return primary
	}
	for _, id := range scope.ExcludeHostListIDs {
		members, ok := snapshot.HostListMembers[id]
		if !ok {
			return scopeUnknown
		}
		if containsHost(members, hostname) {
			return scopeMismatch
		}
	}
	for _, id := range scope.IPSetIDs {
		if match := matchesIPSets([]string{id}, snapshot); match != scopeMatchOK {
			return match
		}
	}
	for _, id := range scope.ExcludeIPSetIDs {
		members, ok := snapshot.IPSetMembers[id]
		if !ok || len(snapshot.TargetEdgeIPs) == 0 {
			return scopeUnknown
		}
		if intersects(members, snapshot.TargetEdgeIPs) {
			return scopeMismatch
		}
	}
	return scopeMatchOK
}
func matchesIPSets(ids []string, snapshot ScopeSnapshot) scopeMatch {
	if len(snapshot.TargetEdgeIPs) == 0 {
		return scopeUnknown
	}
	for _, id := range ids {
		members, ok := snapshot.IPSetMembers[id]
		if !ok {
			return scopeUnknown
		}
		if !intersects(members, snapshot.TargetEdgeIPs) {
			return scopeMismatch
		}
	}
	return scopeMatchOK
}
func containsHost(members []string, hostname string) bool {
	for _, member := range members {
		member = normalizeHost(member)
		if member == hostname || strings.HasSuffix(hostname, "."+member) {
			return true
		}
	}
	return false
}
func normalizeHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}
func intersects(left, right []string) bool {
	seen := map[string]bool{}
	for _, value := range left {
		seen[strings.TrimSpace(value)] = true
	}
	for _, value := range right {
		if seen[strings.TrimSpace(value)] {
			return true
		}
	}
	return false
}

func concreteBoundary(report attribution.AttributionReport) (observatory.Stage, observatory.Transport, bool) {
	for _, finding := range report.Findings {
		if isConcreteFinding(finding, report.Target.RequestedProtocol) {
			return finding.Stage, report.Target.RequestedProtocol, true
		}
	}
	if isConcreteFinding(report.PrimaryFinding, report.Target.RequestedProtocol) {
		return report.PrimaryFinding.Stage, report.Target.RequestedProtocol, true
	}
	return "", "", false
}
func isConcreteFinding(finding attribution.Finding, transport observatory.Transport) bool {
	switch finding.Code {
	case attribution.FindingDNSPathFailureSuspected, attribution.FindingTCPPathFailureSuspected, attribution.FindingTLSPathFailureSuspected, attribution.FindingTCPResetObserved, attribution.FindingTLSResetObserved:
		return true
	case attribution.FindingInsufficientEvidence:
		return transport == observatory.TransportQUIC && finding.Stage == observatory.StageHandshake
	default:
		return false
	}
}
func hasQUICUnsupported(report attribution.AttributionReport) bool {
	return report.Target.RequestedProtocol == observatory.TransportQUIC && (report.PrimaryFinding.Code == attribution.FindingInsufficientEvidence || slices.ContainsFunc(report.Findings, func(f attribution.Finding) bool { return f.Code == attribution.FindingInsufficientEvidence }))
}
func hasFinding(report attribution.AttributionReport, code attribution.FindingCode) bool {
	return report.PrimaryFinding.Code == code || slices.ContainsFunc(report.Findings, func(f attribution.Finding) bool { return f.Code == code })
}
func uniqueStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return slices.Compact(out)
}

type keyedStrategy struct {
	strategy         strategyir.Strategy
	fingerprint, key string
}

func canonicalStrategies(strategies []strategyir.Strategy) []strategyir.Strategy {
	keyed := make([]keyedStrategy, 0, len(strategies))
	for _, strategy := range strategies {
		fingerprint, err := strategyir.Fingerprint(strategy)
		key := strategy.ID
		if err != nil {
			raw, _ := json.Marshal(strategy)
			key += "\x00" + string(raw)
		}
		keyed = append(keyed, keyedStrategy{strategy, fingerprint, key})
	}
	sort.Slice(keyed, func(i, j int) bool {
		if keyed[i].strategy.ID != keyed[j].strategy.ID {
			return keyed[i].strategy.ID < keyed[j].strategy.ID
		}
		if keyed[i].fingerprint != keyed[j].fingerprint {
			return keyed[i].fingerprint < keyed[j].fingerprint
		}
		return keyed[i].key < keyed[j].key
	})
	out := make([]strategyir.Strategy, 0, len(keyed))
	seen := map[string]bool{}
	for _, item := range keyed {
		if item.fingerprint != "" && seen[item.fingerprint] {
			continue
		}
		if item.fingerprint != "" {
			seen[item.fingerprint] = true
		}
		out = append(out, item.strategy)
	}
	return out
}

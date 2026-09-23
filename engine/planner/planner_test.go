package planner

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"unbound/engine/attribution"
	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/strategyir"
)

func evidence(code attribution.FindingCode, stage observatory.Stage, transport observatory.Transport) attribution.AttributionReport {
	finding := attribution.Finding{Code: code, Stage: stage, Kind: attribution.FindingKindFact}
	return attribution.AttributionReport{SchemaVersion: attribution.SchemaVersion, AttributionID: "attribution-test", Target: attribution.TargetRef{Hostname: "target.test", Port: "443", RequestedProtocol: transport}, PrimaryFinding: finding, Findings: []attribution.Finding{finding}}
}
func tlsStrategy(id string) strategyir.Strategy {
	return strategyir.Strategy{SchemaVersion: strategyir.SchemaVersion, ID: id, Name: id, Transport: []strategyir.Transport{strategyir.TransportTCP}, Selector: strategyir.TrafficSelector{ApplicationProtocols: []strategyir.ApplicationProtocol{strategyir.ApplicationTLS}, IPFamilies: []strategyir.IPFamily{strategyir.IPFamilyAny}, Direction: strategyir.DirectionOutbound, TCPPorts: []strategyir.PortRange{{Start: 443, End: 443}}, Scope: strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeAll}}}, Operations: []strategyir.Operation{{Type: strategyir.OperationMultiSplit, Positions: []strategyir.PositionExpr{{Absolute: new(1)}}}}, Safety: strategyir.SafetyPolicy{Aggressiveness: "LOW"}}
}
func httpStrategy(id string) strategyir.Strategy {
	s := tlsStrategy(id)
	s.Selector.ApplicationProtocols = []strategyir.ApplicationProtocol{strategyir.ApplicationHTTP}
	s.Operations = []strategyir.Operation{{Type: strategyir.OperationHTTPHostCase}}
	return s
}
func fakeStrategy(id string) strategyir.Strategy {
	s := tlsStrategy(id)
	s.Operations = []strategyir.Operation{{Type: strategyir.OperationFakeInjection, PayloadRef: "tls-clienthello-default", Fake: &strategyir.FakeModifiers{Repeat: 1}}}
	return s
}
func quicStrategy(id string) strategyir.Strategy {
	s := fakeStrategy(id)
	s.Transport = []strategyir.Transport{strategyir.TransportQUIC}
	s.Selector.ApplicationProtocols = []strategyir.ApplicationProtocol{strategyir.ApplicationQUIC}
	s.Selector.TCPPorts = nil
	s.Selector.UDPPorts = []strategyir.PortRange{{Start: 443, End: 443}}
	return s
}
func candidate(report PlannerReport, id string) CandidateAssessment {
	for _, c := range report.Candidates {
		if c.StrategyID == id {
			return c
		}
	}
	return CandidateAssessment{}
}

func TestEvidenceGatesAndStructuralApplicability(t *testing.T) {
	tls, http := tlsStrategy("tls"), httpStrategy("http")
	cases := []struct {
		name       string
		report     attribution.AttributionReport
		want       Disposition
		strategies []strategyir.Strategy
	}{
		{"no anomaly", evidence(attribution.FindingNoAnomaly, observatory.StageHTTP, observatory.TransportTCP), DispositionNoActionNeeded, []strategyir.Strategy{tls}},
		{"reachable directly", evidence(attribution.FindingReachableDirectly, observatory.StageHTTP, observatory.TransportTCP), DispositionNoActionNeeded, []strategyir.Strategy{tls}},
		{"http application", evidence(attribution.FindingHTTPApplicationFailure, observatory.StageHTTP, observatory.TransportTCP), DispositionNoPacketStrategyIndicated, []strategyir.Strategy{tls}},
		{"dns failure", evidence(attribution.FindingDNSPathFailureSuspected, observatory.StageResolve, observatory.TransportTCP), DispositionNoCompatibleCandidates, []strategyir.Strategy{tls}},
		{"connect failure", evidence(attribution.FindingTCPPathFailureSuspected, observatory.StageConnect, observatory.TransportTCP), DispositionNoCompatibleCandidates, []strategyir.Strategy{tls}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Plan(Request{Attribution: tc.report, Backend: backendcap.Zapret2Windows, Strategies: tc.strategies})
			if got.Disposition != tc.want {
				t.Fatalf("disposition = %s, want %s", got.Disposition, tc.want)
			}
			if tc.want == DispositionNoCompatibleCandidates && candidate(got, "tls").Status != StatusStructurallyInapplicable {
				t.Fatalf("candidate = %#v", candidate(got, "tls"))
			}
		})
	}

	tlsReport := evidence(attribution.FindingTLSPathFailureSuspected, observatory.StageHandshake, observatory.TransportTCP)
	got := Plan(Request{Attribution: tlsReport, Backend: backendcap.Zapret2Windows, Strategies: []strategyir.Strategy{tls, http}})
	if got.Disposition != DispositionCandidatesAvailable || candidate(got, "tls").Status != StatusEligible || candidate(got, "http").Status != StatusStructurallyInapplicable {
		t.Fatalf("TLS plan = %#v", got)
	}
}

func TestQUICEvidenceSemantics(t *testing.T) {
	unsupported := evidence(attribution.FindingInsufficientEvidence, observatory.StageConnect, observatory.TransportQUIC)
	if got := Plan(Request{Attribution: unsupported, Backend: backendcap.Zapret2Windows, Strategies: []strategyir.Strategy{quicStrategy("quic")}}); got.Disposition != DispositionInsufficientEvidence || len(got.Candidates) != 0 {
		t.Fatalf("unsupported QUIC = %#v", got)
	}

	// A synthetic future real QUIC handshake boundary is eligible only for QUIC.
	real := evidence(attribution.FindingTLSPathFailureSuspected, observatory.StageHandshake, observatory.TransportQUIC)
	got := Plan(Request{Attribution: real, Backend: backendcap.Zapret2Windows, Strategies: []strategyir.Strategy{tlsStrategy("tcp"), quicStrategy("quic")}})
	if candidate(got, "quic").Status != StatusEligible || candidate(got, "tcp").Status != StatusStructurallyInapplicable {
		t.Fatalf("real QUIC plan = %#v", got)
	}
}

func TestTargetScopeMatchingFailsClosed(t *testing.T) {
	report := evidence(attribution.FindingTLSPathFailureSuspected, observatory.StageHandshake, observatory.TransportTCP)
	managed := tlsStrategy("managed")
	managed.Selector.Scope.Host = strategyir.HostScope{Mode: strategyir.HostScopeManagedList, ID: "trusted"}
	for _, tc := range []struct {
		name  string
		scope ScopeSnapshot
		want  CandidateStatus
	}{
		{"known match", ScopeSnapshot{HostListMembers: map[string][]string{"trusted": {"target.test"}}}, StatusEligible},
		{"known miss", ScopeSnapshot{HostListMembers: map[string][]string{"trusted": {"other.test"}}}, StatusTargetScopeMismatch},
		{"unknown", ScopeSnapshot{}, StatusTargetScopeUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := candidate(Plan(Request{Attribution: report, Backend: backendcap.Zapret2Windows, Strategies: []strategyir.Strategy{managed}, Scope: tc.scope}), "managed").Status; got != tc.want {
				t.Fatalf("status = %s, want %s", got, tc.want)
			}
		})
	}

	explicit := tlsStrategy("explicit")
	explicit.Selector.Scope.Host = strategyir.HostScope{Mode: strategyir.HostScopeExplicit, Hosts: []string{"target.test"}}
	if candidate(Plan(Request{Attribution: report, Backend: backendcap.Zapret2Windows, Strategies: []strategyir.Strategy{explicit}}), "explicit").Status != StatusEligible {
		t.Fatal("exact explicit host did not match")
	}
	explicit.Selector.Scope.Host.Hosts = []string{"other.test"}
	if candidate(Plan(Request{Attribution: report, Backend: backendcap.Zapret2Windows, Strategies: []strategyir.Strategy{explicit}}), "explicit").Status != StatusTargetScopeMismatch {
		t.Fatal("explicit mismatch became eligible")
	}

	ipset := tlsStrategy("ipset")
	ipset.Selector.Scope.Host = strategyir.HostScope{Mode: strategyir.HostScopeIPSetReference, ID: "edges"}
	known := ScopeSnapshot{IPSetMembers: map[string][]string{"edges": {"192.0.2.1"}}, TargetEdgeIPs: []string{"192.0.2.1"}}
	if candidate(Plan(Request{Attribution: report, Backend: backendcap.Zapret2Windows, Strategies: []strategyir.Strategy{ipset}, Scope: known}), "ipset").Status != StatusEligible {
		t.Fatal("known IP-set edge did not match")
	}
	if candidate(Plan(Request{Attribution: report, Backend: backendcap.Zapret2Windows, Strategies: []strategyir.Strategy{ipset}, Scope: ScopeSnapshot{IPSetMembers: known.IPSetMembers}}), "ipset").Status != StatusTargetScopeUnknown {
		t.Fatal("IP-set without target edge data was not fail-closed")
	}
}

func TestBackendCompilationAndDeterminism(t *testing.T) {
	report := evidence(attribution.FindingTLSPathFailureSuspected, observatory.StageHandshake, observatory.TransportTCP)
	strategy := fakeStrategy("fake")
	for _, backend := range []backendcap.Backend{backendcap.Zapret2Windows, backendcap.Zapret2Linux} {
		if got := candidate(Plan(Request{Attribution: report, Backend: backend, Strategies: []strategyir.Strategy{strategy}}), "fake"); got.Status != StatusEligible || got.CompileStatus != backendcap.StatusCompiled {
			t.Fatalf("%s = %#v", backend, got)
		}
	}
	if got := candidate(Plan(Request{Attribution: report, Backend: backendcap.Zapret1TPWSDarwin, Strategies: []strategyir.Strategy{strategy}}), "fake"); got.Status != StatusBackendUnsupported || len(got.CompilerReasons) == 0 {
		t.Fatalf("tpws unsupported details lost: %#v", got)
	}

	other := tlsStrategy("other")
	first := Plan(Request{Attribution: report, Backend: backendcap.Zapret2Windows, Strategies: []strategyir.Strategy{other, strategy, strategy}})
	second := Plan(Request{Attribution: report, Backend: backendcap.Zapret2Windows, Strategies: []strategyir.Strategy{strategy, other}})
	firstJSON, _ := json.Marshal(first)
	secondJSON, _ := json.Marshal(second)
	if string(firstJSON) != string(secondJSON) || len(first.Candidates) != 2 || !slices.IsSortedFunc(first.Candidates, func(a, b CandidateAssessment) int { return strings.Compare(a.StrategyID, b.StrategyID) }) {
		t.Fatalf("candidate ordering or deduplication changed: %s / %s", firstJSON, secondJSON)
	}
}

func TestCounterevidenceAndProfileLabelsRetainConcreteBoundary(t *testing.T) {
	report := evidence(attribution.FindingStillFailing, observatory.StageHandshake, observatory.TransportTCP)
	report.Findings = []attribution.Finding{report.PrimaryFinding, {Code: attribution.FindingTLSPathFailureSuspected, Stage: observatory.StageHandshake}, {Code: attribution.FindingEdgeDependentFailure, Stage: observatory.StageHandshake}}
	report.Counterevidence = []attribution.EvidenceRef{{RunID: "successful-edge", Stage: observatory.StageHTTP}}
	got := Plan(Request{Attribution: report, Backend: backendcap.Zapret2Windows, Strategies: []strategyir.Strategy{tlsStrategy("tls")}})
	if candidate(got, "tls").Status != StatusEligible || !slices.ContainsFunc(got.Limitations, func(value string) bool { return strings.Contains(value, "EDGE_DEPENDENT") }) {
		t.Fatalf("profile label displaced TLS boundary or lost counterevidence: %#v", got)
	}
}

func TestReportHasNoRankingSurface(t *testing.T) {
	for _, name := range []string{"Score", "Rank", "Winner", "Best", "Probability", "ExpectedSuccess"} {
		if _, ok := reflect.TypeFor[CandidateAssessment]().FieldByName(name); ok {
			t.Fatalf("forbidden ranking field %s", name)
		}
	}
}

func TestSanitizedPhysicalFixtures(t *testing.T) {
	type cohort struct {
		Target   []observatory.ObservationResult `json:"target"`
		Controls []observatory.ObservationResult `json:"controls"`
	}
	data, err := os.ReadFile("../attribution/testdata/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures map[string]cohort
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	cloudflare := attribution.Analyze(fixtures["success"].Target)
	if got := Plan(Request{Attribution: cloudflare, Backend: backendcap.Zapret2Windows}); got.Disposition != DispositionNoActionNeeded {
		t.Fatalf("Cloudflare success plan = %#v", got)
	}
	youtube := attribution.AnalyzeCohort(attribution.Cohort{Target: fixtures["healthy_control_tls"].Target, Controls: fixtures["healthy_control_tls"].Controls})
	youtubeStrategy := strategyir.RepresentativeFixtures()["alternative-multisplit"]
	if got := Plan(Request{Attribution: youtube, Backend: backendcap.Zapret2Windows, Strategies: []strategyir.Strategy{youtubeStrategy}, Scope: ScopeSnapshot{HostListMembers: map[string][]string{"youtube": {"www.youtube.com"}}}}); candidate(got, youtubeStrategy.ID).Status != StatusEligible {
		t.Fatalf("YouTube TLS plan = %#v", got)
	}
	discordData, err := os.ReadFile("../attribution/testdata/discord-observation.json")
	if err != nil {
		t.Fatal(err)
	}
	var discordObservation observatory.ObservationResult
	if err := json.Unmarshal(discordData, &discordObservation); err != nil {
		t.Fatal(err)
	}
	discord := attribution.Analyze([]observatory.ObservationResult{discordObservation})
	if got := Plan(Request{Attribution: discord, Backend: backendcap.Zapret2Windows, Strategies: []strategyir.Strategy{strategyir.RepresentativeFixtures()["discord-tcp"]}}); !slices.ContainsFunc(got.Limitations, func(value string) bool { return strings.Contains(value, "EDGE_DEPENDENT") }) {
		t.Fatalf("Discord edge limitation missing: %#v", got)
	}
}

func TestRawUDPRequiresEvidenceModel(t *testing.T) {
	strategy := tlsStrategy("udp")
	strategy.Transport = []strategyir.Transport{strategyir.TransportUDP}
	strategy.Selector.ApplicationProtocols = []strategyir.ApplicationProtocol{strategyir.ApplicationAny}
	strategy.Selector.TCPPorts = nil
	strategy.Selector.UDPPorts = []strategyir.PortRange{{Start: 443, End: 443}}
	report := evidence(attribution.FindingTLSPathFailureSuspected, observatory.StageHandshake, observatory.TransportTCP)
	if got := candidate(Plan(Request{Attribution: report, Backend: backendcap.Zapret2Windows, Strategies: []strategyir.Strategy{strategy}}), "udp"); got.Status != StatusInsufficientEvidence {
		t.Fatalf("raw UDP candidate guessed a TCP effect: %#v", got)
	}
}

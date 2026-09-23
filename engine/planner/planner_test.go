package planner

import (
	"encoding/json"
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

func TestTargetTrafficSelectorApplicability(t *testing.T) {
	report := evidence(attribution.FindingTLSPathFailureSuspected, observatory.StageHandshake, observatory.TransportTCP)
	request := Request{Attribution: report, Backend: backendcap.Zapret2Windows, Evidence: EvidenceContext{AddressFamily: observatory.AddressFamilyIPv4}}

	tls := tlsStrategy("tls-443")
	request.Strategies = []strategyir.Strategy{tls}
	if got := candidate(Plan(request), tls.ID); got.Status != StatusEligible {
		t.Fatalf("target 443 did not match 443 selector: %#v", got)
	}

	tls.Selector.TCPPorts = []strategyir.PortRange{{Start: 8443, End: 8443}}
	if got := candidate(Plan(Request{Attribution: report, Backend: backendcap.Zapret2Windows, Strategies: []strategyir.Strategy{tls}}), tls.ID); got.Status != StatusStructurallyInapplicable || got.Reasons[0].Code != "TARGET_PORT_MISMATCH" {
		t.Fatalf("target 443 matched 8443 selector: %#v", got)
	}

	report.Target.Port = "not-a-port"
	if got := candidate(Plan(requestWithStrategy(Request{Attribution: report, Backend: backendcap.Zapret2Windows}, tlsStrategy("unknown-port"))), "unknown-port"); got.Status != StatusInsufficientEvidence || got.Reasons[0].Code != "TARGET_PORT_UNKNOWN" {
		t.Fatalf("malformed target port did not fail closed: %#v", got)
	}
	report.Target.Port = "443"
	both := tlsStrategy("both")
	both.Selector.Direction = strategyir.DirectionBoth
	if got := candidate(Plan(requestWithStrategy(request, both)), both.ID); got.Status != StatusEligible {
		t.Fatalf("BOTH selector did not match outbound evidence: %#v", got)
	}

	inbound := tlsStrategy("inbound")
	inbound.Selector.Direction = strategyir.DirectionInbound
	if got := candidate(Plan(requestWithStrategy(request, inbound)), inbound.ID); got.Status != StatusStructurallyInapplicable || got.Reasons[0].Code != "TARGET_DIRECTION_MISMATCH" {
		t.Fatalf("inbound selector became applicable to outbound evidence: %#v", got)
	}

	ipv4 := tlsStrategy("ipv4")
	ipv4.Selector.IPFamilies = []strategyir.IPFamily{strategyir.IPFamilyV4}
	if got := candidate(Plan(requestWithStrategy(request, ipv4)), ipv4.ID); got.Status != StatusEligible {
		t.Fatalf("IPv4 selector did not match IPv4 evidence: %#v", got)
	}

	ipv6 := tlsStrategy("ipv6")
	ipv6.Selector.IPFamilies = []strategyir.IPFamily{strategyir.IPFamilyV6}
	if got := candidate(Plan(requestWithStrategy(request, ipv6)), ipv6.ID); got.Status != StatusStructurallyInapplicable || got.Reasons[0].Code != "TARGET_IP_FAMILY_MISMATCH" {
		t.Fatalf("IPv6 selector matched IPv4 evidence: %#v", got)
	}
	if got := candidate(Plan(requestWithStrategy(Request{Attribution: report, Backend: backendcap.Zapret2Windows}, ipv6)), ipv6.ID); got.Status != StatusInsufficientEvidence {
		t.Fatalf("specific family accepted unknown evidence family: %#v", got)
	}
	if got := candidate(Plan(requestWithStrategy(Request{Attribution: report, Backend: backendcap.Zapret2Windows}, tlsStrategy("any"))), "any"); got.Status != StatusEligible {
		t.Fatalf("ANY family required evidence narrowing: %#v", got)
	}
}

func requestWithStrategy(request Request, strategy strategyir.Strategy) Request {
	request.Strategies = []strategyir.Strategy{strategy}
	return request
}

func TestIPSetScopeSemantics(t *testing.T) {
	target := "target.test"
	for _, tc := range []struct {
		name  string
		scope strategyir.Scope
		data  ScopeSnapshot
		want  scopeMatch
	}{
		{
			name:  "exact IPv4 match",
			scope: strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeIPSetReference, ID: "edges"}},
			data:  ScopeSnapshot{IPSetMembers: map[string][]string{"edges": {"192.0.2.15"}}, TargetEdgeIPs: []string{"192.0.2.15"}},
			want:  scopeMatchOK,
		},
		{
			name:  "IPv4 CIDR match",
			scope: strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeIPSetReference, ID: "edges"}},
			data:  ScopeSnapshot{IPSetMembers: map[string][]string{"edges": {"192.0.2.0/24"}}, TargetEdgeIPs: []string{"192.0.2.15"}},
			want:  scopeMatchOK,
		},
		{
			name:  "IPv6 CIDR match",
			scope: strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeIPSetReference, ID: "edges"}},
			data:  ScopeSnapshot{IPSetMembers: map[string][]string{"edges": {"2001:db8::/32"}}, TargetEdgeIPs: []string{"2001:db8:1::1"}},
			want:  scopeMatchOK,
		},
		{
			name:  "second positive set matches",
			scope: strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeAll}, IPSetIDs: []string{"first", "second"}},
			data:  ScopeSnapshot{IPSetMembers: map[string][]string{"first": {"192.0.2.1"}, "second": {"192.0.2.15"}}, TargetEdgeIPs: []string{"192.0.2.15"}},
			want:  scopeMatchOK,
		},
		{
			name:  "all positive sets miss",
			scope: strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeAll}, IPSetIDs: []string{"first", "second"}},
			data:  ScopeSnapshot{IPSetMembers: map[string][]string{"first": {"192.0.2.1"}, "second": {"192.0.2.2"}}, TargetEdgeIPs: []string{"192.0.2.15"}},
			want:  scopeMismatch,
		},
		{
			name:  "positive miss with unknown set",
			scope: strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeAll}, IPSetIDs: []string{"known", "unknown"}},
			data:  ScopeSnapshot{IPSetMembers: map[string][]string{"known": {"192.0.2.1"}}, TargetEdgeIPs: []string{"192.0.2.15"}},
			want:  scopeUnknown,
		},
		{
			name:  "host and additional IP set both required",
			scope: strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeExplicit, Hosts: []string{target}}, IPSetIDs: []string{"edges"}},
			data:  ScopeSnapshot{IPSetMembers: map[string][]string{"edges": {"192.0.2.1"}}, TargetEdgeIPs: []string{"192.0.2.15"}},
			want:  scopeMismatch,
		},

		{
			name:  "IP set reference joins positive union",
			scope: strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeIPSetReference, ID: "first"}, IPSetIDs: []string{"second"}},
			data:  ScopeSnapshot{IPSetMembers: map[string][]string{"first": {"192.0.2.1"}, "second": {"192.0.2.15"}}, TargetEdgeIPs: []string{"192.0.2.15"}},
			want:  scopeMatchOK,
		},
		{
			name:  "malformed membership is unknown",
			scope: strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeIPSetReference, ID: "edges"}},
			data:  ScopeSnapshot{IPSetMembers: map[string][]string{"edges": {"not-an-address"}}, TargetEdgeIPs: []string{"192.0.2.15"}},
			want:  scopeUnknown,
		},
		{
			name:  "exclude CIDR vetoes",
			scope: strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeAll}, ExcludeIPSetIDs: []string{"excluded"}},
			data:  ScopeSnapshot{IPSetMembers: map[string][]string{"excluded": {"192.0.2.0/24"}}, TargetEdgeIPs: []string{"192.0.2.15"}},
			want:  scopeMismatch,
		},
		{
			name:  "known exclusion vetoes despite unknown exclusion",
			scope: strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeAll}, ExcludeIPSetIDs: []string{"unknown", "excluded"}},
			data:  ScopeSnapshot{IPSetMembers: map[string][]string{"excluded": {"192.0.2.15"}}, TargetEdgeIPs: []string{"192.0.2.15"}},
			want:  scopeMismatch,
		},
		{
			name:  "known host exclusion vetoes despite unknown exclusion",
			scope: strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeAll}, ExcludeHostListIDs: []string{"unknown", "excluded"}},
			data:  ScopeSnapshot{HostListMembers: map[string][]string{"excluded": {target}}},
			want:  scopeMismatch,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := MatchTargetScope(tc.scope, target, tc.data); got != tc.want {
				t.Fatalf("scope match = %d, want %d", got, tc.want)
			}
		})
	}
}

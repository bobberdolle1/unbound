package attribution

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"unbound/engine/observatory"
)

type storedCohort struct {
	Target   []observatory.ObservationResult `json:"target"`
	Controls []observatory.ObservationResult `json:"controls"`
}

func loadFixtures(t *testing.T) map[string]storedCohort {
	t.Helper()
	data, err := os.ReadFile("testdata/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures map[string]storedCohort
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func reportFor(t *testing.T, name string) AttributionReport {
	t.Helper()
	fixture, ok := loadFixtures(t)[name]
	if !ok {
		t.Fatalf("fixture %q not found", name)
	}
	return AnalyzeCohort(Cohort{Target: fixture.Target, Controls: fixture.Controls})
}

func finding(t *testing.T, report AttributionReport, code FindingCode) Finding {
	t.Helper()
	for _, candidate := range report.Findings {
		if candidate.Code == code {
			return candidate
		}
	}
	t.Fatalf("finding %s not present: %#v", code, report.Findings)
	return Finding{}
}

func TestSingleRunAttribution(t *testing.T) {
	for _, test := range []struct {
		name       string
		code       FindingCode
		confidence Confidence
	}{
		{"success", FindingNoAnomaly, ConfidenceHigh},
		{"dns_failure", FindingDNSPathFailureSuspected, ConfidenceLow},
		{"tcp_timeout", FindingTCPPathFailureSuspected, ConfidenceLow},
		{"tls_timeout", FindingTLSPathFailureSuspected, ConfidenceLow},
		{"http_403", FindingHTTPApplicationFailure, ConfidenceHigh},
		{"http_503", FindingHTTPApplicationFailure, ConfidenceHigh},
		{"quic_unsupported", FindingInsufficientEvidence, ConfidenceLow},
		{"tls_reset", FindingTLSResetObserved, ConfidenceHigh},
	} {
		t.Run(test.name, func(t *testing.T) {
			report := reportFor(t, test.name)
			got := finding(t, report, test.code)
			if got.Confidence != test.confidence {
				t.Fatalf("confidence = %s, want %s", got.Confidence, test.confidence)
			}
		})
	}
}

func TestHTTPFailuresAreCompletedApplicationPaths(t *testing.T) {
	for _, name := range []string{"http_403", "http_503"} {
		t.Run(name, func(t *testing.T) {
			report := reportFor(t, name)
			got := finding(t, report, FindingHTTPApplicationFailure)
			if got.Stage != observatory.StageHTTP || !slices.Contains(got.PrerequisitesMet, "path_complete") {
				t.Fatalf("HTTP finding = %#v", got)
			}
			if hasFinding(report, FindingTLSPathFailureSuspected) {
				t.Fatal("valid HTTP response became TLS failure attribution")
			}
		})
	}
}

func TestHealthyControlRaisesRepeatedTLSOnlyToMedium(t *testing.T) {
	fixture := loadFixtures(t)["healthy_control_tls"]
	target := cloneObservation(t, fixture.Target[0], "youtube-windows-repeat", time.Minute)
	control := cloneObservation(t, fixture.Controls[0], "cloudflare-windows-repeat", 2*time.Minute)
	report := AnalyzeCohort(Cohort{
		Target:   append(fixture.Target, target),
		Controls: append(fixture.Controls, control),
	})
	tls := finding(t, report, FindingTLSPathFailureSuspected)
	if tls.Confidence != ConfidenceMedium {
		t.Fatalf("TLS confidence = %s, want MEDIUM", tls.Confidence)
	}
	controlFinding := finding(t, report, FindingControlPathHealthy)
	if controlFinding.Confidence != ConfidenceHigh {
		t.Fatalf("control confidence = %s", controlFinding.Confidence)
	}
	if strings.Contains(tls.Summary, "DPI") {
		t.Fatal("control comparison inferred a forbidden DPI mechanism")
	}
}

func TestFailingControlDoesNotSupportTargetSpecificElevation(t *testing.T) {
	report := reportFor(t, "failing_control_tls")
	if got := finding(t, report, FindingNetworkContextFailure).Confidence; got != ConfidenceLow {
		t.Fatalf("network-context confidence = %s, want LOW when the control also fails", got)
	}
	finding(t, report, FindingControlPathDegraded)
}

func TestPhysicalDiscordFixtureRetainsEdgeFacts(t *testing.T) {
	data, err := os.ReadFile("testdata/discord-observation.json")
	if err != nil {
		t.Fatal(err)
	}
	var observation observatory.ObservationResult
	if err := json.Unmarshal(data, &observation); err != nil {
		t.Fatal(err)
	}
	report := Analyze([]observatory.ObservationResult{observation})
	if report.PrimaryFinding.Code != FindingEdgeDependentFailure {
		t.Fatalf("primary = %s, want edge-dependent", report.PrimaryFinding.Code)
	}
	if got := finding(t, report, FindingTLSPathFailureSuspected).Confidence; got != ConfidenceLow {
		t.Fatalf("uncontrolled Discord TLS confidence = %s, want LOW", got)
	}
}

func TestMixedEdgesRemainEdgeDependent(t *testing.T) {
	report := reportFor(t, "mixed_edges")
	if report.PrimaryFinding.Code != FindingEdgeDependentFailure {
		t.Fatalf("primary = %s, want edge-dependent", report.PrimaryFinding.Code)
	}
	finding(t, report, FindingTLSPathFailureSuspected)
}

func TestProfileComparisonLabels(t *testing.T) {
	for _, test := range []struct {
		name string
		code FindingCode
	}{
		{"profile_fixed", FindingFixedByProfile},
		{"profile_broken", FindingBrokenByProfile},
		{"profile_still_failing", FindingStillFailing},
		{"profile_reachable_directly", FindingReachableDirectly},
	} {
		t.Run(test.name, func(t *testing.T) {
			report := reportFor(t, test.name)
			if report.PrimaryFinding.Code != test.code {
				t.Fatalf("primary = %s, want %s", report.PrimaryFinding.Code, test.code)
			}
			if report.PrimaryFinding.Confidence != ConfidenceHigh {
				t.Fatalf("profile comparison confidence = %s", report.PrimaryFinding.Confidence)
			}
		})
	}
}

func TestIncompatibleContextsDoNotProduceProfileLabel(t *testing.T) {
	fixture := loadFixtures(t)["profile_fixed"]
	fixture.Target[1].NetworkContext.NetworkLabel = "mobile-hotspot"
	report := Analyze(fixture.Target)
	if hasFinding(report, FindingFixedByProfile) {
		t.Fatal("incompatible network contexts produced FIXED_BY_PROFILE")
	}
	if !containsLimitation(report, "outside the compatible") {
		t.Fatalf("missing incompatibility limitation: %#v", report.Limitations)
	}
}

func TestQUICAndResetRemainFactualAndConservative(t *testing.T) {
	quic := reportFor(t, "quic_unsupported")
	if quic.PrimaryFinding.Code != FindingInsufficientEvidence || hasFinding(quic, FindingTLSPathFailureSuspected) {
		t.Fatalf("QUIC report = %#v", quic)
	}
	reset := reportFor(t, "tls_reset")
	encoded := string(mustJSON(t, reset))
	if !hasFinding(reset, FindingTLSResetObserved) || strings.Contains(encoded, "RST_INJECTED") {
		t.Fatalf("reset report inferred injection: %s", encoded)
	}
}

func TestCounterevidenceDowngradesTLSFinding(t *testing.T) {
	fixture := loadFixtures(t)["healthy_control_tls"]
	success := loadFixtures(t)["success"].Target[0].Attempts[0]
	success.ResolvedIP = "142.251.157.4"
	fixture.Target[0].Attempts = append(fixture.Target[0].Attempts, success)
	report := AnalyzeCohort(Cohort{Target: fixture.Target, Controls: fixture.Controls})
	tls := finding(t, report, FindingTLSPathFailureSuspected)
	if tls.Confidence != ConfidenceLow || len(tls.Counterevidence) == 0 {
		t.Fatalf("counterevidence did not downgrade TLS finding: %#v", tls)
	}
	finding(t, report, FindingEdgeDependentFailure)
}

func TestDeterministicConfidenceAndResolverOrdering(t *testing.T) {
	fixture := loadFixtures(t)["healthy_control_tls"]
	first := AnalyzeCohort(Cohort{Target: fixture.Target, Controls: fixture.Controls})
	second := AnalyzeCohort(Cohort{Target: fixture.Target, Controls: fixture.Controls})
	if first.AttributionID != second.AttributionID || string(mustJSON(t, first.PrimaryFinding)) != string(mustJSON(t, second.PrimaryFinding)) {
		t.Fatalf("analysis is not deterministic: %#v %#v", first, second)
	}
	fixture.Target[0].Attempts[0], fixture.Target[0].Attempts[1] = fixture.Target[0].Attempts[1], fixture.Target[0].Attempts[0]
	for index := range fixture.Target[0].ResolvedAddresses {
		fixture.Target[0].ResolvedAddresses[index].ResolverOrder = len(fixture.Target[0].ResolvedAddresses) - index - 1
	}
	reordered := AnalyzeCohort(Cohort{Target: fixture.Target, Controls: fixture.Controls})
	if finding(t, first, FindingTLSPathFailureSuspected).Confidence != finding(t, reordered, FindingTLSPathFailureSuspected).Confidence {
		t.Fatal("resolver ordering alone changed cohort conclusion")
	}
}

func TestDeeperCompletedStageNeverCreatesShallowerFailure(t *testing.T) {
	fixture := loadFixtures(t)["tcp_timeout"]
	before := Analyze(fixture.Target)
	http := loadFixtures(t)["http_503"].Target[0]
	http.RunID = "later-http"
	fixture.Target = append(fixture.Target, http)
	after := Analyze(fixture.Target)
	if stageDepth(after.PrimaryFinding.Stage) < stageDepth(before.PrimaryFinding.Stage) {
		t.Fatalf("deeper evidence produced shallower primary: before=%#v after=%#v", before.PrimaryFinding, after.PrimaryFinding)
	}
}

func TestAnalysisDoesNotMutateObservationEvidence(t *testing.T) {
	fixture := loadFixtures(t)["healthy_control_tls"]
	before := mustJSON(t, fixture)
	_ = AnalyzeCohort(Cohort{Target: fixture.Target, Controls: fixture.Controls})
	if after := mustJSON(t, fixture); string(before) != string(after) {
		t.Fatal("pure attribution mutated Observatory evidence")
	}
}

func TestStructuralApplicability(t *testing.T) {
	tcpHello := StrategyCapabilities{AffectedStages: []AffectedStage{AffectedStageHello}, Transports: []observatory.Transport{observatory.TransportTCP}}
	quicHandshake := StrategyCapabilities{AffectedStages: []AffectedStage{AffectedStageHandshake}, Transports: []observatory.Transport{observatory.TransportQUIC}}
	httpOnly := StrategyCapabilities{AffectedStages: []AffectedStage{AffectedStageHTTP}, Transports: []observatory.Transport{observatory.TransportTCP}}
	if !CouldStrategyAffectFailure(tcpHello, observatory.StageHandshake, observatory.TransportTCP) {
		t.Fatal("TCP ClientHello capability must affect TCP handshake")
	}
	if CouldStrategyAffectFailure(tcpHello, observatory.StageResolve, observatory.TransportTCP) {
		t.Fatal("TCP ClientHello capability must not affect DNS")
	}
	if CouldStrategyAffectFailure(tcpHello, observatory.StageHandshake, observatory.TransportQUIC) {
		t.Fatal("TCP-only capability must not affect a QUIC handshake")
	}
	if !CouldStrategyAffectFailure(quicHandshake, observatory.StageHandshake, observatory.TransportQUIC) {
		t.Fatal("QUIC handshake capability must affect a QUIC handshake")
	}
	if CouldStrategyAffectFailure(quicHandshake, observatory.StageHandshake, observatory.TransportTCP) {
		t.Fatal("QUIC capability must not affect a TCP handshake")
	}
	if CouldStrategyAffectFailure(quicHandshake, observatory.StageConnect, observatory.TransportQUIC) {
		t.Fatal("QUIC unsupported at connect must not imply handshake applicability")
	}
	if CouldStrategyAffectFailure(httpOnly, observatory.StageHandshake, observatory.TransportTCP) {
		t.Fatal("HTTP-only capability must not affect TLS")
	}
}

func TestReportJSONRoundTrip(t *testing.T) {
	original := reportFor(t, "healthy_control_tls")
	data := mustJSON(t, original)
	var decoded AttributionReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SchemaVersion != SchemaVersion || decoded.PrimaryFinding.Code != original.PrimaryFinding.Code || len(decoded.EvidenceRefs) == 0 {
		t.Fatalf("round trip lost report evidence: %#v", decoded)
	}
}

func TestTargetIdentityIncludesPrivacySafeEndpointPath(t *testing.T) {
	left := observatory.ObservationResult{Target: observatory.Target{
		URL:               "https://user:secret@example.com:443/api/auth?token=one#fragment",
		Hostname:          "example.com",
		Port:              "443",
		RequestedProtocol: observatory.TransportTCP,
	}}
	samePath := left
	samePath.Target.URL = "https://other:credential@example.com/api/auth?token=two#other"
	differentPath := left
	differentPath.Target.URL = "https://example.com/generate_204"
	if !sameTarget(left, samePath) {
		t.Fatal("same host and path must be the same target")
	}
	if sameTarget(left, differentPath) {
		t.Fatal("different endpoint paths must not be pooled")
	}
	ref := targetRef(left.Target)
	encoded := string(mustJSON(t, ref))
	if ref.Scheme != "https" || ref.Path != "/api/auth" || strings.Contains(encoded, "secret") || strings.Contains(encoded, "token") || strings.Contains(encoded, "fragment") {
		t.Fatalf("target reference leaked URL material or lost endpoint identity: %s", encoded)
	}
	profile := loadFixtures(t)["profile_fixed"]
	profile.Target[0].Target.URL = "https://target.test/api/auth"
	profile.Target[1].Target.URL = "https://target.test/generate_204"
	if report := Analyze(profile.Target); hasFinding(report, FindingFixedByProfile) {
		t.Fatal("different endpoint paths produced a profile differential")
	}
}

func TestSameEdgeProfileDifferentialRetainsTargetEvidence(t *testing.T) {
	report := reportFor(t, "profile_fixed")
	if report.PrimaryFinding.Code != FindingFixedByProfile || report.PrimaryFinding.Confidence != ConfidenceHigh {
		t.Fatalf("same-edge profile differential = %#v", report.PrimaryFinding)
	}
	finding(t, report, FindingTCPPathFailureSuspected)
}

func TestProfileComparisonRequiresSameResolvedEdge(t *testing.T) {
	t.Run("same resolved set different selected edge", func(t *testing.T) {
		fixture := loadFixtures(t)["profile_fixed"]
		fixture.Target[0].ResolvedAddresses = []observatory.ResolvedAddress{
			{IP: "192.0.2.40", AddressFamily: observatory.AddressFamilyIPv4},
			{IP: "192.0.2.41", AddressFamily: observatory.AddressFamilyIPv4},
		}
		fixture.Target[1].ResolvedAddresses = append([]observatory.ResolvedAddress(nil), fixture.Target[0].ResolvedAddresses...)
		fixture.Target[1].Attempts[0].ResolvedIP = "192.0.2.41"
		report := Analyze(fixture.Target)
		if hasFinding(report, FindingFixedByProfile) || !containsLimitation(report, "same_resolved_edge") {
			t.Fatalf("different attempted edge produced A/B claim: %#v", report)
		}
	})
	t.Run("disjoint edge sets", func(t *testing.T) {
		fixture := loadFixtures(t)["profile_fixed"]
		fixture.Target[1].Attempts[0].ResolvedIP = "192.0.2.41"
		report := Analyze(fixture.Target)
		if hasFinding(report, FindingFixedByProfile) || !containsLimitation(report, "same_resolved_edge") {
			t.Fatalf("disjoint edges produced A/B claim: %#v", report)
		}
	})
	t.Run("overlap prefers same edge", func(t *testing.T) {
		fixture := loadFixtures(t)["profile_fixed"]
		success := cloneObservation(t, fixture.Target[1], "direct-common-edge", 0).Attempts[0]
		success.ResolvedIP = "192.0.2.41"
		fixture.Target[0].Attempts = append(fixture.Target[0].Attempts, success)
		fixture.Target[1].Attempts[0].ResolvedIP = "192.0.2.41"
		report := Analyze(fixture.Target)
		if report.PrimaryFinding.Code != FindingReachableDirectly || report.PrimaryFinding.Confidence != ConfidenceHigh {
			t.Fatalf("overlapping same edge was not selected: %#v", report.PrimaryFinding)
		}
	})
}

func TestIndependentControlsAndDuplicateRuns(t *testing.T) {
	fixture := loadFixtures(t)["healthy_control_tls"]
	single := AnalyzeCohort(Cohort{Target: fixture.Target, Controls: fixture.Controls})
	if got := finding(t, single, FindingTLSPathFailureSuspected).Confidence; got != ConfidenceLow {
		t.Fatalf("one target run plus one multi-attempt control run = %s, want LOW", got)
	}
	if got := finding(t, single, FindingControlPathHealthy).Confidence; got != ConfidenceMedium {
		t.Fatalf("one independent control run = %s, want MEDIUM", got)
	}
	duplicated := AnalyzeCohort(Cohort{
		Target:   append(append([]observatory.ObservationResult(nil), fixture.Target...), fixture.Target...),
		Controls: append(append([]observatory.ObservationResult(nil), fixture.Controls...), fixture.Controls...),
	})
	if got := finding(t, duplicated, FindingTLSPathFailureSuspected).Confidence; got != ConfidenceLow {
		t.Fatalf("duplicated run elevated confidence to %s", got)
	}
}

func TestDegradedControlsDoNotElevateTarget(t *testing.T) {
	fixture := loadFixtures(t)["healthy_control_tls"]
	target := cloneObservation(t, fixture.Target[0], "target-repeat", time.Minute)
	controlFailure := cloneObservation(t, fixture.Controls[0], "control-failure-a", time.Minute)
	controlFailure.FinalBoundary = observatory.StageHandshake
	controlFailure.Classification = observatory.ClassTLSHandshakeTimeout
	controlFailure.Attempts = append([]observatory.ConnectionAttempt(nil), fixture.Target[0].Attempts[:1]...)
	controlFailure.Attempts[0].ResolvedIP = "198.51.100.71"
	controlFailure2 := cloneObservation(t, controlFailure, "control-failure-b", 2*time.Minute)
	report := AnalyzeCohort(Cohort{Target: append(fixture.Target, target), Controls: []observatory.ObservationResult{controlFailure, controlFailure2}})
	if hasFinding(report, FindingControlPathHealthy) {
		t.Fatalf("degraded controls were labeled healthy: %#v", report.Findings)
	}
	if hasFinding(report, FindingTLSPathFailureSuspected) && finding(t, report, FindingTLSPathFailureSuspected).Confidence != ConfidenceLow {
		t.Fatalf("degraded controls elevated target TLS confidence: %#v", report.Findings)
	}
}

func TestCrossRunEdgeDependenceAndVariability(t *testing.T) {
	fixtures := loadFixtures(t)
	tls := fixtures["tls_timeout"].Target[0]
	success := fixtures["success"].Target[0]
	success.Target = tls.Target
	t.Run("stable edge-specific outcomes", func(t *testing.T) {
		a1 := cloneObservation(t, tls, "edge-a-1", 0)
		a2 := cloneObservation(t, tls, "edge-a-2", time.Minute)
		b1 := cloneObservation(t, success, "edge-b-1", 2*time.Minute)
		b2 := cloneObservation(t, success, "edge-b-2", 3*time.Minute)
		for _, observation := range []*observatory.ObservationResult{&a1, &a2} {
			observation.Attempts[0].ResolvedIP = "192.0.2.50"
		}
		for _, observation := range []*observatory.ObservationResult{&b1, &b2} {
			observation.Attempts[0].ResolvedIP = "192.0.2.51"
		}
		report := Analyze([]observatory.ObservationResult{a1, a2, b1, b2})
		if report.PrimaryFinding.Code != FindingEdgeDependentFailure {
			t.Fatalf("cross-run edge pattern = %#v", report.PrimaryFinding)
		}
	})
	t.Run("same edge variable outcomes", func(t *testing.T) {
		failure := cloneObservation(t, tls, "edge-variable-failure", 0)
		passed := cloneObservation(t, success, "edge-variable-success", time.Minute)
		failure.Attempts[0].ResolvedIP = "192.0.2.60"
		passed.Attempts[0].ResolvedIP = "192.0.2.60"
		report := Analyze([]observatory.ObservationResult{failure, passed})
		if hasFinding(report, FindingEdgeDependentFailure) {
			t.Fatalf("variable same edge was misattributed to edge identity: %#v", report.Findings)
		}
		if got := finding(t, report, FindingOutcomeVariability); got.Kind != FindingKindFact || got.Confidence != ConfidenceHigh {
			t.Fatalf("outcome variability finding = %#v", got)
		}
	})
}

func TestHeterogeneousFailuresReduceConfidence(t *testing.T) {
	fixtures := loadFixtures(t)
	tls := fixtures["tls_timeout"].Target[0]
	control := fixtures["success"].Target[0]
	control.Target = fixtures["healthy_control_tls"].Controls[0].Target
	controls := []observatory.ObservationResult{
		cloneObservation(t, control, "healthy-control-a", 0),
		cloneObservation(t, control, "healthy-control-b", time.Minute),
	}
	homogeneous := []observatory.ObservationResult{
		cloneObservation(t, tls, "tls-a", 0),
		cloneObservation(t, tls, "tls-b", time.Minute),
		cloneObservation(t, tls, "tls-c", 2*time.Minute),
	}
	if got := finding(t, AnalyzeCohort(Cohort{Target: homogeneous, Controls: controls}), FindingTLSPathFailureSuspected).Confidence; got != ConfidenceMedium {
		t.Fatalf("independent homogeneous TLS failures = %s, want MEDIUM", got)
	}
	tcp := cloneObservation(t, fixtures["tcp_timeout"].Target[0], "tcp-a", 3*time.Minute)
	tcp.Target = tls.Target
	tcp2 := cloneObservation(t, tcp, "tcp-b", 4*time.Minute)
	tcp3 := cloneObservation(t, tcp, "tcp-c", 5*time.Minute)
	heterogeneous := append(homogeneous, tcp, tcp2, tcp3)
	tlsFinding := finding(t, AnalyzeCohort(Cohort{Target: heterogeneous, Controls: controls}), FindingTLSPathFailureSuspected)
	if tlsFinding.Confidence != ConfidenceLow || len(tlsFinding.Counterevidence) == 0 {
		t.Fatalf("heterogeneous boundaries did not reduce confidence: %#v", tlsFinding)
	}
}

func cloneObservation(t *testing.T, source observatory.ObservationResult, runID string, offset time.Duration) observatory.ObservationResult {
	t.Helper()
	var clone observatory.ObservationResult
	if err := json.Unmarshal(mustJSON(t, source), &clone); err != nil {
		t.Fatal(err)
	}
	clone.RunID = runID
	clone.StartedAt = clone.StartedAt.Add(offset)
	clone.FinishedAt = clone.FinishedAt.Add(offset)
	return clone
}

func hasFinding(report AttributionReport, code FindingCode) bool {
	for _, candidate := range report.Findings {
		if candidate.Code == code {
			return true
		}
	}
	return false
}

func containsLimitation(report AttributionReport, fragment string) bool {
	for _, limitation := range report.Limitations {
		if strings.Contains(limitation, fragment) {
			return true
		}
	}
	return false
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

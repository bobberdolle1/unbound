package attribution

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

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
	report := reportFor(t, "healthy_control_tls")
	tls := finding(t, report, FindingTLSPathFailureSuspected)
	if tls.Confidence != ConfidenceMedium {
		t.Fatalf("TLS confidence = %s, want MEDIUM", tls.Confidence)
	}
	control := finding(t, report, FindingControlPathHealthy)
	if control.Confidence != ConfidenceHigh {
		t.Fatalf("control confidence = %s", control.Confidence)
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
	if !containsLimitation(report, "not compared") {
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
	if !CouldStrategyAffectFailure(tcpHello, observatory.StageHandshake, observatory.TransportTCP) {
		t.Fatal("ClientHello capability must affect TLS handshake")
	}
	if CouldStrategyAffectFailure(tcpHello, observatory.StageResolve, observatory.TransportTCP) {
		t.Fatal("ClientHello capability must not affect DNS")
	}
	if CouldStrategyAffectFailure(tcpHello, observatory.StageHandshake, observatory.TransportQUIC) {
		t.Fatal("TCP-only capability must not affect QUIC")
	}
	if CouldStrategyAffectFailure(StrategyCapabilities{AffectedStages: []AffectedStage{AffectedStageDNS}, Transports: []observatory.Transport{observatory.TransportTCP}}, observatory.StageHTTP, observatory.TransportTCP) {
		t.Fatal("DNS capability must not be credited for HTTP status")
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

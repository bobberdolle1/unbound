package autotunevnext

import (
	"context"
	"testing"

	"unbound/engine/observatory"
)

type recordingAdvisor struct {
	called   bool
	invalid  bool
	received AdvisorContext
}

func (advisor *recordingAdvisor) Advise(context AdvisorContext) Advice {
	advisor.called, advisor.received = true, context
	if advisor.invalid {
		return Advice{SchemaVersion: advisorSchemaVersion, DiagnosisID: context.Diagnosis.DiagnosisID, PlannerAttributionID: context.Planner.AttributionID, Candidates: []CandidateAdvice{{StrategyFingerprint: context.Eligible[0].StrategyFingerprint}, {StrategyFingerprint: context.Eligible[0].StrategyFingerprint}}}
	}
	out := Advice{SchemaVersion: advisorSchemaVersion, DiagnosisID: context.Diagnosis.DiagnosisID, PlannerAttributionID: context.Planner.AttributionID}
	for index := len(context.Eligible) - 1; index >= 0; index-- {
		out.Candidates = append(out.Candidates, CandidateAdvice{StrategyFingerprint: context.Eligible[index].StrategyFingerprint, ReasonCodes: []string{"COMPATIBLE_RECENT_NEGATIVE"}, HistoryUsed: true})
	}
	return out
}

func TestAdvisorRunsAfterFreshDiagnosisAndOrdersOnlyEligibleCandidates(t *testing.T) {
	low, broad := tlsStrategy("low"), tlsStrategy("broad")
	broad.Safety.TargetOnly = false
	req := request(low, broad)
	probe := testV2Probe(req.Target, observatory.ControlRolePrimary)
	advisor := &recordingAdvisor{}
	req.TargetProbe, req.Advisor, req.Policy.MaxCandidates = &probe, advisor, 1
	observer := &fakeObserver{results: []observatory.ObservationResult{
		observation("baseline", false, "192.0.2.1", req.Target.URL), observation("before", false, "192.0.2.1", req.Target.URL), observation("active", true, "192.0.2.1", req.Target.URL), observation("after", false, "192.0.2.1", req.Target.URL),
	}}
	result := Run(context.Background(), req, observer, &fakeExecutor{}, supportedPreflight{PreflightSupported}, fakeAssets{})
	if !advisor.called || result.DiagnosisReport.DiagnosisID == "" || len(advisor.received.Eligible) != 2 {
		t.Fatalf("advisor did not receive fresh diagnosed Planner candidates: %#v", result)
	}
	if len(result.Experiments) == 0 || result.Experiments[0].StrategyID != "broad" || result.Experiments[0].Outcome != OutcomeVerifiedFixed {
		t.Fatalf("valid advisor ordering was not applied: %#v", result.Experiments)
	}
	if len(result.OutcomeEvidence) != 1 || len(result.OutcomeEvidence[0].Validation) != 1 {
		t.Fatalf("validation provenance missing: %#v", result.OutcomeEvidence)
	}
}

func TestInvalidAdvisorFallsBackToSafetyOrder(t *testing.T) {
	low, broad := tlsStrategy("low"), tlsStrategy("broad")
	broad.Safety.TargetOnly = false
	req := request(low, broad)
	probe := testV2Probe(req.Target, observatory.ControlRolePrimary)
	advisor := &recordingAdvisor{invalid: true}
	req.TargetProbe, req.Advisor, req.Policy.MaxCandidates = &probe, advisor, 1
	observer := &fakeObserver{results: []observatory.ObservationResult{
		observation("baseline", false, "192.0.2.1", req.Target.URL), observation("before", false, "192.0.2.1", req.Target.URL), observation("active", true, "192.0.2.1", req.Target.URL), observation("after", false, "192.0.2.1", req.Target.URL),
	}}
	result := Run(context.Background(), req, observer, &fakeExecutor{}, supportedPreflight{PreflightSupported}, fakeAssets{})
	if result.Experiments[0].StrategyID != "low" {
		t.Fatalf("invalid advisor changed safety order: %#v", result.Experiments)
	}
	if !hasLimitation(result.Limitations, "RECOMMENDATION_INVALID_FALLBACK") {
		t.Fatalf("missing fallback limitation: %v", result.Limitations)
	}
}

func testV2Probe(target Target, role observatory.ControlRole) observatory.ProbeSpec {
	return observatory.ProbeSpec{SchemaVersion: observatory.ProbeSpecSchemaVersion, ID: "autotune-vnext-test-v1", ServiceID: "test", TargetContractRevision: "https-reachability-2xx-v1", Target: observatory.Target{URL: target.URL, Hostname: "blocked.test", Port: "443", RequestedProtocol: observatory.TransportTCP}, Transport: observatory.TransportTCP, AddressFamilyPolicy: target.AddressFamily, Mode: observatory.ProbeModeHTTPSGet, ExpectedResponse: observatory.ExpectedResponse{AllowedStatusCodes: []int{200}, RequirePathComplete: true}, ControlRole: role, Privacy: observatory.PrivacyModeRedacted}
}
func hasLimitation(limitations []string, wanted string) bool {
	for _, limitation := range limitations {
		if limitation == wanted {
			return true
		}
	}
	return false
}

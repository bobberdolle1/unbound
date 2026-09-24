package main

import (
	"context"
	"strings"
	"testing"

	"unbound/engine"
	"unbound/engine/attribution"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/planner"
	"unbound/engine/strategyir"
)

func productCatalog(t *testing.T, hostname string) []strategyir.Strategy {
	t.Helper()
	catalog, err := productionVNextStrategyCatalog(hostname)
	if err != nil {
		t.Fatal(err)
	}
	return catalog
}

func productCatalogEvidence(code attribution.FindingCode, stage observatory.Stage, hostname string) attribution.AttributionReport {
	finding := attribution.Finding{Code: code, Stage: stage, Kind: attribution.FindingKindFact}
	return attribution.AttributionReport{
		SchemaVersion: attribution.SchemaVersion,
		AttributionID: "production-catalog-test",
		Target: attribution.TargetRef{
			Hostname: hostname, Port: "443", RequestedProtocol: observatory.TransportTCP,
		},
		PrimaryFinding: finding,
		Findings:       []attribution.Finding{finding},
	}
}

func productCatalogCandidate(report planner.PlannerReport, id string) planner.CandidateAssessment {
	for _, candidate := range report.Candidates {
		if candidate.StrategyID == id {
			return candidate
		}
	}
	return planner.CandidateAssessment{}
}

func TestProductionVNextCatalogIsAuditedAndTargetLocal(t *testing.T) {
	catalog := productCatalog(t, "a.example.com")
	if len(catalog) != 3 || len(catalog) > autotunevnext.DefaultPolicy().MaxCandidates {
		t.Fatalf("catalog size=%d", len(catalog))
	}
	wantIDs := []string{"prod-tls-multisplit-1-v1", "prod-tls-multisplit-overlap-v1", "prod-tls-hostfakesplit-v1"}
	fingerprints := map[string]struct{}{}
	for index, strategy := range catalog {
		if strategy.ID != wantIDs[index] {
			t.Fatalf("catalog[%d] id=%q, want %q", index, strategy.ID, wantIDs[index])
		}
		if err := strategyir.Validate(strategy); err != nil {
			t.Fatalf("%s validation: %v", strategy.ID, err)
		}
		canonical, err := strategyir.Canonicalize(strategy)
		if err != nil {
			t.Fatalf("%s canonicalization: %v", strategy.ID, err)
		}
		fingerprint, err := strategyir.Fingerprint(canonical)
		if err != nil {
			t.Fatalf("%s fingerprint: %v", strategy.ID, err)
		}
		if _, exists := fingerprints[fingerprint]; exists {
			t.Fatalf("duplicate fingerprint %s", fingerprint)
		}
		fingerprints[fingerprint] = struct{}{}
		if len(strategy.Transport) != 1 || strategy.Transport[0] != strategyir.TransportTCP || len(strategy.Selector.ApplicationProtocols) != 1 || strategy.Selector.ApplicationProtocols[0] != strategyir.ApplicationTLS || strategy.Selector.Direction != strategyir.DirectionOutbound || len(strategy.Selector.TCPPorts) != 1 || strategy.Selector.TCPPorts[0] != (strategyir.PortRange{Start: 443, End: 443}) || len(strategy.Selector.UDPPorts) != 0 {
			t.Fatalf("%s has non-V1 traffic selector: %#v", strategy.ID, strategy.Selector)
		}
		if strategy.Selector.Scope.Host.Mode != strategyir.HostScopeExplicit || len(strategy.Selector.Scope.Host.Hosts) != 1 || strategy.Selector.Scope.Host.Hosts[0] != "a.example.com" || strategy.Selector.Scope.Host.ID != "" || len(strategy.Selector.Scope.ExcludeHostListIDs) != 0 || len(strategy.Selector.Scope.IPSetIDs) != 0 || len(strategy.Selector.Scope.ExcludeIPSetIDs) != 0 {
			t.Fatalf("%s has broad or non-local scope: %#v", strategy.ID, strategy.Selector.Scope)
		}
		if !strategy.Safety.TargetOnly || strategy.Safety.MayAffectSteam || strategy.Safety.Aggressiveness == "HIGH" || strategy.Metadata.Source == "" || strategy.Metadata.Description == "" {
			t.Fatalf("%s safety/provenance=%#v %#v", strategy.ID, strategy.Safety, strategy.Metadata)
		}
		for _, operation := range strategy.Operations {
			if operation.Type == strategyir.OperationFakeInjection || operation.Type == strategyir.OperationMultiDisorder {
				t.Fatalf("%s promoted high-risk fake TLS operation %s", strategy.ID, operation.Type)
			}
		}
		switch strategy.ID {
		case "prod-tls-multisplit-1-v1":
			if len(strategy.Operations) != 1 || strategy.Operations[0].Type != strategyir.OperationMultiSplit || len(strategy.Operations[0].Positions) != 1 || strategy.Operations[0].Positions[0].Absolute == nil || *strategy.Operations[0].Positions[0].Absolute != 1 || strategy.Operations[0].Fake != nil || strategy.Range != nil {
				t.Fatalf("unexpected simple multisplit semantics: %#v", strategy)
			}
		case "prod-tls-multisplit-overlap-v1":
			operation := strategy.Operations[0]
			if len(strategy.Operations) != 1 || operation.Type != strategyir.OperationMultiSplit || len(operation.Positions) != 1 || operation.Positions[0].Absolute == nil || *operation.Positions[0].Absolute != 2 || operation.SequenceOverlap == nil || *operation.SequenceOverlap != 652 || operation.OverlapPatternRef != "tls-google" || strategy.Range == nil || strategy.Range.Direction != strategyir.RangeDirectionOut || strategy.Range.Counter != strategyir.RangeCounterDataPacketNumber || strategy.Range.Limit != 8 {
				t.Fatalf("unexpected overlap multisplit semantics: %#v", strategy)
			}
		case "prod-tls-hostfakesplit-v1":
			operation := strategy.Operations[0]
			if len(strategy.Operations) != 1 || operation.Type != strategyir.OperationHostFakeSplit || len(operation.Positions) != 1 || operation.Positions[0].Anchor != strategyir.AnchorMidSLD || operation.HostTemplate != "ozon.ru" || operation.Fake == nil || operation.Fake.Repeat != 4 || !operation.Fake.TCPMD5 || !operation.Fake.TCPTimestamp || strategy.Range == nil || strategy.Range.Direction != strategyir.RangeDirectionOut || strategy.Range.Counter != strategyir.RangeCounterDataPacketNumber || strategy.Range.Limit != 8 {
				t.Fatalf("unexpected host fake split semantics: %#v", strategy)
			}
		}
	}
	if catalog[0].Safety.Aggressiveness != "LOW" || catalog[1].Safety.Aggressiveness != "MEDIUM" || catalog[2].Safety.Aggressiveness != "MEDIUM" {
		t.Fatalf("catalog safety order=%q,%q,%q", catalog[0].Safety.Aggressiveness, catalog[1].Safety.Aggressiveness, catalog[2].Safety.Aggressiveness)
	}
}

func TestProductionVNextCatalogIsDeterministicAndHostScoped(t *testing.T) {
	first, second := productCatalog(t, "a.example.com"), productCatalog(t, "a.example.com")
	root := productCatalog(t, "example.com")
	for _, strategy := range root {
		if strategy.Selector.Scope.Host.Hosts[0] != "example.com" {
			t.Fatalf("root target scope=%v", strategy.Selector.Scope.Host.Hosts)
		}
	}
	other := productCatalog(t, "other.example.com")
	for index := range first {
		left, err := strategyir.Fingerprint(first[index])
		if err != nil {
			t.Fatal(err)
		}
		right, err := strategyir.Fingerprint(second[index])
		if err != nil {
			t.Fatal(err)
		}
		otherFingerprint, err := strategyir.Fingerprint(other[index])
		if err != nil {
			t.Fatal(err)
		}
		if first[index].ID != second[index].ID || left != right {
			t.Fatalf("catalog is non-deterministic: %#v %#v", first[index], second[index])
		}
		if left == otherFingerprint || other[index].Selector.Scope.Host.Hosts[0] != "other.example.com" {
			t.Fatalf("target hostname did not constrain executable semantics: %q / %q", left, otherFingerprint)
		}
	}
}

func TestProductionVNextCatalogCompilesForExactExecutorCapture(t *testing.T) {
	catalog := productCatalog(t, "a.example.com")
	wantAssets := map[string][]string{
		"prod-tls-multisplit-1-v1":       nil,
		"prod-tls-multisplit-overlap-v1": {"tls-google"},
		"prod-tls-hostfakesplit-v1":      nil,
	}
	for _, strategy := range catalog {
		for _, backend := range []backendcap.Backend{backendcap.Zapret2Windows, backendcap.Zapret2Linux} {
			compiled := backendcap.Compile(strategy, backend)
			if compiled.Status != backendcap.StatusCompiled {
				t.Fatalf("%s %s compile=%#v", strategy.ID, backend, compiled)
			}
			if strategy.ID == "prod-tls-hostfakesplit-v1" && compiled.DerivedRequirements.TCPTimestamps != engine.TimestampsRequired {
				t.Fatalf("%s %s weakened TCP timestamp preflight: %#v", strategy.ID, backend, compiled.DerivedRequirements)
			}
			capture := compiled.Plan.Capture
			if capture.Transport != backendcap.CaptureTransportTCP || capture.Direction != strategyir.DirectionOutbound || len(capture.TCPPorts) != 1 || capture.TCPPorts[0] != (strategyir.PortRange{Start: 443, End: 443}) || len(capture.UDPPorts) != 0 {
				t.Fatalf("%s %s capture=%#v", strategy.ID, backend, capture)
			}
			for _, arg := range compiled.Plan.EngineArgv {
				if strings.HasPrefix(arg, "--wf-") {
					t.Fatalf("%s %s EngineArgv leaked capture flag %q", strategy.ID, backend, arg)
				}
			}
			if len(compiled.RequiredAssets) != len(wantAssets[strategy.ID]) {
				t.Fatalf("%s %s required assets=%v", strategy.ID, backend, compiled.RequiredAssets)
			}
			for index, asset := range wantAssets[strategy.ID] {
				if compiled.RequiredAssets[index] != asset || strings.ContainsAny(asset, `/\\`) {
					t.Fatalf("%s %s required assets=%v", strategy.ID, backend, compiled.RequiredAssets)
				}
			}
		}
	}
}

func TestProductionVNextCatalogPlannerApplicability(t *testing.T) {
	catalog := productCatalog(t, "a.example.com")
	cases := []struct {
		name        string
		report      attribution.AttributionReport
		want        planner.Disposition
		candidate   planner.CandidateStatus
		candidateID string
	}{
		{"tls handshake", productCatalogEvidence(attribution.FindingTLSPathFailureSuspected, observatory.StageHandshake, "a.example.com"), planner.DispositionCandidatesAvailable, planner.StatusEligible, "prod-tls-multisplit-1-v1"},
		{"direct success", productCatalogEvidence(attribution.FindingReachableDirectly, observatory.StageHTTP, "a.example.com"), planner.DispositionNoActionNeeded, "", ""},
		{"http application", productCatalogEvidence(attribution.FindingHTTPApplicationFailure, observatory.StageHTTP, "a.example.com"), planner.DispositionNoPacketStrategyIndicated, "", ""},
		{"scope mismatch", productCatalogEvidence(attribution.FindingTLSPathFailureSuspected, observatory.StageHandshake, "other.example.com"), planner.DispositionNoCompatibleCandidates, planner.StatusTargetScopeMismatch, "prod-tls-multisplit-1-v1"},
		{"connect only", productCatalogEvidence(attribution.FindingTCPPathFailureSuspected, observatory.StageConnect, "a.example.com"), planner.DispositionNoCompatibleCandidates, planner.StatusStructurallyInapplicable, "prod-tls-multisplit-1-v1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report := planner.Plan(planner.Request{Attribution: tc.report, Backend: backendcap.Zapret2Windows, Strategies: catalog})
			if report.Disposition != tc.want {
				t.Fatalf("disposition=%s want=%s report=%#v", report.Disposition, tc.want, report)
			}
			if tc.candidateID != "" && productCatalogCandidate(report, tc.candidateID).Status != tc.candidate {
				t.Fatalf("candidate=%#v want=%s", productCatalogCandidate(report, tc.candidateID), tc.candidate)
			}
		})
	}
}

func TestProductVNextReportsReadyCatalogOnlyAfterConstruction(t *testing.T) {
	manager, _ := productVNextTestManager(t)
	var received autotunevnext.Request
	service := newProductVNextServiceWith(manager, &engine.AssetPaths{}, productVNextDependencies{
		newRuntime: func(autotunevnext.RuntimeProvider, *engine.AssetPaths, func(autotunevnext.PhysicalLog)) (productVNextRuntime, error) {
			return productVNextRuntime{executor: &productVNextNoopExecutor{}, preflight: productVNextSupportedPreflight{}, backend: backendcap.Zapret2Windows}, nil
		},
		newResolver: func(*engine.AssetPaths) (autotunevnext.AssetResolver, error) { return productVNextAssets{}, nil },
		observer:    productVNextObserver{},
		run: func(_ context.Context, request autotunevnext.Request, _ autotunevnext.Observer, _ autotunevnext.Executor, _ autotunevnext.HostPreflight, _ autotunevnext.AssetResolver) (autotunevnext.Result, error) {
			received = request
			return autotunevnext.Result{Status: autotunevnext.StatusCompletedNoEligibleCandidates, Backend: request.Backend}, nil
		},
	})
	result := service.Run(context.Background(), AutoTuneVNextRequest{Target: "https://a.example.com/path?token=secret"})
	if result.CatalogStatus != productVNextCatalogStatus || len(received.Strategies) != 3 || received.Strategies[0].Selector.Scope.Host.Hosts[0] != "a.example.com" {
		t.Fatalf("result=%#v request=%#v", result, received)
	}
	if failed := service.Run(context.Background(), AutoTuneVNextRequest{Target: "http://a.example.com/"}); failed.CatalogStatus != productVNextCatalogStatusUnavailable {
		t.Fatalf("invalid target reported ready catalog: %#v", failed)
	}
}

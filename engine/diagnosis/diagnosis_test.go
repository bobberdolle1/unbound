package diagnosis

import (
	"testing"
	"time"

	"unbound/engine/attribution"
	"unbound/engine/observatory"
)

func TestDiagnoseMappingsAndDeterminism(t *testing.T) {
	for _, test := range []struct {
		name  string
		code  attribution.FindingCode
		stage observatory.Stage
		class observatory.Classification
		want  Kind
	}{
		{"no anomaly", attribution.FindingNoAnomaly, observatory.StageHTTP, observatory.ClassSuccess, KindNoAnomaly},
		{"dns", attribution.FindingDNSPathFailureSuspected, observatory.StageResolve, observatory.ClassDNSFailure, KindDNSPathAnomaly},
		{"tcp", attribution.FindingTCPPathFailureSuspected, observatory.StageConnect, observatory.ClassTCPConnectTimeout, KindTCPPathFailure},
		{"tls", attribution.FindingTLSPathFailureSuspected, observatory.StageHandshake, observatory.ClassTLSHandshakeTimeout, KindTLSHandshakePathFailure},
		{"edge", attribution.FindingEdgeDependentFailure, observatory.StageConnect, observatory.ClassTCPConnectTimeout, KindEdgeDependentFailure},
		{"network", attribution.FindingNetworkContextFailure, observatory.StageConnect, observatory.ClassTCPConnectTimeout, KindNetworkContextFailure},
	} {
		t.Run(test.name, func(t *testing.T) {
			targetRecord := record(t, "target-"+test.name, observatory.TransportTCP, test.stage, test.class, nil)
			finding := attribution.Finding{Code: test.code, Confidence: attribution.ConfidenceLow, Stage: test.stage, SupportingEvidence: []attribution.EvidenceRef{{RunID: targetRecord.Runs[0].Observation.RunID, AttemptIndex: 0, Stage: test.stage}}}
			input := inputFor(targetRecord, finding)
			if test.want == KindNetworkContextFailure {
				control := record(t, "control-"+test.name, observatory.TransportTCP, test.stage, test.class, nil)
				finding.SupportingEvidence = append(finding.SupportingEvidence, attribution.EvidenceRef{RunID: control.Runs[0].Observation.RunID, AttemptIndex: 0, Stage: test.stage})
				input.Controls = []observatory.EvidenceRecord{control}
				input.Attribution.InputRunIDs = []string{targetRecord.Runs[0].Observation.RunID, control.Runs[0].Observation.RunID}
				input.Attribution.PrimaryFinding = finding
				input.Attribution.Findings = []attribution.Finding{finding}
			}
			left, err := Diagnose(input)
			if err != nil {
				t.Fatal(err)
			}
			right, err := Diagnose(input)
			if err != nil {
				t.Fatal(err)
			}
			if left.Kind != test.want || left.DiagnosisID != right.DiagnosisID || left.Confidence != attribution.ConfidenceLow {
				t.Fatalf("report=%#v", left)
			}
		})
	}
}

func TestDiagnoseHonorsHTTPContractAndTargetIsolation(t *testing.T) {
	allowed := httpRecord(t, "allowed-403", 403, []int{403})
	httpFinding := attribution.Finding{Code: attribution.FindingHTTPApplicationFailure, Confidence: attribution.ConfidenceHigh, Stage: observatory.StageHTTP, SupportingEvidence: []attribution.EvidenceRef{{RunID: allowed.Runs[0].Observation.RunID, AttemptIndex: 0, Stage: observatory.StageHTTP}}}
	report, err := Diagnose(inputFor(allowed, httpFinding))
	if err != nil || report.Kind == KindHTTPApplicationFailure {
		t.Fatalf("allowed 403 overclaimed: %#v %v", report, err)
	}
	matching := inputFor(allowed, attribution.Finding{Code: attribution.FindingNoAnomaly, Confidence: attribution.ConfidenceMedium, Stage: observatory.StageHTTP, SupportingEvidence: httpFinding.SupportingEvidence})
	report, err = Diagnose(matching)
	if err != nil || report.Kind != KindNoAnomaly || report.Confidence != attribution.ConfidenceMedium {
		t.Fatalf("matching response=%#v %v", report, err)
	}
	rejected := httpRecord(t, "rejected-403", 403, []int{204})
	rejectedFinding := attribution.Finding{Code: attribution.FindingHTTPApplicationFailure, Confidence: attribution.ConfidenceHigh, Stage: observatory.StageHTTP, SupportingEvidence: []attribution.EvidenceRef{{RunID: rejected.Runs[0].Observation.RunID, AttemptIndex: 0, Stage: observatory.StageHTTP}}}
	report, err = Diagnose(inputFor(rejected, rejectedFinding))
	if err != nil || report.Kind != KindHTTPApplicationFailure {
		t.Fatalf("rejected 403=%#v %v", report, err)
	}

	target := record(t, "tcp-target", observatory.TransportTCP, observatory.StageHTTP, observatory.ClassSuccess, nil)
	quic := record(t, "quic-control", observatory.TransportQUIC, observatory.StageConnect, observatory.ClassQUICUnsupported, nil)
	udp := record(t, "udp-control", observatory.TransportUDP, observatory.StageConnect, observatory.ClassUDPUnsupported, nil)
	noAnomaly := attribution.Finding{Code: attribution.FindingNoAnomaly, Confidence: attribution.ConfidenceLow, Stage: observatory.StageHTTP, SupportingEvidence: []attribution.EvidenceRef{{RunID: target.Runs[0].Observation.RunID, AttemptIndex: 0, Stage: observatory.StageHTTP}}}
	input := inputFor(target, noAnomaly)
	input.Controls = []observatory.EvidenceRecord{quic, udp}
	input.Attribution.InputRunIDs = []string{target.Runs[0].Observation.RunID, quic.Runs[0].Observation.RunID, udp.Runs[0].Observation.RunID}
	first, err := Diagnose(input)
	input.Controls[0], input.Controls[1] = input.Controls[1], input.Controls[0]
	second, secondErr := Diagnose(input)
	if err != nil || secondErr != nil || first.Kind != KindNoAnomaly || first.DiagnosisID != second.DiagnosisID {
		t.Fatalf("unsupported controls=%#v %#v %v %v", first, second, err, secondErr)
	}
}

func TestDiagnoseTransferCohortAndCorrelation(t *testing.T) {
	partial1 := record(t, "partial-1", observatory.TransportTCP, observatory.StageHTTP, observatory.ClassSuccess, &observatory.TransferMeasurement{Status: observatory.MeasurementPartial, ExpectedBytes: 16, ActualBytes: 8})
	partial2 := record(t, "partial-2", observatory.TransportTCP, observatory.StageHTTP, observatory.ClassSuccess, &observatory.TransferMeasurement{Status: observatory.MeasurementPartial, ExpectedBytes: 16, ActualBytes: 7})
	control1 := withControlRole(t, record(t, "control-1", observatory.TransportTCP, observatory.StageHTTP, observatory.ClassSuccess, &observatory.TransferMeasurement{Status: observatory.MeasurementCompleted, ExpectedBytes: 16, ActualBytes: 16, Completed: true}))
	control2 := withControlRole(t, record(t, "control-2", observatory.TransportTCP, observatory.StageHTTP, observatory.ClassSuccess, &observatory.TransferMeasurement{Status: observatory.MeasurementCompleted, ExpectedBytes: 16, ActualBytes: 16, Completed: true}))
	insufficient := attribution.Finding{Code: attribution.FindingInsufficientEvidence, Confidence: attribution.ConfidenceLow}
	healthy := attribution.Finding{Code: attribution.FindingControlPathHealthy, Confidence: attribution.ConfidenceHigh, Stage: observatory.StageHTTP, SupportingEvidence: []attribution.EvidenceRef{{RunID: control1.Runs[0].Observation.RunID, AttemptIndex: 0, Stage: observatory.StageHTTP}, {RunID: control2.Runs[0].Observation.RunID, AttemptIndex: 0, Stage: observatory.StageHTTP}}}
	input := inputFor(partial1, insufficient)
	input.Target = []observatory.EvidenceRecord{partial2, partial1}
	input.Controls = []observatory.EvidenceRecord{control2, control1}
	input.Attribution.Findings = []attribution.Finding{insufficient, healthy}
	input.Attribution.InputRunIDs = []string{partial1.Runs[0].Observation.RunID, partial2.Runs[0].Observation.RunID, control1.Runs[0].Observation.RunID, control2.Runs[0].Observation.RunID}
	first, err := Diagnose(input)
	input.Target[0], input.Target[1] = input.Target[1], input.Target[0]
	input.Controls[0], input.Controls[1] = input.Controls[1], input.Controls[0]
	second, secondErr := Diagnose(input)
	if err != nil || secondErr != nil || first.Kind != KindPartialTransferAnomaly || len(first.SupportingEvidence) != 2 || first.DiagnosisID != second.DiagnosisID {
		t.Fatalf("transfer=%#v %#v %v %v", first, second, err, secondErr)
	}
	input.Controls[0] = record(t, "primary-control", observatory.TransportTCP, observatory.StageHTTP, observatory.ClassSuccess, &observatory.TransferMeasurement{Status: observatory.MeasurementCompleted, ExpectedBytes: 16, ActualBytes: 16, Completed: true})
	input.Attribution.InputRunIDs = []string{partial1.Runs[0].Observation.RunID, partial2.Runs[0].Observation.RunID, input.Controls[0].Runs[0].Observation.RunID, control2.Runs[0].Observation.RunID}
	input.Attribution.Findings[1].SupportingEvidence = []attribution.EvidenceRef{{RunID: control2.Runs[0].Observation.RunID, AttemptIndex: 0, Stage: observatory.StageHTTP}}
	report, err := Diagnose(input)
	if err != nil || report.Kind == KindPartialTransferAnomaly {
		t.Fatalf("primary control transfer=%#v %v", report, err)
	}

	input.Controls = []observatory.EvidenceRecord{control1, control2}
	input.Attribution.InputRunIDs = []string{partial1.Runs[0].Observation.RunID, partial2.Runs[0].Observation.RunID, control1.Runs[0].Observation.RunID, control2.Runs[0].Observation.RunID}
	input.Attribution.Findings = []attribution.Finding{insufficient, healthy, {Code: attribution.FindingControlPathDegraded, Confidence: attribution.ConfidenceLow, Stage: observatory.StageHTTP, SupportingEvidence: []attribution.EvidenceRef{{RunID: control1.Runs[0].Observation.RunID, AttemptIndex: 0, Stage: observatory.StageHTTP}}}}
	report, err = Diagnose(input)
	if err != nil || report.Kind == KindPartialTransferAnomaly {
		t.Fatalf("degraded control transfer=%#v %v", report, err)
	}

	bad := inputFor(partial1, attribution.Finding{Code: attribution.FindingTCPPathFailureSuspected, Confidence: attribution.ConfidenceLow})
	bad.Attribution.InputRunIDs = []string{"missing"}
	if _, err := Diagnose(bad); err == nil {
		t.Fatal("unknown attribution run accepted")
	}
	bad = inputFor(partial1, attribution.Finding{Code: attribution.FindingNoAnomaly, Confidence: attribution.ConfidenceLow})
	bad.Attribution.Target.Scheme = "http"
	if _, err := Diagnose(bad); err == nil {
		t.Fatal("scheme mismatch accepted")
	}
}

func TestDiagnoseRejectsMixedTargetFamiliesAndPreservesLimitations(t *testing.T) {
	ipv4 := withAnyFamily(t, record(t, "v4", observatory.TransportTCP, observatory.StageHTTP, observatory.ClassSuccess, nil), observatory.AddressFamilyIPv4)
	ipv6 := withAnyFamily(t, record(t, "v6", observatory.TransportTCP, observatory.StageHTTP, observatory.ClassSuccess, nil), observatory.AddressFamilyIPv6)
	finding := attribution.Finding{Code: attribution.FindingNoAnomaly, Confidence: attribution.ConfidenceLow}
	input := inputFor(ipv4, finding)
	input.Target = []observatory.EvidenceRecord{ipv4, ipv6}
	input.Attribution.InputRunIDs = []string{ipv4.Runs[0].Observation.RunID, ipv6.Runs[0].Observation.RunID}
	if _, err := Diagnose(input); err == nil {
		t.Fatal("mixed selected address families accepted")
	}
	input.Target = []observatory.EvidenceRecord{ipv4}
	input.Attribution.InputRunIDs = []string{ipv4.Runs[0].Observation.RunID}
	input.Attribution.Limitations = []string{"upstream caveat", "upstream caveat"}
	report, err := Diagnose(input)
	if err != nil || len(report.Limitations) != 1 || report.Limitations[0] != "upstream caveat" {
		t.Fatalf("limitations=%#v %v", report.Limitations, err)
	}
}

func record(t *testing.T, id string, transport observatory.Transport, boundary observatory.Stage, class observatory.Classification, transfer *observatory.TransferMeasurement) observatory.EvidenceRecord {
	t.Helper()
	port := "443"
	spec := observatory.ProbeSpec{SchemaVersion: observatory.ProbeSpecSchemaVersion, ID: "probe", ServiceID: "svc", TargetContractRevision: "v1", Target: observatory.Target{URL: "https://example.test/ok", Hostname: "example.test", Port: port, RequestedProtocol: transport}, Transport: transport, AddressFamilyPolicy: observatory.AddressFamilyIPv4, Mode: observatory.ProbeModeHTTPSGet, ExpectedResponse: observatory.ExpectedResponse{AllowedStatusCodes: []int{204}, RequirePathComplete: true}, ControlRole: observatory.ControlRolePrimary, Privacy: observatory.PrivacyModeRedacted}
	if transfer != nil {
		spec.Transfer = &observatory.TransferContract{AuditID: "audit", ResponseContractRevision: "v1", ExpectedBytes: 16}
	}
	primary := 0
	now := time.Unix(10, 0).UTC()
	stages := []observatory.StageEvidence{{Stage: observatory.StageResolve, Status: observatory.StatusPass, Class: observatory.ClassSuccess}, {Stage: observatory.StageConnect, Status: observatory.StatusPass}, {Stage: observatory.StageHello, Status: observatory.StatusPass}, {Stage: observatory.StageHandshake, Status: observatory.StatusPass}, {Stage: observatory.StageHTTP, Status: observatory.StatusPass, HTTPStatus: 204, PathComplete: true}}
	if boundary != observatory.StageHTTP {
		stages = []observatory.StageEvidence{{Stage: observatory.StageResolve, Status: observatory.StatusPass, Class: observatory.ClassSuccess}, {Stage: boundary, Status: observatory.StatusFail, Class: class}}
	}
	if transport == observatory.TransportQUIC || transport == observatory.TransportUDP {
		stages = []observatory.StageEvidence{{Stage: observatory.StageResolve, Status: observatory.StatusPass, Class: observatory.ClassSuccess}, {Stage: observatory.StageConnect, Status: observatory.StatusSkippedUnsupported, Class: class}}
		boundary = observatory.StageConnect
	}
	obs := observatory.ObservationResult{SchemaVersion: observatory.SchemaVersion, RunID: id, StartedAt: now, FinishedAt: now.Add(time.Second), Target: spec.Target, Attempts: []observatory.ConnectionAttempt{{ResolvedIP: "192.0.2.1", AddressFamily: observatory.AddressFamilyIPv4, Transport: transport, Stages: stages}}, PrimaryAttemptIndex: &primary, FinalBoundary: boundary, Classification: class, ExecutionContext: observatory.ExecutionContext{Mode: "direct"}}
	record, err := observatory.BuildEvidenceRecord(spec, observatory.EvidenceInput{Observations: []observatory.ObservationResult{obs}, Transfer: transfer})
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func httpRecord(t *testing.T, id string, status int, allowed []int) observatory.EvidenceRecord {
	t.Helper()
	port := "443"
	spec := observatory.ProbeSpec{SchemaVersion: observatory.ProbeSpecSchemaVersion, ID: "probe-http", ServiceID: "svc", TargetContractRevision: "v1", Target: observatory.Target{URL: "https://example.test/ok", Hostname: "example.test", Port: port, RequestedProtocol: observatory.TransportTCP}, Transport: observatory.TransportTCP, AddressFamilyPolicy: observatory.AddressFamilyIPv4, Mode: observatory.ProbeModeHTTPSGet, ExpectedResponse: observatory.ExpectedResponse{AllowedStatusCodes: allowed, RequirePathComplete: true}, ControlRole: observatory.ControlRolePrimary, Privacy: observatory.PrivacyModeRedacted}
	primary := 0
	now := time.Unix(10, 0).UTC()
	stages := []observatory.StageEvidence{{Stage: observatory.StageResolve, Status: observatory.StatusPass}, {Stage: observatory.StageConnect, Status: observatory.StatusPass}, {Stage: observatory.StageHello, Status: observatory.StatusPass}, {Stage: observatory.StageHandshake, Status: observatory.StatusPass}, {Stage: observatory.StageHTTP, Status: observatory.StatusPass, HTTPStatus: status, PathComplete: true}}
	observation := observatory.ObservationResult{SchemaVersion: observatory.SchemaVersion, RunID: id, StartedAt: now, FinishedAt: now.Add(time.Second), Target: spec.Target, Attempts: []observatory.ConnectionAttempt{{ResolvedIP: "192.0.2.1", AddressFamily: observatory.AddressFamilyIPv4, Transport: observatory.TransportTCP, Stages: stages}}, PrimaryAttemptIndex: &primary, FinalBoundary: observatory.StageHTTP, Classification: observatory.ClassHTTPStatus, ExecutionContext: observatory.ExecutionContext{Mode: "direct"}}
	record, err := observatory.BuildEvidenceRecord(spec, observatory.EvidenceInput{Observations: []observatory.ObservationResult{observation}})
	if err != nil {
		t.Fatal(err)
	}
	return record
}

func withControlRole(t *testing.T, evidence observatory.EvidenceRecord) observatory.EvidenceRecord {
	t.Helper()
	return rebuildRecord(t, evidence, observatory.ControlRoleHealthy, evidence.Probe.AddressFamilyPolicy, "")
}

func withAnyFamily(t *testing.T, evidence observatory.EvidenceRecord, family observatory.AddressFamily) observatory.EvidenceRecord {
	t.Helper()
	address := "192.0.2.1"
	if family == observatory.AddressFamilyIPv6 {
		address = "2001:db8::1"
	}
	return rebuildRecord(t, evidence, evidence.Probe.ControlRole, observatory.AddressFamilyAny, address)
}

func rebuildRecord(t *testing.T, evidence observatory.EvidenceRecord, role observatory.ControlRole, policy observatory.AddressFamily, address string) observatory.EvidenceRecord {
	t.Helper()
	spec := evidence.Probe
	spec.ControlRole = role
	spec.AddressFamilyPolicy = policy
	observations := make([]observatory.ObservationResult, 0, len(evidence.Runs))
	for _, run := range evidence.Runs {
		observation := run.Observation
		for index := range observation.Attempts {
			if address != "" {
				observation.Attempts[index].ResolvedIP = address
				if address == "2001:db8::1" {
					observation.Attempts[index].AddressFamily = observatory.AddressFamilyIPv6
				} else {
					observation.Attempts[index].AddressFamily = observatory.AddressFamilyIPv4
				}
			}
		}
		observations = append(observations, observation)
	}
	input := observatory.EvidenceInput{Observations: observations}
	if spec.Transfer != nil {
		transfer := evidence.Transfer
		input.Transfer = &transfer
	}
	record, err := observatory.BuildEvidenceRecord(spec, input)
	if err != nil {
		t.Fatal(err)
	}
	return record
}
func inputFor(record observatory.EvidenceRecord, finding attribution.Finding) Input {
	target := record.Probe.Target
	return Input{Target: []observatory.EvidenceRecord{record}, Attribution: attribution.AttributionReport{SchemaVersion: attribution.SchemaVersion, AttributionID: "attr", InputRunIDs: []string{record.Runs[0].Observation.RunID}, Target: attribution.TargetRef{Scheme: "https", Hostname: target.Hostname, Port: target.Port, Path: "/ok", RequestedProtocol: target.RequestedProtocol}, PrimaryFinding: finding, Findings: []attribution.Finding{finding}}}
}

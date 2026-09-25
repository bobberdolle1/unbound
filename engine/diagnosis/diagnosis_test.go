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
		{"http", attribution.FindingHTTPApplicationFailure, observatory.StageHTTP, observatory.ClassHTTPStatus, KindHTTPApplicationFailure},
		{"edge", attribution.FindingEdgeDependentFailure, observatory.StageConnect, observatory.ClassTCPConnectTimeout, KindEdgeDependentFailure},
		{"network", attribution.FindingNetworkContextFailure, observatory.StageConnect, observatory.ClassTCPConnectTimeout, KindNetworkContextFailure},
	} {
		t.Run(test.name, func(t *testing.T) {
			record := record(t, "target-"+test.name, observatory.TransportTCP, test.stage, test.class, nil)
			input := inputFor(record, attribution.Finding{Code: test.code, Confidence: attribution.ConfidenceLow, Stage: test.stage, SupportingEvidence: []attribution.EvidenceRef{{RunID: record.Runs[0].Observation.RunID, AttemptIndex: 0, Stage: test.stage}}})
			if test.want == KindNoAnomaly {
				input.Attribution.Findings[0].Confidence = attribution.ConfidenceHigh
			}
			left, err := Diagnose(input)
			if err != nil {
				t.Fatal(err)
			}
			right, err := Diagnose(input)
			if err != nil {
				t.Fatal(err)
			}
			if left.Kind != test.want || left.DiagnosisID != right.DiagnosisID || left.Confidence != input.Attribution.Findings[0].Confidence {
				t.Fatalf("report=%#v", left)
			}
		})
	}
}

func TestDiagnoseConservativeUnsupportedTransferAndCorrelation(t *testing.T) {
	quic := record(t, "quic", observatory.TransportQUIC, observatory.StageConnect, observatory.ClassQUICUnsupported, nil)
	report, err := Diagnose(inputFor(quic, attribution.Finding{Code: attribution.FindingInsufficientEvidence, Confidence: attribution.ConfidenceLow}))
	if err != nil || report.Kind != KindInsufficientEvidence || len(report.PrerequisitesMissing) == 0 {
		t.Fatalf("QUIC=%#v %v", report, err)
	}
	udp := record(t, "udp", observatory.TransportUDP, observatory.StageConnect, observatory.ClassUDPUnsupported, nil)
	report, err = Diagnose(inputFor(udp, attribution.Finding{Code: attribution.FindingInsufficientEvidence, Confidence: attribution.ConfidenceLow}))
	if err != nil || report.Kind != KindInsufficientEvidence {
		t.Fatalf("UDP=%#v %v", report, err)
	}

	partial1 := record(t, "partial-1", observatory.TransportTCP, observatory.StageHTTP, observatory.ClassSuccess, &observatory.TransferMeasurement{Status: observatory.MeasurementPartial, ExpectedBytes: 16, ActualBytes: 8})
	partial2 := record(t, "partial-2", observatory.TransportTCP, observatory.StageHTTP, observatory.ClassSuccess, &observatory.TransferMeasurement{Status: observatory.MeasurementPartial, ExpectedBytes: 16, ActualBytes: 7})
	control := record(t, "control", observatory.TransportTCP, observatory.StageHTTP, observatory.ClassSuccess, &observatory.TransferMeasurement{Status: observatory.MeasurementCompleted, ExpectedBytes: 16, ActualBytes: 16, Completed: true})
	in := inputFor(partial1, attribution.Finding{Code: attribution.FindingInsufficientEvidence, Confidence: attribution.ConfidenceLow})
	in.Target = []observatory.EvidenceRecord{partial2, partial1}
	in.Controls = []observatory.EvidenceRecord{control}
	in.Attribution.InputRunIDs = []string{partial1.Runs[0].Observation.RunID, partial2.Runs[0].Observation.RunID, control.Runs[0].Observation.RunID}
	report, err = Diagnose(in)
	if err != nil || report.Kind != KindPartialTransferAnomaly {
		t.Fatalf("transfer=%#v %v", report, err)
	}

	bad := inputFor(partial1, attribution.Finding{Code: attribution.FindingTCPPathFailureSuspected, Confidence: attribution.ConfidenceLow})
	bad.Attribution.InputRunIDs = []string{"missing"}
	if _, err := Diagnose(bad); err == nil {
		t.Fatal("unknown attribution run accepted")
	}
	unknown := inputFor(record(t, "unknown", observatory.TransportTCP, observatory.StageConnect, observatory.ClassTCPConnectTimeout, nil), attribution.Finding{})
	report, err = Diagnose(unknown)
	if err != nil || report.Kind != KindUnknown {
		t.Fatalf("unknown=%#v %v", report, err)
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
func inputFor(record observatory.EvidenceRecord, finding attribution.Finding) Input {
	target := record.Probe.Target
	return Input{Target: []observatory.EvidenceRecord{record}, Attribution: attribution.AttributionReport{SchemaVersion: attribution.SchemaVersion, AttributionID: "attr", InputRunIDs: []string{record.Runs[0].Observation.RunID}, Target: attribution.TargetRef{Scheme: "https", Hostname: target.Hostname, Port: target.Port, Path: "/ok", RequestedProtocol: target.RequestedProtocol}, PrimaryFinding: finding, Findings: []attribution.Finding{finding}}}
}

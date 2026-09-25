package observatory

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestProbeSpecValidationAndStableIdentity(t *testing.T) {
	spec := testProbeSpec(t, "https://example.test/generate_204", TransportTCP, AddressFamilyAny)
	identity, err := spec.Identity()
	if err != nil {
		t.Fatal(err)
	}
	reordered := spec
	reordered.ExpectedResponse.AllowedStatusCodes = []int{204, 200}
	spec.ExpectedResponse.AllowedStatusCodes = []int{200, 204}
	reorderedIdentity, err := reordered.Identity()
	if err != nil {
		t.Fatal(err)
	}
	stable, err := spec.Identity()
	if err != nil || stable == "" {
		t.Fatalf("stable identity = %q, %v", stable, err)
	}
	if stable != reorderedIdentity {
		t.Fatal("canonical identity changed with status-code order")
	}
	different := spec
	different.ExpectedResponse.AllowedStatusCodes = []int{204}
	differentIdentity, err := different.Identity()
	if err != nil || identity != differentIdentity || stable == differentIdentity {
		t.Fatalf("expected response semantics identity = %q, %q, %v", identity, differentIdentity, err)
	}

	revision := spec
	revision.TargetContractRevision = "target-v2"
	revisionIdentity, err := revision.Identity()
	if err != nil || revisionIdentity == stable {
		t.Fatalf("revision identity = %q, %v", revisionIdentity, err)
	}
	for _, invalid := range []ProbeSpec{
		func() ProbeSpec { value := spec; value.SchemaVersion = ProbeSpecSchemaVersion + 1; return value }(),
		func() ProbeSpec { value := spec; value.Transport = Transport("sctp"); return value }(),
		func() ProbeSpec { value := spec; value.Target.URL += "?secret=value"; return value }(),
		func() ProbeSpec {
			value := spec
			value.ExpectedResponse.AllowedStatusCodes = []int{204, 204}
			return value
		}(),
		func() ProbeSpec {
			value := spec
			value.Transfer = &TransferContract{AuditID: "audited", ResponseContractRevision: "v1"}
			return value
		}(),
	} {
		if err := invalid.Validate(); err == nil {
			t.Fatalf("invalid probe spec accepted: %#v", invalid)
		}
	}
	for _, transport := range []Transport{TransportQUIC, TransportUDP} {
		unsupported := testProbeSpec(t, "https://localhost:443/", transport, AddressFamilyIPv4)
		if err := unsupported.Validate(); err != nil {
			t.Fatalf("%s spec must remain representable: %v", transport, err)
		}
	}
}

func TestQUICHandshakeFailureClassificationRemainsCompatible(t *testing.T) {
	if ClassQUICHandshakeFailure != "QUIC_HANDSHAKE_FAILURE" {
		t.Fatalf("QUIC handshake failure classification changed: %q", ClassQUICHandshakeFailure)
	}
}

func TestObserveProbeWrapsV1TCPHTTPSEvidence(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	server.StartTLS()
	defer server.Close()

	spec := testProbeSpec(t, server.URL, TransportTCP, AddressFamilyIPv4)
	record, err := NewDirectTCPHTTPSObserver().ObserveProbe(context.Background(), spec, Options{
		ResolvedIP:    net.ParseIP("127.0.0.1"),
		AddressFamily: AddressFamilyIPv4,
		tlsConfig:     serverTLSConfig(server),
	}, EvidenceInput{ExperimentID: "fixture-experiment"})
	if err != nil {
		t.Fatal(err)
	}
	if err := record.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(record.Runs) != 1 || record.Runs[0].Measurement != MeasurementCompleted || record.Runs[0].Response.ExpectedSemantics != ResponseSemanticsMatch {
		t.Fatalf("record = %#v", record)
	}
	run := record.Runs[0]
	if run.Observation.SchemaVersion != SchemaVersion || run.SelectedEdge != "127.0.0.1" || run.AddressFamily != AddressFamilyIPv4 || run.Observation.Target.URL != spec.Target.URL {
		t.Fatalf("V1 observation compatibility lost: %#v", run)
	}
	if run.Response.HTTPStatus != http.StatusNoContent || !run.Response.PathComplete || run.Timing.ElapsedMS < 0 {
		t.Fatalf("HTTP/timing facts = %#v %#v", run.Response, run.Timing)
	}
	if record.Transfer.Status != MeasurementNotRequested {
		t.Fatalf("unrequested transfer = %#v", record.Transfer)
	}
	audited := spec
	audited.Transfer = &TransferContract{AuditID: "audited-fixture", ResponseContractRevision: "v1", ExpectedBytes: 16 * 1024}
	record, err = NewDirectTCPHTTPSObserver().ObserveProbe(context.Background(), audited, Options{
		ResolvedIP:    net.ParseIP("127.0.0.1"),
		AddressFamily: AddressFamilyIPv4,
		tlsConfig:     serverTLSConfig(server),
	}, EvidenceInput{})
	if err != nil {
		t.Fatal(err)
	}
	if record.Transfer.Status != MeasurementUnsupported || record.Transfer.ExpectedBytes != 16*1024 {
		t.Fatalf("unaudited body reader was implied: %#v", record.Transfer)
	}
}

func TestEvidencePreservesIPv4AndIPv6SelectedFamilies(t *testing.T) {
	for _, test := range []struct {
		name   string
		family AddressFamily
		ip     string
	}{
		{name: "ipv4", family: AddressFamilyIPv4, ip: "192.0.2.10"},
		{name: "ipv6", family: AddressFamilyIPv6, ip: "2001:db8::10"},
	} {
		t.Run(test.name, func(t *testing.T) {
			spec := testProbeSpec(t, "https://example.test/ok", TransportTCP, test.family)
			record, err := BuildEvidenceRecord(spec, EvidenceInput{Observations: []ObservationResult{testObservation(spec, "run-"+test.name, test.ip, test.family, ClassSuccess, completedStages(204))}})
			if err != nil {
				t.Fatal(err)
			}
			run := record.Runs[0]
			if run.SelectedEdge != test.ip || run.AddressFamily != test.family || run.Observation.Attempts[0].AddressFamily != test.family {
				t.Fatalf("family collapsed: %#v", run)
			}
		})
	}
}

func TestEvidenceEnforcesAddressFamilyPolicy(t *testing.T) {
	for _, test := range []struct {
		name    string
		policy  AddressFamily
		family  AddressFamily
		ip      string
		wantErr bool
	}{
		{name: "ipv4 accepts ipv4", policy: AddressFamilyIPv4, family: AddressFamilyIPv4, ip: "192.0.2.10"},
		{name: "ipv6 accepts ipv6", policy: AddressFamilyIPv6, family: AddressFamilyIPv6, ip: "2001:db8::10"},
		{name: "any accepts ipv4", policy: AddressFamilyAny, family: AddressFamilyIPv4, ip: "192.0.2.10"},
		{name: "any accepts ipv6", policy: AddressFamilyAny, family: AddressFamilyIPv6, ip: "2001:db8::10"},
		{name: "ipv4 rejects ipv6", policy: AddressFamilyIPv4, family: AddressFamilyIPv6, ip: "2001:db8::10", wantErr: true},
		{name: "ipv6 rejects ipv4", policy: AddressFamilyIPv6, family: AddressFamilyIPv4, ip: "192.0.2.10", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			spec := testProbeSpec(t, "https://example.test/ok", TransportTCP, test.policy)
			observation := testObservation(spec, "family-"+test.name, test.ip, test.family, ClassSuccess, completedStages(http.StatusNoContent))
			_, err := BuildEvidenceRecord(spec, EvidenceInput{Observations: []ObservationResult{observation}})
			if (err != nil) != test.wantErr {
				t.Fatalf("BuildEvidenceRecord error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}

	spec := testProbeSpec(t, "https://example.test/ok", TransportTCP, AddressFamilyIPv4)
	dnsFailure := ObservationResult{SchemaVersion: SchemaVersion, RunID: "dns-no-selected-attempt", Target: spec.Target, FinalBoundary: StageResolve, Classification: ClassDNSFailure}
	if _, err := BuildEvidenceRecord(spec, EvidenceInput{Observations: []ObservationResult{dnsFailure}}); err != nil {
		t.Fatalf("DNS failure without a selected attempt was rejected: %v", err)
	}
	invalidIndex := testObservation(spec, "invalid-primary", "192.0.2.10", AddressFamilyIPv4, ClassSuccess, completedStages(http.StatusNoContent))
	invalid := 1
	invalidIndex.PrimaryAttemptIndex = &invalid
	if _, err := BuildEvidenceRecord(spec, EvidenceInput{Observations: []ObservationResult{invalidIndex}}); err == nil {
		t.Fatal("invalid primary attempt index was accepted")
	}
}

func TestEvidenceRunOrderingIsChronologicalAndCanonical(t *testing.T) {
	spec := testProbeSpec(t, "https://example.test/ok", TransportTCP, AddressFamilyIPv4)
	earlier := testObservation(spec, "z-earlier", "192.0.2.10", AddressFamilyIPv4, ClassSuccess, completedStages(http.StatusNoContent))
	earlier.StartedAt = time.Unix(10, 0).UTC()
	earlier.FinishedAt = time.Unix(11, 0).UTC()
	later := testObservation(spec, "a-later", "192.0.2.11", AddressFamilyIPv4, ClassSuccess, completedStages(http.StatusNoContent))
	later.StartedAt = time.Unix(12, 0).UTC()
	later.FinishedAt = time.Unix(13, 0).UTC()

	left, err := BuildEvidenceRecord(spec, EvidenceInput{Observations: []ObservationResult{later, earlier}})
	if err != nil {
		t.Fatal(err)
	}
	right, err := BuildEvidenceRecord(spec, EvidenceInput{Observations: []ObservationResult{earlier, later}})
	if err != nil {
		t.Fatal(err)
	}
	if left.Fingerprint != right.Fingerprint || left.Runs[0].Observation.RunID != "z-earlier" || left.Runs[1].Observation.RunID != "a-later" {
		t.Fatalf("runs were not canonically chronological: %#v", left.Runs)
	}
	transportMismatch := testObservation(spec, "transport-mismatch", "192.0.2.10", AddressFamilyIPv4, ClassSuccess, completedStages(http.StatusNoContent))
	transportMismatch.Attempts[0].Transport = TransportQUIC
	if _, err := BuildEvidenceRecord(spec, EvidenceInput{Observations: []ObservationResult{transportMismatch}}); err == nil {
		t.Fatal("mismatched attempt transport was accepted")
	}
	missingBoundary := testObservation(spec, "missing-boundary", "192.0.2.10", AddressFamilyIPv4, ClassSuccess, completedStages(http.StatusNoContent))
	missingBoundary.FinalBoundary = StageCarry
	if _, err := BuildEvidenceRecord(spec, EvidenceInput{Observations: []ObservationResult{missingBoundary}}); err == nil {
		t.Fatal("selected attempt without final boundary was accepted")
	}
}

func TestEvidenceDNSAndTCPFailuresStayFactual(t *testing.T) {
	spec := testProbeSpec(t, "https://example.test/ok", TransportTCP, AddressFamilyAny)
	dns := testObservation(spec, "dns-no-answer", "", "", ClassDNSFailure, []StageEvidence{{Stage: StageResolve, Status: StatusFail, Class: ClassDNSFailure, Error: "no_usable_addresses"}})
	dns.FinalBoundary = StageResolve
	dns.Attempts[0].Transport = TransportTCP
	record, err := BuildEvidenceRecord(spec, EvidenceInput{Observations: []ObservationResult{dns}})
	if err != nil {
		t.Fatal(err)
	}
	serialized, err := MarshalEvidenceRecord(record)
	if err != nil {
		t.Fatal(err)
	}
	if record.Runs[0].Measurement != MeasurementFailed || bytes.Contains(serialized, []byte("INTERCEPTION")) || bytes.Contains(serialized, []byte("DNS_PATH_ANOMALY")) {
		t.Fatalf("DNS evidence overstated a conclusion: %s", serialized)
	}

	for _, test := range []struct {
		name   string
		status Status
		class  Classification
		want   MeasurementStatus
	}{
		{name: "connect failure", status: StatusFail, class: ClassTCPConnectionRefused, want: MeasurementFailed},
		{name: "connect timeout", status: StatusTimeout, class: ClassTCPConnectTimeout, want: MeasurementPartial},
	} {
		t.Run(test.name, func(t *testing.T) {
			observation := testObservation(spec, "tcp-"+test.name, "192.0.2.10", AddressFamilyIPv4, test.class, []StageEvidence{{Stage: StageResolve, Status: StatusPass, Class: ClassSuccess}, {Stage: StageConnect, Status: test.status, Class: test.class}})
			observation.FinalBoundary = StageConnect
			record, err := BuildEvidenceRecord(spec, EvidenceInput{Observations: []ObservationResult{observation}})
			if err != nil {
				t.Fatal(err)
			}
			if got := record.Runs[0].Measurement; got != test.want {
				t.Fatalf("measurement = %s, want %s", got, test.want)
			}
		})
	}
}

func TestEvidencePreservesTLSFactsWithoutFingerprintAttribution(t *testing.T) {
	spec := testProbeSpec(t, "https://example.test/ok", TransportTCP, AddressFamilyIPv4)
	hello := time.Unix(10, 0).UTC()
	success := testObservation(spec, "tls-success", "192.0.2.10", AddressFamilyIPv4, ClassSuccess, []StageEvidence{
		{Stage: StageResolve, Status: StatusPass, Class: ClassSuccess},
		{Stage: StageConnect, Status: StatusPass},
		{Stage: StageHello, Status: StatusPass, HelloSentAt: &hello},
		{Stage: StageHandshake, Status: StatusPass, TLSVersion: "TLS1.3", PeerCertificateSHA256: "certificate-fingerprint"},
		{Stage: StageHTTP, Status: StatusPass, HTTPStatus: 204, PathComplete: true},
	})
	record, err := BuildEvidenceRecord(spec, EvidenceInput{Observations: []ObservationResult{success}})
	if err != nil {
		t.Fatal(err)
	}
	stages := record.Runs[0].Observation.Attempts[0].Stages
	if stages[2].HelloSentAt == nil || stages[3].PeerCertificateSHA256 != "certificate-fingerprint" || record.Runs[0].Measurement != MeasurementCompleted {
		t.Fatalf("TLS facts were not preserved: %#v", stages)
	}

	failure := testObservation(spec, "tls-failure", "192.0.2.10", AddressFamilyIPv4, ClassTLSProtocolFailure, []StageEvidence{
		{Stage: StageResolve, Status: StatusPass, Class: ClassSuccess},
		{Stage: StageConnect, Status: StatusPass},
		{Stage: StageHello, Status: StatusPass, HelloSentAt: &hello},
		{Stage: StageHandshake, Status: StatusFail, Class: ClassTLSProtocolFailure},
	})
	failure.FinalBoundary = StageHandshake
	failed, err := BuildEvidenceRecord(spec, EvidenceInput{Observations: []ObservationResult{failure}})
	if err != nil {
		t.Fatal(err)
	}
	data, err := MarshalEvidenceRecord(failed)
	if err != nil {
		t.Fatal(err)
	}
	if failed.Runs[0].Measurement != MeasurementFailed || bytes.Contains(data, []byte("TLS_FINGERPRINT_FILTERING")) {
		t.Fatalf("TLS evidence inferred filtering: %s", data)
	}
}

func TestEvidenceResponseSemanticsAndTransferContracts(t *testing.T) {
	spec := testProbeSpec(t, "https://example.test/ok", TransportTCP, AddressFamilyIPv4)
	observation := testObservation(spec, "http-response", "192.0.2.10", AddressFamilyIPv4, ClassHTTPStatus, completedStages(http.StatusForbidden))
	record, err := BuildEvidenceRecord(spec, EvidenceInput{Observations: []ObservationResult{observation}})
	if err != nil {
		t.Fatal(err)
	}
	if record.Runs[0].Measurement != MeasurementCompleted || record.Runs[0].Response.ExpectedSemantics != ResponseSemanticsMismatch || !record.Runs[0].Response.PathComplete {
		t.Fatalf("unexpected HTTP response representation: %#v", record.Runs[0])
	}

	transferSpec := spec
	transferSpec.Transfer = &TransferContract{AuditID: "audited-16k", ResponseContractRevision: "response-v1", ExpectedBytes: 16 * 1024}
	if err := transferSpec.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name  string
		input *TransferMeasurement
		want  MeasurementStatus
	}{
		{name: "not requested", input: nil, want: MeasurementNotRequested},
		{name: "unsupported", input: &TransferMeasurement{Status: MeasurementUnsupported, ExpectedBytes: 16 * 1024}, want: MeasurementUnsupported},
		{name: "complete exact", input: &TransferMeasurement{Status: MeasurementCompleted, ExpectedBytes: 16 * 1024, ActualBytes: 16 * 1024, Completed: true, FirstByteMS: &[]int64{1}[0], TotalMS: &[]int64{4}[0]}, want: MeasurementCompleted},
		{name: "partial", input: &TransferMeasurement{Status: MeasurementPartial, ExpectedBytes: 16 * 1024, ActualBytes: 1024, FirstByteMS: &[]int64{1}[0], TotalMS: &[]int64{4}[0]}, want: MeasurementPartial},
	} {
		t.Run(test.name, func(t *testing.T) {
			record, err := BuildEvidenceRecord(transferSpec, EvidenceInput{Observations: []ObservationResult{observation}, Transfer: test.input})
			if err != nil {
				t.Fatal(err)
			}
			if record.Transfer.Status != test.want {
				t.Fatalf("transfer = %#v", record.Transfer)
			}
		})
	}
	complete, err := BuildEvidenceRecord(transferSpec, EvidenceInput{Observations: []ObservationResult{observation}, Transfer: &TransferMeasurement{Status: MeasurementCompleted, ExpectedBytes: 16 * 1024, ActualBytes: 16 * 1024, Completed: true}})
	if err != nil {
		t.Fatal(err)
	}
	complete.Transfer.ExpectedBytes = 0
	complete.Fingerprint, err = complete.canonicalFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if err := complete.Validate(); err == nil {
		t.Fatal("missing serialized transfer expected-length fact accepted")
	}
	invalid := transferSpec
	invalid.Transfer = &TransferContract{AuditID: "audited", ResponseContractRevision: "v1", ExpectedBytes: 0}
	if err := invalid.Validate(); err == nil {
		t.Fatal("zero expected transfer length accepted")
	}
	if _, err := BuildEvidenceRecord(transferSpec, EvidenceInput{Observations: []ObservationResult{observation}, Transfer: &TransferMeasurement{Status: MeasurementCompleted, ExpectedBytes: 16 * 1024, ActualBytes: 1, Completed: true}}); err == nil {
		t.Fatal("short completed transfer accepted")
	}
}

func TestObserveProbeKeepsQUICAndUDPExplicitlyUnsupported(t *testing.T) {
	for _, test := range []struct {
		transport Transport
		class     Classification
	}{
		{transport: TransportQUIC, class: ClassQUICUnsupported},
		{transport: TransportUDP, class: ClassUDPUnsupported},
	} {
		t.Run(string(test.transport), func(t *testing.T) {
			spec := testProbeSpec(t, "https://localhost:443/", test.transport, AddressFamilyIPv4)
			record, err := NewDirectTCPHTTPSObserver().ObserveProbe(context.Background(), spec, Options{ResolvedIP: net.ParseIP("127.0.0.1")}, EvidenceInput{})
			if err != nil {
				t.Fatal(err)
			}
			if record.Runs[0].Measurement != MeasurementUnsupported || record.Runs[0].Observation.Classification != test.class {
				t.Fatalf("unsupported transport evidence = %#v", record.Runs[0])
			}
		})
	}
}

func TestEvidenceSerializationRedactionAndCanonicalFingerprint(t *testing.T) {
	spec := testProbeSpec(t, "https://example.test/ok", TransportTCP, AddressFamilyIPv4)
	first := testObservation(spec, "z-run", "192.0.2.10", AddressFamilyIPv4, ClassSuccess, completedStages(204))
	first.Target.URL = "https://user:password@example.test/ok?token=secret#fragment-secret"
	first.NetworkContext = NetworkContext{Interface: "Ethernet secret", LocalAddress: "10.0.0.2", DefaultGateway: "10.0.0.1", ConfiguredResolvers: []string{"10.0.0.53"}, NetworkLabel: "private-network"}
	first.Attempts[0].LocalAddress = "10.0.0.2:12345"
	first.Attempts[0].Stages[len(first.Attempts[0].Stages)-1].Detail = "raw-response-body secret"
	first.Attempts[0].Stages[len(first.Attempts[0].Stages)-1].ResponseHeaders = map[string]string{"authorization": "Bearer secret", "content-type": "text/plain"}
	second := testObservation(spec, "a-run", "192.0.2.11", AddressFamilyIPv4, ClassSuccess, completedStages(204))

	left, err := BuildEvidenceRecord(spec, EvidenceInput{Observations: []ObservationResult{first, second}, StrategyFingerprint: "strategy-fingerprint", BackendFingerprint: "backend-fingerprint"})
	if err != nil {
		t.Fatal(err)
	}
	right, err := BuildEvidenceRecord(spec, EvidenceInput{Observations: []ObservationResult{second, first}, StrategyFingerprint: "strategy-fingerprint", BackendFingerprint: "backend-fingerprint"})
	if err != nil {
		t.Fatal(err)
	}
	if left.Fingerprint != right.Fingerprint {
		t.Fatalf("canonical fingerprint changed with input order: %s != %s", left.Fingerprint, right.Fingerprint)
	}
	canonicalLeft, err := left.CanonicalJSON()
	if err != nil {
		t.Fatal(err)
	}
	canonicalRight, err := right.CanonicalJSON()
	if err != nil || !bytes.Equal(canonicalLeft, canonicalRight) {
		t.Fatalf("canonical JSON is unstable: %s != %s", canonicalLeft, canonicalRight)
	}
	data, err := MarshalEvidenceRecord(left)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"password", "token=secret", "fragment-secret", "raw-response-body", "Bearer secret", "Ethernet secret", "10.0.0.53", "private-network", "authorization"} {
		if bytes.Contains(data, []byte(forbidden)) {
			t.Fatalf("serialized evidence leaked %q: %s", forbidden, data)
		}
	}
	parsed, err := ParseEvidenceRecord(data)
	if err != nil || parsed.Fingerprint != left.Fingerprint {
		t.Fatalf("round trip = %#v, %v", parsed, err)
	}
	var future map[string]any
	if err := json.Unmarshal(data, &future); err != nil {
		t.Fatal(err)
	}
	future["schema_version"] = EvidenceRecordSchemaVersion + 1
	futureData, err := json.Marshal(future)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseEvidenceRecord(futureData); err == nil {
		t.Fatal("future evidence schema accepted")
	}
}

func testProbeSpec(t *testing.T, rawURL string, transport Transport, family AddressFamily) ProbeSpec {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	port := parsed.Port()
	if port == "" {
		port = "443"
	}
	return ProbeSpec{
		SchemaVersion:          ProbeSpecSchemaVersion,
		ID:                     "probe-primary",
		ServiceID:              "service-fixture",
		TargetContractRevision: "target-v1",
		Target:                 Target{URL: sanitizeURL(parsed), Hostname: parsed.Hostname(), Port: port, RequestedProtocol: transport},
		Transport:              transport,
		AddressFamilyPolicy:    family,
		Mode:                   ProbeModeHTTPSGet,
		ExpectedResponse:       ExpectedResponse{AllowedStatusCodes: []int{http.StatusNoContent}, RequirePathComplete: true},
		ControlRole:            ControlRolePrimary,
		Privacy:                PrivacyModeRedacted,
	}
}

func testObservation(spec ProbeSpec, runID, ip string, family AddressFamily, classification Classification, stages []StageEvidence) ObservationResult {
	started := time.Unix(100, 0).UTC()
	primary := 0
	boundary := StageHTTP
	if len(stages) > 0 {
		boundary = stages[len(stages)-1].Stage
	}
	return ObservationResult{
		SchemaVersion:       SchemaVersion,
		RunID:               runID,
		StartedAt:           started,
		FinishedAt:          started.Add(50 * time.Millisecond),
		Target:              spec.Target,
		Attempts:            []ConnectionAttempt{{ResolvedIP: ip, AddressFamily: family, Transport: spec.Transport, Stages: stages}},
		PrimaryAttemptIndex: &primary,
		FinalBoundary:       boundary,
		Classification:      classification,
		ExecutionContext:    ExecutionContext{Mode: "direct"},
	}
}

func completedStages(status int) []StageEvidence {
	return []StageEvidence{
		{Stage: StageResolve, Status: StatusPass, Class: ClassSuccess},
		{Stage: StageConnect, Status: StatusPass},
		{Stage: StageHello, Status: StatusPass},
		{Stage: StageHandshake, Status: StatusPass},
		{Stage: StageHTTP, Status: StatusPass, HTTPStatus: status, PathComplete: true},
	}
}

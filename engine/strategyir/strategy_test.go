package strategyir

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRepresentativeFixturesValidate(t *testing.T) {
	for name, strategy := range RepresentativeFixtures() {
		if err := Validate(strategy); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestFingerprintCanonicalSemanticIdentity(t *testing.T) {
	strategy := RepresentativeFixtures()["alternative-multisplit"]
	baseline, err := Fingerprint(strategy)
	if err != nil {
		t.Fatal(err)
	}
	strategy.ID = "renamed-presentation-id"
	strategy.Name = "Different display name"
	strategy.Metadata.Description = "new description"
	strategy.Metadata.CreatedAt = "2030-01-01T00:00:00Z"
	strategy.Transport = []Transport{TransportTCP}
	strategy.Selector.TCPPorts = []PortRange{{Start: 443, End: 443}, {Start: 80, End: 80}}
	changedPresentation, err := Fingerprint(strategy)
	if err != nil {
		t.Fatal(err)
	}
	if baseline != changedPresentation {
		t.Fatalf("presentation or set ordering changed fingerprint: %s != %s", baseline, changedPresentation)
	}
	strategy.Operations[0].Positions = []PositionExpr{{Absolute: new(1)}}
	changedSemantic, err := Fingerprint(strategy)
	if err != nil {
		t.Fatal(err)
	}
	if baseline == changedSemantic {
		t.Fatal("semantic position change did not change fingerprint")
	}
}

func TestOperationOrderIsSemantic(t *testing.T) {
	strategy := RepresentativeFixtures()["alternative-fake-tls"]
	first, err := Fingerprint(strategy)
	if err != nil {
		t.Fatal(err)
	}
	strategy.Operations[0], strategy.Operations[1] = strategy.Operations[1], strategy.Operations[0]
	second, err := Fingerprint(strategy)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("operation ordering must affect semantics")
	}
}
func TestStrictJSONRejectsUnknownAndUnsafeFields(t *testing.T) {
	valid := `{"schema_version":1,"id":"safe","name":"Safe","transport":["TCP"],"selector":{"application_protocols":["TLS"],"ip_families":["ANY"],"direction":"OUTBOUND","tcp_ports":[{"start":443,"end":443}],"scope":{"host":{"mode":"ALL"}}},"operations":[{"type":"MULTI_SPLIT","positions":[{"anchor":"MIDSLD"}]}],"safety":{"aggressiveness":"LOW","target_only":false}}`
	if _, err := UnmarshalStrict([]byte(valid)); err != nil {
		t.Fatalf("valid strategy rejected: %v", err)
	}
	for _, payload := range []string{
		strings.Replace(valid, `"safety"`, `"shell":"cmd.exe","safety"`, 1),
		strings.Replace(valid, `"MULTI_SPLIT"`, `"RAW_LUA"`, 1),
		strings.Replace(valid, `"mode":"ALL"`, `"mode":"MANAGED_LIST","id":"C:\\unsafe\\hosts.txt"`, 1),
	} {
		if _, err := UnmarshalStrict([]byte(payload)); err == nil {
			t.Fatalf("unsafe input accepted: %s", payload)
		}
	}
}

func TestTypedPositionAndPortValidation(t *testing.T) {
	strategy := RepresentativeFixtures()["alternative-multisplit"]
	strategy.Operations[0].Positions = []PositionExpr{{Anchor: AnchorMidSLD, Offset: -2}, {Anchor: AnchorHost, Offset: 1}, {Anchor: AnchorEndHost, Offset: -1}, {Anchor: AnchorSNIExt, Offset: 1}}
	strategy.Selector.TCPPorts = []PortRange{{Start: 80, End: 80}, {Start: 443, End: 8443}}
	if err := Validate(strategy); err != nil {
		t.Fatalf("typed relative positions rejected: %v", err)
	}
	strategy.Selector.TCPPorts[1] = PortRange{Start: 8443, End: 443}
	if err := Validate(strategy); err == nil {
		t.Fatal("invalid port range accepted")
	}
}

func TestTransportPortScopesAreMandatory(t *testing.T) {
	for _, tc := range []struct {
		name      string
		transport []Transport
		selector  TrafficSelector
		protocols []ApplicationProtocol
	}{
		{"tcp", []Transport{TransportTCP}, TrafficSelector{TCPPorts: []PortRange{{Start: 443, End: 443}}}, []ApplicationProtocol{ApplicationTLS}},
		{"udp", []Transport{TransportUDP}, TrafficSelector{UDPPorts: []PortRange{{Start: 443, End: 443}}}, []ApplicationProtocol{ApplicationAny}},
		{"quic", []Transport{TransportQUIC}, TrafficSelector{UDPPorts: []PortRange{{Start: 443, End: 443}}}, []ApplicationProtocol{ApplicationQUIC}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			strategy := RepresentativeFixtures()["discord-tcp"]
			strategy.Transport = tc.transport
			strategy.Selector.TCPPorts = tc.selector.TCPPorts
			strategy.Selector.UDPPorts = tc.selector.UDPPorts
			strategy.Selector.ApplicationProtocols = tc.protocols
			if err := Validate(strategy); err != nil {
				t.Fatalf("valid %s scope rejected: %v", tc.name, err)
			}
			if tc.transport[0] == TransportTCP {
				strategy.Selector.TCPPorts = nil
			} else {
				strategy.Selector.UDPPorts = nil
			}
			if err := Validate(strategy); err == nil {
				t.Fatal("missing transport port scope accepted")
			}
		})
	}
}

func TestApplicationProtocolSetValidation(t *testing.T) {
	strategy := RepresentativeFixtures()["discord-tcp"]
	strategy.Selector.ApplicationProtocols = []ApplicationProtocol{ApplicationHTTP, ApplicationTLS}
	if err := Validate(strategy); err != nil {
		t.Fatalf("HTTP+TLS rejected: %v", err)
	}
	strategy.Selector.ApplicationProtocols = []ApplicationProtocol{ApplicationAny, ApplicationTLS}
	if err := Validate(strategy); err == nil {
		t.Fatal("ANY+TLS accepted")
	}
	strategy = RepresentativeFixtures()["discord-tcp"]
	strategy.Transport = []Transport{TransportQUIC}
	strategy.Selector.TCPPorts = nil
	strategy.Selector.UDPPorts = []PortRange{{Start: 443, End: 443}}
	strategy.Selector.ApplicationProtocols = []ApplicationProtocol{ApplicationQUIC}
	if err := Validate(strategy); err != nil {
		t.Fatalf("single QUIC protocol rejected: %v", err)
	}
}

func TestTransportApplicationProtocolConsistency(t *testing.T) {
	cases := []struct {
		name string
		set  func(*Strategy)
	}{
		{"QUIC application requires QUIC transport", func(s *Strategy) {
			s.Selector.ApplicationProtocols = []ApplicationProtocol{ApplicationQUIC}
		}},
		{"QUIC transport rejects ANY application", func(s *Strategy) {
			s.Transport = []Transport{TransportQUIC}
			s.Selector.TCPPorts = nil
			s.Selector.UDPPorts = []PortRange{{Start: 443, End: 443}}
			s.Selector.ApplicationProtocols = []ApplicationProtocol{ApplicationAny}
		}},
		{"UDP rejects TCP application protocol", func(s *Strategy) {
			s.Transport = []Transport{TransportUDP}
			s.Selector.TCPPorts = nil
			s.Selector.UDPPorts = []PortRange{{Start: 443, End: 443}}
		}},
		{"mixed transports are fail closed", func(s *Strategy) {
			s.Transport = []Transport{TransportTCP, TransportQUIC}
			s.Selector.UDPPorts = []PortRange{{Start: 443, End: 443}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			strategy := RepresentativeFixtures()["discord-tcp"]
			tc.set(&strategy)
			if err := Validate(strategy); err == nil {
				t.Fatal("ambiguous transport/application selector accepted")
			}
		})
	}

	quic := RepresentativeFixtures()["discord-tcp"]
	quic.Transport = []Transport{TransportQUIC}
	quic.Selector.TCPPorts = nil
	quic.Selector.UDPPorts = []PortRange{{Start: 443, End: 443}}
	quic.Selector.ApplicationProtocols = []ApplicationProtocol{ApplicationQUIC}
	if err := Validate(quic); err != nil {
		t.Fatalf("exact QUIC selector rejected: %v", err)
	}
}

func TestIPFamilyAnyIsExclusiveAndCanonical(t *testing.T) {
	strategy := RepresentativeFixtures()["recommended-hostfakesplit"]
	strategy.Selector.IPFamilies = []IPFamily{IPFamilyAny, IPFamilyV4}
	if err := Validate(strategy); err == nil {
		t.Fatal("ANY+IPv4 accepted")
	}
	if _, err := Fingerprint(strategy); err == nil {
		t.Fatal("invalid redundant ANY selector received a fingerprint")
	}

	first := RepresentativeFixtures()["recommended-hostfakesplit"]
	second := RepresentativeFixtures()["recommended-hostfakesplit"]
	second.Selector.IPFamilies = []IPFamily{IPFamilyV6, IPFamilyV4}
	firstFingerprint, err := Fingerprint(first)
	if err != nil {
		t.Fatal(err)
	}
	secondFingerprint, err := Fingerprint(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstFingerprint != secondFingerprint {
		t.Fatalf("family set ordering changed fingerprint: %s != %s", firstFingerprint, secondFingerprint)
	}
}

func TestStrictOperationUnionAndPositionValidation(t *testing.T) {
	valid := RepresentativeFixtures()["alternative-multisplit"]
	cases := []struct {
		name string
		op   Operation
	}{
		{"split requires one position", Operation{Type: OperationSplit, Positions: []PositionExpr{{Absolute: new(1)}, {Absolute: new(2)}}}},
		{"multisplit rejects payload", Operation{Type: OperationMultiSplit, Positions: []PositionExpr{{Absolute: new(1)}}, PayloadRef: "tls-google"}},
		{"multisplit rejects window", Operation{Type: OperationMultiSplit, Positions: []PositionExpr{{Absolute: new(1)}}, Window: &WindowShaping{Window: 1}}},
		{"fake rejects positions", Operation{Type: OperationFakeInjection, PayloadRef: "tls-google", Positions: []PositionExpr{{Absolute: new(1)}}}},
		{"window rejects fake", Operation{Type: OperationWindowShaping, Window: &WindowShaping{Window: 1}, Fake: &FakeModifiers{Repeat: 1}}},
		{"host case rejects modifiers", Operation{Type: OperationHTTPHostCase, Fake: &FakeModifiers{Repeat: 1}}},
		{"tls record rejects payload", Operation{Type: OperationTLSRecordSplit, Positions: []PositionExpr{{Absolute: new(1)}}, PayloadRef: "tls-google"}},
		{"absolute rejects offset", Operation{Type: OperationMultiSplit, Positions: []PositionExpr{{Absolute: new(1), Offset: 1}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			strategy := valid
			strategy.Operations = []Operation{tc.op}
			if err := Validate(strategy); err == nil {
				t.Fatal("invalid operation shape accepted")
			}
		})
	}
}

func TestCanonicalizationDoesNotMutateInput(t *testing.T) {
	strategy := RepresentativeFixtures()["recommended-hostfakesplit"]
	strategy.Selector.Scope.Host.ID = "YouTube"
	strategy.Selector.Scope.Host.ID = "youtube"
	strategy.Selector.Scope.ExcludeHostListIDs = []string{"z-list", "a-list"}
	strategy.Operations[0].HostTemplate = "OZON.RU"
	before, err := json.Marshal(strategy)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []struct {
		name string
		run  func(Strategy) error
	}{
		{"canonicalize", func(s Strategy) error { _, err := Canonicalize(s); return err }},
		{"marshal", func(s Strategy) error { _, err := Marshal(s); return err }},
		{"fingerprint", func(s Strategy) error { _, err := Fingerprint(s); return err }},
	} {
		t.Run(action.name, func(t *testing.T) {
			if err := action.run(strategy); err != nil {
				t.Fatal(err)
			}
			after, err := json.Marshal(strategy)
			if err != nil {
				t.Fatal(err)
			}
			if string(before) != string(after) {
				t.Fatalf("%s mutated caller-owned strategy", action.name)
			}
		})
	}
}

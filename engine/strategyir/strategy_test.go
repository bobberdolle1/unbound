package strategyir

import (
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

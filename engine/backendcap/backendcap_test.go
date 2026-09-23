package backendcap

import (
	"slices"
	"strings"
	"testing"

	"unbound/engine/strategyir"
)

func TestZapretCompilersShadowRepresentativeStrategies(t *testing.T) {
	fixtures := strategyir.RepresentativeFixtures()
	for _, backend := range []Backend{Zapret2Windows, Zapret2Linux} {
		for name, strategy := range fixtures {
			result := Compile(strategy, backend)
			if result.Status != StatusCompiled {
				t.Fatalf("%s %s: %#v", backend, name, result)
			}
			if len(result.Plan.Argv) == 0 {
				t.Fatalf("%s %s emitted no plan", backend, name)
			}
		}
	}
	recommended := Compile(fixtures["recommended-hostfakesplit"], Zapret2Windows)
	if !contains(recommended.Plan.Argv, "--hostlist=${asset:youtube}") || !contains(recommended.Plan.Argv, "--hostlist-exclude=${asset:steam-web-exclude}") || !containsPrefix(recommended.Plan.Argv, "--lua-desync=hostfakesplit:midhost=midsld:host=ozon.ru") {
		t.Fatalf("recommended scope/operation mismatch: %v", recommended.Plan.Argv)
	}
	multi := Compile(fixtures["alternative-multisplit"], Zapret2Windows)
	if !containsPrefix(multi.Plan.Argv, "--lua-desync=multisplit:pos=2:seqovl=652:seqovl_pattern=${asset:tls-google}") {
		t.Fatalf("multisplit semantics missing: %v", multi.Plan.Argv)
	}
	fake := Compile(fixtures["alternative-fake-tls"], Zapret2Windows)
	if !containsPrefix(fake.Plan.Argv, "--lua-desync=fake:blob=${asset:tls-clienthello-default}:repeats=11:tcp_ack=-66000:tcp_ts") {
		t.Fatalf("fake TLS semantics missing: %v", fake.Plan.Argv)
	}
}

func TestTPWSCompilerAndUnsupportedContract(t *testing.T) {
	strategy := strategyir.Strategy{SchemaVersion: strategyir.SchemaVersion, ID: "tpws-split", Name: "tpws split", Transport: []strategyir.Transport{strategyir.TransportTCP}, Selector: strategyir.TrafficSelector{ApplicationProtocols: []strategyir.ApplicationProtocol{strategyir.ApplicationTLS}, IPFamilies: []strategyir.IPFamily{strategyir.IPFamilyAny}, Direction: strategyir.DirectionOutbound, TCPPorts: []strategyir.PortRange{{Start: 443, End: 443}}, Scope: strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeAll}}}, Operations: []strategyir.Operation{{Type: strategyir.OperationMultiSplit, Positions: []strategyir.PositionExpr{{Absolute: new(1)}, {Anchor: strategyir.AnchorMidSLD}}}, {Type: strategyir.OperationTLSRecordSplit, Positions: []strategyir.PositionExpr{{Absolute: new(1)}, {Anchor: strategyir.AnchorMidSLD}}}, {Type: strategyir.OperationDisorder}}, Safety: strategyir.SafetyPolicy{Aggressiveness: "LOW", TargetOnly: false}}
	result := Compile(strategy, Zapret1TPWSDarwin)
	if result.Status != StatusCompiled || !contains(result.Plan.Argv, "--tlsrec=1,midsld") || !contains(result.Plan.Argv, "--disorder") {
		t.Fatalf("tpws compile = %#v", result)
	}
	for _, unsupported := range []strategyir.Strategy{
		withTransport(strategy, strategyir.TransportUDP),
		withTransport(strategy, strategyir.TransportQUIC),
		withAnchor(strategy, strategyir.AnchorHost),
		strategyir.RepresentativeFixtures()["alternative-fake-tls"],
		strategyir.RepresentativeFixtures()["discord-tcp"],
	} {
		result := Compile(unsupported, Zapret1TPWSDarwin)
		if result.Status != StatusUnsupported || len(result.Unsupported) == 0 || len(result.Plan.Argv) != 0 {
			t.Fatalf("unsupported strategy silently compiled: %#v", result)
		}
	}
}

func TestCompilerDeterminismAndDerivedRequirements(t *testing.T) {
	strategy := strategyir.Strategy{SchemaVersion: strategyir.SchemaVersion, ID: "network-requirements", Name: "network requirements", Transport: []strategyir.Transport{strategyir.TransportQUIC}, Selector: strategyir.TrafficSelector{ApplicationProtocols: []strategyir.ApplicationProtocol{strategyir.ApplicationQUIC}, IPFamilies: []strategyir.IPFamily{strategyir.IPFamilyV6}, Direction: strategyir.DirectionBoth, UDPPorts: []strategyir.PortRange{{Start: 443, End: 443}}, Scope: strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeAutoHostlist, ID: "autodetect"}}}, Operations: []strategyir.Operation{{Type: strategyir.OperationFakeInjection, PayloadRef: "quic-google", Fake: &strategyir.FakeModifiers{Repeat: 6}}}, Safety: strategyir.SafetyPolicy{Aggressiveness: "MEDIUM", TargetOnly: true}}
	first, second := Compile(strategy, Zapret2Windows), Compile(strategy, Zapret2Windows)
	if first.Status != StatusCompiled || !slices.Equal(first.Plan.Argv, second.Plan.Argv) {
		t.Fatalf("non-deterministic compilation: %#v %#v", first, second)
	}
	if !first.DerivedRequirements.QUIC || !first.DerivedRequirements.IPv6 || !first.DerivedRequirements.InboundUDP || first.DerivedRequirements.EngineMinVersion != "v1.0.5.1" || !slices.Equal(first.DerivedRequirements.LuaModules, []string{"zapret-antidpi.lua", "init_vars.lua"}) || len(first.DerivedRequirements.FakePayloads) != 0 {
		t.Fatalf("derived requirements = %#v", first.DerivedRequirements)
	}
}

func TestBackendProvenanceAndNativePlaceholder(t *testing.T) {
	windows := Get(Zapret2Windows)
	if windows.Provenance.Tag != "v1.0.5.1" || windows.Provenance.Commit != "a1bca5a85e25ab138e9617a560c262fcf53e969a" {
		t.Fatalf("Zapret2 provenance = %#v", windows.Provenance)
	}
	macOS := Get(Zapret1TPWSDarwin)
	if macOS.Provenance.BaseTag != "v72.13" || macOS.Provenance.Commit != "d437963452674faadfd45adcd62466272b5a2fcd" {
		t.Fatalf("tpws provenance = %#v", macOS.Provenance)
	}
	result := Compile(strategyir.RepresentativeFixtures()["recommended-hostfakesplit"], NativeWindows)
	if result.Status != StatusUnsupported {
		t.Fatalf("native placeholder compiled: %#v", result)
	}
}

func TestCatalogCoverageHasEveryDisposition(t *testing.T) {
	coverage := strategyir.CatalogCoverage()
	seen := map[strategyir.Coverage]bool{}
	for _, entry := range coverage {
		seen[entry.Coverage] = true
	}
	if !seen[strategyir.Representable] || !seen[strategyir.PartiallyRepresentable] || !seen[strategyir.Unrepresentable] {
		t.Fatalf("coverage report lacks disposition: %#v", coverage)
	}
}

func withTransport(strategy strategyir.Strategy, transport strategyir.Transport) strategyir.Strategy {
	strategy.Transport = []strategyir.Transport{transport}
	strategy.Selector.UDPPorts = nil
	if transport != strategyir.TransportTCP {
		strategy.Selector.TCPPorts = nil
		strategy.Selector.UDPPorts = []strategyir.PortRange{{Start: 443, End: 443}}
	}
	return strategy
}
func withAnchor(strategy strategyir.Strategy, anchor strategyir.PositionAnchor) strategyir.Strategy {
	strategy.Operations[0].Positions = []strategyir.PositionExpr{{Anchor: anchor}}
	return strategy
}
func contains(values []string, expected string) bool { return slices.Contains(values, expected) }
func containsPrefix(values []string, expected string) bool {
	for _, value := range values {
		if strings.HasPrefix(value, expected) {
			return true
		}
	}
	return false
}

func TestCompilerRejectsUnknownPinnedAsset(t *testing.T) {
	strategy := strategyir.RepresentativeFixtures()["alternative-fake-tls"]
	strategy.Operations[0].PayloadRef = "untrusted-payload"
	result := Compile(strategy, Zapret2Windows)
	if result.Status != StatusUnsupported || !hasReason(result.Unsupported, MissingAsset) {
		t.Fatalf("untrusted fake asset compiled: %#v", result)
	}
}

func hasReason(reasons []Reason, expected ReasonCode) bool {
	for _, reason := range reasons {
		if reason.Code == expected {
			return true
		}
	}
	return false
}

func TestTPWSNeverBroadensTargetScope(t *testing.T) {
	result := Compile(strategyir.RepresentativeFixtures()["discord-tcp"], Zapret1TPWSDarwin)
	if result.Status != StatusUnsupported || !hasReason(result.Unsupported, UnsupportedPortScope) || len(result.Plan.Argv) != 0 {
		t.Fatalf("target scope was not rejected before compilation: %#v", result)
	}
}

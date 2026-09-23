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
	strategy := strategyir.Strategy{SchemaVersion: strategyir.SchemaVersion, ID: "tpws-split", Name: "tpws split", Transport: []strategyir.Transport{strategyir.TransportTCP}, Selector: strategyir.TrafficSelector{ApplicationProtocols: []strategyir.ApplicationProtocol{strategyir.ApplicationAny}, IPFamilies: []strategyir.IPFamily{strategyir.IPFamilyAny}, Direction: strategyir.DirectionOutbound, TCPPorts: []strategyir.PortRange{{Start: 443, End: 443}}, Scope: strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeAll}}}, Operations: []strategyir.Operation{{Type: strategyir.OperationMultiSplit, Positions: []strategyir.PositionExpr{{Absolute: new(1)}, {Anchor: strategyir.AnchorMidSLD}}}, {Type: strategyir.OperationTLSRecordSplit, Positions: []strategyir.PositionExpr{{Absolute: new(1)}, {Anchor: strategyir.AnchorMidSLD}}}, {Type: strategyir.OperationDisorder}}, Safety: strategyir.SafetyPolicy{Aggressiveness: "LOW"}}
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
	if transport == strategyir.TransportQUIC {
		strategy.Selector.ApplicationProtocols = []strategyir.ApplicationProtocol{strategyir.ApplicationQUIC}
	} else {
		strategy.Selector.ApplicationProtocols = []strategyir.ApplicationProtocol{strategyir.ApplicationAny}
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
	if result.Status != StatusUnsupported || !hasReason(result.Unsupported, UnsupportedScope) || len(result.Plan.Argv) != 0 {
		t.Fatalf("target scope was not rejected before compilation: %#v", result)
	}
}

func TestTPWSCompilerPreservesHTTPHostCase(t *testing.T) {
	strategy := strategyir.Strategy{
		SchemaVersion: strategyir.SchemaVersion,
		ID:            "tpws-http-host-case",
		Name:          "tpws HTTP host case",
		Transport:     []strategyir.Transport{strategyir.TransportTCP},
		Selector: strategyir.TrafficSelector{
			ApplicationProtocols: []strategyir.ApplicationProtocol{strategyir.ApplicationAny},
			IPFamilies:           []strategyir.IPFamily{strategyir.IPFamilyAny},
			Direction:            strategyir.DirectionOutbound,
			TCPPorts:             []strategyir.PortRange{{Start: 80, End: 80}},
			Scope:                strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeAll}},
		},
		Operations: []strategyir.Operation{{Type: strategyir.OperationHTTPHostCase}},
		Safety:     strategyir.SafetyPolicy{Aggressiveness: "LOW", TargetOnly: false},
	}
	result := Compile(strategy, Zapret1TPWSDarwin)
	if result.Status != StatusCompiled || !contains(result.Plan.Argv, "--hostcase") {
		t.Fatalf("tpws HTTP host case = %#v", result)
	}
}

func TestCompilerRejectsMissingPortScopeAsInvalid(t *testing.T) {
	strategy := strategyir.RepresentativeFixtures()["discord-tcp"]
	strategy.Selector.TCPPorts = nil
	result := Compile(strategy, Zapret2Windows)
	if result.Status != StatusInvalid || !hasReason(result.Unsupported, InvalidIR) || len(result.Plan.Argv) != 0 {
		t.Fatalf("missing port scope compiled: %#v", result)
	}
}

func TestApplicationProtocolCompilationIsExact(t *testing.T) {
	httpTLS := strategyir.RepresentativeFixtures()["discord-tcp"]
	httpTLS.Selector.ApplicationProtocols = []strategyir.ApplicationProtocol{strategyir.ApplicationTLS, strategyir.ApplicationHTTP}
	result := Compile(httpTLS, Zapret2Windows)
	if result.Status != StatusCompiled || !contains(result.Plan.Argv, "--payload=http_req,tls_client_hello") {
		t.Fatalf("HTTP+TLS was not compiled exactly: %#v", result)
	}

	invalid := httpTLS
	invalid.Selector.ApplicationProtocols = []strategyir.ApplicationProtocol{strategyir.ApplicationAny, strategyir.ApplicationTLS}
	if result := Compile(invalid, Zapret2Windows); result.Status != StatusInvalid {
		t.Fatalf("ANY+TLS was not invalid: %#v", result)
	}

	quic := strategyir.RepresentativeFixtures()["discord-tcp"]
	quic.Transport = []strategyir.Transport{strategyir.TransportQUIC}
	quic.Selector.TCPPorts = nil
	quic.Selector.UDPPorts = []strategyir.PortRange{{Start: 443, End: 443}}
	quic.Selector.ApplicationProtocols = []strategyir.ApplicationProtocol{strategyir.ApplicationQUIC}
	result = Compile(quic, Zapret2Windows)
	if result.Status != StatusCompiled || !contains(result.Plan.Argv, "--filter-udp=443") || !contains(result.Plan.Argv, "--payload=quic_initial") {
		t.Fatalf("single QUIC protocol was not compiled exactly: %#v", result)
	}

	if result := Compile(httpTLS, Zapret1TPWSDarwin); result.Status != StatusUnsupported || !hasReason(result.Unsupported, UnsupportedApplicationProtocol) {
		t.Fatalf("tpws accepted an unpreservable protocol set: %#v", result)
	}
}

func TestFakeModifierCapabilitiesAreIndividual(t *testing.T) {
	cases := []struct {
		name string
		set  func(*strategyir.FakeModifiers)
		off  func(*Capabilities)
	}{
		{"repeat", func(f *strategyir.FakeModifiers) { f.Repeat = 1 }, func(c *Capabilities) { c.FakeRepeat = false }},
		{"ttl", func(f *strategyir.FakeModifiers) { f.TTL = new(1) }, func(c *Capabilities) { c.FakeTTL = false }},
		{"sequence offset", func(f *strategyir.FakeModifiers) { f.SequenceOffset = new(1) }, func(c *Capabilities) { c.FakeSequenceOffset = false }},
		{"acknowledgment offset", func(f *strategyir.FakeModifiers) { f.AcknowledgmentOffset = new(1) }, func(c *Capabilities) { c.FakeAcknowledgmentOffset = false }},
		{"tcp md5", func(f *strategyir.FakeModifiers) { f.TCPMD5 = true }, func(c *Capabilities) { c.FakeTCPMD5 = false }},
		{"tcp timestamp", func(f *strategyir.FakeModifiers) { f.TCPTimestamp = true }, func(c *Capabilities) { c.FakeTCPTimestamp = false }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			strategy := strategyir.RepresentativeFixtures()["alternative-fake-tls"]
			strategy.Operations[0].Fake = &strategyir.FakeModifiers{}
			tc.set(strategy.Operations[0].Fake)
			caps := Get(Zapret2Windows)
			tc.off(&caps)
			if reasons := compatibility(strategy, caps); !hasReason(reasons, UnsupportedFakeModifier) {
				t.Fatalf("unsupported modifier was accepted: %#v", reasons)
			}
		})
	}
}

func TestTypedCutoffRangesCompileOrFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name     string
		strategy strategyir.Strategy
		expected string
	}{
		{"out data packet eight", strategyir.RepresentativeFixtures()["recommended-hostfakesplit"], "--out-range=-d8"},
		{"out data packet three", strategyir.RepresentativeFixtures()["steam-safe-game-filter"], "--out-range=-d3"},
		{"in relative sequence 4096", func() strategyir.Strategy {
			s := strategyir.RepresentativeFixtures()["alternative-multisplit"]
			s.Range = &strategyir.Cutoff{Direction: strategyir.RangeDirectionIn, Counter: strategyir.RangeCounterRelativeSequence, Limit: 4096}
			return s
		}(), "--in-range=-s4096"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := Compile(tc.strategy, Zapret2Windows)
			if result.Status != StatusCompiled || !contains(result.Plan.Argv, tc.expected) {
				t.Fatalf("cutoff was not compiled: %#v", result)
			}
		})
	}
	unsupported := strategyir.Strategy{
		SchemaVersion: strategyir.SchemaVersion, ID: "tpws-cutoff", Name: "tpws cutoff",
		Transport:  []strategyir.Transport{strategyir.TransportTCP},
		Selector:   strategyir.TrafficSelector{ApplicationProtocols: []strategyir.ApplicationProtocol{strategyir.ApplicationAny}, IPFamilies: []strategyir.IPFamily{strategyir.IPFamilyAny}, Direction: strategyir.DirectionOutbound, TCPPorts: []strategyir.PortRange{{Start: 443, End: 443}}, Scope: strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeAll}}},
		Range:      &strategyir.Cutoff{Direction: strategyir.RangeDirectionOut, Counter: strategyir.RangeCounterDataPacketNumber, Limit: 8},
		Operations: []strategyir.Operation{{Type: strategyir.OperationMultiSplit, Positions: []strategyir.PositionExpr{{Absolute: new(1)}}}},
		Safety:     strategyir.SafetyPolicy{Aggressiveness: "LOW"},
	}
	if result := Compile(unsupported, Zapret1TPWSDarwin); result.Status != StatusUnsupported || !hasReason(result.Unsupported, UnsupportedCutoff) {
		t.Fatalf("tpws accepted an unpreservable cutoff: %#v", result)
	}
}

func TestSplitIsOnePositionMultiSplitSemanticAlias(t *testing.T) {
	base := strategyir.RepresentativeFixtures()["discord-tcp"]
	base.Operations = []strategyir.Operation{{Type: strategyir.OperationSplit, Positions: []strategyir.PositionExpr{{Absolute: new(1)}}}}
	split := Compile(base, Zapret2Windows)
	base.Operations[0].Type = strategyir.OperationMultiSplit
	multi := Compile(base, Zapret2Windows)
	if split.Status != StatusCompiled || multi.Status != StatusCompiled || !slices.Equal(split.Plan.Argv, multi.Plan.Argv) {
		t.Fatalf("one-position split equivalence changed: %#v %#v", split, multi)
	}
}

func TestRepresentativeGoldenOrderedPlans(t *testing.T) {
	expected := map[string][]string{
		"recommended-hostfakesplit": {
			"--wf-l3=ipv4,ipv6", "--filter-tcp=80,443", "--wf-tcp-out=80,443", "--payload=tls_client_hello",
			"--hostlist=${asset:youtube}", "--hostlist-exclude=${asset:steam-web-exclude}", "--ipset-exclude=${asset:ipset-steam-exclude}",
			"--out-range=-d8", "--lua-desync=hostfakesplit:midhost=midsld:host=ozon.ru:repeats=4:tcp_md5:tcp_ts",
		},
		"alternative-multisplit": {
			"--wf-l3=ipv4,ipv6", "--filter-tcp=80,443", "--wf-tcp-out=80,443", "--payload=tls_client_hello",
			"--hostlist=${asset:youtube}", "--out-range=-d8",
			"--lua-desync=multisplit:pos=2:seqovl=652:seqovl_pattern=${asset:tls-google}",
		},
		"alternative-fake-tls": {
			"--wf-l3=ipv4,ipv6", "--filter-tcp=443", "--wf-tcp-out=443", "--payload=tls_client_hello",
			"--out-range=-d8",
			"--lua-desync=fake:blob=${asset:tls-clienthello-default}:repeats=11:tcp_ack=-66000:tcp_ts",
			"--lua-desync=multidisorder:pos=1,midsld:repeats=11",
		},
		"discord-tcp": {
			"--wf-l3=ipv4,ipv6", "--filter-tcp=443,5222-5223,5228", "--wf-tcp-out=443,5222-5223,5228",
			"--payload=tls_client_hello", "--hostlist-domains=discord.com,gateway.discord.gg", "--lua-desync=multisplit:pos=1",
		},
		"steam-safe-game-filter": {
			"--wf-l3=ipv4,ipv6", "--filter-tcp=1024-65535", "--wf-tcp-out=1024-65535",
			"--ipset=${asset:ipset-all}", "--hostlist-exclude=${asset:steam-web-exclude}", "--ipset-exclude=${asset:ipset-exclude}",
			"--out-range=-d3", "--lua-desync=multisplit:pos=1:seqovl=652:seqovl_pattern=${asset:tls-google}",
		},
	}
	for name, want := range expected {
		t.Run(name, func(t *testing.T) {
			result := Compile(strategyir.RepresentativeFixtures()[name], Zapret2Windows)
			if result.Status != StatusCompiled || !slices.Equal(result.Plan.Argv, want) {
				t.Fatalf("ordered legacy semantic plan mismatch\nwant: %v\ngot:  %v", want, result.Plan.Argv)
			}
		})
	}
}

func TestIPFamilyGoldenPlans(t *testing.T) {
	for _, tc := range []struct {
		name   string
		family strategyir.IPFamily
		value  string
	}{
		{"ipv4", strategyir.IPFamilyV4, "ipv4"},
		{"ipv6", strategyir.IPFamilyV6, "ipv6"},
		{"any", strategyir.IPFamilyAny, "ipv4,ipv6"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			strategy := strategyir.Strategy{
				SchemaVersion: strategyir.SchemaVersion, ID: "family-" + tc.name, Name: "family " + tc.name,
				Transport: []strategyir.Transport{strategyir.TransportTCP},
				Selector: strategyir.TrafficSelector{
					ApplicationProtocols: []strategyir.ApplicationProtocol{strategyir.ApplicationAny},
					IPFamilies:           []strategyir.IPFamily{tc.family},
					Direction:            strategyir.DirectionOutbound,
					TCPPorts:             []strategyir.PortRange{{Start: 443, End: 443}},
					Scope:                strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeAll}},
				},
				Operations: []strategyir.Operation{{Type: strategyir.OperationMultiSplit, Positions: []strategyir.PositionExpr{{Absolute: new(1)}}}},
				Safety:     strategyir.SafetyPolicy{Aggressiveness: "LOW"},
			}
			for _, backend := range []Backend{Zapret2Windows, Zapret2Linux} {
				want := []string{"--wf-l3=" + tc.value, "--filter-tcp=443", "--wf-tcp-out=443", "--lua-desync=multisplit:pos=1"}
				if result := Compile(strategy, backend); result.Status != StatusCompiled || !slices.Equal(result.Plan.Argv, want) {
					t.Fatalf("%s family scope mismatch: %#v", backend, result)
				}
			}
			wantTPWS := []string{"--filter-l3=" + tc.value, "--filter-tcp=443", "--split-pos=1"}
			if result := Compile(strategy, Zapret1TPWSDarwin); result.Status != StatusCompiled || !slices.Equal(result.Plan.Argv, wantTPWS) {
				t.Fatalf("tpws family scope mismatch: %#v", result)
			}
		})
	}
}

func TestIPFamilyAnyRequiresDualStackCapability(t *testing.T) {
	strategy := strategyir.RepresentativeFixtures()["alternative-multisplit"]
	caps := Get(Zapret2Windows)
	caps.IPFamilies = []strategyir.IPFamily{strategyir.IPFamilyV4}
	if !hasReason(compatibility(strategy, caps), UnsupportedIPFamily) {
		t.Fatal("ANY family scope was broadened for an IPv4-only backend")
	}
}

func TestCatalogCoverageIsDeclaredAndCounted(t *testing.T) {
	counts := map[strategyir.Coverage]int{}
	for _, entry := range strategyir.CatalogCoverage() {
		counts[entry.Coverage]++
		if entry.Profile == "Saved discovered profiles" && entry.MissingCapability != "opaque legacy argv import is intentionally absent" {
			t.Fatalf("saved discovered profile coverage implies unsupported parsing behavior: %#v", entry)
		}
	}
	if counts[strategyir.Representable] != 1 || counts[strategyir.PartiallyRepresentable] != 9 || counts[strategyir.Unrepresentable] != 3 {
		t.Fatalf("catalog coverage counts = %#v", counts)
	}
}

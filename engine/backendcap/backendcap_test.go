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
			if len(result.Plan.EngineArgv) == 0 {
				t.Fatalf("%s %s emitted no plan", backend, name)
			}
		}
	}
	recommended := Compile(fixtures["recommended-hostfakesplit"], Zapret2Windows)
	if !contains(recommended.Plan.EngineArgv, "--hostlist=${asset:youtube}") || !contains(recommended.Plan.EngineArgv, "--hostlist-exclude=${asset:steam-web-exclude}") || !containsPrefix(recommended.Plan.EngineArgv, "--lua-desync=hostfakesplit:midhost=midsld:host=ozon.ru") {
		t.Fatalf("recommended scope/operation mismatch: %v", recommended.Plan.EngineArgv)
	}
	multi := Compile(fixtures["alternative-multisplit"], Zapret2Windows)
	if !containsPrefix(multi.Plan.EngineArgv, "--lua-desync=multisplit:pos=2:seqovl=652:seqovl_pattern=${asset:tls-google}") {
		t.Fatalf("multisplit semantics missing: %v", multi.Plan.EngineArgv)
	}
	fake := Compile(fixtures["alternative-fake-tls"], Zapret2Windows)
	if !containsPrefix(fake.Plan.EngineArgv, "--lua-desync=fake:blob=${asset:tls-clienthello-default}:repeats=11:tcp_ack=-66000:tcp_ts") {
		t.Fatalf("fake TLS semantics missing: %v", fake.Plan.EngineArgv)
	}
}

func TestTPWSCompilerAndUnsupportedContract(t *testing.T) {
	strategy := strategyir.Strategy{SchemaVersion: strategyir.SchemaVersion, ID: "tpws-split", Name: "tpws split", Transport: []strategyir.Transport{strategyir.TransportTCP}, Selector: strategyir.TrafficSelector{ApplicationProtocols: []strategyir.ApplicationProtocol{strategyir.ApplicationAny}, IPFamilies: []strategyir.IPFamily{strategyir.IPFamilyAny}, Direction: strategyir.DirectionOutbound, TCPPorts: []strategyir.PortRange{{Start: 443, End: 443}}, Scope: strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeAll}}}, Operations: []strategyir.Operation{{Type: strategyir.OperationMultiSplit, Positions: []strategyir.PositionExpr{{Absolute: new(1)}, {Anchor: strategyir.AnchorMidSLD}}}, {Type: strategyir.OperationTLSRecordSplit, Positions: []strategyir.PositionExpr{{Absolute: new(1)}, {Anchor: strategyir.AnchorMidSLD}}}, {Type: strategyir.OperationDisorder}}, Safety: strategyir.SafetyPolicy{Aggressiveness: "LOW"}}
	result := Compile(strategy, Zapret1TPWSDarwin)
	if result.Status != StatusCompiled || !contains(result.Plan.EngineArgv, "--tlsrec=1,midsld") || !contains(result.Plan.EngineArgv, "--disorder") {
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
		if result.Status != StatusUnsupported || len(result.Unsupported) == 0 || len(result.Plan.EngineArgv) != 0 {
			t.Fatalf("unsupported strategy silently compiled: %#v", result)
		}
	}
}

func TestCompilerDeterminismAndDerivedRequirements(t *testing.T) {
	strategy := strategyir.Strategy{SchemaVersion: strategyir.SchemaVersion, ID: "network-requirements", Name: "network requirements", Transport: []strategyir.Transport{strategyir.TransportQUIC}, Selector: strategyir.TrafficSelector{ApplicationProtocols: []strategyir.ApplicationProtocol{strategyir.ApplicationQUIC}, IPFamilies: []strategyir.IPFamily{strategyir.IPFamilyV6}, Direction: strategyir.DirectionBoth, UDPPorts: []strategyir.PortRange{{Start: 443, End: 443}}, Scope: strategyir.Scope{Host: strategyir.HostScope{Mode: strategyir.HostScopeAutoHostlist, ID: "autodetect"}}}, Operations: []strategyir.Operation{{Type: strategyir.OperationFakeInjection, PayloadRef: "quic-google", Fake: &strategyir.FakeModifiers{Repeat: 6}}}, Safety: strategyir.SafetyPolicy{Aggressiveness: "MEDIUM", TargetOnly: true}}
	first, second := Compile(strategy, Zapret2Windows), Compile(strategy, Zapret2Windows)
	if first.Status != StatusCompiled || !slices.Equal(first.Plan.EngineArgv, second.Plan.EngineArgv) {
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
	if result.Status != StatusUnsupported || !hasReason(result.Unsupported, UnsupportedScope) || len(result.Plan.EngineArgv) != 0 {
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
	if result.Status != StatusCompiled || !contains(result.Plan.EngineArgv, "--hostcase") {
		t.Fatalf("tpws HTTP host case = %#v", result)
	}
}

func TestCompilerRejectsMissingPortScopeAsInvalid(t *testing.T) {
	strategy := strategyir.RepresentativeFixtures()["discord-tcp"]
	strategy.Selector.TCPPorts = nil
	result := Compile(strategy, Zapret2Windows)
	if result.Status != StatusInvalid || !hasReason(result.Unsupported, InvalidIR) || len(result.Plan.EngineArgv) != 0 {
		t.Fatalf("missing port scope compiled: %#v", result)
	}
}

func TestApplicationProtocolCompilationIsExact(t *testing.T) {
	httpTLS := strategyir.RepresentativeFixtures()["discord-tcp"]
	httpTLS.Selector.ApplicationProtocols = []strategyir.ApplicationProtocol{strategyir.ApplicationTLS, strategyir.ApplicationHTTP}
	result := Compile(httpTLS, Zapret2Windows)
	if result.Status != StatusCompiled || !contains(result.Plan.EngineArgv, "--payload=http_req,tls_client_hello") {
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
	if result.Status != StatusCompiled || !contains(result.Plan.EngineArgv, "--filter-udp=443") || !contains(result.Plan.EngineArgv, "--payload=quic_initial") {
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
		off  func(*ProfileCapabilities)
	}{
		{"repeat", func(f *strategyir.FakeModifiers) { f.Repeat = 1 }, func(c *ProfileCapabilities) { c.FakeRepeat = false }},
		{"ttl", func(f *strategyir.FakeModifiers) { f.TTL = new(1) }, func(c *ProfileCapabilities) { c.FakeTTL = false }},
		{"sequence offset", func(f *strategyir.FakeModifiers) { f.SequenceOffset = new(1) }, func(c *ProfileCapabilities) { c.FakeSequenceOffset = false }},
		{"acknowledgment offset", func(f *strategyir.FakeModifiers) { f.AcknowledgmentOffset = new(1) }, func(c *ProfileCapabilities) { c.FakeAcknowledgmentOffset = false }},
		{"tcp md5", func(f *strategyir.FakeModifiers) { f.TCPMD5 = true }, func(c *ProfileCapabilities) { c.FakeTCPMD5 = false }},
		{"tcp timestamp", func(f *strategyir.FakeModifiers) { f.TCPTimestamp = true }, func(c *ProfileCapabilities) { c.FakeTCPTimestamp = false }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			strategy := strategyir.RepresentativeFixtures()["alternative-fake-tls"]
			strategy.Operations[0].Fake = &strategyir.FakeModifiers{}
			tc.set(strategy.Operations[0].Fake)
			caps := Get(Zapret2Windows)
			tc.off(&caps.Profile)
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
			if result.Status != StatusCompiled || !contains(result.Plan.EngineArgv, tc.expected) {
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
	if split.Status != StatusCompiled || multi.Status != StatusCompiled || !slices.Equal(split.Plan.EngineArgv, multi.Plan.EngineArgv) {
		t.Fatalf("one-position split equivalence changed: %#v %#v", split, multi)
	}
}

func TestZapret2GoldenPlansSeparateEngineAndCapture(t *testing.T) {
	type goldenPlan struct {
		engine             []string
		families           []strategyir.IPFamily
		tcpPorts           []strategyir.PortRange
		windowsCaptureArgv []string
	}
	expected := map[string]goldenPlan{
		"recommended-hostfakesplit": {
			engine: []string{
				"--filter-l3=ipv4,ipv6", "--filter-tcp=80,443", "--payload=tls_client_hello",
				"--hostlist=${asset:youtube}", "--hostlist-exclude=${asset:steam-web-exclude}", "--ipset-exclude=${asset:ipset-steam-exclude}",
				"--out-range=-d8", "--lua-desync=hostfakesplit:midhost=midsld:host=ozon.ru:repeats=4:tcp_md5:tcp_ts",
			},
			families:           []strategyir.IPFamily{strategyir.IPFamilyV4, strategyir.IPFamilyV6},
			tcpPorts:           []strategyir.PortRange{{Start: 80, End: 80}, {Start: 443, End: 443}},
			windowsCaptureArgv: []string{"--wf-l3=ipv4,ipv6", "--wf-tcp-out=80,443"},
		},
		"alternative-multisplit": {
			engine: []string{
				"--filter-l3=ipv4,ipv6", "--filter-tcp=80,443", "--payload=tls_client_hello",
				"--hostlist=${asset:youtube}", "--out-range=-d8",
				"--lua-desync=multisplit:pos=2:seqovl=652:seqovl_pattern=${asset:tls-google}",
			},
			families:           []strategyir.IPFamily{strategyir.IPFamilyAny},
			tcpPorts:           []strategyir.PortRange{{Start: 80, End: 80}, {Start: 443, End: 443}},
			windowsCaptureArgv: []string{"--wf-l3=ipv4,ipv6", "--wf-tcp-out=80,443"},
		},
		"alternative-fake-tls": {
			engine: []string{
				"--filter-l3=ipv4,ipv6", "--filter-tcp=443", "--payload=tls_client_hello", "--out-range=-d8",
				"--lua-desync=fake:blob=${asset:tls-clienthello-default}:repeats=11:tcp_ack=-66000:tcp_ts",
				"--lua-desync=multidisorder:pos=1,midsld:repeats=11",
			},
			families:           []strategyir.IPFamily{strategyir.IPFamilyAny},
			tcpPorts:           []strategyir.PortRange{{Start: 443, End: 443}},
			windowsCaptureArgv: []string{"--wf-l3=ipv4,ipv6", "--wf-tcp-out=443"},
		},
		"discord-tcp": {
			engine: []string{
				"--filter-l3=ipv4,ipv6", "--filter-tcp=443,5222-5223,5228", "--payload=tls_client_hello",
				"--hostlist-domains=discord.com,gateway.discord.gg", "--lua-desync=multisplit:pos=1",
			},
			families:           []strategyir.IPFamily{strategyir.IPFamilyAny},
			tcpPorts:           []strategyir.PortRange{{Start: 443, End: 443}, {Start: 5222, End: 5223}, {Start: 5228, End: 5228}},
			windowsCaptureArgv: []string{"--wf-l3=ipv4,ipv6", "--wf-tcp-out=443,5222-5223,5228"},
		},
		"steam-safe-game-filter": {
			engine: []string{
				"--filter-l3=ipv4,ipv6", "--filter-tcp=1024-65535",
				"--ipset=${asset:ipset-all}", "--hostlist-exclude=${asset:steam-web-exclude}", "--ipset-exclude=${asset:ipset-exclude}",
				"--out-range=-d3", "--lua-desync=multisplit:pos=1:seqovl=652:seqovl_pattern=${asset:tls-google}",
			},
			families:           []strategyir.IPFamily{strategyir.IPFamilyAny},
			tcpPorts:           []strategyir.PortRange{{Start: 1024, End: 65535}},
			windowsCaptureArgv: []string{"--wf-l3=ipv4,ipv6", "--wf-tcp-out=1024-65535"},
		},
	}
	for name, want := range expected {
		t.Run(name, func(t *testing.T) {
			strategy := strategyir.RepresentativeFixtures()[name]
			linux := Compile(strategy, Zapret2Linux)
			if linux.Status != StatusCompiled || !slices.Equal(linux.Plan.EngineArgv, want.engine) {
				t.Fatalf("Linux engine argv mismatch\nwant: %v\ngot:  %#v", want.engine, linux)
			}
			if containsPrefix(linux.Plan.EngineArgv, "--wf-") {
				t.Fatalf("Linux EngineArgv leaked WinDivert flags: %v", linux.Plan.EngineArgv)
			}
			assertCapturePlan(t, linux.Plan.Capture, CapturePlan{
				BackendKind: CaptureNFQUEUE, Transport: CaptureTransportTCP, Direction: strategyir.DirectionOutbound,
				IPFamilies: want.families, TCPPorts: want.tcpPorts,
			})

			windows := Compile(strategy, Zapret2Windows)
			if windows.Status != StatusCompiled || !slices.Equal(windows.Plan.EngineArgv, want.engine) {
				t.Fatalf("Windows engine argv mismatch\nwant: %v\ngot:  %#v", want.engine, windows)
			}
			capture := CapturePlan{
				BackendKind: CaptureWinDivert, Transport: CaptureTransportTCP, Direction: strategyir.DirectionOutbound,
				IPFamilies: want.families, TCPPorts: want.tcpPorts,
			}
			assertCapturePlan(t, windows.Plan.Capture, capture)
			rendered, err := RenderWindowsCaptureArgv(windows.Plan.Capture)
			if err != nil || !slices.Equal(rendered, want.windowsCaptureArgv) {
				t.Fatalf("Windows WinDivert capture argv mismatch: %v %v", err, rendered)
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
			wantEngine := []string{"--filter-l3=" + tc.value, "--filter-tcp=443", "--lua-desync=multisplit:pos=1"}
			for _, backend := range []Backend{Zapret2Windows, Zapret2Linux} {
				result := Compile(strategy, backend)
				if result.Status != StatusCompiled || !slices.Equal(result.Plan.EngineArgv, wantEngine) {
					t.Fatalf("%s family engine scope mismatch: %#v", backend, result)
				}
				if backend == Zapret2Linux && containsPrefix(result.Plan.EngineArgv, "--wf-") {
					t.Fatalf("Linux EngineArgv leaked WinDivert flags: %v", result.Plan.EngineArgv)
				}
			}
			windows := Compile(strategy, Zapret2Windows)
			rendered, err := RenderWindowsCaptureArgv(windows.Plan.Capture)
			if err != nil || !slices.Equal(rendered, []string{"--wf-l3=" + tc.value, "--wf-tcp-out=443"}) {
				t.Fatalf("Windows capture family scope mismatch: %v %v", err, rendered)
			}
			wantTPWS := []string{"--filter-l3=" + tc.value, "--filter-tcp=443", "--split-pos=1"}
			tpws := Compile(strategy, Zapret1TPWSDarwin)
			if tpws.Status != StatusCompiled || !slices.Equal(tpws.Plan.EngineArgv, wantTPWS) || containsPrefix(tpws.Plan.EngineArgv, "--wf-") {
				t.Fatalf("tpws family scope mismatch: %#v", tpws)
			}
			assertCapturePlan(t, tpws.Plan.Capture, CapturePlan{
				BackendKind: CaptureSOCKSTCP, Transport: CaptureTransportTCP, Direction: strategyir.DirectionOutbound,
				IPFamilies: []strategyir.IPFamily{tc.family}, TCPPorts: []strategyir.PortRange{{Start: 443, End: 443}},
			})
		})
	}
}

func TestCaptureCapabilitiesAndPlansAreSeparate(t *testing.T) {
	strategy := strategyir.RepresentativeFixtures()["alternative-multisplit"]
	strategy.Selector.IPFamilies = []strategyir.IPFamily{strategyir.IPFamilyAny}
	windows, linux := Compile(strategy, Zapret2Windows), Compile(strategy, Zapret2Linux)
	if windows.Status != StatusCompiled || linux.Status != StatusCompiled || windows.StrategyFingerprint != linux.StrategyFingerprint {
		t.Fatalf("capture backend changed strategy identity: %#v %#v", windows, linux)
	}
	if !slices.Equal(windows.Plan.EngineArgv, linux.Plan.EngineArgv) || windows.Plan.Capture.BackendKind != CaptureWinDivert || linux.Plan.Capture.BackendKind != CaptureNFQUEUE {
		t.Fatalf("backend plan split changed profile semantics: %#v %#v", windows.Plan, linux.Plan)
	}
	repeat := Compile(strategy, Zapret2Linux)
	assertCapturePlan(t, repeat.Plan.Capture, linux.Plan.Capture)

	caps := Get(Zapret2Windows)
	caps.Profile.IPFamilies = []strategyir.IPFamily{strategyir.IPFamilyV4}
	if !hasReason(compatibility(strategy, caps), UnsupportedIPFamily) {
		t.Fatal("ANY profile filter scope was broadened for an IPv4-only engine")
	}
	caps = Get(Zapret2Windows)
	caps.Capture.IPFamilies = []strategyir.IPFamily{strategyir.IPFamilyV4}
	if !hasReason(compatibility(strategy, caps), UnsupportedIPFamily) {
		t.Fatal("ANY capture scope was broadened for an IPv4-only capture backend")
	}
	tpws := Get(Zapret1TPWSDarwin)
	if tpws.Capture.BackendKind != CaptureSOCKSTCP || !slices.Equal(tpws.Capture.Transports, []CaptureTransport{CaptureTransportTCP}) || !slices.Equal(tpws.Capture.Directions, []strategyir.Direction{strategyir.DirectionOutbound}) {
		t.Fatalf("tpws capture capability invented packet capture: %#v", tpws.Capture)
	}
}

func assertCapturePlan(t *testing.T, got, want CapturePlan) {
	t.Helper()
	if got.BackendKind != want.BackendKind || got.Transport != want.Transport || got.Direction != want.Direction ||
		!slices.Equal(got.IPFamilies, want.IPFamilies) || !slices.Equal(got.TCPPorts, want.TCPPorts) ||
		!slices.Equal(got.UDPPorts, want.UDPPorts) || !slices.Equal(got.RawCaptureRefs, want.RawCaptureRefs) {
		t.Fatalf("capture plan mismatch\nwant: %#v\ngot:  %#v", want, got)
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

package strategyir

// RepresentativeFixtures shadow one semantic section of trusted production profiles.
// They do not alter the legacy runtime profile executor.
func RepresentativeFixtures() map[string]Strategy {
	return map[string]Strategy{
		"recommended-hostfakesplit": {
			SchemaVersion: SchemaVersion, ID: "recommended-hostfakesplit", Name: "Recommended (hostfakesplit)",
			Transport:  []Transport{TransportTCP},
			Selector:   TrafficSelector{ApplicationProtocols: []ApplicationProtocol{ApplicationTLS}, IPFamilies: []IPFamily{IPFamilyV4, IPFamilyV6}, Direction: DirectionOutbound, TCPPorts: []PortRange{{Start: 80, End: 80}, {Start: 443, End: 443}}, Scope: Scope{Host: HostScope{Mode: HostScopeManagedList, ID: "youtube"}, ExcludeHostListIDs: []string{"steam-web-exclude"}, ExcludeIPSetIDs: []string{"ipset-steam-exclude"}}},
			Range:      &Cutoff{Direction: RangeDirectionOut, Counter: RangeCounterDataPacketNumber, Limit: 8},
			Operations: []Operation{{Type: OperationHostFakeSplit, Positions: []PositionExpr{{Anchor: AnchorMidSLD}}, HostTemplate: "ozon.ru", Fake: &FakeModifiers{Repeat: 4, TCPMD5: true, TCPTimestamp: true}}},
			Safety:     SafetyPolicy{Aggressiveness: "LOW", TargetOnly: true}, Metadata: Metadata{Source: "GetProfiles Recommended selected TCP section"},
		},
		"alternative-multisplit": {
			SchemaVersion: SchemaVersion, ID: "alternative-multisplit", Name: "Alternative 1 (multisplit)",
			Transport:  []Transport{TransportTCP},
			Selector:   TrafficSelector{ApplicationProtocols: []ApplicationProtocol{ApplicationTLS}, IPFamilies: []IPFamily{IPFamilyAny}, Direction: DirectionOutbound, TCPPorts: []PortRange{{Start: 80, End: 80}, {Start: 443, End: 443}}, Scope: Scope{Host: HostScope{Mode: HostScopeManagedList, ID: "youtube"}}},
			Range:      &Cutoff{Direction: RangeDirectionOut, Counter: RangeCounterDataPacketNumber, Limit: 8},
			Operations: []Operation{{Type: OperationMultiSplit, Positions: []PositionExpr{{Absolute: new(2)}}, SequenceOverlap: new(652), OverlapPatternRef: "tls-google"}},
			Safety:     SafetyPolicy{Aggressiveness: "LOW", TargetOnly: true}, Metadata: Metadata{Source: "GetProfiles Alternative 1 selected TCP section"},
		},
		"alternative-fake-tls": {
			SchemaVersion: SchemaVersion, ID: "alternative-fake-tls", Name: "Alternative 2 (fake TLS)",
			Transport:  []Transport{TransportTCP},
			Selector:   TrafficSelector{ApplicationProtocols: []ApplicationProtocol{ApplicationTLS}, IPFamilies: []IPFamily{IPFamilyAny}, Direction: DirectionOutbound, TCPPorts: []PortRange{{Start: 443, End: 443}}, Scope: Scope{Host: HostScope{Mode: HostScopeAll}}},
			Range:      &Cutoff{Direction: RangeDirectionOut, Counter: RangeCounterDataPacketNumber, Limit: 8},
			Operations: []Operation{{Type: OperationFakeInjection, PayloadRef: "tls-clienthello-default", Fake: &FakeModifiers{Repeat: 11, AcknowledgmentOffset: new(-66000), TCPTimestamp: true}}, {Type: OperationMultiDisorder, Positions: []PositionExpr{{Absolute: new(1)}, {Anchor: AnchorMidSLD}}, Fake: &FakeModifiers{Repeat: 11}}},
			Safety:     SafetyPolicy{Aggressiveness: "HIGH", TargetOnly: false, MayAffectTLS: true}, Metadata: Metadata{Source: "GetProfiles Alternative 2 selected TCP section"},
		},
		"discord-tcp": {
			SchemaVersion: SchemaVersion, ID: "discord-tcp", Name: "Discord TCP profile", Transport: []Transport{TransportTCP},
			Selector:   TrafficSelector{ApplicationProtocols: []ApplicationProtocol{ApplicationTLS}, IPFamilies: []IPFamily{IPFamilyAny}, Direction: DirectionOutbound, TCPPorts: []PortRange{{Start: 443, End: 443}, {Start: 5222, End: 5223}, {Start: 5228, End: 5228}}, Scope: Scope{Host: HostScope{Mode: HostScopeExplicit, Hosts: []string{"discord.com", "gateway.discord.gg"}}}},
			Operations: []Operation{{Type: OperationMultiSplit, Positions: []PositionExpr{{Absolute: new(1)}}}},
			Safety:     SafetyPolicy{Aggressiveness: "LOW", TargetOnly: true}, Metadata: Metadata{Source: "Linux Discord Voice Optimized TCP semantics"},
		},
		"steam-safe-game-filter": {
			SchemaVersion: SchemaVersion, ID: "steam-safe-game-filter", Name: "Games & Steam (Game Filter)", Transport: []Transport{TransportTCP},
			Selector:   TrafficSelector{ApplicationProtocols: []ApplicationProtocol{ApplicationAny}, IPFamilies: []IPFamily{IPFamilyAny}, Direction: DirectionOutbound, TCPPorts: []PortRange{{Start: 1024, End: 65535}}, Scope: Scope{Host: HostScope{Mode: HostScopeIPSetReference, ID: "ipset-all"}, ExcludeHostListIDs: []string{"steam-web-exclude"}, ExcludeIPSetIDs: []string{"ipset-exclude"}}},
			Range:      &Cutoff{Direction: RangeDirectionOut, Counter: RangeCounterDataPacketNumber, Limit: 3},
			Operations: []Operation{{Type: OperationMultiSplit, Positions: []PositionExpr{{Absolute: new(1)}}, SequenceOverlap: new(652), OverlapPatternRef: "tls-google"}},
			Safety:     SafetyPolicy{Aggressiveness: "MEDIUM", TargetOnly: true, MayAffectSteam: true}, Metadata: Metadata{Source: "GetGamesSteamProfiles game TCP section"},
		},
	}
}

type Coverage string

const (
	Representable          Coverage = "REPRESENTABLE"
	PartiallyRepresentable Coverage = "PARTIALLY_REPRESENTABLE"
	Unrepresentable        Coverage = "UNREPRESENTABLE"
)

type LegacyCoverage struct {
	Profile           string   `json:"profile"`
	Coverage          Coverage `json:"coverage"`
	MissingCapability string   `json:"missing_capability,omitempty"`
}

// CatalogCoverage is a declared migration report for reviewed static source profiles.
// It does not parse or verify saved/custom profile data.
func CatalogCoverage() []LegacyCoverage {
	return []LegacyCoverage{
		{"Recommended (hostfakesplit)", PartiallyRepresentable, "multi-section capture/raw filter composition"},
		{"Alternative 1 (multisplit)", PartiallyRepresentable, "multi-section capture/raw filter composition"},
		{"Alternative 2 (fake TLS)", PartiallyRepresentable, "inline hex payload blob"},
		{"Alternative 3 (multisplit SNI)", Unrepresentable, "custom Lua function, syndata, and AutoTTL"},
		{"Universal 2026 (All-in-One)", PartiallyRepresentable, "multi-section capture/raw filter composition"},
		{"Advanced profiles", PartiallyRepresentable, "inline payload mutation and multi-section composition"},
		{"Games & Steam (Game Filter)", PartiallyRepresentable, "UDP game block and multi-section capture composition"},
		{"Adaptive (Experimental)", Unrepresentable, "dynamic Lua orchestration"},
		{"AutoHostlist (Dynamic Detection)", PartiallyRepresentable, "feedback thresholds and inbound event semantics"},
		{"Linux built-in profiles", PartiallyRepresentable, "NFQUEUE/firewall ownership remains executor scope"},
		{"macOS Standard HTTPS/QUIC (tpws TCP strategy)", Representable, ""},
		{"macOS QUIC fallback profiles", PartiallyRepresentable, "PF UDP fallback is executor policy, not packet StrategyIR"},
		{"Saved discovered profiles", Unrepresentable, "opaque legacy argv import is intentionally absent"},
	}
}

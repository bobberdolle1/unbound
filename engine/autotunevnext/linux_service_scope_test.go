package autotunevnext

import (
	"fmt"
	"net"
	"strings"
	"testing"

	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/strategyir"
)

func linuxScopeEdge(ip string, family observatory.AddressFamily) ServiceScopeEdge {
	return ServiceScopeEdge{IP: net.ParseIP(ip), Family: family}
}

func TestLinuxServiceScopeNFQueueSpecRendersExactFamilies(t *testing.T) {
	capture := exactCapture(backendcap.CaptureNFQUEUE, strategyir.IPFamilyAny, strategyir.DirectionOutbound, strategyir.PortRange{Start: 443, End: 443})
	cases := []struct {
		name      string
		edges     []ServiceScopeEdge
		family    string
		addresses []string
		rules     int
	}{
		{
			name:      "IPv4 only",
			edges:     []ServiceScopeEdge{linuxScopeEdge("192.0.2.2", observatory.AddressFamilyIPv4), linuxScopeEdge("192.0.2.1", observatory.AddressFamilyIPv4)},
			family:    "ip",
			addresses: []string{"ip daddr { 192.0.2.1, 192.0.2.2 }"},
			rules:     1,
		},
		{
			name:      "IPv6 only",
			edges:     []ServiceScopeEdge{linuxScopeEdge("2001:db8::2", observatory.AddressFamilyIPv6), linuxScopeEdge("2001:db8::1", observatory.AddressFamilyIPv6)},
			family:    "ip6",
			addresses: []string{"ip6 daddr { 2001:db8::1, 2001:db8::2 }"},
			rules:     1,
		},
		{
			name: "mixed families deduplicated",
			edges: []ServiceScopeEdge{
				linuxScopeEdge("2001:db8::1", observatory.AddressFamilyIPv6),
				linuxScopeEdge("192.0.2.2", observatory.AddressFamilyIPv4),
				linuxScopeEdge("192.0.2.1", observatory.AddressFamilyIPv4),
				linuxScopeEdge("192.0.2.2", observatory.AddressFamilyIPv4),
			},
			family:    "inet",
			addresses: []string{"ip daddr { 192.0.2.1, 192.0.2.2 }", "ip6 daddr 2001:db8::1"},
			rules:     2,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spec, err := NewLinuxServiceScopeNFQueueSpec(capture, tc.edges, "unbound_autotune_scope", 40123, linuxOwnershipPrefix+":scope")
			if err != nil {
				t.Fatal(err)
			}
			if got := spec.nftFamily(); got != tc.family {
				t.Fatalf("nft family=%q want=%q", got, tc.family)
			}
			script := spec.nftScript()
			for _, fragment := range append(tc.addresses, "tcp dport { 443 }", "meta mark and 0x40000000 != 0x40000000", "queue num 40123 bypass", `comment "unbound-autotune-vnext:scope"`) {
				if !strings.Contains(script, fragment) {
					t.Fatalf("script missing %q: %s", fragment, script)
				}
			}
			if got := strings.Count(script, "queue num 40123 bypass"); got != tc.rules {
				t.Fatalf("queue rule count=%d want=%d: %s", got, tc.rules, script)
			}
			if strings.Contains(script, "0.0.0.0") || strings.Contains(script, "::/0") {
				t.Fatalf("scope widened capture: %s", script)
			}
		})
	}
}

func TestLinuxServiceScopeNFQueueSpecRejectsUnrepresentableScope(t *testing.T) {
	capture := exactCapture(backendcap.CaptureNFQUEUE, strategyir.IPFamilyV4, strategyir.DirectionOutbound, strategyir.PortRange{Start: 443, End: 443})
	mixed := []ServiceScopeEdge{linuxScopeEdge("192.0.2.1", observatory.AddressFamilyIPv4), linuxScopeEdge("2001:db8::1", observatory.AddressFamilyIPv6)}
	if _, err := NewLinuxServiceScopeNFQueueSpec(capture, mixed, "table", 40123, "marker"); err == nil {
		t.Fatal("accepted mixed scope when compiled capture omitted IPv6")
	}
	overflow := make([]ServiceScopeEdge, MaxServiceScopeEdges+1)
	for index := range overflow {
		overflow[index] = linuxScopeEdge(net.IPv4(192, 0, 2, byte(index+1)).String(), observatory.AddressFamilyIPv4)
	}
	if _, err := NewLinuxServiceScopeNFQueueSpec(exactCapture(backendcap.CaptureNFQUEUE, strategyir.IPFamilyAny, strategyir.DirectionOutbound, strategyir.PortRange{Start: 443, End: 443}), overflow, "table", 40123, "marker"); err == nil {
		t.Fatal("accepted an over-bound service scope")
	}
}

func nftListingForSpec(spec LinuxNFQueueSpec) string {
	rules := make([]string, 0, len(spec.nftAddressExpressions()))
	for _, address := range spec.nftAddressExpressions() {
		rules = append(rules, fmt.Sprintf("%s tcp dport %s meta mark and 0x40000000 != 0x40000000 queue num %d bypass comment %q", address, spec.nftPorts(), spec.Queue, spec.Marker))
	}
	return fmt.Sprintf("table %s %s { chain output { type filter hook output priority mangle; %s; } }", spec.nftFamily(), spec.Table, strings.Join(rules, "; "))
}

func mutateNFTRule(listing, family, old, replacement string) string {
	needle := family + " daddr"
	start := strings.Index(listing, needle)
	if start < 0 {
		panic("missing nft test rule")
	}
	end := start + strings.Index(listing[start:], ";")
	if end < start {
		panic("unterminated nft test rule")
	}
	return listing[:start] + strings.Replace(listing[start:end], old, replacement, 1) + listing[end:]
}

func TestVerifyNFTRuleSemanticsRequiresCompletePerFamilyRules(t *testing.T) {
	capture := exactCapture(backendcap.CaptureNFQUEUE, strategyir.IPFamilyAny, strategyir.DirectionOutbound, strategyir.PortRange{Start: 443, End: 443})
	spec, err := NewLinuxServiceScopeNFQueueSpec(capture, []ServiceScopeEdge{
		linuxScopeEdge("192.0.2.1", observatory.AddressFamilyIPv4),
		linuxScopeEdge("192.0.2.2", observatory.AddressFamilyIPv4),
		linuxScopeEdge("2001:db8::1", observatory.AddressFamilyIPv6),
	}, "unbound_autotune_mixed", 40123, linuxOwnershipPrefix+":mixed")
	if err != nil {
		t.Fatal(err)
	}
	listing := nftListingForSpec(spec)
	if err := verifyNFTRuleSemantics(listing, spec); err != nil {
		t.Fatalf("complete mixed rules rejected: %v", err)
	}
	cases := []struct {
		name   string
		family string
		old    string
		new    string
	}{
		{name: "IPv4 missing queue", family: "ip", old: "queue num 40123 bypass", new: "queue num 40124 bypass"},
		{name: "IPv4 missing marker", family: "ip", old: `comment "unbound-autotune-vnext:mixed"`, new: `comment "other"`},
		{name: "IPv4 missing mark", family: "ip", old: "meta mark and 0x40000000 != 0x40000000", new: "meta mark 0"},
		{name: "IPv4 missing port", family: "ip", old: "tcp dport { 443 }", new: "tcp dport { 80 }"},
		{name: "IPv4 missing edge", family: "ip", old: "192.0.2.1, ", new: ""},
		{name: "IPv4 missing bypass", family: "ip", old: "queue num 40123 bypass", new: "queue num 40123"},
		{name: "IPv6 wrong queue", family: "ip6", old: "queue num 40123 bypass", new: "queue num 40124 bypass"},
		{name: "IPv6 missing marker", family: "ip6", old: `comment "unbound-autotune-vnext:mixed"`, new: `comment "other"`},
		{name: "IPv6 missing mark", family: "ip6", old: "meta mark and 0x40000000 != 0x40000000", new: "meta mark 0"},
		{name: "IPv6 missing port", family: "ip6", old: "tcp dport { 443 }", new: "tcp dport { 80 }"},
		{name: "IPv6 missing edge", family: "ip6", old: "2001:db8::1", new: "2001:db8::2"},
		{name: "IPv6 missing bypass", family: "ip6", old: "queue num 40123 bypass", new: "queue num 40123"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := verifyNFTRuleSemantics(mutateNFTRule(listing, tc.family, tc.old, tc.new), spec); err == nil {
				t.Fatal("accepted incomplete family rule")
			}
		})
	}
}

func TestVerifyNFTRuleSemanticsRejectsCrossRuleFragments(t *testing.T) {
	capture := exactCapture(backendcap.CaptureNFQUEUE, strategyir.IPFamilyAny, strategyir.DirectionOutbound, strategyir.PortRange{Start: 443, End: 443})
	spec, err := NewLinuxServiceScopeNFQueueSpec(capture, []ServiceScopeEdge{
		linuxScopeEdge("192.0.2.1", observatory.AddressFamilyIPv4),
		linuxScopeEdge("2001:db8::1", observatory.AddressFamilyIPv6),
	}, "unbound_autotune_mixed", 40123, linuxOwnershipPrefix+":mixed")
	if err != nil {
		t.Fatal(err)
	}
	listing := nftListingForSpec(spec)
	listing = mutateNFTRule(listing, "ip", "queue num 40123 bypass comment \"unbound-autotune-vnext:mixed\"", "")
	listing = strings.Replace(listing, "; } }", `; ip daddr 198.51.100.7 tcp dport { 443 } meta mark and 0x40000000 != 0x40000000 queue num 40123 bypass comment "unbound-autotune-vnext:mixed"; } }`, 1)
	if err := verifyNFTRuleSemantics(listing, spec); err == nil {
		t.Fatal("accepted required fragments split across different rules")
	}
}

func TestVerifyNFTRuleSemanticsAcceptsExactSingleAndMultiFamilyRules(t *testing.T) {
	capture := exactCapture(backendcap.CaptureNFQUEUE, strategyir.IPFamilyAny, strategyir.DirectionOutbound, strategyir.PortRange{Start: 443, End: 443})
	cases := [][]ServiceScopeEdge{
		{linuxScopeEdge("192.0.2.1", observatory.AddressFamilyIPv4)},
		{linuxScopeEdge("192.0.2.1", observatory.AddressFamilyIPv4), linuxScopeEdge("192.0.2.2", observatory.AddressFamilyIPv4)},
		{linuxScopeEdge("2001:db8::1", observatory.AddressFamilyIPv6)},
		{linuxScopeEdge("2001:db8::1", observatory.AddressFamilyIPv6), linuxScopeEdge("2001:db8::2", observatory.AddressFamilyIPv6)},
		{linuxScopeEdge("192.0.2.1", observatory.AddressFamilyIPv4), linuxScopeEdge("2001:db8::1", observatory.AddressFamilyIPv6)},
	}
	for _, edges := range cases {
		spec, err := NewLinuxServiceScopeNFQueueSpec(capture, edges, "unbound_autotune_scope", 40123, linuxOwnershipPrefix+":scope")
		if err != nil {
			t.Fatal(err)
		}
		if err := verifyNFTRuleSemantics(nftListingForSpec(spec), spec); err != nil {
			t.Fatalf("exact rules rejected for %#v: %v", edges, err)
		}
	}
}

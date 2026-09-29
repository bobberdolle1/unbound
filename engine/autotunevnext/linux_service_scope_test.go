package autotunevnext

import (
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

package autotunevnext

import (
	"net"
	"strings"
	"testing"

	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/strategyir"
)

func exactCapture(kind backendcap.CaptureBackendKind, family strategyir.IPFamily, direction strategyir.Direction, ports ...strategyir.PortRange) backendcap.CapturePlan {
	return backendcap.CapturePlan{BackendKind: kind, Transport: backendcap.CaptureTransportTCP, Direction: direction, IPFamilies: []strategyir.IPFamily{family}, TCPPorts: ports}
}

func TestRenderWindowsTargetCaptureIsExact(t *testing.T) {
	cases := []struct {
		name, edge, want string
		family           observatory.AddressFamily
		capture          backendcap.CapturePlan
	}{
		{"ipv4 tcp range", "192.0.2.7", "(outbound and ip.DstAddr == 192.0.2.7 and (tcp.DstPort == 443 or (tcp.DstPort >= 50000 and tcp.DstPort <= 50010))) or (inbound and ip.SrcAddr == 192.0.2.7 and (tcp.SrcPort == 443 or (tcp.SrcPort >= 50000 and tcp.SrcPort <= 50010)))", observatory.AddressFamilyIPv4, exactCapture(backendcap.CaptureWinDivert, strategyir.IPFamilyV4, strategyir.DirectionOutbound, strategyir.PortRange{Start: 443, End: 443}, strategyir.PortRange{Start: 50000, End: 50010})},
		{"ipv6 tcp 443", "2001:db8::7", "(outbound and ipv6.DstAddr == 2001:db8::7 and (tcp.DstPort == 443)) or (inbound and ipv6.SrcAddr == 2001:db8::7 and (tcp.SrcPort == 443))", observatory.AddressFamilyIPv6, exactCapture(backendcap.CaptureWinDivert, strategyir.IPFamilyV6, strategyir.DirectionOutbound, strategyir.PortRange{Start: 443, End: 443})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := RenderWindowsTargetCapture(tc.capture, net.ParseIP(tc.edge), tc.family)
			if err != nil || got != tc.want {
				t.Fatalf("filter=%q err=%v", got, err)
			}
		})
	}
	capture := exactCapture(backendcap.CaptureWinDivert, strategyir.IPFamilyV4, strategyir.DirectionOutbound, strategyir.PortRange{Start: 443, End: 443})
	if _, err := RenderWindowsTargetCapture(capture, net.ParseIP("2001:db8::1"), observatory.AddressFamilyIPv6); err == nil {
		t.Fatal("accepted wrong target family")
	}
	if _, err := RenderWindowsTargetCapture(capture, nil, observatory.AddressFamilyIPv4); err == nil {
		t.Fatal("accepted empty target edge")
	}
	udp := capture
	udp.Transport = backendcap.CaptureTransportUDP
	udp.TCPPorts = nil
	udp.UDPPorts = []strategyir.PortRange{{Start: 443, End: 443}}
	if _, err := RenderWindowsTargetCapture(udp, net.ParseIP("192.0.2.7"), observatory.AddressFamilyIPv4); err == nil {
		t.Fatal("expanded physical target guard to unproven UDP semantics")
	}
}

func TestLinuxNFQueueRuleIsExactAndOwned(t *testing.T) {
	capture := exactCapture(backendcap.CaptureNFQUEUE, strategyir.IPFamilyV4, strategyir.DirectionOutbound, strategyir.PortRange{Start: 443, End: 443}, strategyir.PortRange{Start: 50000, End: 50010})
	spec, err := NewLinuxNFQueueSpec(capture, net.ParseIP("192.0.2.9"), observatory.AddressFamilyIPv4, "unbound_autotune_abc", 40123, linuxOwnershipPrefix+":abc")
	if err != nil {
		t.Fatal(err)
	}
	script := spec.nftScript()
	for _, fragment := range []string{"ip daddr 192.0.2.9", "tcp dport { 443, 50000-50010 }", "queue num 40123 bypass", `comment "unbound-autotune-vnext:abc"`} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("nft rule missing %q: %s", fragment, script)
		}
	}
	if lines := strings.Count(strings.TrimSpace(script), "\n") + 1; lines != 3 {
		t.Fatalf("nft owned object must be one three-command batch, got %d lines: %s", lines, script)
	}
	args := strings.Join(spec.iptablesArgs("-I"), " ")
	for _, fragment := range []string{"-d 192.0.2.9", "--dports 443,50000:50010", "--queue-num 40123", "--queue-bypass", linuxOwnershipPrefix + ":abc"} {
		if !strings.Contains(args, fragment) {
			t.Fatalf("iptables rule missing %q: %s", fragment, args)
		}
	}
	wrong := exactCapture(backendcap.CaptureNFQUEUE, strategyir.IPFamilyV6, strategyir.DirectionOutbound, strategyir.PortRange{Start: 443, End: 443})
	if _, err := NewLinuxNFQueueSpec(wrong, net.ParseIP("192.0.2.9"), observatory.AddressFamilyIPv4, "t", 1, "m"); err == nil {
		t.Fatal("accepted wrong family NFQUEUE rule")
	}
	inbound := capture
	inbound.Direction = strategyir.DirectionInbound
	if _, err := NewLinuxNFQueueSpec(inbound, net.ParseIP("192.0.2.9"), observatory.AddressFamilyIPv4, "t", 1, "m"); err == nil {
		t.Fatal("accepted inbound NFQUEUE rule")
	}
}

package engine

import (
	"context"
	"net"
	"strings"
	"testing"
)

func TestGenerateWinDivertFilterForIPs(t *testing.T) {
	ip1 := net.ParseIP("192.168.1.100")
	ip2 := net.ParseIP("10.0.0.1")

	// Single IPv4 on port 443
	f1, err := GenerateWinDivertFilterForIPs([]net.IP{ip1}, []int{443}, "tcp")
	if err != nil {
		t.Fatalf("GenerateWinDivertFilterForIPs failed: %v", err)
	}
	if !strings.Contains(f1, "tcp.DstPort == 443") || !strings.Contains(f1, "ip.DstAddr == 192.168.1.100") {
		t.Errorf("Unexpected filter expression: %s", f1)
	}

	// Multiple IPv4
	f2, err := GenerateWinDivertFilterForIPs([]net.IP{ip1, ip2}, []int{443, 80}, "tcp")
	if err != nil {
		t.Fatalf("GenerateWinDivertFilterForIPs failed: %v", err)
	}
	if !strings.Contains(f2, "192.168.1.100") || !strings.Contains(f2, "10.0.0.1") {
		t.Errorf("Filter missing IPs: %s", f2)
	}
	if !strings.Contains(f2, "80") || !strings.Contains(f2, "443") {
		t.Errorf("Filter missing ports: %s", f2)
	}

	// IPv6 support
	ipv6 := net.ParseIP("2606:4700:4700::1111")
	f6, err := GenerateWinDivertFilterForIPs([]net.IP{ipv6}, []int{443}, "tcp")
	if err != nil {
		t.Fatalf("GenerateWinDivertFilterForIPs failed for IPv6: %v", err)
	}
	if !strings.Contains(f6, "ipv6.DstAddr == 2606:4700:4700::1111") {
		t.Errorf("Unexpected IPv6 filter: %s", f6)
	}
}

func TestGenerateWinDivertFilterRejectsUnsafeInput(t *testing.T) {
	// Empty IP list
	_, err := GenerateWinDivertFilterForIPs(nil, []int{443}, "tcp")
	if err == nil {
		t.Error("Expected error on empty IP list")
	}

	// Empty port list
	ip := net.ParseIP("1.1.1.1")
	_, err = GenerateWinDivertFilterForIPs([]net.IP{ip}, nil, "tcp")
	if err == nil {
		t.Error("Expected error on empty port list")
	}

	// Invalid port
	_, err = GenerateWinDivertFilterForIPs([]net.IP{ip}, []int{0}, "tcp")
	if err == nil {
		t.Error("Expected error on port 0")
	}
	_, err = GenerateWinDivertFilterForIPs([]net.IP{ip}, []int{70000}, "tcp")
	if err == nil {
		t.Error("Expected error on port > 65535")
	}
}

func TestBuildIsolatedWinDivertFilter(t *testing.T) {
	ctx := context.Background()

	// Direct IP target
	cfgIP := TargetFilterConfig{TargetHost: "1.1.1.1", Ports: []int{443}, Protocol: "tcp"}
	f, ips, err := BuildIsolatedWinDivertFilter(ctx, cfgIP)
	if err != nil {
		t.Fatalf("BuildIsolatedWinDivertFilter failed on IP: %v", err)
	}
	if len(ips) != 1 || !ips[0].Equal(net.ParseIP("1.1.1.1")) {
		t.Errorf("Unexpected resolved IPs: %v", ips)
	}
	if !strings.Contains(f, "1.1.1.1") {
		t.Errorf("Filter missing IP: %s", f)
	}

	// Empty host
	_, _, err = BuildIsolatedWinDivertFilter(ctx, TargetFilterConfig{})
	if err == nil {
		t.Error("Expected error on empty host")
	}
}

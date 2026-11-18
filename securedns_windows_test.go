//go:build windows

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"unbound/engine"
)

func TestIsExcludedSecureDNSAdapterFiltersVirtualAndVPNInterfaces(t *testing.T) {
	excluded := []struct {
		alias string
		desc  string
	}{
		{"vEthernet (WSL)", "Hyper-V Virtual Ethernet Adapter"},
		{"Ethernet 2", "Hyper-V Virtual Switch Extension Adapter"},
		{"Tailscale", "Tailscale Tunnel"},
		{"wg0", "WireGuard Tunnel"},
		{"utun0", "Wintun Userspace Tunnel"},
		{"OpenVPN", "TAP-Windows Adapter V9"},
		{"SingBox", "sing-box Tun Interface"},
		{"Meta", "Clash Meta Virtual Adapter"},
		{"Loopback Pseudo-Interface 1", "Software Loopback Interface 1"},
		{"tun0", "Generic Adapter"},
		{"tap1", "Generic Adapter"},
	}
	for _, tc := range excluded {
		if !isExcludedSecureDNSAdapter(tc.alias, tc.desc) {
			t.Errorf("expected adapter (%q, %q) to be excluded", tc.alias, tc.desc)
		}
	}

	physical := []struct {
		alias string
		desc  string
	}{
		{"Ethernet", "Realtek PCIe GbE Family Controller"},
		{"Wi-Fi", "Intel(R) Wi-Fi 6 AX200 160MHz"},
		{"Беспроводная сеть", "Qualcomm FastConnect 6900 Wi-Fi 6E"},
		{"LAN", "Killer E3100G 2.5 Gigabit Ethernet Controller"},
	}
	for _, tc := range physical {
		if isExcludedSecureDNSAdapter(tc.alias, tc.desc) {
			t.Errorf("expected physical adapter (%q, %q) NOT to be excluded", tc.alias, tc.desc)
		}
	}
}

func TestWindowsSecureDNSAppliesOnlyToPhysicalAndProtectsCustomDNS(t *testing.T) {
	restoreDir := engine.SetConfigDirForTest(t.TempDir())
	defer restoreDir()

	prevRunner := runSecureDNSPowerShell
	defer func() { runSecureDNSPowerShell = prevRunner }()

	currentAdapters := []windowsNetAdapterDNS{
		{
			InterfaceIndex:       5,
			InterfaceAlias:       "Ethernet",
			InterfaceDescription: "Intel(R) Ethernet Controller I225-V",
			ServerAddresses:      []string{"10.20.30.40"},
		},
		{
			InterfaceIndex:       17,
			InterfaceAlias:       "Tailscale",
			InterfaceDescription: "Tailscale Tunnel",
			ServerAddresses:      []string{"100.100.100.100"},
		},
		{
			InterfaceIndex:       22,
			InterfaceAlias:       "vEthernet (WSL)",
			InterfaceDescription: "Hyper-V Virtual Ethernet Adapter",
			ServerAddresses:      []string{"172.24.16.1"},
		},
	}

	var executedMutations []string
	runSecureDNSPowerShell = func(script string) ([]byte, error) {
		if script == windowsQueryAdaptersScript {
			return json.Marshal(currentAdapters)
		}
		executedMutations = append(executedMutations, script)
		return nil, nil
	}

	// 1. Disabling SecureDNS when custom user DNS (10.20.30.40) is set and
	// SecureDNS was never applied MUST NOT run -ResetServerAddress!
	if err := setSecureDNSImpl(false); err != nil {
		t.Fatalf("setSecureDNSImpl(false) on untouched custom DNS: %v", err)
	}
	if len(executedMutations) != 0 {
		t.Fatalf("setSecureDNSImpl(false) clobbered untouched custom DNS: %v", executedMutations)
	}

	// 2. Enabling SecureDNS applies ONLY to InterfaceIndex 5 (Ethernet), skipping Tailscale & WSL,
	// and snapshots 10.20.30.40.
	if err := setSecureDNSImpl(true); err != nil {
		t.Fatalf("setSecureDNSImpl(true): %v", err)
	}
	if len(executedMutations) != 1 {
		t.Fatalf("expected 1 PowerShell mutation, got %d: %v", len(executedMutations), executedMutations)
	}
	if !strings.Contains(executedMutations[0], "-InterfaceIndex 5 ") ||
		strings.Contains(executedMutations[0], "17") ||
		strings.Contains(executedMutations[0], "22") ||
		!strings.Contains(executedMutations[0], "'1.1.1.1'") {
		t.Fatalf("unexpected enable script: %s", executedMutations[0])
	}

	// Simulate OS now reporting 1.1.1.1 on Ethernet and also 1.1.1.1 on Clash virtual NIC
	currentAdapters[0].ServerAddresses = []string{"1.1.1.1", "1.0.0.1"}
	if !isSecureDNSEnabledImpl() {
		t.Fatal("expected isSecureDNSEnabledImpl() == true when physical adapter has 1.1.1.1")
	}

	// 3. Disabling SecureDNS restores the snapshotted static DNS (10.20.30.40) on InterfaceIndex 5!
	executedMutations = nil
	if err := setSecureDNSImpl(false); err != nil {
		t.Fatalf("setSecureDNSImpl(false) after enable: %v", err)
	}
	if len(executedMutations) != 1 {
		t.Fatalf("expected 1 restore mutation, got %d: %v", len(executedMutations), executedMutations)
	}
	if !strings.Contains(executedMutations[0], "-InterfaceIndex 5 -ServerAddresses @('10.20.30.40')") {
		t.Fatalf("expected prior static DNS restore, got: %s", executedMutations[0])
	}

	// 4. If 8.8.8.8 is present without a saved snapshot, disabling resets ONLY InterfaceIndex 5.
	currentAdapters[0].ServerAddresses = []string{"8.8.8.8", "8.8.4.4"}
	executedMutations = nil
	if err := setSecureDNSImpl(false); err != nil {
		t.Fatalf("setSecureDNSImpl(false) with 8.8.8.8: %v", err)
	}
	if len(executedMutations) != 1 || !strings.Contains(executedMutations[0], "-InterfaceIndex 5 -ResetServerAddress") {
		t.Fatalf("expected -ResetServerAddress on InterfaceIndex 5, got: %v", executedMutations)
	}
}

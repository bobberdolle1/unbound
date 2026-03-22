package engine

import (
	"testing"
)

func TestRunDiagnostics(t *testing.T) {
	report := RunDiagnostics()
	
	if len(report.Results) == 0 {
		t.Fatal("Expected diagnostic results, got none")
	}
	
	if report.Summary == "" {
		t.Error("Expected summary, got empty string")
	}
	
	t.Logf("Diagnostics Score: %d", report.Score)
	t.Logf("Summary: %s", report.Summary)
	
	for _, result := range report.Results {
		t.Logf("[%s] %s: %s", result.Status, result.Name, result.Message)
	}
}

func TestCheckBaseFilteringEngine(t *testing.T) {
	result := checkBaseFilteringEngine()
	
	if result.Name != "Base Filtering Engine" {
		t.Errorf("Expected name 'Base Filtering Engine', got '%s'", result.Name)
	}
	
	t.Logf("BFE Status: %s - %s", result.Status, result.Message)
}

func TestCheckProxySettings(t *testing.T) {
	result := checkProxySettings()
	
	if result.Name != "Proxy Settings" {
		t.Errorf("Expected name 'Proxy Settings', got '%s'", result.Name)
	}
	
	t.Logf("Proxy Status: %s - %s", result.Status, result.Message)
}

func TestCheckTCPTimestamps(t *testing.T) {
	result := checkTCPTimestamps()
	
	if result.Name != "TCP Timestamps" {
		t.Errorf("Expected name 'TCP Timestamps', got '%s'", result.Name)
	}
	
	t.Logf("TCP Timestamps Status: %s - %s", result.Status, result.Message)
}

func TestCheckConflictingSoftware(t *testing.T) {
	results := checkConflictingSoftware()
	
	if len(results) == 0 {
		t.Fatal("Expected at least one result")
	}
	
	for _, result := range results {
		t.Logf("Software Check: %s - %s", result.Status, result.Message)
	}
}

func TestCheckWinDivertConflicts(t *testing.T) {
	result := checkWinDivertConflicts()
	
	if result.Name != "WinDivert Conflicts" {
		t.Errorf("Expected name 'WinDivert Conflicts', got '%s'", result.Name)
	}
	
	t.Logf("WinDivert Status: %s - %s", result.Status, result.Message)
}

func TestCheckVPNServices(t *testing.T) {
	result := checkVPNServices()
	
	if result.Name != "VPN Services" {
		t.Errorf("Expected name 'VPN Services', got '%s'", result.Name)
	}
	
	t.Logf("VPN Status: %s - %s", result.Status, result.Message)
}

func TestCheckSecureDNS(t *testing.T) {
	result := checkSecureDNS()
	
	if result.Name != "Secure DNS" {
		t.Errorf("Expected name 'Secure DNS', got '%s'", result.Name)
	}
	
	t.Logf("DNS Status: %s - %s", result.Status, result.Message)
}

func TestCheckHostsFile(t *testing.T) {
	result := checkHostsFile()
	
	if result.Name != "Hosts File" {
		t.Errorf("Expected name 'Hosts File', got '%s'", result.Name)
	}
	
	t.Logf("Hosts File Status: %s - %s", result.Status, result.Message)
}

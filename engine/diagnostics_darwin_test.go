//go:build darwin
// +build darwin

package engine

import (
	"testing"
)

func TestRunDiagnostics_Mac(t *testing.T) {
	results := RunDiagnostics()
	if len(results) == 0 {
		t.Fatalf("expected diagnostic results on macOS, got 0")
	}

	foundEngineCheck := false
	foundNetworkCheck := false

	for _, res := range results {
		if res.Component == "Engine Binary" {
			foundEngineCheck = true
		}
		if res.Component == "Network" {
			foundNetworkCheck = true
		}
	}

	if !foundEngineCheck {
		t.Errorf("expected 'Engine Binary' diagnostic check in results")
	}
	if !foundNetworkCheck {
		t.Errorf("expected 'Network' diagnostic check in results")
	}
}

func TestCheckEngineStatusMac(t *testing.T) {
	result := checkEngineStatusMac()
	if result.Component != "Engine Binary" {
		t.Errorf("expected Component to be 'Engine Binary', got %q", result.Component)
	}
}

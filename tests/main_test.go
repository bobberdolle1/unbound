package main

import (
	"os"
	"runtime"
	"testing"
	"unbound/engine"
)

// hasElevatedPrivileges reports whether the test process can touch the network
// stack. This previously shelled out to `net session`, which only exists on
// Windows, so on Linux/macOS the command simply failed and the test skipped for
// the wrong reason.
func hasElevatedPrivileges() bool {
	if runtime.GOOS == "windows" {
		// An unelevated Windows process cannot open the physical drive
		// namespace; a successful open implies an elevated token.
		f, err := os.Open(`\\.\PHYSICALDRIVE0`)
		if err != nil {
			return false
		}
		f.Close()
		return true
	}
	return os.Geteuid() == 0
}

func TestHealthCheck(t *testing.T) {
	if !hasElevatedPrivileges() {
		t.Skip("Skipping health check - requires administrator/root privileges")
	}

	if err := engine.RunHealthCheck(); err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
}

package main

import (
	"os/exec"
	"testing"
	"unbound/engine"
)

func TestHealthCheck(t *testing.T) {
	// Check privileges
	cmd := exec.Command("net", "session")
	if err := cmd.Run(); err != nil {
		t.Skip("Skipping health check - requires administrator privileges")
	}

	err := engine.RunHealthCheck()
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
}

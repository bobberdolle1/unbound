package main

import (
	"testing"
	"unbound/engine"
)

func TestHealthCheck(t *testing.T) {
	err := engine.RunHealthCheck()
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
}

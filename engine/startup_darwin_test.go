//go:build darwin
// +build darwin

package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMacAutoStart_PathResolution(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", MacOSPlistFilename)
	if plistPath == "" {
		t.Errorf("expected non-empty plist path")
	}

	// Test IsAutoStartEnabled doesn't panic
	_, _ = IsAutoStartEnabled()
}

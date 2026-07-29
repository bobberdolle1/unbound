//go:build windows

package engine

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"unbound/engine/providers"
)

func TestAutoTune(t *testing.T) {
	// Check privileges
	cmd := exec.Command("net", "session")
	if err := cmd.Run(); err != nil {
		t.Skip("Skipping TestAutoTune - requires administrator privileges")
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🤖 AUTO-TUNE: Finding optimal bypass profile")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	assets, err := ExtractAssets()
	if err != nil {
		t.Fatalf("Failed to extract assets: %v", err)
	}

	provider := providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, assets.ListDir, assets.EngineSHA256, true, false)

	profiles := GetProfiles(assets.LuaDir)

	for _, prof := range profiles {
		provider.RegisterProfile(prof.Name, prof.Args)
	}

	result, err := RunAutoTuneV2WithContext(context.Background(), provider, profiles)
	if err != nil {
		t.Fatalf("Auto-tune failed: %v", err)
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("🎯 WINNER: %s\n", result.ProfileName)
	fmt.Printf("⏱️  Latency: %v\n", result.Latency)
	fmt.Println("📊 Test Results:")
	for target, targetStatus := range result.Results {
		status := "❌"
		if targetStatus.OK {
			status = "✅"
		}
		fmt.Printf("   %s %s\n", status, target)
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

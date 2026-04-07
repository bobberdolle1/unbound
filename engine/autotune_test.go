package engine

import (
	"context"
	"fmt"
	"testing"
	"unbound/engine/providers"
)

func TestAutoTune(t *testing.T) {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🤖 AUTO-TUNE: Finding optimal bypass profile")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	assets, err := ExtractAssets()
	if err != nil {
		t.Fatalf("Failed to extract assets: %v", err)
	}

	provider := providers.NewZapret2WindowsProvider(
		assets.BinDir,
		assets.LuaDir,
		assets.ListDir,
		true,
		false,
	)

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


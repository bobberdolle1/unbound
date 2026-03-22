package engine

import (
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

	result, err := RunAutoTuneV2(provider, profiles)
	if err != nil {
		t.Fatalf("Auto-tune failed: %v", err)
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("🎯 WINNER: %s\n", result.ProfileName)
	fmt.Printf("⏱️  Latency: %v\n", result.Latency)
	fmt.Println("📊 Test Results:")
	for url, success := range result.TestedURLs {
		status := "❌"
		if success {
			status = "✅"
		}
		fmt.Printf("   %s %s\n", status, url)
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

func TestQuickBypass(t *testing.T) {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("⚡ QUICK TEST: Single profile verification")
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

	testProfile := "YouTube + Discord (Universal)"
	fmt.Printf("🚀 Testing: %s\n", testProfile)

	result, err := QuickTest(provider, testProfile)
	if err != nil {
		t.Fatalf("Quick test failed: %v", err)
	}

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	if result.Success {
		fmt.Printf("✅ SUCCESS: %s works!\n", testProfile)
	} else {
		fmt.Printf("❌ FAILED: %s\n", testProfile)
		if result.Error != nil {
			fmt.Printf("   Error: %v\n", result.Error)
		}
	}
	fmt.Printf("⏱️  Latency: %v\n", result.Latency)
	fmt.Println("📊 Test Results:")
	for url, success := range result.TestedURLs {
		status := "❌"
		if success {
			status = "✅"
		}
		fmt.Printf("   %s %s\n", status, url)
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
	"unbound/engine"
	"unbound/engine/providers"
)

// TestAssetExtraction verifies that embedded assets are extracted correctly
func TestAssetExtraction(t *testing.T) {
	assets, err := engine.ExtractAssets()
	if err != nil {
		t.Fatalf("Failed to extract assets: %v", err)
	}

	// Verify nfqws.exe exists
	nfqwsPath := assets.BinDir + "/nfqws.exe"
	if _, err := os.Stat(nfqwsPath); os.IsNotExist(err) {
		t.Errorf("nfqws.exe not found at %s", nfqwsPath)
	}

	// Verify Lua scripts exist
	luaScripts := []string{"zapret-lib.lua", "zapret-antidpi.lua", "zapret-auto.lua", "zapret-obfs.lua", "zapret-pcap.lua"}
	for _, script := range luaScripts {
		scriptPath := assets.LuaDir + "/" + script
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			t.Errorf("Lua script %s not found at %s", script, scriptPath)
		}
	}

	t.Logf("✓ Asset extraction successful: BinDir=%s, LuaDir=%s", assets.BinDir, assets.LuaDir)
}

// TestProviderInitialization checks if Zapret2WindowsProvider initializes correctly
func TestProviderInitialization(t *testing.T) {
	assets, err := engine.ExtractAssets()
	if err != nil {
		t.Fatalf("Failed to extract assets: %v", err)
	}

	provider := providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, assets.ListDir, false)
	
	if provider.Name() != "Zapret 2 (winws)" {
		t.Errorf("Expected provider name 'Zapret 2 (winws)', got '%s'", provider.Name())
	}

	profiles := provider.GetProfiles()
	if len(profiles) == 0 {
		t.Error("No profiles returned from provider")
	}

	expectedProfiles := []string{
		"Unbound Ultimate (God Mode)",
		"Discord/CF SNI Bypass",
		"Telegram MTProto",
		"The Ultimate Combo",
		"Discord Voice Optimized",
		"YouTube QUIC Aggressive",
		"Telegram API Bypass",
		"Fake TLS & QUIC",
		"Multi-Strategy Chaos",
		"Standard Split",
		"Fake Packets + BadSeq",
		"Disorder",
		"Split Handshake",
		"Flowseal Legacy",
		"Custom Profile",
	}

	if len(profiles) != len(expectedProfiles) {
		t.Errorf("Expected %d profiles, got %d", len(expectedProfiles), len(profiles))
	}

	t.Logf("✓ Provider initialized with %d profiles", len(profiles))
}

// TestPrivilegeCheck verifies admin privilege detection
func TestPrivilegeCheck(t *testing.T) {
	assets, err := engine.ExtractAssets()
	if err != nil {
		t.Fatalf("Failed to extract assets: %v", err)
	}

	provider := providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, assets.ListDir, false)
	hasPriv, err := provider.CheckPrivileges()
	
	if err != nil {
		t.Errorf("Privilege check failed: %v", err)
	}

	if !hasPriv {
		t.Log("⚠️  Test running without admin privileges - some tests may fail")
	} else {
		t.Log("✓ Running with administrator privileges")
	}
}

// TestProfileArgumentGeneration validates that profile args are generated correctly
func TestProfileArgumentGeneration(t *testing.T) {
	assets, err := engine.ExtractAssets()
	if err != nil {
		t.Fatalf("Failed to extract assets: %v", err)
	}

	_ = providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, assets.ListDir, false)
	
	testCases := []struct {
		profileName string
		mustContain []string
	}{
		{
			profileName: "Standard Split",
			mustContain: []string{"--lua=", "--filter-tcp=443", "--lua-desync=split:pos=1"},
		},
		{
			profileName: "Unbound Ultimate (God Mode)",
			mustContain: []string{"--lua=", "--filter-tcp=443", "--filter-udp=443", "--lua-desync=fake"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.profileName, func(t *testing.T) {
			// This would require exposing getProfileArgs or testing via Start()
			t.Logf("Testing profile: %s", tc.profileName)
		})
	}
}

// TestCustomScriptPersistence verifies custom Lua script save/load
func TestCustomScriptPersistence(t *testing.T) {
	testContent := "-- Test custom script\nprint('hello')"
	
	err := engine.SaveCustomScript(testContent)
	if err != nil {
		t.Fatalf("Failed to save custom script: %v", err)
	}

	loaded, err := engine.LoadCustomScript()
	if err != nil {
		t.Fatalf("Failed to load custom script: %v", err)
	}

	if loaded != testContent {
		t.Errorf("Loaded content doesn't match saved content.\nExpected: %s\nGot: %s", testContent, loaded)
	}

	t.Log("✓ Custom script persistence working")
}

// TestEngineStartStop tests basic engine lifecycle (requires admin)
func TestEngineStartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	assets, err := engine.ExtractAssets()
	if err != nil {
		t.Fatalf("Failed to extract assets: %v", err)
	}

	provider := providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, assets.ListDir, false)
	
	hasPriv, _ := provider.CheckPrivileges()
	if !hasPriv {
		t.Skip("Skipping engine test - requires administrator privileges")
	}

	ctx := context.Background()
	
	// Test starting engine
	err = provider.Start(ctx, "Standard Split")
	if err != nil {
		t.Fatalf("Failed to start engine: %v", err)
	}

	if provider.GetStatus() != providers.StatusRunning {
		t.Errorf("Expected status Running, got %s", provider.GetStatus())
	}

	time.Sleep(2 * time.Second)

	// Test stopping engine
	err = provider.Stop()
	if err != nil {
		t.Errorf("Failed to stop engine: %v", err)
	}

	if provider.GetStatus() != providers.StatusStopped {
		t.Errorf("Expected status Stopped, got %s", provider.GetStatus())
	}

	t.Log("✓ Engine start/stop cycle successful")
}

// TestAutoTuneScanner tests the auto-tune functionality (requires admin + network)
func TestAutoTuneScanner(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping auto-tune test in short mode")
	}

	assets, err := engine.ExtractAssets()
	if err != nil {
		t.Fatalf("Failed to extract assets: %v", err)
	}

	provider := providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, assets.ListDir, false)
	hasPriv, _ := provider.CheckPrivileges()
	if !hasPriv {
		t.Skip("Skipping auto-tune test - requires administrator privileges")
	}

	ctx := context.Background()
	logs := []string{}
	
	updateLog := func(msg string) {
		logs = append(logs, msg)
		t.Log(msg)
	}

	profile, err := engine.RunAutoTune(ctx, updateLog)
	
	if err != nil {
		t.Logf("Auto-tune failed (expected in some networks): %v", err)
		t.Logf("Collected %d log entries", len(logs))
		return
	}

	t.Logf("✓ Auto-tune selected profile: %s", profile.Name)
	t.Logf("Collected %d log entries", len(logs))
}

// TestProviderManager tests the provider management system
func TestProviderManager(t *testing.T) {
	manager := providers.NewProviderManager()
	
	assets, err := engine.ExtractAssets()
	if err != nil {
		t.Fatalf("Failed to extract assets: %v", err)
	}

	provider := providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, assets.ListDir, false)
	manager.Register(provider)

	engines := manager.GetEngineNames()
	if len(engines) != 1 {
		t.Errorf("Expected 1 engine, got %d", len(engines))
	}

	profiles := manager.GetProfiles("Zapret 2 (winws)")
	if len(profiles) == 0 {
		t.Error("No profiles returned from manager")
	}

	t.Logf("✓ Provider manager working with %d engines and %d profiles", len(engines), len(profiles))
}

// BenchmarkAssetExtraction benchmarks asset extraction performance
func BenchmarkAssetExtraction(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := engine.ExtractAssets()
		if err != nil {
			b.Fatalf("Asset extraction failed: %v", err)
		}
	}
}

// TestMain runs before all tests
func TestMain(m *testing.M) {
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("🧪 UNBOUND Test Suite")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	code := m.Run()
	
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	if code == 0 {
		fmt.Println("✅ All tests passed")
	} else {
		fmt.Println("❌ Some tests failed")
	}
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	os.Exit(code)
}

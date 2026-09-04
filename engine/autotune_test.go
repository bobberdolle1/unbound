//go:build windows

package engine

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"
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

func TestProfileAggressivenessRanking(t *testing.T) {
	rec := ProfileAggressiveness("Recommended (hostfakesplit)")
	alt1 := ProfileAggressiveness("Alternative 1 (multisplit)")
	alt2 := ProfileAggressiveness("Alternative 2 (fake TLS)")
	alt3 := ProfileAggressiveness("Alternative 3 (multisplit SNI)")
	univ := ProfileAggressiveness("Universal 2026 (All-in-One)")
	alt4 := ProfileAggressiveness("Alternative 4 (fake TLS multisplit)")

	if !(rec < alt1 && alt1 < alt2 && alt2 < alt3 && alt3 < univ && univ < alt4) {
		t.Errorf("Aggressiveness order violation: rec=%d alt1=%d alt2=%d alt3=%d univ=%d alt4=%d",
			rec, alt1, alt2, alt3, univ, alt4)
	}
}

func TestBetterAutoTuneResultPrefersLessAggressiveOnTie(t *testing.T) {
	// Two profiles with identical score and latency
	lessAggressive := &AutoTuneResult{
		ProfileName:    "Recommended (hostfakesplit)",
		Score:          100,
		Latency:        50 * time.Millisecond,
		Aggressiveness: ProfileAggressiveness("Recommended (hostfakesplit)"),
	}

	moreAggressive := &AutoTuneResult{
		ProfileName:    "Alternative 4 (fake TLS multisplit)",
		Score:          100,
		Latency:        50 * time.Millisecond,
		Aggressiveness: ProfileAggressiveness("Alternative 4 (fake TLS multisplit)"),
	}

	if !betterAutoTuneResult(lessAggressive, moreAggressive) {
		t.Error("Expected less aggressive profile to win on score/latency tie")
	}
	if betterAutoTuneResult(moreAggressive, lessAggressive) {
		t.Error("More aggressive profile should not beat less aggressive on tie")
	}
}

func TestFilterTargets(t *testing.T) {
	// Empty filter -> returns all targets
	all := FilterTargets(nil)
	if len(all) != len(testTargets) {
		t.Errorf("Expected %d targets, got %d", len(testTargets), len(all))
	}

	// Filter YouTube only
	yt := FilterTargets([]string{"youtube"})
	if len(yt) != 1 || yt[0].Name != "YouTube" {
		t.Errorf("Expected only YouTube target, got %+v", yt)
	}

	// Filter YouTube + Discord + Steam
	three := FilterTargets([]string{"youtube", "discord", "steam"})
	if len(three) != 3 {
		t.Errorf("Expected 3 targets, got %d", len(three))
	}
}

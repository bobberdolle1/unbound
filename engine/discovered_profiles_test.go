package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoveredProfilesPersistence(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "discovered_profiles.json")
	cleanup := SetDiscoveredProfilesPathForTest(tempFile)
	defer cleanup()

	// 1. Initial load should be empty
	profs, err := LoadDiscoveredProfiles()
	if err != nil {
		t.Fatalf("LoadDiscoveredProfiles failed on empty: %v", err)
	}
	if len(profs) != 0 {
		t.Fatalf("Expected 0 profiles, got %d", len(profs))
	}

	// 2. Save a discovered profile
	cand1 := StrategyCandidate{
		ID:             "cand_1",
		Name:           "TLS Multisplit Midsld",
		Protocol:       "TLS1.3",
		Source:         "blockcheck2",
		Aggressiveness: AggressivenessLow,
		Zapret2Args:    []string{"--filter-tcp=443", "--lua-desync=multisplit:pos=midsld"},
	}

	err = SaveDiscoveredProfile("YouTube Custom Bypass", cand1, "youtube.com")
	if err != nil {
		t.Fatalf("SaveDiscoveredProfile failed: %v", err)
	}

	if _, err := os.Stat(tempFile); err != nil {
		t.Fatalf("discovered_profiles.json does not exist: %v", err)
	}
	profs, err = LoadDiscoveredProfiles()
	if err != nil {
		t.Fatalf("LoadDiscoveredProfiles failed: %v", err)
	}
	if len(profs) != 1 {
		t.Fatalf("Expected 1 profile, got %d", len(profs))
	}
	p := profs[0]
	if p.Name != "YouTube Custom Bypass" {
		t.Errorf("Name = %s; want YouTube Custom Bypass", p.Name)
	}
	if p.Target != "youtube.com" {
		t.Errorf("Target = %s; want youtube.com", p.Target)
	}
	if p.Protocol != "TLS1.3" {
		t.Errorf("Protocol = %s; want TLS1.3", p.Protocol)
	}
	if len(p.Args) != 2 || p.Args[1] != "--lua-desync=multisplit:pos=midsld" {
		t.Errorf("Args = %v; want 2 args", p.Args)
	}

	// 4. Save second profile
	cand2 := StrategyCandidate{
		ID:             "cand_2",
		Name:           "Discord Fake TLS",
		Protocol:       "TLS1.2",
		Source:         "user custom",
		Aggressiveness: AggressivenessMedium,
		Zapret2Args:    []string{"--filter-tcp=443", "--lua-desync=fake:repeats=2"},
	}
	err = SaveDiscoveredProfile("Discord Voice Bypass", cand2, "discord.com")
	if err != nil {
		t.Fatalf("SaveDiscoveredProfile cand2 failed: %v", err)
	}

	profs, err = LoadDiscoveredProfiles()
	if err != nil {
		t.Fatalf("LoadDiscoveredProfiles failed: %v", err)
	}
	if len(profs) != 2 {
		t.Fatalf("Expected 2 profiles, got %d", len(profs))
	}

	// 5. Update cand1 by same name
	cand1Updated := cand1
	cand1Updated.Zapret2Args = []string{"--filter-tcp=443", "--lua-desync=multisplit:pos=midsld:repeats=2"}
	err = SaveDiscoveredProfile("YouTube Custom Bypass", cand1Updated, "youtube.com")
	if err != nil {
		t.Fatalf("SaveDiscoveredProfile update failed: %v", err)
	}

	profs, err = LoadDiscoveredProfiles()
	if err != nil {
		t.Fatalf("LoadDiscoveredProfiles failed: %v", err)
	}
	if len(profs) != 2 {
		t.Fatalf("Expected still 2 profiles after update, got %d", len(profs))
	}
	for _, prof := range profs {
		if prof.Name == "YouTube Custom Bypass" {
			if prof.Args[1] != "--lua-desync=multisplit:pos=midsld:repeats=2" {
				t.Errorf("Profile was not updated: %v", prof.Args)
			}
		}
	}
}

package engine

import (
	"testing"
)

func TestGetProfiles(t *testing.T) {
	profiles := GetProfiles("")
	
	if len(profiles) == 0 {
		t.Fatal("Expected profiles, got none")
	}
	
	t.Logf("Standard profiles count: %d", len(profiles))
	
	for _, p := range profiles {
		if p.Name == "" {
			t.Error("Profile has empty name")
		}
		if len(p.Args) == 0 {
			t.Errorf("Profile '%s' has no args", p.Name)
		}
	}
}

func TestGetAdvancedProfiles(t *testing.T) {
	profiles := GetAdvancedProfiles("")
	
	if len(profiles) == 0 {
		t.Fatal("Expected advanced profiles, got none")
	}
	
	expectedCount := 12
	if len(profiles) != expectedCount {
		t.Errorf("Expected %d advanced profiles, got %d", expectedCount, len(profiles))
	}
	
	t.Logf("Advanced profiles count: %d", len(profiles))
	
	for _, p := range profiles {
		if p.Name == "" {
			t.Error("Profile has empty name")
		}
		if p.Description == "" {
			t.Errorf("Profile '%s' has empty description", p.Name)
		}
		if p.Category == "" {
			t.Errorf("Profile '%s' has empty category", p.Name)
		}
		if len(p.Args) == 0 {
			t.Errorf("Profile '%s' has no args", p.Name)
		}
		if len(p.Techniques) == 0 {
			t.Errorf("Profile '%s' has no techniques", p.Name)
		}
		
		t.Logf("Profile: %s [%s] - %d techniques", p.Name, p.Category, len(p.Techniques))
	}
}

func TestGetProfileCategories(t *testing.T) {
	categories := GetProfileCategories()
	
	expectedCount := 11
	if len(categories) != expectedCount {
		t.Errorf("Expected %d categories, got %d", expectedCount, len(categories))
	}
	
	expectedCategories := map[string]bool{
		"universal":    true,
		"aggressive":   true,
		"smart":        true,
		"experimental": true,
		"stealth":      true,
		"deep":         true,
		"chaos":        true,
		"handshake":    true,
		"quic":         true,
		"http":         true,
		"stateful":     true,
	}
	
	for _, cat := range categories {
		if !expectedCategories[cat] {
			t.Errorf("Unexpected category: %s", cat)
		}
	}
}

func TestGetProfilesByCategory(t *testing.T) {
	testCases := []struct {
		category     string
		expectedMin  int
	}{
		{"universal", 1},
		{"aggressive", 1},
		{"smart", 1},
		{"experimental", 1},
		{"stealth", 1},
	}
	
	for _, tc := range testCases {
		profiles := GetProfilesByCategory(tc.category)
		
		if len(profiles) < tc.expectedMin {
			t.Errorf("Category '%s': expected at least %d profiles, got %d", 
				tc.category, tc.expectedMin, len(profiles))
		}
		
		for _, p := range profiles {
			if p.Category != tc.category {
				t.Errorf("Profile '%s' has wrong category: expected '%s', got '%s'", 
					p.Name, tc.category, p.Category)
			}
		}
		
		t.Logf("Category '%s': %d profiles", tc.category, len(profiles))
	}
}

func TestAdvancedProfileTechniques(t *testing.T) {
	profiles := GetAdvancedProfiles("")
	
	expectedTechniques := map[string][]string{
		"Aggressive Fake + BadSeq": {"fake", "tcp_md5", "badseq", "multisplit"},
		"AutoTTL + Fake":           {"autottl", "fake", "multisplit"},
		"BadSum + Disorder":        {"badsum", "multidisorder"},
		"SNI Randomization":        {"fake", "sni_random", "multisplit"},
		"QUIC Aggressive":          {"fake", "multisplit", "udp_length"},
	}
	
	for _, p := range profiles {
		if expected, ok := expectedTechniques[p.Name]; ok {
			if len(p.Techniques) != len(expected) {
				t.Errorf("Profile '%s': expected %d techniques, got %d", 
					p.Name, len(expected), len(p.Techniques))
			}
			
			for i, tech := range expected {
				if i >= len(p.Techniques) || p.Techniques[i] != tech {
					t.Errorf("Profile '%s': technique mismatch at index %d", p.Name, i)
				}
			}
		}
	}
}

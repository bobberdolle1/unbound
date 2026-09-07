package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DiscoveredProfile represents a concrete working strategy discovered in Strategy Lab.
type DiscoveredProfile struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Args           []string               `json:"args"`
	Target         string                 `json:"target"`
	Protocol       string                 `json:"protocol"`
	Source         string                 `json:"source"`
	Aggressiveness StrategyAggressiveness `json:"aggressiveness"`
	CreatedAt      time.Time              `json:"createdAt"`
}

var (
	discoveredProfilesMu sync.RWMutex

	pathOverrideMu                 sync.RWMutex
	discoveredProfilesPathOverride string
)

const DiscoveredProfilesFileName = "discovered_profiles.json"

// SetDiscoveredProfilesPathForTest overrides the JSON storage path for unit tests.
func SetDiscoveredProfilesPathForTest(path string) func() {
	pathOverrideMu.Lock()
	old := discoveredProfilesPathOverride
	discoveredProfilesPathOverride = path
	pathOverrideMu.Unlock()
	return func() {
		pathOverrideMu.Lock()
		discoveredProfilesPathOverride = old
		pathOverrideMu.Unlock()
	}
}

// GetDiscoveredProfilesPath returns the canonical path to discovered_profiles.json.
func GetDiscoveredProfilesPath() (string, error) {
	pathOverrideMu.RLock()
	if discoveredProfilesPathOverride != "" {
		p := discoveredProfilesPathOverride
		pathOverrideMu.RUnlock()
		return p, nil
	}
	pathOverrideMu.RUnlock()

	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "discovered_profiles.json"), nil
}

// LoadDiscoveredProfiles reads and parses all saved discovered profiles from disk.
func LoadDiscoveredProfiles() ([]DiscoveredProfile, error) {
	discoveredProfilesMu.RLock()
	defer discoveredProfilesMu.RUnlock()

	filePath, err := GetDiscoveredProfilesPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var profiles []DiscoveredProfile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return nil, fmt.Errorf("failed to decode %s: %w", DiscoveredProfilesFileName, err)
	}
	return profiles, nil
}

// SaveDiscoveredProfile stores a discovered candidate strategy into discovered_profiles.json.
// It creates a unique ID, timestamps the entry, and performs an atomic write.
func SaveDiscoveredProfile(name string, candidate StrategyCandidate, targetHost string) error {
	discoveredProfilesMu.Lock()
	defer discoveredProfilesMu.Unlock()

	filePath, err := GetDiscoveredProfilesPath()
	if err != nil {
		return err
	}

	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		cleanName = fmt.Sprintf("Discovered - %s", candidate.Name)
	}

	var profiles []DiscoveredProfile
	data, err := os.ReadFile(filePath)
	if err == nil {
		_ = json.Unmarshal(data, &profiles)
	}

	// Update existing by name, or append new
	id := fmt.Sprintf("disc_%d", time.Now().UnixNano())
	newProf := DiscoveredProfile{
		ID:             id,
		Name:           cleanName,
		Args:           append([]string(nil), candidate.Zapret2Args...),
		Target:         targetHost,
		Protocol:       candidate.Protocol,
		Source:         candidate.Source,
		Aggressiveness: candidate.Aggressiveness,
		CreatedAt:      time.Now(),
	}

	found := false
	for i, p := range profiles {
		if strings.EqualFold(p.Name, cleanName) {
			profiles[i] = newProf
			found = true
			break
		}
	}
	if !found {
		profiles = append(profiles, newProf)
	}

	tmpFile := filePath + ".tmp"
	marshaled, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(tmpFile, marshaled, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmpFile, filePath); err != nil {
		return err
	}

	GetLogger().Infof("Lab", "[LAB] saved discovered profile %q with %d args into %s", cleanName, len(candidate.Zapret2Args), filePath)
	return nil
}

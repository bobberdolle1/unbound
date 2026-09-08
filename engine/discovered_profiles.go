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
	Target         string                 `json:"target"`
	Protocol       string                 `json:"protocol"`
	CandidateArgs  []string               `json:"candidateArgs"`
	Args           []string               `json:"args"`
	Source         string                 `json:"source"`
	Aggressiveness StrategyAggressiveness `json:"aggressiveness"`
	CreatedAt      time.Time              `json:"createdAt"`
	Version        int                    `json:"version"` // 1 = legacy v0.6.1 (unscoped), 2 = v0.6.2 (scoped)
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

// BuildDiscoveredProfileRuntimeArgs compiles a DiscoveredProfile into effective runtime winws2 arguments.
// It scopes the candidate desync actions to the target domain, ensures transport filters are present,
// and applies Steam safety exclusions.
func BuildDiscoveredProfileRuntimeArgs(dp DiscoveredProfile, listsDir string) []string {
	srcArgs := dp.CandidateArgs
	if len(srcArgs) == 0 {
		srcArgs = dp.Args
	}

	cleanTarget := strings.TrimSpace(dp.Target)
	if h := extractHost(cleanTarget); h != "" {
		cleanTarget = h
	}
	// Split by --new into individual profile sections
	var sections [][]string
	currentSection := make([]string, 0, len(srcArgs))
	for _, a := range srcArgs {
		if strings.HasPrefix(a, "--new") {
			if len(currentSection) > 0 {
				sections = append(sections, currentSection)
				currentSection = make([]string, 0, 4)
			}
		}
		currentSection = append(currentSection, a)
	}
	if len(currentSection) > 0 {
		sections = append(sections, currentSection)
	}

	var compiled []string
	protoUpper := strings.ToUpper(dp.Protocol)

	for i, sec := range sections {
		if i > 0 {
			compiled = append(compiled, sec[0])
			sec = sec[1:]
		}

		hasTransport := false
		hasHostScope := false
		for _, a := range sec {
			if strings.HasPrefix(a, "--filter-tcp=") || strings.HasPrefix(a, "--filter-udp=") {
				hasTransport = true
			}
			if strings.HasPrefix(a, "--hostlist") {
				hasHostScope = true
			}
		}

		if !hasTransport {
			if protoUpper == "HTTP" {
				compiled = append(compiled, "--filter-tcp=80")
			} else if protoUpper == "QUIC" {
				compiled = append(compiled, "--filter-udp=443")
			} else {
				compiled = append(compiled, "--filter-tcp=443")
			}
		}

		if !hasHostScope && cleanTarget != "" {
			compiled = append(compiled, "--hostlist-domains="+cleanTarget)
		}

		compiled = append(compiled, sec...)
	}

	return steamSafeArgs(compiled, listsDir)
}

// LoadDiscoveredProfiles reads and parses all saved discovered profiles from disk.
// Transparently migrates v0.6.1 legacy unscoped profiles to v0.6.2 scoped profiles.
func LoadDiscoveredProfiles() ([]DiscoveredProfile, error) {
	discoveredProfilesMu.RLock()
	filePath, err := GetDiscoveredProfilesPath()
	if err != nil {
		discoveredProfilesMu.RUnlock()
		return nil, err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		discoveredProfilesMu.RUnlock()
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var profiles []DiscoveredProfile
	if err := json.Unmarshal(data, &profiles); err != nil {
		discoveredProfilesMu.RUnlock()
		return nil, fmt.Errorf("failed to decode %s: %w", DiscoveredProfilesFileName, err)
	}
	discoveredProfilesMu.RUnlock()

	// Schema migration for v0.6.1 profiles
	migrated := false
	listsDir, _ := GetListsDir()
	for i := range profiles {
		if profiles[i].Version < 2 {
			if len(profiles[i].CandidateArgs) == 0 {
				profiles[i].CandidateArgs = append([]string(nil), profiles[i].Args...)
			}
			profiles[i].Args = BuildDiscoveredProfileRuntimeArgs(profiles[i], listsDir)
			profiles[i].Version = 2
			migrated = true
		}
	}

	if migrated {
		discoveredProfilesMu.Lock()
		if migData, err := json.MarshalIndent(profiles, "", "  "); err == nil {
			tmpPath := filePath + ".tmp"
			if wErr := os.WriteFile(tmpPath, migData, 0644); wErr == nil {
				_ = os.Rename(tmpPath, filePath)
			}
		}
		discoveredProfilesMu.Unlock()
	}

	return profiles, nil
}

// SaveDiscoveredProfile stores a discovered candidate strategy into discovered_profiles.json.
// It creates a unique ID, timestamps the entry, and performs an atomic write with domain scoping.
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

	listsDir, _ := GetListsDir()
	id := fmt.Sprintf("disc_%d", time.Now().UnixNano())
	effectiveProto := candidate.TestedProtocol
	if effectiveProto == "" {
		effectiveProto = candidate.Protocol
	}
	if effectiveProto == "" {
		effectiveProto = "TLS1.3"
	}

	newProf := DiscoveredProfile{
		ID:             id,
		Name:           cleanName,
		Target:         targetHost,
		Protocol:       effectiveProto,
		CandidateArgs:  append([]string(nil), candidate.Zapret2Args...),
		Source:         candidate.Source,
		Aggressiveness: candidate.Aggressiveness,
		CreatedAt:      time.Now(),
		Version:        2,
	}
	newProf.Args = BuildDiscoveredProfileRuntimeArgs(newProf, listsDir)

	// Update existing by name, or append new
	found := false
	for i, p := range profiles {
		if strings.EqualFold(p.Name, cleanName) {
			newProf.ID = p.ID
			profiles[i] = newProf
			found = true
			break
		}
	}
	if !found {
		profiles = append(profiles, newProf)
	}

	outBytes, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode profiles: %w", err)
	}

	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, outBytes, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", tmpPath, err)
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to commit %s: %w", filePath, err)
	}

	GetLogger().Infof("Lab", "[LAB] saved discovered profile %q with %d runtime args (target=%s) into %s",
		cleanName, len(newProf.Args), targetHost, filePath)
	return nil
}

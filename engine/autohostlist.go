package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// AutoHostlistEntry stores rich metadata for dynamically detected blocked domains.
type AutoHostlistEntry struct {
	Domain        string    `json:"domain"`
	FirstDetected time.Time `json:"firstDetected"`
	LastDetected  time.Time `json:"lastDetected"`
	Reason        string    `json:"reason"`
	HitCount      int       `json:"hitCount"`
	Promoted      bool      `json:"promoted"`
}

// AutoHostlistManager orchestrates thread-safe reading, writing, synchronization, and promotion of auto-discovered hosts.
type AutoHostlistManager struct {
	mu       sync.RWMutex
	listsDir string
	metaPath string
	listPath string
	entries  map[string]*AutoHostlistEntry
}

var (
	globalAutoHostlist     *AutoHostlistManager
	globalAutoHostlistOnce sync.Once
)

// GetAutoHostlistManager returns the singleton AutoHostlistManager.
func GetAutoHostlistManager() *AutoHostlistManager {
	globalAutoHostlistOnce.Do(func() {
		listsDir, _ := GetListsDir()
		globalAutoHostlist = NewAutoHostlistManager(listsDir)
	})
	return globalAutoHostlist
}

// NewAutoHostlistManager initializes manager instance with paths.
func NewAutoHostlistManager(listsDir string) *AutoHostlistManager {
	mgr := &AutoHostlistManager{
		listsDir: listsDir,
		listPath: filepath.Join(listsDir, "autodetect.txt"),
		metaPath: filepath.Join(listsDir, "autodetect_meta.json"),
		entries:  make(map[string]*AutoHostlistEntry),
	}
	_ = mgr.load()
	return mgr
}

var protectedExclusions = []string{
	"localhost",
	"127.0.0.1",
	"::1",
	"local",
	"internal",
	"github.com",
	"api.github.com",
	"raw.githubusercontent.com",
	"steampowered.com",
	"steamcommunity.com",
	"steamstatic.com",
	"steamcontent.com",
	"s.team",
}

// IsExcludedDomain verifies if a domain must be excluded from auto-detection.
func IsExcludedDomain(domain string) bool {
	clean := strings.ToLower(strings.TrimSpace(domain))
	if clean == "" {
		return true
	}

	for _, p := range protectedExclusions {
		if clean == p || strings.HasSuffix(clean, "."+p) {
			return true
		}
	}

	// Reject raw LAN IPs
	if strings.HasPrefix(clean, "192.168.") || strings.HasPrefix(clean, "10.") || strings.HasPrefix(clean, "172.16.") {
		return true
	}

	return false
}

// GetEntries returns all detected domains sorted by LastDetected descending.
func (m *AutoHostlistManager) GetEntries() []AutoHostlistEntry {
	m.SyncFromDisk()

	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]AutoHostlistEntry, 0, len(m.entries))
	for _, e := range m.entries {
		list = append(list, *e)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].LastDetected.After(list[j].LastDetected)
	})
	return list
}

// AddDomain records a domain into the dynamic hostlist and metadata table.
func (m *AutoHostlistManager) AddDomain(domain, reason string) error {
	clean := strings.ToLower(strings.TrimSpace(domain))
	if clean == "" {
		return errors.New("domain cannot be empty")
	}
	if IsExcludedDomain(clean) {
		GetLogger().Warnf("AutoHostlist", "[AUTOHOSTLIST] rejected excluded domain from dynamic list: %s", clean)
		return fmt.Errorf("domain %s is in the protected exclusion list", clean)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	entry, exists := m.entries[clean]
	if !exists {
		entry = &AutoHostlistEntry{
			Domain:        clean,
			FirstDetected: now,
			LastDetected:  now,
			Reason:        reason,
			HitCount:      1,
		}
		m.entries[clean] = entry
	} else {
		entry.LastDetected = now
		entry.HitCount++
		if reason != "" {
			entry.Reason = reason
		}
	}

	GetLogger().Infof("AutoHostlist", "[AUTOHOSTLIST] added/updated domain: %s (reason=%s, hits=%d)", clean, reason, entry.HitCount)
	return m.saveAtomicLocked()
}

// RemoveDomain deletes a domain from the dynamic list.
func (m *AutoHostlistManager) RemoveDomain(domain string) error {
	clean := strings.ToLower(strings.TrimSpace(domain))

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.entries[clean]; !exists {
		return nil
	}

	delete(m.entries, clean)
	GetLogger().Infof("AutoHostlist", "[AUTOHOSTLIST] removed domain: %s", clean)
	return m.saveAtomicLocked()
}

// ClearDynamicList empties the autodetect list completely.
func (m *AutoHostlistManager) ClearDynamicList() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.entries = make(map[string]*AutoHostlistEntry)
	GetLogger().Info("AutoHostlist", "[AUTOHOSTLIST] cleared dynamic autodetect list")
	return m.saveAtomicLocked()
}

// PromoteDomain moves a domain from autodetect.txt to a permanent curated list (e.g. other.txt or youtube.txt).
func (m *AutoHostlistManager) PromoteDomain(domain, targetListFilename string) error {
	clean := strings.ToLower(strings.TrimSpace(domain))
	if clean == "" {
		return errors.New("domain cannot be empty")
	}

	targetFile := filepath.Clean(filepath.Join(m.listsDir, targetListFilename))
	if filepath.Dir(targetFile) != filepath.Clean(m.listsDir) {
		return errors.New("invalid target list filename")
	}

	// 1. Read existing permanent list
	var lines []string
	if data, err := os.ReadFile(targetFile); err == nil {
		lines = strings.Split(string(data), "\n")
	}

	alreadyPresent := false
	for _, l := range lines {
		if strings.EqualFold(strings.TrimSpace(l), clean) {
			alreadyPresent = true
			break
		}
	}

	if !alreadyPresent {
		lines = append(lines, clean)
		var outLines []string
		for _, l := range lines {
			if trimmed := strings.TrimSpace(l); trimmed != "" {
				outLines = append(outLines, trimmed)
			}
		}
		if err := os.WriteFile(targetFile, []byte(strings.Join(outLines, "\n")+"\n"), 0644); err != nil {
			return fmt.Errorf("failed to append domain to %s: %w", targetListFilename, err)
		}
	}

	// 2. Mark promoted in metadata and remove from autodetect.txt
	m.mu.Lock()
	if entry, exists := m.entries[clean]; exists {
		entry.Promoted = true
	}
	delete(m.entries, clean)
	_ = m.saveAtomicLocked()
	m.mu.Unlock()

	GetLogger().Infof("AutoHostlist", "[AUTOHOSTLIST] promoted domain %s to %s", clean, targetListFilename)
	return nil
}

// SyncFromDisk synchronizes entries with autodetect.txt written directly by winws2.
func (m *AutoHostlistManager) SyncFromDisk() {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.listPath)
	if err != nil {
		return
	}

	now := time.Now()
	diskDomains := make(map[string]bool)
	for _, line := range strings.Split(string(data), "\n") {
		clean := strings.ToLower(strings.TrimSpace(line))
		if clean == "" || strings.HasPrefix(clean, "#") {
			continue
		}
		diskDomains[clean] = true
		if _, exists := m.entries[clean]; !exists {
			m.entries[clean] = &AutoHostlistEntry{
				Domain:        clean,
				FirstDetected: now,
				LastDetected:  now,
				Reason:        "Winws2 runtime auto-detection",
				HitCount:      1,
			}
		}
	}

	// Save metadata if new domains appeared
	_ = m.saveMetaLocked()
}

func (m *AutoHostlistManager) load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.metaPath)
	if err == nil {
		_ = json.Unmarshal(data, &m.entries)
	}
	return nil
}

func (m *AutoHostlistManager) saveAtomicLocked() error {
	// Write autodetect.txt
	var domains []string
	for d := range m.entries {
		domains = append(domains, d)
	}
	sort.Strings(domains)

	tmpList := m.listPath + ".tmp"
	listContent := strings.Join(domains, "\n")
	if len(domains) > 0 {
		listContent += "\n"
	}
	if err := os.WriteFile(tmpList, []byte(listContent), 0644); err != nil {
		return err
	}
	_ = os.Rename(tmpList, m.listPath)

	return m.saveMetaLocked()
}

func (m *AutoHostlistManager) saveMetaLocked() error {
	tmpMeta := m.metaPath + ".tmp"
	metaBytes, err := json.MarshalIndent(m.entries, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(tmpMeta, metaBytes, 0644); err != nil {
		return err
	}
	return os.Rename(tmpMeta, m.metaPath)
}

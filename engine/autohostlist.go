package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// AutoHostlistEntry represents an automatically detected domain with hit statistics.
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
	mu                sync.RWMutex
	listsDir          string
	metaPath          string
	listPath          string
	entries           map[string]*AutoHostlistEntry
	tombstones        map[string]bool
	lastDiskMtime     time.Time
	lastDiskSize      int64
	beforeReplaceHook func()
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

// GetAutoHostlistPath returns the single canonical path to autodetect.txt in lists directory.
// Migrates any legacy autodetect.txt from configDir to listsDir if old file exists.
func GetAutoHostlistPath() (string, error) {
	listsDir, err := GetListsDir()
	if err != nil {
		return "", err
	}
	canonicalPath := filepath.Join(listsDir, "autodetect.txt")

	// Check legacy location in configDir and migrate if needed
	if configDir, err := GetConfigDir(); err == nil && configDir != "" {
		legacyPath := filepath.Join(configDir, "autodetect.txt")
		if _, err := os.Stat(canonicalPath); os.IsNotExist(err) {
			if _, lErr := os.Stat(legacyPath); lErr == nil {
				if data, rErr := os.ReadFile(legacyPath); rErr == nil {
					_ = os.WriteFile(canonicalPath, data, 0644)
					_ = os.Remove(legacyPath)
					GetLogger().Infof("AutoHostlist", "[AUTOHOSTLIST] migrated legacy autodetect.txt from %s to %s", legacyPath, canonicalPath)
				}
			}
		}
	}

	return canonicalPath, nil
}

// NewAutoHostlistManager initializes manager instance with canonical paths.
func NewAutoHostlistManager(listsDir string) *AutoHostlistManager {
	listPath := filepath.Join(listsDir, "autodetect.txt")
	mgr := &AutoHostlistManager{
		listsDir:   listsDir,
		listPath:   listPath,
		metaPath:   filepath.Join(listsDir, "autodetect_meta.json"),
		entries:    make(map[string]*AutoHostlistEntry),
		tombstones: make(map[string]bool),
	}
	_ = mgr.load()
	return mgr
}

var protectedExclusions = []string{
	"localhost",
	"127.0.0.1",
	"::1",
	"github.com",
	"api.github.com",
	"raw.githubusercontent.com",
	"objects.githubusercontent.com",
	"store.steampowered.com",
	"steamcommunity.com",
	"api.steampowered.com",
	"steamstatic.com",
	"steamcontent.com",
}

// IsExcludedDomain verifies if domain belongs to essential network infrastructure,
// local networks, Steam/Valve gaming networks, or Unbound GitHub update endpoints.
func IsExcludedDomain(domain string) bool {
	clean := strings.ToLower(strings.TrimSpace(domain))
	if clean == "" {
		return true
	}

	// 1. Static protected list
	for _, excl := range protectedExclusions {
		if clean == excl || strings.HasSuffix(clean, "."+excl) {
			return true
		}
	}

	// 2. IP / Localhost checks
	if clean == "localhost" {
		return true
	}
	if ip := parseIPOnly(clean); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			return true
		}
	}

	return false
}

func parseIPOnly(s string) net.IP {
	if idx := strings.Index(s, ":"); idx != -1 {
		s = s[:idx]
	}
	return net.ParseIP(s)
}

// GetEntries returns a snapshot of detected domains.
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
	m.syncFromDiskLocked()
	delete(m.tombstones, clean)

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

		if m.beforeReplaceHook != nil {
			m.beforeReplaceHook()
		}

		// Append directly using O_APPEND: kernel-level atomic append, completely immune to TOCTOU overwrite race
		f, err := os.OpenFile(m.listPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open autodetect file for append: %w", err)
		}
		if _, err := f.WriteString(clean + "\n"); err != nil {
			_ = f.Close()
			return fmt.Errorf("failed to append to autodetect file: %w", err)
		}
		_ = f.Close()

		if fi, err := os.Stat(m.listPath); err == nil {
			m.lastDiskMtime = fi.ModTime()
			m.lastDiskSize = fi.Size()
		}
	} else {
		entry.LastDetected = now
		entry.HitCount++
		if reason != "" {
			entry.Reason = reason
		}
	}

	GetLogger().Infof("AutoHostlist", "[AUTOHOSTLIST] added/updated domain: %s (reason=%s, hits=%d)", clean, reason, entry.HitCount)
	return m.saveMetaLocked()
}

// RemoveDomain deletes a domain from the dynamic list.
func (m *AutoHostlistManager) RemoveDomain(domain string) error {
	clean := strings.ToLower(strings.TrimSpace(domain))

	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncFromDiskLocked()
	m.tombstones[clean] = true

	if _, exists := m.entries[clean]; !exists {
		return nil
	}

	delete(m.entries, clean)
	GetLogger().Infof("AutoHostlist", "[AUTOHOSTLIST] removed domain: %s", clean)
	return m.saveAtomicLocked()
}

// ClearDynamicList empties the autodetect list completely.
// First synchronizes from disk so any externally appended entries are tombstoned and wiped.
func (m *AutoHostlistManager) ClearDynamicList() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.syncFromDiskLocked()
	for d := range m.entries {
		m.tombstones[d] = true
	}
	m.entries = make(map[string]*AutoHostlistEntry)
	GetLogger().Info("AutoHostlist", "[AUTOHOSTLIST] cleared dynamic autodetect list")
	return m.saveAtomicLocked()
}

// SetBeforeReplaceHook registers a deterministic test hook called immediately prior to replacing autodetect.txt.
func (m *AutoHostlistManager) SetBeforeReplaceHook(hook func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.beforeReplaceHook = hook
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
	m.syncFromDiskLocked()
	m.tombstones[clean] = true
	if entry, exists := m.entries[clean]; exists {
		entry.Promoted = true
	}
	delete(m.entries, clean)
	err := m.saveAtomicLocked()
	m.mu.Unlock()

	GetLogger().Infof("AutoHostlist", "[AUTOHOSTLIST] promoted domain %s to %s", clean, targetListFilename)
	return err
}

// SyncFromDisk synchronizes entries with autodetect.txt written directly by winws2.
func (m *AutoHostlistManager) SyncFromDisk() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncFromDiskLocked()
	_ = m.saveMetaLocked()
}

func (m *AutoHostlistManager) syncFromDiskLocked() {
	fi, err := os.Stat(m.listPath)
	if err != nil {
		return
	}
	m.lastDiskMtime = fi.ModTime()
	m.lastDiskSize = fi.Size()

	data, err := os.ReadFile(m.listPath)
	if err != nil {
		return
	}

	now := time.Now()
	for _, line := range strings.Split(string(data), "\n") {
		clean := strings.ToLower(strings.TrimSpace(line))
		if clean == "" || strings.HasPrefix(clean, "#") || IsExcludedDomain(clean) {
			continue
		}
		if m.tombstones[clean] {
			continue
		}
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
	tmpList := m.listPath + ".tmp"

	var renameErr error
	for attempt := range 15 {
		var currentDiskSize int64
		if fi, err := os.Stat(m.listPath); err == nil {
			currentDiskSize = fi.Size()
		}

		// Re-read disk file before each attempt so any domain appended by winws2 is merged
		if diskBytes, err := os.ReadFile(m.listPath); err == nil {
			now := time.Now()
			for _, line := range strings.Split(string(diskBytes), "\n") {
				clean := strings.ToLower(strings.TrimSpace(line))
				if clean == "" || strings.HasPrefix(clean, "#") || IsExcludedDomain(clean) || m.tombstones[clean] {
					continue
				}
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
		}

		var domains []string
		for d := range m.entries {
			if !m.tombstones[d] {
				domains = append(domains, d)
			}
		}
		sort.Strings(domains)
		outLines := []string{"# Auto-detected domains by UNBOUND / winws2"}
		outLines = append(outLines, domains...)
		if m.beforeReplaceHook != nil {
			m.beforeReplaceHook()
		}

		// Atomic write tmp file
		if err := os.WriteFile(tmpList, []byte(strings.Join(outLines, "\n")+"\n"), 0644); err != nil {
			return fmt.Errorf("failed to write tmp auto hostlist: %w", err)
		}

		// Check if file size changed on disk since we read it
		if fi, err := os.Stat(m.listPath); err == nil && fi.Size() != currentDiskSize {
			// File changed! Someone (winws2) appended while we prepared the replacement.
			// Discard tmp file and retry OCC merge.
			_ = os.Remove(tmpList)
			continue
		}

		renameErr = os.Rename(tmpList, m.listPath)
		if renameErr == nil {
			break
		}
		time.Sleep(time.Duration(10*(1<<attempt)) * time.Millisecond)
	}

	_ = os.Remove(tmpList)
	if renameErr != nil {
		return renameErr
	}
	if fi, err := os.Stat(m.listPath); err == nil {
		m.lastDiskMtime = fi.ModTime()
		m.lastDiskSize = fi.Size()
	}
	m.tombstones = make(map[string]bool)

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

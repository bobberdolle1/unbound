package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsExcludedDomain(t *testing.T) {
	// Protected exclusions
	excluded := []string{
		"localhost",
		"127.0.0.1",
		"::1",
		"github.com",
		"api.github.com",
		"store.steampowered.com",
		"steamcommunity.com",
		"192.168.1.1",
		"10.0.0.5",
		"",
	}
	for _, d := range excluded {
		if !IsExcludedDomain(d) {
			t.Errorf("Expected domain to be excluded: %s", d)
		}
	}

	// Permitted domains
	permitted := []string{
		"example.com",
		"rutracker.org",
		"blocked-news.org",
		"video-portal.net",
	}
	for _, d := range permitted {
		if IsExcludedDomain(d) {
			t.Errorf("Expected domain to be permitted: %s", d)
		}
	}
}

func TestAutoHostlistManagerLifecycle(t *testing.T) {
	tempLists := t.TempDir()
	mgr := NewAutoHostlistManager(tempLists)

	// 1. Add domain
	err := mgr.AddDomain("example.com", "Repeated TCP RST")
	if err != nil {
		t.Fatalf("AddDomain failed: %v", err)
	}

	entries := mgr.GetEntries()
	if len(entries) != 1 || entries[0].Domain != "example.com" {
		t.Fatalf("Expected 1 entry 'example.com', got %+v", entries)
	}

	// 2. Verify file on disk
	listBytes, err := os.ReadFile(filepath.Join(tempLists, "autodetect.txt"))
	if err != nil {
		t.Fatalf("Failed to read autodetect.txt: %v", err)
	}
	if !strings.Contains(string(listBytes), "example.com") {
		t.Errorf("autodetect.txt does not contain example.com: %s", string(listBytes))
	}

	// 3. Add excluded domain -> rejected
	err = mgr.AddDomain("store.steampowered.com", "test")
	if err == nil {
		t.Error("Expected error when adding protected domain, got nil")
	}

	// 4. Promote domain to permanent list (other.txt)
	err = mgr.PromoteDomain("example.com", "other.txt")
	if err != nil {
		t.Fatalf("PromoteDomain failed: %v", err)
	}

	// Check permanent list has it
	otherBytes, err := os.ReadFile(filepath.Join(tempLists, "other.txt"))
	if err != nil {
		t.Fatalf("Failed to read other.txt: %v", err)
	}
	if !strings.Contains(string(otherBytes), "example.com") {
		t.Errorf("other.txt does not contain promoted domain: %s", string(otherBytes))
	}

	// Check autodetect.txt no longer has it
	listBytes, _ = os.ReadFile(filepath.Join(tempLists, "autodetect.txt"))
	if strings.Contains(string(listBytes), "example.com") {
		t.Errorf("autodetect.txt still contains promoted domain: %s", string(listBytes))
	}

	// 5. External sync from disk
	_ = os.WriteFile(filepath.Join(tempLists, "autodetect.txt"), []byte("external-detected.com\n"), 0644)
	mgr.SyncFromDisk()

	entries = mgr.GetEntries()
	foundExternal := false
	for _, e := range entries {
		if e.Domain == "external-detected.com" {
			foundExternal = true
			break
		}
	}
	if !foundExternal {
		t.Error("SyncFromDisk failed to discover externally written domain")
	}

	// 6. Clear dynamic list
	err = mgr.ClearDynamicList()
	if err != nil {
		t.Fatalf("ClearDynamicList failed: %v", err)
	}
	if len(mgr.GetEntries()) != 0 {
		t.Errorf("Expected 0 entries after clear, got %d", len(mgr.GetEntries()))
	}
}

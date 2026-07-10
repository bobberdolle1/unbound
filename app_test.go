package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unbound/engine"
)

func TestGetBypassLists(t *testing.T) {
	app := NewApp()
	lists := app.GetBypassLists()
	expected := []string{"youtube.txt", "discord.txt", "other.txt", "ipset-exclude.txt"}

	if len(lists) != len(expected) {
		t.Fatalf("expected %d lists, got %d", len(expected), len(lists))
	}

	for i, name := range expected {
		if lists[i] != name {
			t.Errorf("expected list %d to be %s, got %s", i, name, lists[i])
		}
	}
}

func TestReadBypassList_InvalidPath(t *testing.T) {
	app := NewApp()
	_, err := app.ReadBypassList("../etc/passwd")
	if err == nil {
		t.Error("expected error for directory traversal attempt, got nil")
	} else if !strings.Contains(err.Error(), "file access denied") {
		t.Errorf("expected 'file access denied' error, got: %v", err)
	}
}

func TestSaveBypassList_InvalidPath(t *testing.T) {
	app := NewApp()
	err := app.SaveBypassList("../etc/passwd", "malicious content")
	if err == nil {
		t.Error("expected error for directory traversal attempt, got nil")
	} else if !strings.Contains(err.Error(), "file access denied") {
		t.Errorf("expected 'file access denied' error, got: %v", err)
	}
}

func TestReadSaveBypassList_Success(t *testing.T) {
	app := NewApp()
	
	// Create mock config/lists dir for testing
	listsDir, err := engine.GetListsDir()
	if err != nil {
		t.Fatalf("failed to get lists dir: %v", err)
	}

	testFileName := "other.txt"
	originalPath := filepath.Join(listsDir, testFileName)
	
	// Backup original content if it exists
	var backup []byte
	backupExists := false
	if _, err := os.Stat(originalPath); err == nil {
		backup, _ = os.ReadFile(originalPath)
		backupExists = true
	}

	testContent := "example-domain.com\nanother-example.net"
	err = app.SaveBypassList(testFileName, testContent)
	if err != nil {
		t.Fatalf("failed to save bypass list: %v", err)
	}

	readContent, err := app.ReadBypassList(testFileName)
	if err != nil {
		t.Fatalf("failed to read bypass list: %v", err)
	}

	if readContent != testContent {
		t.Errorf("expected read content %q, got %q", testContent, readContent)
	}

	// Restore backup or cleanup
	if backupExists {
		os.WriteFile(originalPath, backup, 0644)
	} else {
		os.Remove(originalPath)
	}
}

func TestSecureDNS_Verification(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()

	// Just call IsSecureDNSEnabled to verify it doesn't panic
	// (Actual setting of secure DNS requires admin privileges and modify system network state, so we won't execute it to avoid breaking user's DNS during test)
	_ = app.IsSecureDNSEnabled()
}

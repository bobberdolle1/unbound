//go:build windows

package engine

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestProtectWindowsDirectoryTakesOwnershipOfPreexistingParent(t *testing.T) {
	if !windows.GetCurrentProcessToken().IsElevated() {
		t.Skip("changing directory ownership requires an elevated token")
	}

	parent := filepath.Join(t.TempDir(), "preexisting", "Unbound")
	if err := os.MkdirAll(parent, 0777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(parent, "attacker-controlled.txt"), []byte("marker"), 0666); err != nil {
		t.Fatal(err)
	}

	if err := protectWindowsDirectory(parent); err != nil {
		t.Fatalf("protectWindowsDirectory: %v", err)
	}
	adminSID, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyWindowsDirectoryProtection(parent, adminSID); err != nil {
		t.Fatalf("pre-existing parent was not secured: %v", err)
	}
}

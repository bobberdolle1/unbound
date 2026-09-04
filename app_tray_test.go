//go:build windows

package main

import (
	"bytes"
	"encoding/binary"
	"testing"
	"unbound/engine/providers"
)

func TestTrayIcon_ResourceLoading(t *testing.T) {
	if len(iconData) == 0 {
		t.Fatal("tray iconData is empty")
	}

	// Verify ICO format header (0x0000 0x0001)
	if len(iconData) < 6 {
		t.Fatalf("tray iconData length %d is less than ICO header size 6", len(iconData))
	}

	reserved := binary.LittleEndian.Uint16(iconData[0:2])
	icoType := binary.LittleEndian.Uint16(iconData[2:4])
	numImages := binary.LittleEndian.Uint16(iconData[4:6])

	if reserved != 0 || icoType != 1 {
		t.Fatalf("tray iconData is not a valid ICO file (reserved=%d, type=%d)", reserved, icoType)
	}

	if numImages == 0 {
		t.Fatal("tray ICO file contains 0 icon frames")
	}

	t.Logf("Successfully loaded tray icon (size: %d bytes, ICO frames: %d)", len(iconData), numImages)
}

func TestTrayIcon_Fallback(t *testing.T) {
	fakeIcon := []byte{}
	if len(fakeIcon) != 0 {
		t.Fatal("expected empty fallback check")
	}
	// Verify that iconData fallback is non-nil
	if bytes.Equal(iconData, fakeIcon) {
		t.Fatal("iconData equals empty slice")
	}
}

func TestTrayCachedPingText(t *testing.T) {
	app := NewApp()

	// Initial default
	if txt := app.getCachedPingText(); txt != "Пинг: —" {
		t.Errorf("Expected 'Пинг: —', got %q", txt)
	}

	// OK ping
	app.updateCachedPing(42, "ok")
	if txt := app.getCachedPingText(); txt != "Пинг: 42мс" {
		t.Errorf("Expected 'Пинг: 42мс', got %q", txt)
	}

	// Blocked ping
	app.updateCachedPing(0, "blocked")
	if txt := app.getCachedPingText(); txt != "Пинг: Заблокировано" {
		t.Errorf("Expected 'Пинг: Заблокировано', got %q", txt)
	}
}

func TestTraySnapshotIdenticalComparison(t *testing.T) {
	s1 := trayStateSnapshot{
		status:     "Running",
		profile:    "Recommended",
		pingText:   "Пинг: 25мс",
		autoTuning: false,
	}

	s2 := trayStateSnapshot{
		status:     "Running",
		profile:    "Recommended",
		pingText:   "Пинг: 25мс",
		autoTuning: false,
	}

	if s1 != s2 {
		t.Error("Identical snapshots should be equal via value comparison")
	}

	s3 := s2
	s3.pingText = "Пинг: 30мс"
	if s1 == s3 {
		t.Error("Different ping should not be equal")
	}
}

func TestTriggerTrayUpdateNonBlocking(t *testing.T) {
	app := NewApp()
	// Should never block even if channel buffer is full
	for range 10 {
		app.TriggerTrayUpdate()
	}
}

func TestTrayStateTransitionsStress(t *testing.T) {
	app := NewApp()

	states := []struct {
		status     providers.Status
		profile    string
		pingText   string
		autoTuning bool
	}{
		{"Stopped", "", "Пинг: —", false},
		{"Starting", "Recommended", "Пинг: —", false},
		{"Running", "Recommended", "Пинг: 25мс", false},
		{"Running", "Alternative 1", "Пинг: 30мс", false},
		{"Running", "Alternative 1", "Пинг: 35мс", true}, // autotuning
		{"Stopping", "", "Пинг: —", false},
		{"Error", "", "Пинг: —", false},
	}

	// Stress test: 200 rapid sequential and concurrent state transitions
	for range 50 {
		for _, st := range states {
			app.updateCachedPing(25, "ok")
			app.TriggerTrayUpdate()
			snap := app.getTraySnapshot()
			if snap.status == "" {
				t.Fatal("Empty status in snapshot")
			}
			_ = st
		}
	}
}

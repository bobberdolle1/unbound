//go:build windows

package main

import (
	"bytes"
	"encoding/binary"
	"testing"
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

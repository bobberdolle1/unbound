package engine

import (
	"runtime"
	"testing"
)

func TestDefaultSettingsDoNotReferencePlatformSpecificProfile(t *testing.T) {
	settings := getDefaultSettings()
	if settings.DefaultProfile != "" {
		t.Fatalf("default profile = %q, want dynamic platform selection", settings.DefaultProfile)
	}
	if settings.StartupProfileMode != "Последний использованный" {
		t.Fatalf("startup mode = %q", settings.StartupProfileMode)
	}
}

func TestSaveLastProfilePreservesOtherSettings(t *testing.T) {
	configRoot := t.TempDir()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("APPDATA", configRoot)
	case "darwin":
		t.Setenv("HOME", configRoot)
	default:
		t.Setenv("XDG_CONFIG_HOME", configRoot)
	}

	original := getDefaultSettings()
	original.SecureDNS = true
	original.GameFilter = false
	settingsMu.Lock()
	if err := writeSettings(original); err != nil {
		settingsMu.Unlock()
		t.Fatal(err)
	}
	settingsMu.Unlock()

	if err := SaveLastProfile("Native Profile"); err != nil {
		t.Fatal(err)
	}
	loaded, err := GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DefaultProfile != "Native Profile" || !loaded.SecureDNS || loaded.GameFilter {
		t.Fatalf("profile update changed unrelated settings: %+v", loaded)
	}
}

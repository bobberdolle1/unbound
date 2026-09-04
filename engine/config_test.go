package engine

import (
	"os"
	"path/filepath"
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

func TestLegacySettingsMigrateAutoStartProfile(t *testing.T) {
	configRoot := t.TempDir()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("APPDATA", configRoot)
	case "darwin":
		t.Setenv("HOME", configRoot)
	default:
		t.Setenv("XDG_CONFIG_HOME", configRoot)
	}

	// Settings written before autoStartProfile existed: the single
	// "Автозапуск" flag drove both OS registration and profile startup.
	legacy := `{"autoStart":true,"startMinimized":false,"defaultProfile":"X","startupProfileMode":"Последний использованный"}`
	settingsPath, err := GetSettingsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}

	loaded, err := GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.AutoStartProfile {
		t.Fatalf("legacy autoStart=true settings did not migrate to autoStartProfile: %+v", loaded)
	}

	// A settings file that already carries autoStartProfile must not be
	// rewritten by the migration.
	modern := `{"autoStart":true,"autoStartProfile":false}`
	if err := os.WriteFile(settingsPath, []byte(modern), 0644); err != nil {
		t.Fatal(err)
	}
	loaded, err = GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AutoStartProfile {
		t.Fatalf("explicit autoStartProfile=false was overridden: %+v", loaded)
	}
}

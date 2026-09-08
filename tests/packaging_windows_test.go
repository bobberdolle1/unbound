package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unbound/engine"
	"unbound/engine/providers"
)

func TestWindowsPackagingLaunchers(t *testing.T) {
	root := ".."
	controlDir := filepath.Join(root, "scripts", "control_windows")

	requiredLaunchers := map[string]string{
		"general_recommended.cmd":     "rec",
		"general_autotune.cmd":        "--autotune",
		"general_universal.cmd":       "universal",
		"general_alt1_multisplit.cmd": "alt1",
		"general_alt2_fake_tls.cmd":   "alt2",
		"service_control.cmd":         "--control",
	}

	// Initialize manager to get actual current profiles
	manager := providers.NewProviderManager()
	assets, _ := engine.ExtractAssets()
	if assets != nil {
		listsDir, _ := engine.GetListsDir()
		if listsDir == "" {
			listsDir = assets.ListDir
		}
		wp := providers.NewZapret2WindowsProvider(
			filepath.Join(assets.BinDir, "winws2.exe"),
			assets.LuaDir,
			listsDir,
			assets.EngineSHA256,
			false,
			true,
		)
		if wp != nil {
			engine.RegisterWindowsProfileCatalog(wp, assets.LuaDir)
			manager.Register(wp)
		}
	}

	availableProfiles := manager.GetProfiles("Zapret 2 (winws)")

	for launcherName, expectedArg := range requiredLaunchers {
		launcherPath := filepath.Join(controlDir, launcherName)
		contentBytes, err := os.ReadFile(launcherPath)
		if err != nil {
			t.Fatalf("Required launcher %s missing from %s: %v", launcherName, controlDir, err)
		}
		content := string(contentBytes)

		// 1. Must contain elevation check
		if !strings.Contains(content, "net session") {
			t.Errorf("Launcher %s is missing 'net session' elevation check", launcherName)
		}

		// 2. Must reference Unbound.exe
		if !strings.Contains(content, "Unbound.exe") {
			t.Errorf("Launcher %s does not reference Unbound.exe", launcherName)
		}

		// 3. Must reference the expected CLI mode or profile alias
		if !strings.Contains(content, expectedArg) {
			t.Errorf("Launcher %s missing expected argument %q", launcherName, expectedArg)
		}

		// 4. Must NOT reference legacy external winws paths or invalid flags
		if strings.Contains(content, "core_bin\\winws2.exe") || strings.Contains(content, "--driver") {
			t.Errorf("Launcher %s references obsolete or internal winws2 paths", launcherName)
		}

		// 5. If it's a profile launcher, verify the profile alias resolves to an actual profile
		if !strings.HasPrefix(expectedArg, "--") && len(availableProfiles) > 0 {
			resolved := false
			for _, p := range availableProfiles {
				pLower := strings.ToLower(p)
				if strings.Contains(pLower, expectedArg) || (expectedArg == "rec" && strings.Contains(pLower, "recommended")) ||
					(expectedArg == "univ" && strings.Contains(pLower, "universal")) ||
					(expectedArg == "alt1" && strings.Contains(pLower, "alternative 1")) ||
					(expectedArg == "alt2" && strings.Contains(pLower, "alternative 2")) {
					resolved = true
					break
				}
			}
			if !resolved {
				t.Errorf("Launcher %s uses alias %q which does not match any current engine profiles: %v",
					launcherName, expectedArg, availableProfiles)
			}
		}
	}
}

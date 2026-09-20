package main

import (
	"archive/zip"
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

		// 3. A synchronous launcher must propagate its child failure rather
		// than pause or turn it into a successful cmd.exe exit.
		if !strings.Contains(strings.ToLower(content), "exit /b %errorlevel%") {
			t.Errorf("Launcher %s does not propagate the child exit code", launcherName)
		}
		if strings.Contains(strings.ToLower(content), "pause") {
			t.Errorf("Launcher %s pauses instead of returning a scriptable failure", launcherName)
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

func TestWindowsPackagingArchive(t *testing.T) {
	archivePath := os.Getenv("UNBOUND_WINDOWS_ARCHIVE")
	if archivePath == "" {
		t.Skip("set UNBOUND_WINDOWS_ARCHIVE to validate a staged Windows release archive")
	}

	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatalf("open Windows release archive: %v", err)
	}
	defer archive.Close()

	requiredFiles := map[string]bool{
		"Unbound.exe":                           false,
		"README.md":                             false,
		"CHANGELOG.md":                          false,
		"LICENSE":                               false,
		"ZAPRET2_LICENSE.txt":                   false,
		"ZAPRET_LICENSE.txt":                    false,
		"ENGINE_PROVENANCE.json":                false,
		"verify_uac_acceptance.ps1":             false,
		"run_offline_physical_acceptance.ps1":   false,
		"start_offline_physical_acceptance.ps1": false,
		"general_recommended.cmd":               false,
		"general_autotune.cmd":                  false,
		"general_universal.cmd":                 false,
		"general_alt1_multisplit.cmd":           false,
		"general_alt2_fake_tls.cmd":             false,
		"service_control.cmd":                   false,
		"final_acceptance_v0.6.9.ps1":           false,
		"final_acceptance_v0.6.9.cmd":           false,
		"CANDIDATE.json":                        false,
		"BUNDLE_SHA256SUMS.txt":                 false,
	}
	for _, file := range archive.File {
		if _, required := requiredFiles[file.Name]; required {
			requiredFiles[file.Name] = true
		}
	}
	for name, found := range requiredFiles {
		if !found {
			t.Errorf("staged Windows archive missing %s", name)
		}
	}
}

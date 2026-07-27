package engine

import (
	"os"
	"testing"
)

func TestGetProfiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "unbound_profiles_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	profiles := GetProfiles(tempDir)
	if len(profiles) == 0 {
		t.Errorf("GetProfiles returned empty list")
	}

	for _, p := range profiles {
		if p.Name == "" {
			t.Errorf("Profile has empty name")
		}
		if len(p.Args) == 0 {
			t.Errorf("Profile %s has no arguments", p.Name)
		}
	}
}

func TestGetAdvancedProfiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "unbound_adv_profiles_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	advProfiles := GetAdvancedProfiles(tempDir)
	if len(advProfiles) == 0 {
		t.Errorf("GetAdvancedProfiles returned empty list")
	}

	for _, p := range advProfiles {
		if p.Name == "" {
			t.Errorf("Advanced profile has empty name")
		}
	}
}

func TestCustomScriptPersistence(t *testing.T) {
	testScript := `-- Custom LUA Strategy Test
function desync_custom(ctx)
    DLOG("Custom strategy executed")
end
`

	err := SaveCustomScript(testScript)
	if err != nil {
		t.Fatalf("SaveCustomScript failed: %v", err)
	}

	loaded, err := LoadCustomScript()
	if err != nil {
		t.Fatalf("LoadCustomScript failed: %v", err)
	}

	if loaded != testScript {
		t.Errorf("Loaded custom script does not match saved script.\nExpected: %s\nGot: %s", testScript, loaded)
	}
}

func TestUserAgent(t *testing.T) {
	ua := UserAgent()
	if ua == "" {
		t.Errorf("UserAgent returned empty string")
	}
	if ua != "Unbound/"+Version {
		t.Errorf("UserAgent mismatch. Got %s, expected Unbound/%s", ua, Version)
	}
}

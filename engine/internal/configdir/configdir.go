package configdir

import (
	"os"
	"path/filepath"
	"sync"
)

const ConfigDirName = "Unbound"

var (
	testConfigDir string
	testConfigMu  sync.RWMutex
)

// SetForTest overrides the configuration directory during tests and returns a
// restore callback.
func SetForTest(dir string) func() {
	testConfigMu.Lock()
	old := testConfigDir
	testConfigDir = dir
	testConfigMu.Unlock()
	return func() {
		testConfigMu.Lock()
		testConfigDir = old
		testConfigMu.Unlock()
	}
}

// GetConfigDir returns the active Unbound configuration directory, honoring
// test overrides configured via SetForTest / engine.SetConfigDirForTest.
func GetConfigDir() (string, error) {
	testConfigMu.RLock()
	if testConfigDir != "" {
		dir := testConfigDir
		testConfigMu.RUnlock()
		return dir, nil
	}
	testConfigMu.RUnlock()

	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	configPath := filepath.Join(userConfigDir, ConfigDirName)
	if err := os.MkdirAll(configPath, 0755); err != nil {
		return "", err
	}
	return configPath, nil
}

// GetListsDir returns the managed hostlists/ipsets directory inside GetConfigDir.
func GetListsDir() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	listsDir := filepath.Join(configDir, "lists")
	if err := os.MkdirAll(listsDir, 0755); err != nil {
		return "", err
	}
	return listsDir, nil
}

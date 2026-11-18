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

	hooksMu       sync.Mutex
	onChangeHooks []func()
)

// RegisterOnChange registers a callback invoked whenever SetForTest switches or
// restores the overridden config directory (e.g., closing open log handles
// inside a per-test temp directory before t.TempDir cleanup on Windows).
func RegisterOnChange(fn func()) {
	if fn == nil {
		return
	}
	hooksMu.Lock()
	onChangeHooks = append(onChangeHooks, fn)
	hooksMu.Unlock()
}

func notifyChangeHooks() {
	hooksMu.Lock()
	hooks := make([]func(), len(onChangeHooks))
	copy(hooks, onChangeHooks)
	hooksMu.Unlock()
	for _, fn := range hooks {
		fn()
	}
}

// SetForTest overrides the configuration directory during tests and returns a
// restore callback.
func SetForTest(dir string) func() {
	notifyChangeHooks()
	testConfigMu.Lock()
	old := testConfigDir
	testConfigDir = dir
	testConfigMu.Unlock()
	return func() {
		notifyChangeHooks()
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

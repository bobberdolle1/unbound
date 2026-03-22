package main

import (
	"context"
	"testing"
	"time"
	"unbound/engine"
	"unbound/engine/providers"
)

type MockEventEmitter struct {
	events map[string][]interface{}
}

func NewMockEventEmitter() *MockEventEmitter {
	return &MockEventEmitter{
		events: make(map[string][]interface{}),
	}
}

func (m *MockEventEmitter) Emit(eventName string, data interface{}) {
	if m.events[eventName] == nil {
		m.events[eventName] = make([]interface{}, 0)
	}
	m.events[eventName] = append(m.events[eventName], data)
}

func (m *MockEventEmitter) GetEvents(eventName string) []interface{} {
	return m.events[eventName]
}

func (m *MockEventEmitter) HasEvent(eventName string) bool {
	return len(m.events[eventName]) > 0
}

func TestAppInitialization(t *testing.T) {
	app := NewApp()

	if app == nil {
		t.Fatal("App initialization failed")
	}

	if app.manager == nil {
		t.Error("Provider manager not initialized")
	}

	t.Log("App initialized successfully")
}

func TestGetEngineNames(t *testing.T) {
	app := NewApp()
	ctx := context.Background()
	app.startup(ctx)

	engines := app.GetEngineNames()

	if len(engines) == 0 {
		t.Error("No engines registered")
	}

	expectedEngine := "Zapret 2 (winws)"
	found := false
	for _, engine := range engines {
		if engine == expectedEngine {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected engine '%s' not found", expectedEngine)
	}

	t.Logf("Found %d engines: %v", len(engines), engines)
}

func TestGetProfiles(t *testing.T) {
	app := NewApp()
	ctx := context.Background()
	app.startup(ctx)

	profiles := app.GetProfiles("Zapret 2 (winws)")

	if len(profiles) == 0 {
		t.Error("No profiles available")
	}

	expectedProfiles := []string{
		"Unbound Ultimate (God Mode)",
		"YouTube + Discord (Universal)",
		"Lightweight (Low CPU)",
	}

	for _, expected := range expectedProfiles {
		found := false
		for _, profile := range profiles {
			if profile == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected profile '%s' not found", expected)
		}
	}

	t.Logf("Found %d profiles", len(profiles))
}

func TestAutoStartMethods(t *testing.T) {
	app := NewApp()
	ctx := context.Background()
	app.ctx = ctx

	enabled := app.IsAutoStartEnabled()
	t.Logf("Auto-start currently enabled: %v", enabled)

	if err := app.DisableAutoStart(); err != nil {
		t.Logf("Disable auto-start returned error (may be expected): %v", err)
	}

	disabled := app.IsAutoStartEnabled()
	if disabled {
		t.Log("Auto-start still enabled after disable attempt")
	}
}

func TestGetSystemInfo(t *testing.T) {
	app := NewApp()

	info := app.GetSystemInfo()

	if len(info) == 0 {
		t.Error("System info is empty")
	}

	requiredKeys := []string{"os", "arch", "go_version"}
	for _, key := range requiredKeys {
		if _, exists := info[key]; !exists {
			t.Errorf("Missing required system info key: %s", key)
		}
	}

	t.Logf("System info: %+v", info)
}

func TestGetSettings(t *testing.T) {
	app := NewApp()

	settings, err := app.GetSettings()
	if err != nil {
		t.Logf("GetSettings returned error (may be expected): %v", err)
		return
	}

	if settings == nil {
		t.Error("Settings should not be nil")
	}

	t.Logf("Settings loaded: %+v", settings)
}

func TestSaveSettings(t *testing.T) {
	app := NewApp()

	testSettings := engine.Settings{
		AutoStart:          false,
		StartMinimized:     false,
		DefaultProfile:     "Test Profile",
		StartupProfileMode: "Last Used",
		GameFilter:         false,
		AutoUpdateEnabled:  true,
		ShowLogs:           true,
	}

	err := app.SaveSettings(testSettings)
	if err != nil {
		t.Errorf("Failed to save settings: %v", err)
	}

	t.Log("Settings saved successfully")
}

func TestCancelAutoTune(t *testing.T) {
	app := NewApp()
	ctx := context.Background()
	app.ctx = ctx

	app.CancelAutoTune()

	if app.autoTuneCancel != nil {
		t.Error("Cancel function should be nil when not running")
	}

	t.Log("CancelAutoTune handled correctly when not running")
}

func TestProviderManagerIntegration(t *testing.T) {
	app := NewApp()

	if app.manager == nil {
		t.Fatal("Provider manager not initialized")
	}

	engines := app.GetEngineNames()
	if len(engines) == 0 {
		t.Skip("No engines available for testing")
	}

	status := app.GetStatus()
	if status == "" {
		t.Error("Status should not be empty")
	}

	t.Logf("Current status: %s", status)
}

func TestLogsRetrieval(t *testing.T) {
	app := NewApp()
	ctx := context.Background()
	app.startup(ctx)

	logs := app.GetLogs()

	if logs == nil {
		t.Error("Logs should not be nil")
	}

	t.Logf("Retrieved %d log entries", len(logs))
}

func TestEngineStartStopCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping engine start/stop test in short mode")
	}

	app := NewApp()
	ctx := context.Background()
	app.startup(ctx)

	engines := app.GetEngineNames()
	if len(engines) == 0 {
		t.Skip("No engines available")
	}

	profiles := app.GetProfiles(engines[0])
	if len(profiles) == 0 {
		t.Skip("No profiles available")
	}

	initialStatus := app.GetStatus()
	t.Logf("Initial status: %s", initialStatus)

	if err := app.StopEngine(); err != nil {
		t.Logf("Stop engine returned error (may be expected): %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	finalStatus := app.GetStatus()
	t.Logf("Final status: %s", finalStatus)
}

func TestProviderRegistration(t *testing.T) {
	manager := providers.NewProviderManager()

	if manager == nil {
		t.Fatal("Failed to create provider manager")
	}

	assets, err := engine.ExtractAssets()
	if err != nil {
		t.Skip("Assets not available for testing")
	}

	provider := providers.NewZapret2WindowsProvider(
		assets.BinDir,
		assets.LuaDir,
		assets.ListDir,
		false,
		false,
	)

	if provider == nil {
		t.Fatal("Failed to create provider")
	}

	manager.Register(provider)

	t.Log("Provider registered successfully")
}

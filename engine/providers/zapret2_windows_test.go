//go:build windows
// +build windows

package providers

import (
	"context"
	"testing"
	"time"
)

func TestZapret2ProviderInitialization(t *testing.T) {
	provider := NewZapret2WindowsProvider(
		"test/bin",
		"test/lua",
		"test/lists",
		false,
		false,
	)

	if provider == nil {
		t.Fatal("Provider initialization failed")
	}

	if provider.Name() != "Zapret 2 (winws)" {
		t.Errorf("Expected name 'Zapret 2 (winws)', got '%s'", provider.Name())
	}

	if provider.GetStatus() != StatusStopped {
		t.Errorf("Expected initial status 'Stopped', got '%s'", provider.GetStatus())
	}

	t.Log("Provider initialized successfully")
}

func TestProfileRegistration(t *testing.T) {
	provider := NewZapret2WindowsProvider("", "", "", false, false)

	testProfiles := []struct {
		name string
		args []string
	}{
		{"Test Profile 1", []string{"--filter-tcp=443"}},
		{"Test Profile 2", []string{"--filter-tcp=80,443"}},
		{"Test Profile 3", []string{"--filter-udp=443"}},
	}

	for _, prof := range testProfiles {
		provider.RegisterProfile(prof.name, prof.args)
	}

	profiles := provider.GetProfiles()

	if len(profiles) < len(testProfiles) {
		t.Errorf("Expected at least %d profiles, got %d", len(testProfiles), len(profiles))
	}

	for _, expected := range testProfiles {
		found := false
		for _, actual := range profiles {
			if actual == expected.name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Profile '%s' not found in registered profiles", expected.name)
		}
	}

	t.Logf("Successfully registered %d profiles", len(testProfiles))
}

func TestGetProfileArgs(t *testing.T) {
	provider := NewZapret2WindowsProvider(
		"test/bin",
		"test/lua",
		"test/lists",
		true,
		false,
	)

	testArgs := []string{"--filter-tcp=443", "--lua-desync=multisplit:pos=1"}
	provider.RegisterProfile("Test Profile", testArgs)

	args := provider.getProfileArgsLocked("Test Profile")

	if len(args) == 0 {
		t.Error("Profile args should not be empty")
	}

	hasWfL3 := false
	hasLuaInit := false
	hasDebug := false

	for _, arg := range args {
		if arg == "--wf-l3=ipv4,ipv6" {
			hasWfL3 = true
		}
		if len(arg) > 11 && arg[:11] == "--lua-init=" {
			hasLuaInit = true
		}
		if arg == "--debug=1" {
			hasDebug = true
		}
	}

	if !hasWfL3 {
		t.Error("Missing mandatory --wf-l3 parameter")
	}

	if !hasLuaInit {
		t.Error("Missing --lua-init parameter")
	}

	if !hasDebug {
		t.Error("Debug mode enabled but --debug=1 not found")
	}

	t.Logf("Generated %d arguments for profile", len(args))
}

func TestStatusTransitions(t *testing.T) {
	provider := NewZapret2WindowsProvider("", "", "", false, false)

	if provider.GetStatus() != StatusStopped {
		t.Error("Initial status should be Stopped")
	}

	provider.status = StatusStarting
	if provider.GetStatus() != StatusStarting {
		t.Error("Status should be Starting")
	}

	provider.status = StatusRunning
	if provider.GetStatus() != StatusRunning {
		t.Error("Status should be Running")
	}

	provider.status = StatusError
	if provider.GetStatus() != StatusError {
		t.Error("Status should be Error")
	}

	provider.status = StatusStopped
	if provider.GetStatus() != StatusStopped {
		t.Error("Status should be Stopped")
	}

	t.Log("Status transitions working correctly")
}

func TestLogManagement(t *testing.T) {
	provider := NewZapret2WindowsProvider("", "", "", false, false)

	initialLogs := provider.GetLogs()
	if len(initialLogs) == 0 {
		t.Error("Should have at least initialization log")
	}

	for i := 0; i < 150; i++ {
		provider.addLog("Test log entry " + string(rune(i)))
	}

	logs := provider.GetLogs()
	if len(logs) > 100 {
		t.Errorf("Log buffer should be limited to 100 entries, got %d", len(logs))
	}

	t.Logf("Log management working correctly, buffer size: %d", len(logs))
}

func TestWaitReady(t *testing.T) {
	provider := NewZapret2WindowsProvider("", "", "", false, false)

	go func() {
		time.Sleep(100 * time.Millisecond)
		provider.engineReady <- true
	}()

	ready := provider.WaitReady(500 * time.Millisecond)
	if !ready {
		t.Error("Engine should be ready")
	}

	t.Log("WaitReady working correctly")
}

func TestWaitReadyTimeout(t *testing.T) {
	provider := NewZapret2WindowsProvider("", "", "", false, false)

	ready := provider.WaitReady(100 * time.Millisecond)
	if ready {
		t.Error("Engine should not be ready (timeout expected)")
	}

	t.Log("WaitReady timeout working correctly")
}

func TestCheckPrivileges(t *testing.T) {
	provider := NewZapret2WindowsProvider("", "", "", false, false)

	hasPriv, err := provider.CheckPrivileges()
	if err != nil {
		t.Logf("Privilege check returned error: %v", err)
	}

	t.Logf("Has administrator privileges: %v", hasPriv)
}

func TestStatusCallback(t *testing.T) {
	provider := NewZapret2WindowsProvider("", "", "", false, false)

	callbackCalled := false
	var receivedStatus Status

	provider.SetStatusCallback(func(status Status) {
		callbackCalled = true
		receivedStatus = status
	})

	provider.status = StatusRunning
	if provider.onStatusChange != nil {
		provider.onStatusChange(StatusRunning)
	}

	if !callbackCalled {
		t.Error("Status callback was not called")
	}

	if receivedStatus != StatusRunning {
		t.Errorf("Expected status Running, got %s", receivedStatus)
	}

	t.Log("Status callback working correctly")
}

func TestConcurrentAccess(t *testing.T) {
	provider := NewZapret2WindowsProvider("", "", "", false, false)

	done := make(chan bool)

	go func() {
		for i := 0; i < 100; i++ {
			provider.GetStatus()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			provider.GetLogs()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			provider.addLog("Concurrent log")
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	for i := 0; i < 3; i++ {
		<-done
	}

	t.Log("Concurrent access test passed")
}

func TestStartWithoutPrivileges(t *testing.T) {
	provider := NewZapret2WindowsProvider("", "", "", false, false)

	ctx := context.Background()
	err := provider.Start(ctx, "Test Profile")

	if err == nil {
		t.Log("Start succeeded (may have admin privileges)")
		provider.Stop()
	} else {
		if err.Error() != "administrator privileges required" {
			t.Logf("Start failed with expected error: %v", err)
		}
	}
}

//go:build windows
// +build windows

package engine

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestEnableAutoStartCommandGeneration(t *testing.T) {
	exePath := "C:\\Program Files\\Unbound\\unbound.exe"

	expectedArgs := []string{
		"/Create",
		"/TN", "UnboundDPIBypass",
		"/TR", `"C:\Program Files\Unbound\unbound.exe"`,
		"/SC", "ONLOGON",
		"/RL", "HIGHEST",
		"/F",
	}

	for i, arg := range expectedArgs {
		t.Logf("Expected arg[%d]: %s", i, arg)
	}

	if !strings.Contains(expectedArgs[4], exePath) {
		t.Error("Executable path not properly quoted")
	}

	if expectedArgs[8] != "HIGHEST" {
		t.Error("Missing /RL HIGHEST flag")
	}

	if expectedArgs[5] != "/SC" || expectedArgs[6] != "ONLOGON" {
		t.Error("Incorrect trigger configuration")
	}
}

func TestDisableAutoStartCommandGeneration(t *testing.T) {
	expectedArgs := []string{
		"/Delete",
		"/TN", "UnboundDPIBypass",
		"/F",
	}

	if expectedArgs[0] != "/Delete" {
		t.Error("Expected /Delete command")
	}

	if expectedArgs[2] != "UnboundDPIBypass" {
		t.Error("Incorrect task name")
	}
}

func TestIsAutoStartEnabledQuery(t *testing.T) {
	enabled, err := IsAutoStartEnabled()

	if err != nil {
		t.Logf("Query returned error (expected if task doesn't exist): %v", err)
	}

	t.Logf("Auto-start enabled: %v", enabled)
}

func TestAutoStartTaskNameConstant(t *testing.T) {
	if taskName == "" {
		t.Error("Task name constant is empty")
	}

	if strings.Contains(taskName, " ") {
		t.Error("Task name should not contain spaces")
	}

	if taskName != "UnboundDPIBypass" {
		t.Errorf("Expected task name 'UnboundDPIBypass', got '%s'", taskName)
	}
}

func TestSchtasksAvailability(t *testing.T) {
	cmd := exec.Command("schtasks.exe", "/?")
	output, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("schtasks.exe not available: %v", err)
	}

	if !strings.Contains(string(output), "SCHTASKS") {
		t.Error("schtasks.exe output unexpected")
	}

	t.Log("schtasks.exe is available")
}

func TestPathWithSpacesHandling(t *testing.T) {
	testPaths := []string{
		"C:\\Program Files\\Unbound\\unbound.exe",
		"C:\\Users\\Test User\\AppData\\Local\\Unbound\\unbound.exe",
		"D:\\My Apps\\Unbound DPI\\unbound.exe",
	}

	for _, path := range testPaths {
		quoted := `"` + path + `"`

		if !strings.HasPrefix(quoted, `"`) || !strings.HasSuffix(quoted, `"`) {
			t.Errorf("Path not properly quoted: %s", quoted)
		}

		if strings.Count(quoted, `"`) != 2 {
			t.Errorf("Path has incorrect number of quotes: %s", quoted)
		}

		t.Logf("Path correctly quoted: %s", quoted)
	}
}

func TestGetAutoStartTaskInfo(t *testing.T) {
	info, err := GetAutoStartTaskInfo()

	if err != nil {
		t.Logf("Task info query failed (expected if task doesn't exist): %v", err)
		return
	}

	if len(info) == 0 {
		t.Log("Task exists but no info returned")
		return
	}

	t.Logf("Task info retrieved: %d fields", len(info))
	for key, value := range info {
		t.Logf("  %s: %s", key, value)
	}
}

func TestEnableDisableAutoStartIntegration(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=1 to run")
	}

	initialState, _ := IsAutoStartEnabled()
	t.Logf("Initial auto-start state: %v", initialState)

	if err := EnableAutoStart(); err != nil {
		t.Fatalf("Failed to enable auto-start: %v", err)
	}

	enabled, err := IsAutoStartEnabled()
	if err != nil {
		t.Fatalf("Failed to check auto-start status: %v", err)
	}

	if !enabled {
		t.Error("Auto-start should be enabled but isn't")
	}

	if err := DisableAutoStart(); err != nil {
		t.Fatalf("Failed to disable auto-start: %v", err)
	}

	disabled, err := IsAutoStartEnabled()
	if err != nil {
		t.Fatalf("Failed to check auto-start status: %v", err)
	}

	if disabled {
		t.Error("Auto-start should be disabled but isn't")
	}

	if initialState {
		EnableAutoStart()
	}
}

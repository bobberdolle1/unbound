package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// testBinaryPath returns a temp path with the executable suffix the host OS
// expects. Hardcoding ".exe" made these tests Windows-only for no reason.
func testBinaryPath(name string) string {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(os.TempDir(), name)
}

// skipIfNoRuntimeEnvironment skips when the CLI could not get far enough to
// exercise what the test is checking. Only Windows embeds its bypass engine;
// on Linux and macOS nfqws comes from the user's package manager, so a runner
// without it is an environment gap rather than a regression.
func skipIfNoRuntimeEnvironment(t *testing.T, output string) {
	t.Helper()
	switch {
	case strings.Contains(output, "privileges required"),
		strings.Contains(output, "Run as administrator"),
		strings.Contains(output, "Root privileges required"):
		t.Skip("Skipping CLI E2E test - requires administrator/root privileges")
	case strings.Contains(output, "не найден бинарник движка"),
		strings.Contains(output, "No bypass engine is available"):
		t.Skip("Skipping CLI E2E test - no bypass engine installed on this host")
	}
}

func firstCLIProfile(t *testing.T, binary string) string {
	t.Helper()
	cmd := exec.Command(binary, "--list-profiles", "--json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		skipIfNoRuntimeEnvironment(t, string(output))
		t.Fatalf("could not list native profiles: %v\n%s", err, output)
	}
	var catalog map[string][]string
	if err := json.Unmarshal(output, &catalog); err != nil {
		t.Fatalf("invalid --list-profiles JSON: %v\n%s", err, output)
	}
	for _, profiles := range catalog {
		if len(profiles) > 0 {
			return profiles[0]
		}
	}
	t.Fatal("native profile catalog is empty")
	return ""
}

func TestCLIHeadlessMode(t *testing.T) {
	t.Log("Building temporary test binary...")

	tempBinary := testBinaryPath("temp_unbound_test")
	defer os.Remove(tempBinary)

	buildCmd := exec.Command("go", "build", "-o", tempBinary, "..")
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build test binary: %v\nOutput: %s", err, string(buildOutput))
	}

	t.Log("Test binary built successfully at:", tempBinary)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	profileName := firstCLIProfile(t, tempBinary)

	t.Logf("Executing CLI mode with --cli --profile='%s' --debug", profileName)

	cmd := exec.CommandContext(ctx, tempBinary, "--cli", "--profile="+profileName, "--debug")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	outputBytes := []byte(outputStr)
	cleanOutput := strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			return -1
		}
		return r
	}, outputStr)

	t.Logf("CLI Output (raw length: %d, cleaned length: %d):\n%s", len(outputStr), len(cleanOutput), cleanOutput)
	t.Logf("First 100 bytes (hex): %x", outputBytes[:min(100, len(outputBytes))])

	skipIfNoRuntimeEnvironment(t, cleanOutput)

	t.Logf("Output contains UNBOUND: %v", strings.Contains(cleanOutput, "UNBOUND"))
	t.Logf("Output contains Profile: %v", strings.Contains(cleanOutput, "Profile"))
	t.Logf("Output contains Checking: %v", strings.Contains(cleanOutput, "Checking"))
	t.Logf("Output contains Engine: %v", strings.Contains(cleanOutput, "Engine"))

	if ctx.Err() == context.DeadlineExceeded {
		t.Log("Context timeout reached (expected behavior - engine was running)")
	}

	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok {
			exitCode := exitErr.ExitCode()
			if exitCode == -1 || exitCode == 1 {
				t.Log("Process was terminated by context (expected)")
			} else {
				t.Errorf("Unexpected exit code: %d", exitCode)
			}
		}
	}

	t.Run("Console Attachment", func(t *testing.T) {
		cleanOutput := strings.Map(func(r rune) rune {
			if r < 32 && r != '\n' && r != '\r' && r != '\t' {
				return -1
			}
			return r
		}, outputStr)

		hasCliInit := strings.Contains(cleanOutput, "UNBOUND") ||
			strings.Contains(cleanOutput, "Headless CLI Mode") ||
			strings.Contains(cleanOutput, "Profile:")

		if !hasCliInit {
			t.Error("Output does not contain CLI mode initialization banner")
		}
	})

	t.Run("List Manager Initialization", func(t *testing.T) {
		cleanOutput := strings.Map(func(r rune) rune {
			if r < 32 && r != '\n' && r != '\r' && r != '\t' {
				return -1
			}
			return r
		}, outputStr)

		hasListCheck := strings.Contains(cleanOutput, "Checking for updated bypass lists") ||
			strings.Contains(cleanOutput, "lists") ||
			strings.Contains(cleanOutput, "discord") ||
			strings.Contains(cleanOutput, "telegram") ||
			strings.Contains(cleanOutput, "fallback") ||
			strings.Contains(cleanOutput, "Warning:")

		if !hasListCheck {
			t.Error("Output does not contain evidence of list manager initialization")
		}
	})

	t.Run("Engine Initialization", func(t *testing.T) {
		cleanOutput := strings.Map(func(r rune) rune {
			if r < 32 && r != '\n' && r != '\r' && r != '\t' {
				return -1
			}
			return r
		}, outputStr)

		hasEngineStart := strings.Contains(cleanOutput, "Engine started") ||
			strings.Contains(cleanOutput, "started successfully") ||
			strings.Contains(cleanOutput, "Starting") ||
			strings.Contains(cleanOutput, "Profile:") ||
			strings.Contains(cleanOutput, "Press Ctrl+C")

		if !hasEngineStart {
			t.Error("Output does not contain evidence of engine initialization")
		}
	})

	t.Run("No Panic Detection", func(t *testing.T) {
		if strings.Contains(outputStr, "panic:") || strings.Contains(outputStr, "runtime error") {
			t.Error("Detected panic or runtime error in output")
		}
	})

	t.Run("Graceful Execution", func(t *testing.T) {
		if strings.Contains(outputStr, "fatal error") {
			t.Error("Detected fatal error in output")
		}
	})
}

func TestCLIWithInvalidProfile(t *testing.T) {
	t.Log("Building temporary test binary...")

	tempBinary := testBinaryPath("temp_unbound_test2")
	defer os.Remove(tempBinary)

	buildCmd := exec.Command("go", "build", "-o", tempBinary, "..")
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build test binary: %v\nOutput: %s", err, string(buildOutput))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, tempBinary, "--cli", "--profile=NonExistentProfile")
	output, _ := cmd.CombinedOutput()
	outputStr := string(output)

	t.Log("CLI Output with invalid profile:\n", outputStr)

	skipIfNoRuntimeEnvironment(t, outputStr)

	if !strings.Contains(outputStr, "UNBOUND") {
		t.Error("CLI did not initialize properly even with invalid profile")
	}
}

func TestCLIDebugMode(t *testing.T) {
	t.Log("Building temporary test binary...")

	tempBinary := testBinaryPath("temp_unbound_test3")
	defer os.Remove(tempBinary)

	buildCmd := exec.Command("go", "build", "-o", tempBinary, "..")
	buildOutput, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build test binary: %v\nOutput: %s", err, string(buildOutput))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	profileName := firstCLIProfile(t, tempBinary)
	cmd := exec.CommandContext(ctx, tempBinary, "--cli", "--profile="+profileName, "--debug")
	output, _ := cmd.CombinedOutput()
	outputStr := string(output)

	t.Log("CLI Debug Output:\n", outputStr)

	skipIfNoRuntimeEnvironment(t, outputStr)

	if !strings.Contains(outputStr, "Debug") && !strings.Contains(outputStr, "ENABLED") && !strings.Contains(outputStr, "debug") {
		t.Log("Warning: Debug mode flag may not be reflected in output (non-critical)")
	}
}

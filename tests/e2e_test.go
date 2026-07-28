package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"unbound/engine"
	"unbound/engine/providers"
)

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

// buildTestBinary compiles the main binary once per test run and returns its
// path. It uses t.TempDir so the OS removes it after the test.
func buildTestBinary(t *testing.T) string {
	t.Helper()

	name := "unbound_e2e_test"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	bin := filepath.Join(t.TempDir(), name)

	// Build from the repo root (parent of tests/).
	cmd := exec.Command("go", "build", "-o", bin, "..")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}
	return bin
}

// runBinary runs the test binary with the given args, enforcing a timeout.
func runBinary(t *testing.T, bin string, timeout time.Duration, args ...string) (string, int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			exitCode = -1 // killed by timeout
		} else {
			exitCode = -2
		}
	}
	return string(out), exitCode
}

// cleanOutput strips non-printable characters (emoji sequences, etc.) that
// make string matching unreliable.
func cleanOutput(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\r' && r != '\t' {
			return -1
		}
		return r
	}, s)
}

// ──────────────────────────────────────────────────────────────────────────────
// E2E: CLI Flag Tests (no root required)
// ──────────────────────────────────────────────────────────────────────────────

func TestE2E_VersionFlag(t *testing.T) {
	bin := buildTestBinary(t)

	out, code := runBinary(t, bin, 10*time.Second, "--version")
	if code != 0 {
		t.Fatalf("--version exited %d; output: %s", code, out)
	}

	out = cleanOutput(out)

	// Must contain version string, OS, and arch.
	if !strings.Contains(out, engine.Version) {
		t.Errorf("version output missing %q: %s", engine.Version, out)
	}
	if !strings.Contains(out, runtime.GOOS) {
		t.Errorf("version output missing GOOS %q: %s", runtime.GOOS, out)
	}
	if !strings.Contains(out, runtime.GOARCH) {
		t.Errorf("version output missing GOARCH %q: %s", runtime.GOARCH, out)
	}

	// Must match the expected format: "unbound <version> (<os>/<arch>)"
	pattern := regexp.MustCompile(`unbound\s+\S+\s+\(\w+/\w+\)`)
	if !pattern.MatchString(out) {
		t.Errorf("version output does not match expected format: %s", out)
	}
}

func TestE2E_HelpFlag(t *testing.T) {
	bin := buildTestBinary(t)

	// --help is not an explicit flag, so flag.Parse prints usage and exits 2.
	out, code := runBinary(t, bin, 10*time.Second, "--help")
	out = cleanOutput(out)

	// Go's flag package exits 2 for --help.
	if code != 2 && code != 0 {
		t.Logf("--help exited %d (expected 0 or 2)", code)
	}

	// Must list all known flags.
	requiredFlags := []string{"-cli", "-profile", "-tray", "-debug", "-version", "-list-profiles"}
	for _, f := range requiredFlags {
		if !strings.Contains(out, f) {
			t.Errorf("help output missing flag %q", f)
		}
	}
}

func TestE2E_ListProfiles(t *testing.T) {
	bin := buildTestBinary(t)

	out, code := runBinary(t, bin, 15*time.Second, "--list-profiles")
	out = cleanOutput(out)

	// On platforms without the engine binary, it may exit 1.
	if code != 0 {
		if strings.Contains(out, "не найден") || strings.Contains(out, "No bypass engine") ||
			strings.Contains(out, "Установите") {
			t.Skip("No bypass engine binary on this host")
		}
		t.Fatalf("--list-profiles exited %d; output: %s", code, out)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines (engine + 1 profile), got %d: %s", len(lines), out)
	}

	// Count profiles (indented lines).
	profileCount := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "  ") {
			profileCount++
		}
	}
	if profileCount == 0 {
		t.Error("no indented profile lines found in output")
	}
	t.Logf("Found %d profiles across output", profileCount)
}

func TestE2E_InvalidFlag(t *testing.T) {
	bin := buildTestBinary(t)

	_, code := runBinary(t, bin, 10*time.Second, "--nonexistent-flag")
	if code == 0 {
		t.Error("expected non-zero exit code for invalid flag")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// E2E: CLI Execution Tests (may require root — skip if unprivileged)
// ──────────────────────────────────────────────────────────────────────────────

func TestE2E_CLIHeadlessStartStop(t *testing.T) {
	bin := buildTestBinary(t)

	// Run CLI mode with a short timeout so it gets killed.
	out, code := runBinary(t, bin, 5*time.Second, "--cli", "--debug")
	out = cleanOutput(out)

	// Skip if we lack privileges or the engine binary.
	switch {
	case strings.Contains(out, "privileges required"),
		strings.Contains(out, "Run as administrator"),
		strings.Contains(out, "Root privileges required"):
		t.Skip("Requires root/admin privileges")
	case strings.Contains(out, "не найден") || strings.Contains(out, "No bypass engine"):
		t.Skip("No bypass engine binary available")
	}

	// Verify CLI mode banner appeared.
	if !strings.Contains(out, "UNBOUND") {
		t.Errorf("missing UNBOUND banner in output: %.200s", out)
	}
	if !strings.Contains(out, "Profile:") {
		t.Errorf("missing Profile: line in output: %.200s", out)
	}

	// Verify no panic.
	if strings.Contains(out, "panic:") || strings.Contains(out, "runtime error") {
		t.Fatalf("detected panic in CLI output: %s", out)
	}

	// Context timeout should kill the process (exit -1 or signal).
	if code == 0 {
		t.Log("CLI exited cleanly (engine may have failed to start)")
	} else {
		t.Logf("CLI exited with code %d (expected: killed by timeout)", code)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Unit-level: Engine Internals
// ──────────────────────────────────────────────────────────────────────────────

func TestEngine_VersionConsistency(t *testing.T) {
	v := engine.Version
	if v == "" {
		t.Fatal("engine.Version is empty")
	}
	ua := engine.UserAgent()
	expected := "Unbound/" + v
	if ua != expected {
		t.Errorf("UserAgent() = %q, want %q", ua, expected)
	}
}

func TestEngine_AssetExtraction(t *testing.T) {
	assets, err := engine.ExtractAssets()
	if err != nil {
		t.Fatalf("ExtractAssets: %v", err)
	}
	if assets.BinDir == "" {
		t.Error("BinDir is empty")
	}
	if assets.LuaDir == "" {
		t.Error("LuaDir is empty")
	}
	if assets.ListDir == "" {
		t.Error("ListDir is empty")
	}
	// Verify the directories actually exist.
	for _, dir := range []string{assets.BinDir, assets.LuaDir, assets.ListDir} {
		if _, err := os.Stat(dir); err != nil {
			t.Errorf("directory does not exist: %s", dir)
		}
	}
}

func TestEngine_ProfileGeneration(t *testing.T) {
	assets, err := engine.ExtractAssets()
	if err != nil {
		t.Fatalf("ExtractAssets: %v", err)
	}

	profiles := engine.GetProfiles(assets.LuaDir)
	if len(profiles) == 0 {
		t.Fatal("GetProfiles returned 0 profiles")
	}

	for _, p := range profiles {
		if p.Name == "" {
			t.Error("profile has empty name")
		}
		if len(p.Args) == 0 {
			t.Errorf("profile %q has no args", p.Name)
		}
	}

	advProfiles := engine.GetAdvancedProfiles(assets.LuaDir)
	if len(advProfiles) == 0 {
		t.Fatal("GetAdvancedProfiles returned 0 profiles")
	}

	t.Logf("Base profiles: %d, Advanced profiles: %d", len(profiles), len(advProfiles))
}

func TestEngine_ConfigDir(t *testing.T) {
	dir, err := engine.GetConfigDir()
	if err != nil {
		t.Fatalf("GetConfigDir: %v", err)
	}
	if dir == "" {
		t.Fatal("config dir is empty")
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("config dir does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("config dir is not a directory: %s", dir)
	}
}

func TestEngine_SettingsRoundTrip(t *testing.T) {
	// Save settings, read them back, verify.
	original := &engine.Settings{
		AutoStart:             false,
		StartMinimized:        true,
		DefaultProfile:        "TestProfile",
		StartupProfileMode:    "Последний использованный",
		GameFilter:            true,
		AutoUpdateEnabled:     false,
		ShowLogs:              true,
		EnableTCPTimestamps:   false,
		DiscordCacheAutoClean: false,
		SecureDNS:             false,
	}
	if err := engine.SaveSettings(original); err != nil {
		t.Fatalf("SaveSettings: %v", err)
	}

	loaded, err := engine.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}

	if loaded.DefaultProfile != original.DefaultProfile {
		t.Errorf("DefaultProfile: got %q, want %q", loaded.DefaultProfile, original.DefaultProfile)
	}
	if loaded.StartMinimized != original.StartMinimized {
		t.Errorf("StartMinimized: got %v, want %v", loaded.StartMinimized, original.StartMinimized)
	}
	if loaded.GameFilter != original.GameFilter {
		t.Errorf("GameFilter: got %v, want %v", loaded.GameFilter, original.GameFilter)
	}
}

func TestEngine_CustomScriptRoundTrip(t *testing.T) {
	script := "-- test lua\nprint('hello')\n"
	if err := engine.SaveCustomScript(script); err != nil {
		t.Fatalf("SaveCustomScript: %v", err)
	}
	loaded, err := engine.LoadCustomScript()
	if err != nil {
		t.Fatalf("LoadCustomScript: %v", err)
	}
	if loaded != script {
		t.Errorf("script round-trip mismatch:\ngot:  %q\nwant: %q", loaded, script)
	}
}

func TestEngine_DNSProviders(t *testing.T) {
	// Verify all known providers exist.
	for _, name := range []string{"Cloudflare", "Google", "Quad9", "AdGuard"} {
		servers, ok := engine.DNSProviders[name]
		if !ok {
			t.Errorf("DNS provider %q not found", name)
			continue
		}
		if len(servers) < 2 {
			t.Errorf("DNS provider %q has fewer than 2 servers", name)
		}
	}

	// Test SetDNSProvider.
	engine.SetDNSProvider("Google")
	if engine.SecureDNSServers[0] != "8.8.8.8" {
		t.Errorf("after SetDNSProvider(Google), primary = %q, want 8.8.8.8", engine.SecureDNSServers[0])
	}

	// Restore default.
	engine.SetDNSProvider("Cloudflare")
	if engine.SecureDNSServers[0] != "1.1.1.1" {
		t.Errorf("after SetDNSProvider(Cloudflare), primary = %q, want 1.1.1.1", engine.SecureDNSServers[0])
	}
}

func TestEngine_IPCache(t *testing.T) {
	cache := engine.NewIPCache(5 * time.Second)

	// Set and get.
	cache.Set("example.com", []string{"1.2.3.4"}, 5*time.Second)
	ips, ok := cache.Get("example.com")
	if !ok {
		t.Fatal("cache miss for example.com")
	}
	if len(ips) != 1 || ips[0] != "1.2.3.4" {
		t.Errorf("unexpected IPs: %v", ips)
	}

	// Size.
	if cache.Size() != 1 {
		t.Errorf("Size() = %d, want 1", cache.Size())
	}

	// Delete.
	cache.Delete("example.com")
	_, ok = cache.Get("example.com")
	if ok {
		t.Error("expected cache miss after Delete")
	}

	// Clear.
	cache.Set("a.com", []string{"1.1.1.1"}, 5*time.Second)
	cache.Set("b.com", []string{"2.2.2.2"}, 5*time.Second)
	cache.Clear()
	if cache.Size() != 0 {
		t.Errorf("Size() after Clear = %d, want 0", cache.Size())
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Provider Manager Tests (cross-platform)
// ──────────────────────────────────────────────────────────────────────────────

func TestProviderManager_Lifecycle(t *testing.T) {
	m := providers.NewProviderManager()

	// No engines registered.
	names := m.GetEngineNames()
	if len(names) != 0 {
		t.Errorf("expected 0 engines, got %d", len(names))
	}

	// Status should be Stopped.
	if s := m.GetStatus(); s != providers.StatusStopped {
		t.Errorf("status = %q, want %q", s, providers.StatusStopped)
	}

	// Stop on empty manager should not error.
	if err := m.Stop(); err != nil {
		t.Errorf("Stop on empty manager: %v", err)
	}

	// Start with nonexistent engine should error.
	err := m.Start(context.Background(), "nonexistent", "profile")
	if err == nil {
		t.Error("expected error starting nonexistent engine")
	}
	if !strings.Contains(err.Error(), "engine not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestProviderManager_GetUptime(t *testing.T) {
	m := providers.NewProviderManager()

	uptime := m.GetUptime()
	if uptime != 0 {
		t.Errorf("uptime on fresh manager = %v, want 0", uptime)
	}

	info := m.GetStatusInfo()
	if info["status"] != string(providers.StatusStopped) {
		t.Errorf("status = %v, want Stopped", info["status"])
	}
	if info["uptime_seconds"].(int64) != 0 {
		t.Errorf("uptime_seconds = %v, want 0", info["uptime_seconds"])
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Asset Verification
// ──────────────────────────────────────────────────────────────────────────────

func TestEngine_AssetVerification(t *testing.T) {
	result := engine.VerifyAssets()
	if !result.Verified {
		t.Errorf("asset verification failed: %s", result.Error)
	}
	if result.TotalFiles == 0 {
		t.Error("TotalFiles is 0")
	}
	t.Logf("Verified %d engine assets", result.TotalFiles)
}

// ──────────────────────────────────────────────────────────────────────────────
// Logger Tests
// ──────────────────────────────────────────────────────────────────────────────

func TestEngine_Logger(t *testing.T) {
	logger := engine.NewLogger(100, engine.LogLevelDebug)

	logger.Debug("Test", "debug msg")
	logger.Info("Test", "info msg")
	logger.Warn("Test", "warn msg")
	logger.Error("Test", "error msg")
	logger.Infof("Test", "formatted %d", 42)

	entries := logger.GetEntries()
	if len(entries) != 5 {
		t.Errorf("expected 5 entries, got %d", len(entries))
	}

	formatted := logger.GetEntriesFormatted()
	if len(formatted) != 5 {
		t.Errorf("expected 5 formatted entries, got %d", len(formatted))
	}

	// Check that formatted entries contain the messages.
	found := false
	for _, f := range formatted {
		if strings.Contains(f, "formatted 42") {
			found = true
			break
		}
	}
	if !found {
		t.Error("formatted entries missing 'formatted 42'")
	}

	// Test log level filtering.
	filteredLogger := engine.NewLogger(50, engine.LogLevelWarn)
	filteredLogger.Debug("X", "should be dropped")
	filteredLogger.Info("X", "should be dropped")
	filteredLogger.Warn("X", "should appear")

	if len(filteredLogger.GetEntries()) != 1 {
		t.Errorf("expected 1 entry with Warn filter, got %d", len(filteredLogger.GetEntries()))
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Updater Tests
// ──────────────────────────────────────────────────────────────────────────────

func TestEngine_VersionComparison(t *testing.T) {
	// CheckForUpdates talks to GitHub API - we test the version string handling.
	v := engine.Version
	if v == "" {
		t.Fatal("engine.Version is empty")
	}

	// Verify version follows semver-ish pattern.
	if !strings.Contains(v, ".") {
		t.Errorf("version %q doesn't look like semver", v)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Orchestrator Tests
// ──────────────────────────────────────────────────────────────────────────────

func TestEngine_Orchestrator(t *testing.T) {
	o := engine.NewEngineOrchestrator()

	if o.IsRunning() {
		t.Error("fresh orchestrator reports running")
	}
	if err := o.StopEngine(); err != nil {
		t.Errorf("StopEngine on idle orchestrator: %v", err)
	}
	m := o.GetMetrics()
	if m.PacketsSent != 0 {
		t.Errorf("metrics.PacketsSent = %d on idle orchestrator", m.PacketsSent)
	}
}

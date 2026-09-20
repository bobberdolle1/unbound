//go:build windows
// +build windows

package providers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestZapret2ProviderInitialization(t *testing.T) {
	provider := NewZapret2WindowsProvider("test/bin", "test/lua", "test/lists", "", false, false)

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

func TestVerifyFileSHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "winws2.exe")
	data := []byte("trusted engine")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(data)
	expected := hex.EncodeToString(hash[:])
	if err := verifyFileSHA256(path, expected); err != nil {
		t.Fatalf("trusted file rejected: %v", err)
	}
	if err := verifyFileSHA256(path, ""); err == nil {
		t.Fatal("missing expected hash was accepted")
	}
	if err := os.WriteFile(path, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyFileSHA256(path, expected); err == nil {
		t.Fatal("tampered engine was accepted")
	}
}

func TestProfileRegistration(t *testing.T) {
	provider := NewZapret2WindowsProvider("", "", "", "", false, false)

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

func TestProfileRegistrationIsIdempotentAndUnknownFails(t *testing.T) {
	provider := NewZapret2WindowsProvider("", "", "", "", false, false)
	provider.RegisterProfile("Known", []string{"--filter-tcp=443"})
	provider.RegisterProfile("Known", []string{"--filter-tcp=80"})

	names := provider.GetProfiles()
	knownCount := 0
	for _, name := range names {
		if name == "Known" {
			knownCount++
		}
	}
	if knownCount != 1 {
		t.Fatalf("duplicate registration produced %d Known entries", knownCount)
	}
	if _, err := provider.getProfileArgsLocked("Unknown"); err == nil {
		t.Fatal("unknown profile silently fell back to a different strategy")
	}
}

func TestGetProfileArgs(t *testing.T) {
	provider := NewZapret2WindowsProvider("test/bin", "test/lua", "test/lists", "", true, false)

	testArgs := []string{"--filter-tcp=443", "--lua-desync=multisplit:pos=1"}
	provider.RegisterProfile("Test Profile", testArgs)

	args, err := provider.getProfileArgsLocked("Test Profile")
	if err != nil {
		t.Fatal(err)
	}

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
	provider := NewZapret2WindowsProvider("", "", "", "", false, false)

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

func TestSetStoppedClearsEveryOwnedState(t *testing.T) {
	provider := NewZapret2WindowsProvider("", "", "", "", false, false)
	provider.status = StatusRunning
	provider.currentProfile = "Profile A"
	provider.ownedPID = 1234
	provider.processDone = make(chan struct{})

	provider.setStoppedLocked("test")

	if provider.status != StatusStopped || provider.currentProfile != "" || provider.cmd != nil || provider.ownedPID != 0 || provider.processDone != nil {
		t.Fatalf("stopped invariant violated: status=%s profile=%q cmd=%v pid=%d done=%v",
			provider.status, provider.currentProfile, provider.cmd, provider.ownedPID, provider.processDone)
	}
}

func TestLogManagement(t *testing.T) {
	provider := NewZapret2WindowsProvider("", "", "", "", false, false)

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
	provider := NewZapret2WindowsProvider("", "", "", "", false, false)

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
	provider := NewZapret2WindowsProvider("", "", "", "", false, false)

	ready := provider.WaitReady(100 * time.Millisecond)
	if ready {
		t.Error("Engine should not be ready (timeout expected)")
	}

	t.Log("WaitReady timeout working correctly")
}

func TestCheckPrivileges(t *testing.T) {
	provider := NewZapret2WindowsProvider("", "", "", "", false, false)

	hasPriv, err := provider.CheckPrivileges()
	if err != nil {
		t.Logf("Privilege check returned error: %v", err)
	}

	t.Logf("Has administrator privileges: %v", hasPriv)
}

func TestStatusCallback(t *testing.T) {
	provider := NewZapret2WindowsProvider("", "", "", "", false, false)

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
	provider := NewZapret2WindowsProvider("", "", "", "", false, false)

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
	provider := NewZapret2WindowsProvider("", "", "", "", false, false)

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

func TestZapret2WindowsProviderHelperProcess(t *testing.T) {
	if os.Getenv("UNBOUND_PROVIDER_HELPER") != "1" {
		return
	}
	switch os.Getenv("UNBOUND_PROVIDER_HELPER_MODE") {
	case "FAST_EXIT":
		return
	case "UNEXPECTED_EXIT":
		time.Sleep(25 * time.Millisecond)
		os.Exit(23)
	default:
		time.Sleep(time.Hour)
	}
}

type lifecycleTestProvider struct {
	*Zapret2WindowsProvider
	mu         sync.Mutex
	spawnCount int
	spawnedAt  []time.Time
	mode       string
}

func newLifecycleTestProvider(t *testing.T, mode string) *lifecycleTestProvider {
	t.Helper()
	binDir := t.TempDir()
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	enginePath := filepath.Join(binDir, "winws2.exe")
	source, err := os.Open(testBinary)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	target, err := os.Create(enginePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(target, source); err != nil {
		target.Close()
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}
	engine, err := os.ReadFile(enginePath)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(engine)
	provider := NewZapret2WindowsProvider(binDir, binDir, binDir, hex.EncodeToString(hash[:]), false, false)
	fixture := &lifecycleTestProvider{Zapret2WindowsProvider: provider, mode: mode}
	provider.RegisterProfile("A", nil)
	provider.RegisterProfile("B", nil)
	provider.checkPrivileges = func() (bool, error) { return true, nil }
	provider.syncHostlists = func() error { return nil }
	provider.versionProbe = func(string) string { return "test engine" }
	provider.stopTimeout = 2 * time.Second
	provider.commandFactory = func(_ string, _ ...string) *exec.Cmd {
		fixture.mu.Lock()
		fixture.spawnCount++
		fixture.spawnedAt = append(fixture.spawnedAt, time.Now())
		fixture.mu.Unlock()
		cmd := exec.Command(testBinary, "-test.run=^TestZapret2WindowsProviderHelperProcess$")
		cmd.Env = append(os.Environ(), "UNBOUND_PROVIDER_HELPER=1", "UNBOUND_PROVIDER_HELPER_MODE="+fixture.mode)
		return cmd
	}
	provider.terminateTree = func(pid int) error {
		provider.mu.Lock()
		cmd := provider.cmd
		provider.mu.Unlock()
		if cmd == nil || cmd.Process == nil || cmd.Process.Pid != pid {
			return errors.New("test helper PID is no longer owned")
		}
		return cmd.Process.Kill()
	}
	return fixture
}

func (p *lifecycleTestProvider) spawns() (int, []time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.spawnCount, append([]time.Time(nil), p.spawnedAt...)
}

func (p *lifecycleTestProvider) snapshot() (Status, string, *exec.Cmd, int) {
	p.Zapret2WindowsProvider.mu.Lock()
	defer p.Zapret2WindowsProvider.mu.Unlock()
	return p.status, p.currentProfile, p.cmd, p.ownedPID
}

func requireRunningOwnedChild(t *testing.T, p *lifecycleTestProvider, profile string) int {
	t.Helper()
	status, currentProfile, cmd, pid := p.snapshot()
	if status != StatusRunning || currentProfile != profile || cmd == nil || pid <= 0 || !isWindowsPIDAlive(pid) {
		t.Fatalf("running invariant violated: status=%s profile=%q cmd=%v pid=%d alive=%t", status, currentProfile, cmd != nil, pid, isWindowsPIDAlive(pid))
	}
	return pid
}

func requireStoppedChild(t *testing.T, p *lifecycleTestProvider, oldPID int) {
	t.Helper()
	status, profile, cmd, pid := p.snapshot()
	if status != StatusStopped || profile != "" || cmd != nil || pid != 0 || isWindowsPIDAlive(oldPID) {
		t.Fatalf("stopped invariant violated: status=%s profile=%q cmd=%v pid=%d oldPIDAlive=%t", status, profile, cmd != nil, pid, isWindowsPIDAlive(oldPID))
	}
}

func waitForProviderStopped(t *testing.T, p *lifecycleTestProvider, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, profile, cmd, pid := p.snapshot()
		if status == StatusStopped && profile == "" && cmd == nil && pid == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("provider did not converge to stopped state")
}

func TestZapret2WindowsProviderSerialLifecycleStress(t *testing.T) {
	p := newLifecycleTestProvider(t, "NORMAL")
	for i := range 55 {
		if err := p.Start(context.Background(), "A"); err != nil {
			t.Fatalf("cycle %d start A: %v", i, err)
		}
		aPID := requireRunningOwnedChild(t, p, "A")
		if err := p.Stop(); err != nil {
			t.Fatalf("cycle %d stop A: %v", i, err)
		}
		requireStoppedChild(t, p, aPID)
		if err := p.Start(context.Background(), "B"); err != nil {
			t.Fatalf("cycle %d start B: %v", i, err)
		}
		bPID := requireRunningOwnedChild(t, p, "B")
		if err := p.Stop(); err != nil {
			t.Fatalf("cycle %d stop B: %v", i, err)
		}
		requireStoppedChild(t, p, bPID)
	}
	spawns, _ := p.spawns()
	if spawns != 110 {
		t.Fatalf("expected one owned child per serial start, got %d", spawns)
	}
}

func TestZapret2WindowsProviderConcurrentSameProfileStart(t *testing.T) {
	p := newLifecycleTestProvider(t, "NORMAL")
	errs := make(chan error, 5)
	for range 5 {
		go func() { errs <- p.Start(context.Background(), "A") }()
	}
	for range 5 {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent Start(A): %v", err)
		}
	}
	pid := requireRunningOwnedChild(t, p, "A")
	if spawns, _ := p.spawns(); spawns != 1 {
		t.Fatalf("concurrent Start(A) spawned %d children", spawns)
	}
	if err := p.Stop(); err != nil {
		t.Fatal(err)
	}
	requireStoppedChild(t, p, pid)
}

func TestZapret2WindowsProviderDelayedStopOrdersNextStart(t *testing.T) {
	p := newLifecycleTestProvider(t, "DELAYED_STOP")
	if err := p.Start(context.Background(), "A"); err != nil {
		t.Fatal(err)
	}
	aPID := requireRunningOwnedChild(t, p, "A")
	terminateCalled := make(chan struct{})
	allowTerminate := make(chan struct{})
	aExited := make(chan time.Time, 1)
	p.terminateTree = func(pid int) error {
		close(terminateCalled)
		<-allowTerminate
		p.Zapret2WindowsProvider.mu.Lock()
		cmd := p.cmd
		p.Zapret2WindowsProvider.mu.Unlock()
		if cmd == nil || cmd.Process == nil || cmd.Process.Pid != pid {
			return errors.New("test helper PID is no longer owned")
		}
		if err := cmd.Process.Kill(); err != nil {
			return err
		}
		for isWindowsPIDAlive(pid) {
			time.Sleep(5 * time.Millisecond)
		}
		aExited <- time.Now()
		return nil
	}
	stopResult := make(chan error, 1)
	go func() { stopResult <- p.Stop() }()
	<-terminateCalled
	startAttempted := make(chan struct{})
	startResult := make(chan error, 1)
	go func() {
		close(startAttempted)
		startResult <- p.Start(context.Background(), "B")
	}()
	<-startAttempted
	close(allowTerminate)
	if err := <-stopResult; err != nil {
		t.Fatal(err)
	}
	stopReturned := time.Now()
	exitAt := <-aExited
	if exitAt.After(stopReturned) {
		t.Fatalf("A PID exit %s followed Stop return %s", exitAt, stopReturned)
	}
	if err := <-startResult; err != nil {
		t.Fatal(err)
	}
	p.terminateTree = func(pid int) error {
		p.Zapret2WindowsProvider.mu.Lock()
		cmd := p.cmd
		p.Zapret2WindowsProvider.mu.Unlock()
		if cmd == nil || cmd.Process == nil || cmd.Process.Pid != pid {
			return errors.New("test helper PID is no longer owned")
		}
		return cmd.Process.Kill()
	}
	bPID := requireRunningOwnedChild(t, p, "B")
	_, spawnedAt := p.spawns()
	if len(spawnedAt) != 2 || !exitAt.Before(spawnedAt[1]) {
		t.Fatalf("B spawn did not follow A PID exit: exit=%s spawns=%v", exitAt, spawnedAt)
	}
	if err := p.Stop(); err != nil {
		t.Fatal(err)
	}
	requireStoppedChild(t, p, bPID)
	if isWindowsPIDAlive(aPID) {
		t.Fatal("A remained alive after successful Stop")
	}
}

func TestZapret2WindowsProviderUnexpectedExitRecovers(t *testing.T) {
	p := newLifecycleTestProvider(t, "UNEXPECTED_EXIT")
	if err := p.Start(context.Background(), "A"); err != nil {
		t.Fatal(err)
	}
	waitForProviderStopped(t, p, 2*time.Second)
	p.mode = "NORMAL"
	if err := p.Start(context.Background(), "B"); err != nil {
		t.Fatal(err)
	}
	pid := requireRunningOwnedChild(t, p, "B")
	if err := p.Stop(); err != nil {
		t.Fatal(err)
	}
	requireStoppedChild(t, p, pid)
}

func TestZapret2WindowsProviderStopTimeoutPreventsSecondChild(t *testing.T) {
	p := newLifecycleTestProvider(t, "STOP_TIMEOUT")
	p.stopTimeout = 50 * time.Millisecond
	if err := p.Start(context.Background(), "A"); err != nil {
		t.Fatal(err)
	}
	aPID := requireRunningOwnedChild(t, p, "A")
	p.terminateTree = func(int) error { return nil }
	if err := p.Stop(); err == nil {
		t.Fatal("Stop unexpectedly succeeded while the helper remained alive")
	}
	status, profile, cmd, pid := p.snapshot()
	if status != StatusError || profile != "A" || cmd == nil || pid != aPID || !isWindowsPIDAlive(aPID) {
		t.Fatalf("timeout lost owned child state: status=%s profile=%q cmd=%v pid=%d alive=%t", status, profile, cmd != nil, pid, isWindowsPIDAlive(aPID))
	}
	if err := p.Start(context.Background(), "B"); err == nil {
		t.Fatal("Start(B) succeeded while the owned A PID was alive")
	}
	if spawns, _ := p.spawns(); spawns != 1 {
		t.Fatalf("live owned PID allowed a second child: spawns=%d", spawns)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	waitForProviderStopped(t, p, 2*time.Second)
	requireStoppedChild(t, p, aPID)
}

func TestZapret2WindowsProviderCancellationLeaksNoChild(t *testing.T) {
	p := newLifecycleTestProvider(t, "NORMAL")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.Start(ctx, "A"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled start error = %v, want context.Canceled", err)
	}
	if spawns, _ := p.spawns(); spawns != 0 {
		t.Fatalf("cancelled Start spawned %d children", spawns)
	}
	requireStoppedChild(t, p, 0)
}

func TestZapret2WindowsProviderFastExitConverges(t *testing.T) {
	p := newLifecycleTestProvider(t, "FAST_EXIT")
	if err := p.Start(context.Background(), "A"); err != nil {
		t.Fatal(err)
	}
	waitForProviderStopped(t, p, 2*time.Second)
	if spawns, _ := p.spawns(); spawns != 1 {
		t.Fatalf("fast-exit fixture spawned %d children", spawns)
	}
}

func TestZapret2WindowsProviderProfileSwitchAbortsOnStopFailure(t *testing.T) {
	p := newLifecycleTestProvider(t, "NORMAL")
	if err := p.Start(context.Background(), "A"); err != nil {
		t.Fatal(err)
	}
	aPID := requireRunningOwnedChild(t, p, "A")
	p.terminateTree = func(int) error { return errors.New("simulated termination failure") }
	if err := p.Start(context.Background(), "B"); err == nil {
		t.Fatal("profile switch started B despite failure to stop live A")
	}
	if spawns, _ := p.spawns(); spawns != 1 {
		t.Fatalf("failed profile switch spawned %d children", spawns)
	}
	status, profile, cmd, pid := p.snapshot()
	if status != StatusError || profile != "A" || cmd == nil || pid != aPID || !isWindowsPIDAlive(aPID) {
		t.Fatalf("failed profile switch lost live ownership: status=%s profile=%q cmd=%v pid=%d alive=%t", status, profile, cmd != nil, pid, isWindowsPIDAlive(aPID))
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	waitForProviderStopped(t, p, 2*time.Second)
	requireStoppedChild(t, p, aPID)
}

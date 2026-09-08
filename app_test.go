package main

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unbound/engine"
)

func TestGetBypassLists(t *testing.T) {
	app := NewApp()
	lists := app.GetBypassLists()
	expected := []string{"youtube.txt", "discord.txt", "other.txt", "ipset-exclude.txt"}

	if len(lists) != len(expected) {
		t.Fatalf("expected %d lists, got %d", len(expected), len(lists))
	}

	for i, name := range expected {
		if lists[i] != name {
			t.Errorf("expected list %d to be %s, got %s", i, name, lists[i])
		}
	}
}

func TestReadBypassList_InvalidPath(t *testing.T) {
	app := NewApp()
	_, err := app.ReadBypassList("../etc/passwd")
	if err == nil {
		t.Error("expected error for directory traversal attempt, got nil")
	} else if !strings.Contains(err.Error(), "file access denied") {
		t.Errorf("expected 'file access denied' error, got: %v", err)
	}
}

func TestSaveBypassList_InvalidPath(t *testing.T) {
	app := NewApp()
	err := app.SaveBypassList("../etc/passwd", "malicious content")
	if err == nil {
		t.Error("expected error for directory traversal attempt, got nil")
	} else if !strings.Contains(err.Error(), "file access denied") {
		t.Errorf("expected 'file access denied' error, got: %v", err)
	}
}

func TestReadSaveBypassList_Success(t *testing.T) {
	app := NewApp()

	// Create mock config/lists dir for testing
	listsDir, err := engine.GetListsDir()
	if err != nil {
		t.Fatalf("failed to get lists dir: %v", err)
	}

	testFileName := "other.txt"
	originalPath := filepath.Join(listsDir, testFileName)

	// Backup original content if it exists
	var backup []byte
	backupExists := false
	if _, err := os.Stat(originalPath); err == nil {
		backup, _ = os.ReadFile(originalPath)
		backupExists = true
	}

	testContent := "example-domain.com\nanother-example.net"
	err = app.SaveBypassList(testFileName, testContent)
	if err != nil {
		t.Fatalf("failed to save bypass list: %v", err)
	}

	readContent, err := app.ReadBypassList(testFileName)
	if err != nil {
		t.Fatalf("failed to read bypass list: %v", err)
	}

	if readContent != testContent {
		t.Errorf("expected read content %q, got %q", testContent, readContent)
	}

	// Restore backup or cleanup
	if backupExists {
		os.WriteFile(originalPath, backup, 0644)
	} else {
		os.Remove(originalPath)
	}
}

func TestSecureDNS_Verification(t *testing.T) {
	app := NewApp()
	app.ctx = context.Background()

	// Just call IsSecureDNSEnabled to verify it doesn't panic
	// (Actual setting of secure DNS requires admin privileges and modify system network state, so we won't execute it to avoid breaking user's DNS during test)
	_ = app.IsSecureDNSEnabled()
}

func TestAutoReconnectDoesNotStartDuringAutoTuneOrShutdown(t *testing.T) {
	for _, setup := range []struct {
		name string
		run  func(*App)
	}{
		{name: "AutoTune active", run: func(app *App) { app.autoTuneCancel = func() {} }},
		{name: "shutdown active", run: func(app *App) { app.closing = true }},
	} {
		t.Run(setup.name, func(t *testing.T) {
			app := NewApp()
			app.ctx = context.Background()
			setup.run(app)
			app.AutoReconnectMonitor()
			if app.autoReconnectCancel != nil || app.autoReconnectID != 0 {
				t.Fatal("auto-reconnect monitor started during an exclusive lifecycle")
			}
		})
	}
}

func TestRequiresElevationForMode(t *testing.T) {
	tests := []struct {
		name                                                                                                                      string
		showVersion, testMode, listProfiles, cliMode, autoTuneMode, installService, uninstallService, controlMode, acceptanceTest bool
		want                                                                                                                      bool
	}{
		{name: "GUI", want: true},
		{name: "acceptance test", acceptanceTest: true, want: true},
		{name: "version", showVersion: true},
		{name: "diagnostic probe", testMode: true},
		{name: "profile catalog", listProfiles: true},
		{name: "CLI", cliMode: true},
		{name: "AutoTune CLI", autoTuneMode: true},
		{name: "service install", installService: true},
		{name: "service uninstall", uninstallService: true},
		{name: "control center", controlMode: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := requiresElevationForMode(
				test.showVersion,
				test.testMode,
				test.listProfiles,
				test.cliMode,
				test.autoTuneMode,
				test.installService,
				test.uninstallService,
				test.controlMode,
				test.acceptanceTest,
			)
			if got != test.want {
				t.Fatalf("requiresElevationForMode() = %v, want %v", got, test.want)
			}
		})
	}
}

type mockAcceptanceProcess struct {
	state   engine.CandidateProcessState
	alive   bool
	stopErr error
}

func (p *mockAcceptanceProcess) PID() int                            { return 77777 }
func (p *mockAcceptanceProcess) Argv() []string                      { return []string{"--acceptance"} }
func (p *mockAcceptanceProcess) State() engine.CandidateProcessState { return p.state }
func (p *mockAcceptanceProcess) Alive() bool                         { return p.alive }
func (p *mockAcceptanceProcess) WaitErr() error                      { return errors.New("simulated exit") }
func (p *mockAcceptanceProcess) Stop() error {
	if p.stopErr == nil {
		p.alive = false
	}
	return p.stopErr
}

type mockAcceptanceRunner struct {
	proc     *mockAcceptanceProcess
	startErr error
}

func (r *mockAcceptanceRunner) StartCandidate(ctx context.Context, cand engine.StrategyCandidate, rawFilter string) (engine.CandidateProcess, error) {
	if r.startErr != nil {
		return nil, r.startErr
	}
	return r.proc, nil
}

func TestExecuteAcceptanceProbeFailures(t *testing.T) {
	cand := engine.StrategyCandidate{
		ID:          "acceptance_cand",
		Name:        "Acceptance HostFakeSplit",
		Protocol:    "TLS1.3",
		Zapret2Args: []string{"--payload=tls_client_hello", "--lua-desync=hostfakesplit:repeats=2"},
	}

	// 1. Capture not ready failure
	t.Run("CaptureNotReady", func(t *testing.T) {
		runner := &mockAcceptanceRunner{
			proc: &mockAcceptanceProcess{
				state: engine.ProcessStateStarting, // not CAPTURE_READY
				alive: true,
			},
		}
		err := executeAcceptanceProbe(context.Background(), runner, cand, "dummy_filter", net.ParseIP("1.1.1.1"))
		if err == nil || !strings.Contains(err.Error(), "CAPTURE_READY: FAIL") {
			t.Fatalf("Expected CAPTURE_READY: FAIL, got %v", err)
		}
	})

	// 2. TLS Probe Failure (pinned to local IP without port 443 listener)
	t.Run("TLSProbeFailure", func(t *testing.T) {
		runner := &mockAcceptanceRunner{
			proc: &mockAcceptanceProcess{
				state: engine.ProcessStateCaptureReady,
				alive: true,
			},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		err := executeAcceptanceProbe(ctx, runner, cand, "dummy_filter", net.ParseIP("127.0.0.1"))
		if err == nil || !strings.Contains(err.Error(), "TLS handshake: FAIL") {
			t.Fatalf("Expected TLS handshake: FAIL when probe fails, got %v", err)
		}
	})

	// 3. Process death during handshake
	t.Run("ProcessDiedDuringProbe", func(t *testing.T) {
		runner := &mockAcceptanceRunner{
			proc: &mockAcceptanceProcess{
				state: engine.ProcessStateCaptureReady,
				alive: false, // process died
			},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		err := executeAcceptanceProbe(ctx, runner, cand, "dummy_filter", net.ParseIP("192.0.2.1"))
		// Either TLS failed or Process died reported
		if err == nil {
			t.Fatal("Expected error when process dies or probe fails, got nil")
		}
	})

	// 4. Process stop failure
	t.Run("ProcessStopFailure", func(t *testing.T) {
		runner := &mockAcceptanceRunner{
			proc: &mockAcceptanceProcess{
				state:   engine.ProcessStateCaptureReady,
				alive:   true,
				stopErr: errors.New("cannot release WinDivert handle"),
			},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		// Probe will fail first on unreachable IP, which is expected
		err := executeAcceptanceProbe(ctx, runner, cand, "dummy_filter", net.ParseIP("192.0.2.1"))
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
	})
}

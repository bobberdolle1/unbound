package engine

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
	"unbound/engine/providers"
)

func TestGetBlockCheck2Candidates(t *testing.T) {
	tlsCandidates := GetBlockCheck2Candidates("TLS1.3")
	if len(tlsCandidates) == 0 {
		t.Fatal("Expected TLS 1.3 candidates, got none")
	}

	httpCandidates := GetBlockCheck2Candidates("HTTP")
	if len(httpCandidates) == 0 {
		t.Fatal("Expected HTTP candidates, got none")
	}

	quicCandidates := GetBlockCheck2Candidates("QUIC")
	if len(quicCandidates) == 0 {
		t.Fatal("Expected QUIC candidates, got none")
	}

	// Verify aggressiveness levels are assigned
	for _, c := range tlsCandidates {
		if c.Aggressiveness < AggressivenessLow || c.Aggressiveness > AggressivenessExperimental {
			t.Errorf("Candidate %s has invalid aggressiveness: %v", c.Name, c.Aggressiveness)
		}
		if len(c.Zapret2Args) == 0 {
			t.Errorf("Candidate %s has empty Zapret2Args", c.Name)
		}
	}
}

func TestStrategyLabCandidateRanking(t *testing.T) {
	results := []CandidateTestResult{
		{
			Candidate:     StrategyCandidate{Name: "HighAggPass", Aggressiveness: AggressivenessHigh},
			PassCount:     3,
			TotalAttempts: 3,
			AvgLatency:    50 * time.Millisecond,
		},
		{
			Candidate:     StrategyCandidate{Name: "LowAggPass", Aggressiveness: AggressivenessLow},
			PassCount:     3,
			TotalAttempts: 3,
			AvgLatency:    60 * time.Millisecond,
		},
		{
			Candidate:     StrategyCandidate{Name: "PartialPass", Aggressiveness: AggressivenessLow},
			PassCount:     2,
			TotalAttempts: 3,
			AvgLatency:    40 * time.Millisecond,
		},
	}

	sort.Slice(results, func(i, j int) bool {
		ci := results[i]
		cj := results[j]

		if ci.PassCount != cj.PassCount {
			return ci.PassCount > cj.PassCount
		}
		if ci.Candidate.Aggressiveness != cj.Candidate.Aggressiveness {
			return ci.Candidate.Aggressiveness < cj.Candidate.Aggressiveness
		}
		return ci.AvgLatency < cj.AvgLatency
	})

	// Winner must be LowAggPass (3/3 passes, Low Aggressiveness beats High Aggressiveness)
	if results[0].Candidate.Name != "LowAggPass" {
		t.Errorf("Winner = %s; want LowAggPass", results[0].Candidate.Name)
	}
	if results[1].Candidate.Name != "HighAggPass" {
		t.Errorf("Second = %s; want HighAggPass", results[1].Candidate.Name)
	}
	if results[2].Candidate.Name != "PartialPass" {
		t.Errorf("Third = %s; want PartialPass", results[2].Candidate.Name)
	}
}

func TestRunStrategyLabBaselineReachableEarlyExit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	mock := &mockProviderController{
		profile: "Recommended (hostfakesplit)",
		status:  providers.StatusRunning,
	}

	cleanHost := strings.TrimPrefix(ts.URL, "http://")
	cfg := StrategyLabTargetConfig{
		TargetHost:            cleanHost,
		Protocol:              "HTTP",
		ForceProbeIfReachable: false,
	}

	report, err := RunStrategyLab(context.Background(), mock, cfg, nil)
	if err != nil {
		t.Fatalf("RunStrategyLab failed: %v", err)
	}

	if !report.BaselineReachable {
		t.Error("Expected baseline to be reachable on test server")
	}
	if report.TestedCandidates != 0 {
		t.Errorf("Expected 0 tested candidates on early exit, got %d", report.TestedCandidates)
	}

	// Verify profile was restored
	if mock.status != providers.StatusRunning || mock.profile != "Recommended (hostfakesplit)" {
		t.Errorf("Profile not restored after early exit: status=%v, profile=%s", mock.status, mock.profile)
	}
}

func TestSaveDiscoveredProfileJSON(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "discovered_profiles.json")
	cleanup := SetDiscoveredProfilesPathForTest(tempFile)
	defer cleanup()

	cand := StrategyCandidate{
		ID:             "cand_test",
		Name:           "Test Multisplit",
		Protocol:       "TLS1.3",
		Zapret2Args:    []string{"--payload=tls_client_hello", "--lua-desync=multisplit:pos=midsld"},
		Aggressiveness: AggressivenessLow,
		Source:         "test",
	}

	err := SaveDiscoveredProfile("My Discovered Profile", cand, "example.com")
	if err != nil {
		t.Fatalf("SaveDiscoveredProfile failed: %v", err)
	}

	profs, err := LoadDiscoveredProfiles()
	if err != nil {
		t.Fatalf("LoadDiscoveredProfiles failed: %v", err)
	}
	if len(profs) != 1 {
		t.Fatalf("Expected 1 profile, got %d", len(profs))
	}
	if profs[0].Name != "My Discovered Profile" {
		t.Errorf("Name = %s; want My Discovered Profile", profs[0].Name)
	}
	if profs[0].Target != "example.com" {
		t.Errorf("Target = %s; want example.com", profs[0].Target)
	}
}

func TestStrategyLabRunnerFilterInvariant(t *testing.T) {
	runner := &DefaultCandidateRunner{
		assets: &AssetPaths{},
	}
	cand := StrategyCandidate{
		Name:        "Test",
		Zapret2Args: []string{"--lua-desync=fake"},
	}
	_, err := runner.StartCandidate(context.Background(), cand, "")
	if err == nil {
		t.Fatal("Expected error when starting candidate with empty raw filter, got nil")
	}
	if !strings.Contains(err.Error(), "rawFilter cannot be empty") {
		t.Errorf("Expected rawFilter invariant error, got: %v", err)
	}
}

type recordingCandidateRunner struct {
	startedCandidates []string
	stoppedCandidates []string
	activeCount       int
	maxConcurrent     int
}

type dummyProcess struct {
	name   string
	runner *recordingCandidateRunner
}

func (d *dummyProcess) PID() int                     { return 12345 }
func (d *dummyProcess) Argv() []string               { return []string{"--dummy"} }
func (d *dummyProcess) State() CandidateProcessState { return ProcessStateCaptureReady }
func (d *dummyProcess) Alive() bool                  { return true }
func (d *dummyProcess) WaitErr() error               { return nil }
func (d *dummyProcess) Stop() error {
	d.runner.stoppedCandidates = append(d.runner.stoppedCandidates, d.name)
	d.runner.activeCount--
	return nil
}

func (r *recordingCandidateRunner) StartCandidate(ctx context.Context, cand StrategyCandidate, rawFilter string) (CandidateProcess, error) {
	if rawFilter == "" {
		return nil, errors.New("rawFilter invariant violation")
	}
	r.startedCandidates = append(r.startedCandidates, cand.Name)
	r.activeCount++
	if r.activeCount > r.maxConcurrent {
		r.maxConcurrent = r.activeCount
	}
	return &dummyProcess{name: cand.Name, runner: r}, nil
}

func TestStrategyLabExecutionWithRunner(t *testing.T) {
	mock := &mockProviderController{
		profile: "Recommended (hostfakesplit)",
		status:  providers.StatusRunning,
	}

	runner := &recordingCandidateRunner{}
	cfg := StrategyLabTargetConfig{
		TargetHost:            "127.0.0.1",
		Protocol:              "HTTP",
		ForceProbeIfReachable: true,
		CustomCandidateArgs:   []string{"--filter-tcp=80", "--lua-desync=fake"},
	}

	report, err := RunStrategyLabWithRunner(context.Background(), mock, runner, cfg, nil)
	if err != nil {
		t.Fatalf("RunStrategyLabWithRunner failed: %v", err)
	}

	if report == nil {
		t.Fatal("Expected report, got nil")
	}

	// Invariant: At no point should multiple candidate winws2 processes run concurrently
	if runner.maxConcurrent > 1 {
		t.Errorf("Isolation violation: maxConcurrent processes was %d, want <= 1", runner.maxConcurrent)
	}

	// Invariant: Every started candidate process must have been stopped
	if len(runner.startedCandidates) != len(runner.stoppedCandidates) {
		t.Errorf("Leaked processes: started %d, stopped %d", len(runner.startedCandidates), len(runner.stoppedCandidates))
	}

	// Invariant: Baseline profile must be restored
	if mock.profile != "Recommended (hostfakesplit)" || mock.status != providers.StatusRunning {
		t.Errorf("Baseline profile not restored: %s (%v)", mock.profile, mock.status)
	}
}

func TestSanitizeCandidateArgs(t *testing.T) {
	validArgs := []string{
		"--payload=tls_client_hello",
		"--lua-desync=hostfakesplit:midhost=midsld:repeats=2",
		"--filter-tcp=443",
	}
	sanitized, err := SanitizeCandidateArgs(validArgs)
	if err != nil {
		t.Fatalf("Expected valid args to pass, got error: %v", err)
	}
	if len(sanitized) != 3 {
		t.Errorf("Expected 3 sanitized args, got %d", len(sanitized))
	}

	forbiddenCases := [][]string{
		{"--wf-raw=true"},
		{"--wf-tcp-out=443"},
		{"--daemon"},
		{"--pidfile=test.pid"},
		{"--lua-init=malicious.lua"},
		{"--intercept=0"},
	}

	for _, forbidden := range forbiddenCases {
		_, err := SanitizeCandidateArgs(forbidden)
		if err == nil {
			t.Errorf("Expected forbidden arg %v to be rejected, but it passed", forbidden)
		}
	}
}

func TestGetServiceValidationEndpoints(t *testing.T) {
	yt := GetServiceValidationEndpoints("youtube")
	if len(yt) != 2 || yt[0] != "www.youtube.com" || yt[1] != "i.ytimg.com" {
		t.Errorf("Unexpected youtube endpoints: %v", yt)
	}

	dc := GetServiceValidationEndpoints("discord")
	if len(dc) != 2 || dc[0] != "discord.com" || dc[1] != "gateway.discord.gg" {
		t.Errorf("Unexpected discord endpoints: %v", dc)
	}

	steam := GetServiceValidationEndpoints("steam")
	if len(steam) != 2 || steam[0] != "store.steampowered.com" || steam[1] != "api.steampowered.com" {
		t.Errorf("Unexpected steam endpoints: %v", steam)
	}

	unknown := GetServiceValidationEndpoints("nonexistent")
	if unknown != nil {
		t.Errorf("Expected nil for unknown service, got: %v", unknown)
	}
}

func TestBuildDiscoveredProfileRuntimeArgs(t *testing.T) {
	tempLists := t.TempDir()

	// 1. TLS profile
	tlsProf := DiscoveredProfile{
		Name:          "Test TLS",
		Target:        "https://youtube.com/watch?v=123",
		Protocol:      "TLS1.3",
		CandidateArgs: []string{"--lua-desync=hostfakesplit:midhost=midsld:repeats=2"},
	}
	tlsArgs := BuildDiscoveredProfileRuntimeArgs(tlsProf, tempLists)
	joinedTLS := strings.Join(tlsArgs, " ")

	if !strings.Contains(joinedTLS, "--filter-tcp=443") {
		t.Errorf("Missing default --filter-tcp=443 in args: %v", tlsArgs)
	}
	if !strings.Contains(joinedTLS, "--hostlist-domains=youtube.com") {
		t.Errorf("Missing scoped --hostlist-domains=youtube.com in args: %v", tlsArgs)
	}
	if !strings.Contains(joinedTLS, "steam-web-exclude.txt") {
		t.Errorf("Missing steam exclusions in args: %v", tlsArgs)
	}

	// 2. HTTP profile
	httpProf := DiscoveredProfile{
		Name:          "Test HTTP",
		Target:        "rutracker.org",
		Protocol:      "HTTP",
		CandidateArgs: []string{"--lua-desync=multisplit:pos=midsld"},
	}
	httpArgs := BuildDiscoveredProfileRuntimeArgs(httpProf, tempLists)
	joinedHTTP := strings.Join(httpArgs, " ")
	if !strings.Contains(joinedHTTP, "--filter-tcp=80") {
		t.Errorf("Missing --filter-tcp=80 for HTTP protocol in args: %v", httpArgs)
	}
	if !strings.Contains(joinedHTTP, "--hostlist-domains=rutracker.org") {
		t.Errorf("Missing scoped --hostlist-domains=rutracker.org in args: %v", httpArgs)
	}

	// 3. QUIC profile
	quicProf := DiscoveredProfile{
		Name:          "Test QUIC",
		Target:        "youtube.com",
		Protocol:      "QUIC",
		CandidateArgs: []string{"--lua-desync=fake:repeats=2"},
	}
	quicArgs := BuildDiscoveredProfileRuntimeArgs(quicProf, tempLists)
	joinedQUIC := strings.Join(quicArgs, " ")
	if !strings.Contains(joinedQUIC, "--filter-udp=443") {
		t.Errorf("Missing --filter-udp=443 for QUIC protocol in args: %v", quicArgs)
	}
}

func TestBuildDiscoveredProfileRuntimeArgsMultiSection(t *testing.T) {
	tempLists := t.TempDir()

	multiProf := DiscoveredProfile{
		Name:     "Multi-Section Candidate",
		Target:   "discord.com",
		Protocol: "TLS1.3",
		CandidateArgs: []string{
			"--filter-tcp=80",
			"--lua-desync=multisplit",
			"--new",
			"--payload=tls_client_hello",
			"--lua-desync=fake:repeats=2",
		},
	}

	args := BuildDiscoveredProfileRuntimeArgs(multiProf, tempLists)
	joined := strings.Join(args, " ")

	// Invariant: Both sections must be scoped to discord.com
	scopeCount := strings.Count(joined, "--hostlist-domains=discord.com")
	if scopeCount != 2 {
		t.Errorf("Expected 2 scoped --hostlist-domains=discord.com across sections, got %d in: %s", scopeCount, joined)
	}

	// Invariant: Second section without explicit transport must get default --filter-tcp=443
	if !strings.Contains(joined, "--filter-tcp=443") {
		t.Errorf("Expected default --filter-tcp=443 in second section: %s", joined)
	}
}

func TestStrategyLabBaselineProtocolCorrectness(t *testing.T) {
	mockPC := &mockProviderController{
		profile: "Recommended (hostfakesplit)",
		status:  providers.StatusRunning,
	}

	// 1. HTTP PASS + QUIC FAIL must NOT early-exit QUIC Strategy Lab
	t.Run("HTTPPassQUICFailDoesNotEarlyExitQUICLab", func(t *testing.T) {
		if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
			t.Skip("QUIC candidate testing requires WinDivert (Windows) or NFQUEUE (Linux)")
		}
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		u, _ := neturl.Parse(ts.URL)
		cfg := StrategyLabTargetConfig{
			TargetHost:    u.Host,
			Protocol:      "QUIC",
			ServicePreset: "Custom",
		}

		runner := &recordingCandidateRunner{}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		report, err := RunStrategyLabWithRunner(ctx, mockPC, runner, cfg, nil)
		if err != nil {
			t.Fatalf("RunStrategyLabWithRunner failed: %v", err)
		}

		if report.BaselineReachable {
			t.Errorf("Expected QUIC baseline to FAIL because no QUIC server is running, got reachable=true")
		}
		if report.BaselineProtocolLabel != "Baseline QUIC" {
			t.Errorf("Expected BaselineProtocolLabel='Baseline QUIC', got %q", report.BaselineProtocolLabel)
		}
		if len(runner.startedCandidates) == 0 {
			t.Errorf("QUIC discovery exited early even though QUIC baseline was unreachable")
		}
	})

	// 2. TLS 1.3 PASS + TLS 1.2 FAIL must NOT early-exit TLS 1.2 Strategy Lab
	t.Run("TLS13PassTLS12FailDoesNotEarlyExitTLS12Lab", func(t *testing.T) {
		ts := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		ts.TLS = &tls.Config{
			MinVersion: tls.VersionTLS13,
			MaxVersion: tls.VersionTLS13,
		}
		ts.StartTLS()
		defer ts.Close()

		u, _ := neturl.Parse(ts.URL)
		cfg := StrategyLabTargetConfig{
			TargetHost:    u.Host,
			Protocol:      "TLS1.2",
			ServicePreset: "Custom",
		}
		runner := &recordingCandidateRunner{}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		report, err := RunStrategyLabWithRunner(ctx, mockPC, runner, cfg, nil)
		if err != nil {
			t.Fatalf("RunStrategyLabWithRunner failed: %v", err)
		}

		if report.BaselineReachable {
			t.Errorf("Expected TLS 1.2 baseline to FAIL against TLS 1.3-only server, got reachable=true")
		}
		if report.BaselineProtocolLabel != "Baseline TLS 1.2" {
			t.Errorf("Expected BaselineProtocolLabel='Baseline TLS 1.2', got %q", report.BaselineProtocolLabel)
		}
		if len(runner.startedCandidates) == 0 {
			t.Errorf("TLS 1.2 discovery exited early even though TLS 1.2 baseline was unreachable")
		}
	})
}

type dyingCandidateProcess struct {
	alive bool
}

func (d *dyingCandidateProcess) PID() int                     { return 99999 }
func (d *dyingCandidateProcess) Argv() []string               { return []string{"--dying"} }
func (d *dyingCandidateProcess) State() CandidateProcessState { return ProcessStateExitedEarly }
func (d *dyingCandidateProcess) Alive() bool                  { return d.alive }
func (d *dyingCandidateProcess) WaitErr() error               { return errors.New("signal: killed") }
func (d *dyingCandidateProcess) Stop() error                  { return nil }

func TestCandidateProcessLivenessDetection(t *testing.T) {
	ce := NewConnectivityEngine(time.Second)
	cand := StrategyCandidate{
		ID:          "cand_liveness_test",
		Name:        "Liveness Test Candidate",
		Protocol:    "HTTP",
		Zapret2Args: []string{"--dummy"},
	}

	// 1. Process already dead before attempt
	deadProc := &dyingCandidateProcess{alive: false}
	res := testCandidateWithProcess(context.Background(), ce, deadProc, cand, "http://127.0.0.1:80", "HTTP", false)
	if res.Status != StatusFail {
		t.Errorf("Expected FAIL when candidate is dead, got %s", res.Status)
	}
	if !strings.Contains(res.Error, "ENGINE_EXITED") {
		t.Errorf("Expected ENGINE_EXITED in error, got: %s", res.Error)
	}
}

type failingCandidateRunner struct{}

func (f *failingCandidateRunner) StartCandidate(ctx context.Context, cand StrategyCandidate, rawFilter string) (CandidateProcess, error) {
	return nil, errors.New("simulated candidate start failure")
}

func TestAllFailedCandidatesNeverBecomeBestCandidate(t *testing.T) {
	mockPC := &mockProviderController{
		profile: "Recommended (hostfakesplit)",
		status:  providers.StatusRunning,
	}

	cfg := StrategyLabTargetConfig{
		TargetHost:            "127.0.0.1",
		Protocol:              "HTTP",
		ServicePreset:         "Custom",
		ForceProbeIfReachable: true,
	}

	runner := &failingCandidateRunner{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	report, err := RunStrategyLabWithRunner(ctx, mockPC, runner, cfg, nil)
	if err != nil {
		t.Fatalf("RunStrategyLabWithRunner failed: %v", err)
	}

	if report.BestCandidate != nil {
		t.Fatalf("CRITICAL INTEGRITY REGRESSION: BestCandidate must be nil when all candidates fail, got: %s", report.BestCandidate.Candidate.Name)
	}
	if len(report.WorkingCandidates) != 0 {
		t.Errorf("Expected 0 WorkingCandidates, got %d", len(report.WorkingCandidates))
	}
	if len(report.CandidateResults) == 0 {
		t.Errorf("Expected CandidateResults to record all failed attempts, got 0")
	}
	if report.ValidationStatus != ValidationStatusNotVerified {
		t.Errorf("Expected ValidationStatus %s, got %s", ValidationStatusNotVerified, report.ValidationStatus)
	}
}

type selectiveCandidateRunner struct {
	passName string
}

func (s *selectiveCandidateRunner) StartCandidate(ctx context.Context, cand StrategyCandidate, rawFilter string) (CandidateProcess, error) {
	if cand.Name == s.passName {
		return &dummyProcess{name: cand.Name, runner: &recordingCandidateRunner{}}, nil
	}
	return nil, errors.New("candidate failed")
}

func TestMixedFailAndOnePassSelectsWorkingWinner(t *testing.T) {
	mockPC := &mockProviderController{
		profile: "Recommended (hostfakesplit)",
		status:  providers.StatusRunning,
	}

	// Run mock HTTP server so the passing candidate probe succeeds
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	u, _ := neturl.Parse(ts.URL)

	cfg := StrategyLabTargetConfig{
		TargetHost:            u.Host,
		Protocol:              "HTTP",
		ServicePreset:         "Custom",
		ForceProbeIfReachable: true,
		CustomCandidateArgs:   []string{"--filter-tcp=80", "--lua-desync=fake"},
	}
	runner := &selectiveCandidateRunner{passName: "Custom Strategy"}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	report, err := RunStrategyLabWithRunner(ctx, mockPC, runner, cfg, nil)
	if err != nil {
		t.Fatalf("RunStrategyLabWithRunner failed: %v", err)
	}

	for _, cr := range report.CandidateResults {
		t.Logf("Candidate %s: status=%s, passCount=%d, error=%s", cr.Candidate.Name, cr.Status, cr.PassCount, cr.Error)
	}

	if report.BestCandidate == nil {
		t.Fatal("Expected BestCandidate to be selected from working candidate, got nil")
	}
	if report.BestCandidate.Candidate.Name != "Custom Strategy" {
		t.Errorf("Expected 'Custom Strategy' as winner, got: %s", report.BestCandidate.Candidate.Name)
	}
	if len(report.WorkingCandidates) != 1 {
		t.Errorf("Expected exactly 1 working candidate, got %d", len(report.WorkingCandidates))
	}
	if report.BestCandidate.Candidate.TestedProtocol != "HTTP" {
		t.Errorf("Expected TestedProtocol='HTTP', got %q", report.BestCandidate.Candidate.TestedProtocol)
	}
}

func TestCandidateProcessWaitErrPreservedAfterAlive(t *testing.T) {
	done := make(chan struct{})
	expectedErr := errors.New("exit status 123")

	proc := &OSZapretCandidateProcess{
		pid:     112233,
		argv:    []string{"--test"},
		done:    done,
		waitErr: expectedErr,
		state:   ProcessStateRunning,
	}

	// Close done to simulate process exit
	close(done)

	// 1. Alive() must return false
	if proc.Alive() {
		t.Error("Expected Alive() to return false after process exit")
	}

	// 2. WaitErr() must return the original error
	err1 := proc.WaitErr()
	if err1 != expectedErr {
		t.Errorf("WaitErr() = %v, want %v", err1, expectedErr)
	}

	// 3. Repeated Alive() and WaitErr() calls must preserve the error without eating it
	if proc.Alive() {
		t.Error("Alive() returned true on second call")
	}
	err2 := proc.WaitErr()
	if err2 != expectedErr {
		t.Errorf("WaitErr() on second call = %v, want %v", err2, expectedErr)
	}
}

func TestSaveDiscoveredProfilePreservesTestedProtocol(t *testing.T) {
	tempFile := filepath.Join(t.TempDir(), "discovered_profiles_proto.json")
	cleanup := SetDiscoveredProfilesPathForTest(tempFile)
	defer cleanup()

	cand := StrategyCandidate{
		ID:             "cand_proto_test",
		Name:           "TLS Candidate Tested on TLS 1.2",
		Protocol:       "TLS1.3", // Catalog default
		TestedProtocol: "TLS1.2", // Actual tested protocol in Strategy Lab
		Zapret2Args:    []string{"--filter-tcp=443", "--lua-desync=fake"},
	}

	err := SaveDiscoveredProfile("TLS 1.2 Discovered", cand, "example.com")
	if err != nil {
		t.Fatalf("SaveDiscoveredProfile failed: %v", err)
	}

	profs, err := LoadDiscoveredProfiles()
	if err != nil {
		t.Fatalf("LoadDiscoveredProfiles failed: %v", err)
	}
	if len(profs) != 1 {
		t.Fatalf("Expected 1 profile, got %d", len(profs))
	}
	if profs[0].Protocol != "TLS1.2" {
		t.Errorf("Expected saved profile protocol to be 'TLS1.2', got %q", profs[0].Protocol)
	}
}

func TestCandidateProcessAliveRemainsTrueUntilDoneClosed(t *testing.T) {
	done := make(chan struct{})
	proc := &OSZapretCandidateProcess{
		pid:   12345,
		done:  done,
		state: ProcessStateRunning,
	}

	// Process is running: Alive() must be true
	if !proc.Alive() {
		t.Fatal("Expected Alive() to be true while done channel is open")
	}

	// Close done channel to signal actual OS process termination
	close(done)

	// Now Alive() must be false
	if proc.Alive() {
		t.Fatal("Expected Alive() to be false after done channel closed")
	}
}

type failingStopCandidateProcess struct {
	name string
	dead chan struct{}
}

func (p *failingStopCandidateProcess) PID() int                     { return 88888 }
func (p *failingStopCandidateProcess) Argv() []string               { return []string{"--fail-stop"} }
func (p *failingStopCandidateProcess) State() CandidateProcessState { return ProcessStateCaptureReady }
func (p *failingStopCandidateProcess) Alive() bool                  { return true }
func (p *failingStopCandidateProcess) WaitErr() error               { return nil }
func (p *failingStopCandidateProcess) Stop() error {
	return errors.New("simulated candidate stop failure: process hung")
}

type abortTrackingCandidateRunner struct {
	startedCandidates []string
	failCandidateName string
}

func (r *abortTrackingCandidateRunner) StartCandidate(ctx context.Context, cand StrategyCandidate, rawFilter string) (CandidateProcess, error) {
	r.startedCandidates = append(r.startedCandidates, cand.Name)
	if cand.Name == r.failCandidateName {
		return &failingStopCandidateProcess{name: cand.Name}, nil
	}
	return &dummyProcess{name: cand.Name, runner: &recordingCandidateRunner{}}, nil
}

func TestCandidateCleanupFailureAbortsNextCandidate(t *testing.T) {
	mockPC := &mockProviderController{
		status:  providers.StatusStopped,
		profile: "",
	}

	runner := &abortTrackingCandidateRunner{
		failCandidateName: "Custom Strategy", // Custom candidate will run first and fail on Stop()
	}

	cfg := StrategyLabTargetConfig{
		TargetHost:            "127.0.0.1",
		Protocol:              "HTTP",
		CustomCandidateArgs:   []string{"--filter-tcp=80", "--lua-desync=fake"},
		ForceProbeIfReachable: true,
	}

	report, err := RunStrategyLabWithRunner(context.Background(), mockPC, runner, cfg, nil)
	if err == nil {
		t.Fatal("Expected RunStrategyLabWithRunner to abort with error on cleanup failure, got nil")
	}
	if !strings.Contains(err.Error(), "cleanup failed") {
		t.Errorf("Expected error to contain 'cleanup failed', got: %v", err)
	}

	// Invariant: candidate N+1 must NEVER start after candidate N fails Stop()
	if len(runner.startedCandidates) != 1 {
		t.Fatalf("Expected exactly 1 candidate started before abort, got %d (%v)",
			len(runner.startedCandidates), runner.startedCandidates)
	}
	if report == nil {
		t.Fatal("Expected non-nil report even on abort")
	}
}

type failingStopProviderController struct {
	stopErr error
}

func (c *failingStopProviderController) CurrentProfile() string                          { return "Recommended" }
func (c *failingStopProviderController) GetStatus() providers.Status                     { return providers.StatusRunning }
func (c *failingStopProviderController) Start(ctx context.Context, profile string) error { return nil }
func (c *failingStopProviderController) Stop() error {
	return c.stopErr
}

func TestPreviousEngineStopFailureAbortsBaseline(t *testing.T) {
	failingPC := &failingStopProviderController{
		stopErr: errors.New("simulated driver lock error on Stop"),
	}

	runner := &recordingCandidateRunner{}
	cfg := StrategyLabTargetConfig{
		TargetHost: "127.0.0.1",
		Protocol:   "HTTP",
	}

	report, err := RunStrategyLabWithRunner(context.Background(), failingPC, runner, cfg, nil)
	if err == nil {
		t.Fatal("Expected RunStrategyLabWithRunner to abort when active engine fails to stop, got nil")
	}
	if !strings.Contains(err.Error(), "failed to stop active engine for clean baseline") {
		t.Errorf("Expected error message to specify engine stop failure, got: %v", err)
	}
	if report != nil {
		t.Errorf("Expected nil report on baseline abort, got %v", report)
	}
	if len(runner.startedCandidates) != 0 {
		t.Errorf("Expected 0 candidates started, got %d", len(runner.startedCandidates))
	}
}

type failingStartProviderController struct {
	startErr error
	running  bool
}

func (c *failingStartProviderController) CurrentProfile() string { return "Recommended" }
func (c *failingStartProviderController) GetStatus() providers.Status {
	if c.running {
		return providers.StatusRunning
	}
	return providers.StatusStopped
}
func (c *failingStartProviderController) Start(ctx context.Context, profile string) error {
	return c.startErr
}
func (c *failingStartProviderController) Stop() error {
	c.running = false
	return nil
}

func TestPreviousProfileRestorationFailureVisible(t *testing.T) {
	failingPC := &failingStartProviderController{
		startErr: errors.New("driver failed to rehook on restore"),
		running:  true,
	}

	runner := &recordingCandidateRunner{}
	cfg := StrategyLabTargetConfig{
		TargetHost:            "127.0.0.1",
		Protocol:              "HTTP",
		ForceProbeIfReachable: true,
	}

	report, err := RunStrategyLabWithRunner(context.Background(), failingPC, runner, cfg, nil)
	if err != nil {
		t.Fatalf("RunStrategyLabWithRunner failed: %v", err)
	}
	if report == nil {
		t.Fatal("Expected non-nil report")
	}
	if report.RestorationStatus != RestorationStatusFailed {
		t.Errorf("Expected RestorationStatus=FAILED, got %s", report.RestorationStatus)
	}
	if !strings.Contains(report.RestorationError, "driver failed to rehook on restore") {
		t.Errorf("Expected RestorationError to contain root cause, got: %s", report.RestorationError)
	}
}

func TestWinnerStopFailureMarksValidationFailed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	u, _ := neturl.Parse(ts.URL)

	mockPC := &mockProviderController{
		status:  providers.StatusStopped,
		profile: "",
	}

	callCount := 0
	winnerStopFailingRunner := &failingWinnerStopRunner{
		onStart: func(name string, filter string) CandidateProcess {
			callCount++
			if callCount > 3 {
				// Winner validation run
				return &failingStopCandidateProcess{name: name}
			}
			return &dummyProcess{name: name, runner: &recordingCandidateRunner{}}
		},
	}

	cfg := StrategyLabTargetConfig{
		TargetHost:            u.Host,
		Protocol:              "HTTP",
		CustomCandidateArgs:   []string{"--filter-tcp=80", "--lua-desync=fake"},
		ForceProbeIfReachable: true,
	}

	report, err := RunStrategyLabWithRunner(context.Background(), mockPC, winnerStopFailingRunner, cfg, nil)
	t.Logf("err=%v, report=%+v", err, report)
	if err == nil {
		t.Fatal("Expected error when winner stop fails, got nil")
	}
	if report != nil && report.ValidationStatus != ValidationStatusFailed {
		t.Errorf("Expected ValidationStatus=FAILED when winner cleanup fails, got %s", report.ValidationStatus)
	}
}

type failingWinnerStopRunner struct {
	onStart func(name string, filter string) CandidateProcess
}

func (r *failingWinnerStopRunner) StartCandidate(ctx context.Context, cand StrategyCandidate, rawFilter string) (CandidateProcess, error) {
	return r.onStart(cand.Name, rawFilter), nil
}

func TestANYModeQUICCandidateDoesNotPassFromTLSProbe(t *testing.T) {
	// Server answers HTTP and TLS, but NOT QUIC
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	u, _ := neturl.Parse(ts.URL)

	mockPC := &mockProviderController{
		status:  providers.StatusStopped,
		profile: "",
	}

	runner := &recordingCandidateRunner{}

	// In ANY mode, test candidate with QUIC protocol
	cfg := StrategyLabTargetConfig{
		TargetHost:            u.Host,
		Protocol:              "ANY",
		CustomCandidateArgs:   []string{"--payload=quic", "--lua-desync=fake"},
		ForceProbeIfReachable: true,
	}

	report, err := RunStrategyLabWithRunner(context.Background(), mockPC, runner, cfg, nil)
	if err != nil {
		t.Fatalf("RunStrategyLabWithRunner failed: %v", err)
	}
	if report == nil {
		t.Fatal("Expected report, got nil")
	}

	// Find the custom QUIC candidate in CandidateResults
	var quicResult *CandidateTestResult
	for i, cr := range report.CandidateResults {
		if cr.Candidate.Name == "Custom Strategy" {
			quicResult = &report.CandidateResults[i]
			break
		}
	}

	if quicResult == nil {
		t.Fatal("Expected Custom Strategy to be in CandidateResults")
	}

	// Invariant: QUIC candidate must be probed with QUIC (which fails), NOT with TLS (which passes)
	if quicResult.Candidate.TestedProtocol != "QUIC" {
		t.Errorf("Expected TestedProtocol='QUIC', got %q", quicResult.Candidate.TestedProtocol)
	}
	if quicResult.Status == StatusPass {
		t.Errorf("CRITICAL BUG: QUIC candidate falsely PASSED in ANY mode when QUIC was unreachable: %+v", quicResult)
	}
	if quicResult.ExecutionStatus == ExecutionStatusPass {
		t.Errorf("Expected ExecutionStatus != PASS, got %s", quicResult.ExecutionStatus)
	}
}

func TestCandidateProcessStopTimeoutReturnsError(t *testing.T) {
	// An unclosed done channel simulates a hung or unkillable process
	done := make(chan struct{})
	proc := &OSZapretCandidateProcess{
		pid:   999999,
		done:  done,
		state: ProcessStateRunning,
	}

	err := proc.Stop()
	if err == nil {
		t.Fatal("Expected Stop() to return an error when process fails to terminate, got nil")
	}
	if proc.State() != ProcessStateStopFailed {
		t.Errorf("Expected state to be STOP_FAILED, got %s", proc.State())
	}
	if proc.StopErr() == nil {
		t.Error("Expected StopErr() to be recorded on process")
	}
	// Invariant: Alive() must accurately return true because process has not exited!
	if !proc.Alive() {
		t.Error("Expected Alive() to return true when done channel was never closed")
	}
}

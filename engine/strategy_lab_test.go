package engine

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"path/filepath"
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

func (d *dummyProcess) PID() int                       { return 12345 }
func (d *dummyProcess) Argv() []string                 { return []string{"--dummy"} }
func (d *dummyProcess) State() CandidateProcessState   { return ProcessStateCaptureReady }
func (d *dummyProcess) Alive() bool                    { return true }
func (d *dummyProcess) WaitErr() error                 { return nil }
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
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		u, _ := neturl.Parse(ts.URL)
		host, _, _ := net.SplitHostPort(u.Host)

		cfg := StrategyLabTargetConfig{
			TargetHost:    host,
			Protocol:      "QUIC",
			ServicePreset: "Custom",
		}

		runner := &recordingCandidateRunner{}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
		host, _, _ := net.SplitHostPort(u.Host)

		cfg := StrategyLabTargetConfig{
			TargetHost:    host,
			Protocol:      "TLS1.2",
			ServicePreset: "Custom",
		}

		runner := &recordingCandidateRunner{}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

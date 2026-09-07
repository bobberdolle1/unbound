package engine

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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

func (d *dummyProcess) PID() int           { return 12345 }
func (d *dummyProcess) Argv() []string     { return []string{"--dummy"} }
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

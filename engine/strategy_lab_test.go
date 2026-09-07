package engine

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestSaveDiscoveredProfile(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("APPDATA", tempDir)
	t.Setenv("HOME", tempDir)
	t.Setenv("XDG_CONFIG_HOME", tempDir)

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

	configDir, _ := GetConfigDir()
	savedFile := filepath.Join(configDir, "custom_profile.lua")
	data, err := os.ReadFile(savedFile)
	if err != nil {
		t.Fatalf("Failed to read saved custom profile: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "My Discovered Profile") || !strings.Contains(content, "example.com") {
		t.Errorf("Missing metadata in custom profile: %s", content)
	}
	if !strings.Contains(content, "--lua-desync=multisplit:pos=midsld") {
		t.Errorf("Missing arguments in custom profile: %s", content)
	}
}

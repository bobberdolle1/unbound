package engine

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"unbound/engine/providers"
)

type mockProviderController struct {
	profile string
	status  providers.Status
	startErr error
	stopErr  error
	startCalls int
	stopCalls  int
}

func (m *mockProviderController) CurrentProfile() string {
	return m.profile
}

func (m *mockProviderController) GetStatus() providers.Status {
	return m.status
}

func (m *mockProviderController) Start(ctx context.Context, profile string) error {
	m.startCalls++
	if m.startErr != nil {
		return m.startErr
	}
	m.profile = profile
	m.status = providers.StatusRunning
	return nil
}

func (m *mockProviderController) Stop() error {
	m.stopCalls++
	if m.stopErr != nil {
		return m.stopErr
	}
	m.status = providers.StatusStopped
	return nil
}

func TestEvaluateComparisonItem(t *testing.T) {
	def := ProbeDefinition{ID: "test_probe", Service: "YouTube", Name: "YouTube Web"}

	// 1. Fixed by profile: Baseline FAIL, Profile PASS
	itemFixed := evaluateComparisonItem(def, ProbeResult{Status: StatusFail}, ProbeResult{Status: StatusPass})
	if itemFixed.Verdict != VerdictFixedByProfile {
		t.Errorf("Expected VerdictFixedByProfile, got %s", itemFixed.Verdict)
	}

	// 2. Reachable directly: Baseline PASS, Profile PASS
	itemDirect := evaluateComparisonItem(def, ProbeResult{Status: StatusPass}, ProbeResult{Status: StatusPass})
	if itemDirect.Verdict != VerdictReachableDirectly {
		t.Errorf("Expected VerdictReachableDirectly, got %s", itemDirect.Verdict)
	}

	// 3. Still blocked: Baseline FAIL, Profile FAIL
	itemBlocked := evaluateComparisonItem(def, ProbeResult{Status: StatusFail}, ProbeResult{Status: StatusFail})
	if itemBlocked.Verdict != VerdictStillBlocked {
		t.Errorf("Expected VerdictStillBlocked, got %s", itemBlocked.Verdict)
	}

	// 4. Broken by profile: Baseline PASS, Profile FAIL
	itemBroken := evaluateComparisonItem(def, ProbeResult{Status: StatusPass}, ProbeResult{Status: StatusFail})
	if itemBroken.Verdict != VerdictBrokenByProfile {
		t.Errorf("Expected VerdictBrokenByProfile, got %s", itemBroken.Verdict)
	}
}

func TestRunBypassComparisonRestoresEngineState(t *testing.T) {
	mock := &mockProviderController{
		profile: "OriginalProfile",
		status:  providers.StatusRunning,
	}

	probes := []ProbeDefinition{
		{
			ID:      "p1",
			Service: "Web",
			Name:    "Test Probe",
			Run: func(ctx context.Context, ce *ConnectivityEngine) ProbeResult {
				return ProbeResult{Status: StatusPass}
			},
		},
	}

	ctx := context.Background()
	res, err := RunBypassComparison(ctx, mock, probes)
	if err != nil {
		t.Fatalf("RunBypassComparison failed: %v", err)
	}

	if mock.status != providers.StatusRunning {
		t.Errorf("Expected mock status restored to StatusRunning, got %v", mock.status)
	}
	if mock.profile != "OriginalProfile" {
		t.Errorf("Expected mock profile restored to 'OriginalProfile', got %s", mock.profile)
	}
	if len(res.Items) != 1 {
		t.Errorf("Expected 1 comparison item, got %d", len(res.Items))
	}
}

func TestRunBypassComparisonRestoresOnFailure(t *testing.T) {
	mock := &mockProviderController{
		profile: "OriginalProfile",
		status:  providers.StatusRunning,
	}

	// Probe that triggers context cancellation mid-flight
	ctx, cancel := context.WithCancel(context.Background())
	probes := []ProbeDefinition{
		{
			ID:      "p1",
			Service: "Web",
			Name:    "Cancelled Probe",
			Run: func(c context.Context, ce *ConnectivityEngine) ProbeResult {
				cancel() // Cancel context during probe execution
				return ProbeResult{Status: StatusFail}
			},
		},
	}

	_, err := RunBypassComparison(ctx, mock, probes)
	if !errors.Is(err, context.Canceled) {
		t.Logf("Returned err: %v (expected cancellation)", err)
	}

	// Verify transactional cleanup restored original profile
	if mock.status != providers.StatusRunning {
		t.Errorf("Expected status restored to Running despite failure, got %v", mock.status)
	}
	if mock.profile != "OriginalProfile" {
		t.Errorf("Expected profile restored to OriginalProfile, got %s", mock.profile)
	}
}

func TestBypassComparisonFormatMarkdown(t *testing.T) {
	res := &BypassComparisonResult{
		ProfileName:    "Alternative 1 (multisplit)",
		OverallSummary: "Восстановлено: 1",
		Duration:       500 * time.Millisecond,
		Items: []ComparisonItem{
			{
				Service: "YouTube",
				Name:    "YouTube Web",
				Baseline: ProbeResult{Status: StatusFail},
				Profile:  ProbeResult{Status: StatusPass},
				Verdict:  VerdictFixedByProfile,
			},
		},
	}

	md := res.FormatMarkdown()
	if !strings.Contains(md, "# UNBOUND A/B Bypass Comparison Report") {
		t.Error("Missing header in markdown")
	}
	if !strings.Contains(md, "Alternative 1 (multisplit)") {
		t.Error("Missing profile name in markdown")
	}
	if !strings.Contains(md, "Восстановлено профилем") {
		t.Error("Missing verdict in markdown")
	}
}

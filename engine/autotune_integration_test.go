package engine

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"unbound/engine/providers"
)

type fakeAutoTuneProvider struct {
	mu      sync.Mutex
	active  string
	starts  []string
	stopCnt int
}

func (p *fakeAutoTuneProvider) Name() string                   { return "fake" }
func (p *fakeAutoTuneProvider) CheckPrivileges() (bool, error) { return true, nil }
func (p *fakeAutoTuneProvider) GetProfiles() []string          { return []string{"First", "Best"} }
func (p *fakeAutoTuneProvider) Start(_ context.Context, profile string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.active = profile
	p.starts = append(p.starts, profile)
	return nil
}
func (p *fakeAutoTuneProvider) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.active = ""
	p.stopCnt++
	return nil
}
func (p *fakeAutoTuneProvider) GetStatus() providers.Status {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.active == "" {
		return providers.StatusStopped
	}
	return providers.StatusRunning
}
func (p *fakeAutoTuneProvider) GetLogs() []string { return nil }
func (p *fakeAutoTuneProvider) CurrentProfile() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.active
}

func TestAutoTuneV3MeasuresBaselineAndRanksEveryProfile(t *testing.T) {
	provider := &fakeAutoTuneProvider{}
	targets := []Target{
		{Name: "blocked", URL: "https://blocked.test", Priority: 30},
		{Name: "open", URL: "https://open.test", Priority: 10},
	}
	probe := func(_ context.Context, targetURL string) (ProbeResult, error) {
		active := provider.CurrentProfile()
		if targetURL == "https://blocked.test" && active == "" {
			return ProbeResult{URL: targetURL, Error: "blocked"}, errors.New("blocked")
		}
		latency := 80 * time.Millisecond
		if active == "Best" {
			latency = 20 * time.Millisecond
		}
		return ProbeResult{URL: targetURL, Success: true, CertValid: true, TLSVersion: 0x0304, Latency: latency}, nil
	}
	options := AutoTuneOptions{Targets: targets, Probe: probe, ProbeTimeout: time.Second, MinimumOK: 2}
	profiles := []Profile{{Name: "First"}, {Name: "Best"}}

	result, err := RunAutoTuneV3(context.Background(), provider, profiles, nil, options)
	if err != nil {
		t.Fatal(err)
	}
	if result.ProfileName != "Best" {
		t.Fatalf("winner = %q, want Best", result.ProfileName)
	}
	if result.RecoveredTargets != 1 || result.BaselineAvailable != 1 {
		t.Fatalf("unexpected baseline delta: %+v", result)
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if len(provider.starts) != len(profiles) {
		t.Fatalf("tested %d profiles, want %d", len(provider.starts), len(profiles))
	}
	if provider.active != "" {
		t.Fatalf("benchmark provider left active on %q", provider.active)
	}
}

func TestAutoTuneV3RejectsConnectivityRegression(t *testing.T) {
	provider := &fakeAutoTuneProvider{}
	targets := []Target{
		{Name: "blocked", URL: "https://blocked.test", Priority: 30},
		{Name: "open", URL: "https://open.test", Priority: 30},
	}
	probe := func(_ context.Context, targetURL string) (ProbeResult, error) {
		active := provider.CurrentProfile()
		ok := (targetURL == "https://open.test" && active == "") || (targetURL == "https://blocked.test" && active != "")
		if !ok {
			return ProbeResult{URL: targetURL, Error: "unreachable"}, errors.New("unreachable")
		}
		return ProbeResult{URL: targetURL, Success: true, CertValid: true, Latency: 20 * time.Millisecond}, nil
	}
	options := AutoTuneOptions{Targets: targets, Probe: probe, ProbeTimeout: time.Second, MinimumOK: 1}

	result, err := RunAutoTuneV3(context.Background(), provider, []Profile{{Name: "Regressive"}}, nil, options)
	if err == nil || result != nil {
		t.Fatalf("regressive profile accepted: result=%+v err=%v", result, err)
	}
}

func TestAutoTuneV3CancellationStopsActiveProvider(t *testing.T) {
	provider := &fakeAutoTuneProvider{}
	ctx, cancel := context.WithCancel(context.Background())
	options := AutoTuneOptions{
		Targets: []Target{{Name: "target", URL: "https://target.test", Priority: 1}},
		Probe: func(context.Context, string) (ProbeResult, error) {
			return ProbeResult{Success: true, CertValid: true}, nil
		},
		ProbeTimeout:       time.Second,
		StabilizationDelay: time.Minute,
		MinimumOK:          1,
	}
	done := make(chan error, 1)
	go func() {
		_, err := RunAutoTuneV3(ctx, provider, []Profile{{Name: "Profile"}}, nil, options)
		done <- err
	}()

	deadline := time.Now().Add(time.Second)
	for provider.GetStatus() != providers.StatusRunning && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("AutoTune did not stop after cancellation")
	}
	if provider.GetStatus() != providers.StatusStopped {
		t.Fatal("provider remained active after cancellation")
	}
}

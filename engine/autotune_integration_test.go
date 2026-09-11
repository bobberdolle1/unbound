package engine

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"unbound/engine/providers"
)

type fakeAutoTuneProvider struct {
	mu            sync.Mutex
	active        string
	starts        []string
	stopCnt       int
	startErr      error
	stopErr       error
	stopErrOnCall int
	stopDelay     time.Duration
	onStart       func(string)
}

func (p *fakeAutoTuneProvider) Name() string                   { return "fake" }
func (p *fakeAutoTuneProvider) CheckPrivileges() (bool, error) { return true, nil }
func (p *fakeAutoTuneProvider) GetProfiles() []string          { return []string{"First", "Best"} }
func (p *fakeAutoTuneProvider) Start(_ context.Context, profile string) error {
	p.mu.Lock()
	if p.startErr != nil {
		p.mu.Unlock()
		return p.startErr
	}
	p.active = profile
	p.starts = append(p.starts, profile)
	onStart := p.onStart
	p.mu.Unlock()
	if onStart != nil {
		onStart(profile)
	}
	return nil
}
func (p *fakeAutoTuneProvider) Stop() error {
	p.mu.Lock()
	p.stopCnt++
	stopCnt := p.stopCnt
	stopErr := p.stopErr
	stopErrOnCall := p.stopErrOnCall
	stopDelay := p.stopDelay
	p.mu.Unlock()
	if stopDelay > 0 {
		time.Sleep(stopDelay)
	}
	if stopErr != nil && (stopErrOnCall == 0 || stopErrOnCall == stopCnt) {
		return stopErr
	}
	p.mu.Lock()
	p.active = ""
	p.mu.Unlock()
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
	if strings.Contains(err.Error(), "AUTOTUNE_LIFECYCLE_FAILURE") {
		t.Fatalf("ordinary strategy regression became lifecycle failure: %v", err)
	}
}

func TestAutoTuneV3AbortsOnLifecycleStartFailure(t *testing.T) {
	provider := &fakeAutoTuneProvider{startErr: errors.New("another profile () started while this one (First) was in progress")}
	options := AutoTuneOptions{
		Targets: []Target{{Name: "target", URL: "https://target.test", Priority: 1}},
		Probe: func(_ context.Context, _ string) (ProbeResult, error) {
			if provider.CurrentProfile() != "" {
				t.Fatal("profile probe ran after lifecycle start failure")
			}
			return ProbeResult{Success: true, CertValid: true}, nil
		},
		ProbeTimeout: time.Second,
		MinimumOK:    1,
	}

	result, err := RunAutoTuneV3(context.Background(), provider, []Profile{{Name: "First"}, {Name: "Second"}}, nil, options)
	if err == nil || result == nil {
		t.Fatalf("lifecycle failure did not return structured failure: result=%+v err=%v", result, err)
	}
	if result.Completed || result.ErrorCategory != "AUTOTUNE_LIFECYCLE_FAILURE" || result.LifecycleFailures != 1 || result.ProfilesAttempted != 1 || result.ProfilesFailedToStart != 1 {
		t.Fatalf("invalid lifecycle failure report: %+v", result)
	}
	if !strings.Contains(err.Error(), "AUTOTUNE_LIFECYCLE_FAILURE") {
		t.Fatalf("lifecycle category missing from error: %v", err)
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
func lifecycleTestOptions(provider *fakeAutoTuneProvider) AutoTuneOptions {
	return AutoTuneOptions{
		Targets: []Target{{Name: "blocked", URL: "https://blocked.test", Priority: 1}},
		Probe: func(_ context.Context, _ string) (ProbeResult, error) {
			if provider.CurrentProfile() == "" {
				return ProbeResult{Error: "blocked"}, errors.New("blocked")
			}
			return ProbeResult{Success: true, CertValid: true}, nil
		},
		ProbeTimeout: time.Second,
		MinimumOK:    1,
	}
}

func requireAutoTuneLifecycleFailure(t *testing.T, result *AutoTuneResult, err error) {
	t.Helper()
	if err == nil || result == nil {
		t.Fatalf("lifecycle failure did not return structured failure: result=%+v err=%v", result, err)
	}
	if result.Completed || result.ProfileName != "" || result.ErrorCategory != "AUTOTUNE_LIFECYCLE_FAILURE" || result.LifecycleFailures != 1 {
		t.Fatalf("invalid lifecycle failure report: %+v", result)
	}
}

func TestAutoTuneV3DelayedStopCompletes(t *testing.T) {
	provider := &fakeAutoTuneProvider{stopDelay: 20 * time.Millisecond}
	result, err := RunAutoTuneV3(context.Background(), provider, []Profile{{Name: "Profile"}}, nil, lifecycleTestOptions(provider))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Completed || result.LifecycleFailures != 0 || provider.GetStatus() != providers.StatusStopped {
		t.Fatalf("delayed Stop did not complete cleanly: result=%+v status=%s", result, provider.GetStatus())
	}
}

func TestAutoTuneV3StopTimeoutAborts(t *testing.T) {
	provider := &fakeAutoTuneProvider{stopErr: errors.New("owned process did not stop"), stopErrOnCall: 2}
	result, err := RunAutoTuneV3(context.Background(), provider, []Profile{{Name: "Profile"}}, nil, lifecycleTestOptions(provider))
	requireAutoTuneLifecycleFailure(t, result, err)
	if result.ProfilesAttempted != 1 || result.ProfilesCompleted != 0 {
		t.Fatalf("stop failure counts are incoherent: %+v", result)
	}
}

func TestAutoTuneV3UnexpectedExitAborts(t *testing.T) {
	provider := &fakeAutoTuneProvider{}
	started := make(chan struct{})
	exitNow := make(chan struct{})
	provider.onStart = func(string) {
		close(started)
		go func() {
			<-exitNow
			provider.mu.Lock()
			provider.active = ""
			provider.mu.Unlock()
		}()
	}
	options := lifecycleTestOptions(provider)
	options.StabilizationDelay = time.Second
	done := make(chan struct {
		result *AutoTuneResult
		err    error
	}, 1)
	go func() {
		result, err := RunAutoTuneV3(context.Background(), provider, []Profile{{Name: "Profile"}}, nil, options)
		done <- struct {
			result *AutoTuneResult
			err    error
		}{result, err}
	}()
	<-started
	close(exitNow)
	outcome := <-done
	requireAutoTuneLifecycleFailure(t, outcome.result, outcome.err)
}

func TestAutoTuneV3CorruptProviderStateAborts(t *testing.T) {
	provider := &fakeAutoTuneProvider{}
	provider.onStart = func(string) {
		provider.mu.Lock()
		provider.active = ""
		provider.mu.Unlock()
	}
	result, err := RunAutoTuneV3(context.Background(), provider, []Profile{{Name: "Profile"}}, nil, lifecycleTestOptions(provider))
	requireAutoTuneLifecycleFailure(t, result, err)
}

func TestAutoTuneV3ProbeFailureRemainsBenchmarkFailure(t *testing.T) {
	provider := &fakeAutoTuneProvider{}
	options := lifecycleTestOptions(provider)
	options.Probe = func(context.Context, string) (ProbeResult, error) {
		return ProbeResult{Error: "probe failed"}, errors.New("probe failed")
	}
	result, err := RunAutoTuneV3(context.Background(), provider, []Profile{{Name: "Profile"}}, nil, options)
	if err == nil || result != nil || strings.Contains(err.Error(), "AUTOTUNE_LIFECYCLE_FAILURE") {
		t.Fatalf("probe failure classification changed: result=%+v err=%v", result, err)
	}
}

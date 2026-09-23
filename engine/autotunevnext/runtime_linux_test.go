//go:build linux

package autotunevnext

import (
	"context"
	"errors"
	"testing"

	"unbound/engine"
	"unbound/engine/providers"
)

type linuxRuntimeProvider struct {
	status  providers.Status
	profile string
	stopErr error
}

func (p *linuxRuntimeProvider) CheckPrivileges() (bool, error) { return true, nil }
func (p *linuxRuntimeProvider) Start(_ context.Context, profile string) error {
	p.status, p.profile = providers.StatusRunning, profile
	return nil
}
func (p *linuxRuntimeProvider) Stop() error {
	if p.stopErr != nil {
		return p.stopErr
	}
	p.status, p.profile = providers.StatusStopped, ""
	return nil
}
func (p *linuxRuntimeProvider) GetStatus() providers.Status { return p.status }
func (p *linuxRuntimeProvider) CurrentProfile() string      { return p.profile }
func (p *linuxRuntimeProvider) Name() string                { return "fake-linux" }

func TestLinuxRuntimeDirectLifecycleRestoresOriginalProvider(t *testing.T) {
	provider := &linuxRuntimeProvider{status: providers.StatusRunning, profile: "original"}
	runtime, err := NewLinuxRuntime(RuntimeOptions{Provider: provider, Assets: &engine.AssetPaths{}})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := runtime.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.EstablishDirect(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	if provider.status != providers.StatusStopped || provider.profile != "" {
		t.Fatalf("direct lifecycle did not stop original provider: %#v", provider)
	}
	if err := runtime.Restore(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	if err := runtime.VerifyRestored(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
}

func TestLinuxRuntimeDirectLifecycleRefusesUnstoppedProvider(t *testing.T) {
	provider := &linuxRuntimeProvider{status: providers.StatusRunning, profile: "original", stopErr: errors.New("stop failed")}
	runtime, err := NewLinuxRuntime(RuntimeOptions{Provider: provider, Assets: &engine.AssetPaths{}})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := runtime.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.EstablishDirect(context.Background(), snapshot); err == nil {
		t.Fatal("accepted uncertain direct-observation boundary")
	}
}

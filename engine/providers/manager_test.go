package providers

import (
	"context"
	"testing"
)

type fakeCallbackProvider struct {
	name           string
	profiles       []string
	status         Status
	currentProfile string
}

func (f *fakeCallbackProvider) Name() string                     { return f.name }
func (f *fakeCallbackProvider) CheckPrivileges() (bool, error)   { return true, nil }
func (f *fakeCallbackProvider) GetProfiles() []string            { return append([]string(nil), f.profiles...) }
func (f *fakeCallbackProvider) GetStatus() Status                { return f.status }
func (f *fakeCallbackProvider) GetLogs() []string                { return nil }
func (f *fakeCallbackProvider) SetStatusCallback(func(Status))   {}
func (f *fakeCallbackProvider) SetLogCallback(func(string))      {}
func (f *fakeCallbackProvider) RegisterProfile(string, []string) {}
func (f *fakeCallbackProvider) CurrentProfile() string           { return f.currentProfile }
func (f *fakeCallbackProvider) Start(_ context.Context, p string) error {
	f.currentProfile = p
	f.status = StatusRunning
	return nil
}
func (f *fakeCallbackProvider) Stop() error {
	f.currentProfile = ""
	f.status = StatusStopped
	return nil
}

func TestProviderManagerCurrentProfileNameDelegatesEmptyEngineToActiveProvider(t *testing.T) {
	mgr := NewProviderManager()
	p1 := &fakeCallbackProvider{
		name:     "Zapret 2 (winws)",
		profiles: []string{"Recommended (hostfakesplit)", "Alternative 1 (multisplit)"},
		status:   StatusStopped,
	}
	p2 := &fakeCallbackProvider{
		name:     "Zapret 2 (nfqws2)",
		profiles: []string{"Ultimate Bypass (Multi-Strategy)"},
		status:   StatusStopped,
	}
	mgr.Register(p1)
	mgr.Register(p2)

	if got := mgr.CurrentProfileName(""); got != "" {
		t.Fatalf("CurrentProfileName(\"\") before start = %q, want empty", got)
	}

	if err := mgr.Start(context.Background(), p1.Name(), "Alternative 1 (multisplit)"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if got := mgr.CurrentProfileName(""); got != "Alternative 1 (multisplit)" {
		t.Fatalf("CurrentProfileName(\"\") = %q, want %q", got, "Alternative 1 (multisplit)")
	}
	if got := mgr.CurrentProfileName(p1.Name()); got != "Alternative 1 (multisplit)" {
		t.Fatalf("CurrentProfileName(%q) = %q, want %q", p1.Name(), got, "Alternative 1 (multisplit)")
	}
	if got := mgr.CurrentProfileName(p2.Name()); got != "" {
		t.Fatalf("CurrentProfileName(%q) for inactive engine = %q, want empty", p2.Name(), got)
	}
	if got := mgr.ActiveProfileName(); got != "Alternative 1 (multisplit)" {
		t.Fatalf("ActiveProfileName() = %q, want %q", got, "Alternative 1 (multisplit)")
	}

	if err := mgr.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if got := mgr.CurrentProfileName(""); got != "" {
		t.Fatalf("CurrentProfileName(\"\") after stop = %q, want empty", got)
	}
}

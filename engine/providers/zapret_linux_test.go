//go:build linux

package providers

import (
	"slices"
	"strings"
	"testing"
)

func newTestProvider(t *testing.T) *ZapretLinuxProvider {
	t.Helper()
	p, ok := NewZapretLinuxProvider("/nonexistent/nfqws", t.TempDir()).(*ZapretLinuxProvider)
	if !ok {
		t.Fatal("NewZapretLinuxProvider did not return *ZapretLinuxProvider")
	}
	return p
}

func TestGetProfilesIsStable(t *testing.T) {
	p := newTestProvider(t)

	first := p.GetProfiles()
	if len(first) != len(profileOrder) {
		t.Fatalf("got %d profiles, want %d", len(first), len(profileOrder))
	}

	// Ranging over the profile map directly would reshuffle the UI dropdown on
	// every call, so the order has to come from profileOrder.
	for i := 0; i < 20; i++ {
		if !slices.Equal(p.GetProfiles(), first) {
			t.Fatal("profile order is not stable across calls")
		}
	}
}

func TestRegisterProfileAppearsAfterBuiltins(t *testing.T) {
	p := newTestProvider(t)

	p.RegisterProfile("Custom Profile", []string{"--filter-tcp=443", "--dpi-desync=fake"})
	profiles := p.GetProfiles()

	if len(profiles) != len(profileOrder)+1 {
		t.Fatalf("got %d profiles, want %d", len(profiles), len(profileOrder)+1)
	}
	if profiles[len(profiles)-1] != "Custom Profile" {
		t.Errorf("custom profile should sort last, got %v", profiles)
	}
}

// Registering the same name twice used to be possible via the map while the
// ordering slice grew, which would show the profile twice in the dropdown.
func TestRegisterProfileIsIdempotent(t *testing.T) {
	p := newTestProvider(t)

	p.RegisterProfile("Custom Profile", []string{"--a"})
	p.RegisterProfile("Custom Profile", []string{"--b"})

	profiles := p.GetProfiles()
	count := 0
	for _, name := range profiles {
		if name == "Custom Profile" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("profile listed %d times, want 1", count)
	}

	resolved, err := p.resolveProfile("Custom Profile")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(resolved.Args, []string{"--b"}) {
		t.Errorf("re-registering should replace the args, got %v", resolved.Args)
	}
}

// A registered profile must not shadow a built-in of the same name, otherwise
// the auto-tuner (which registers every profile it knows) would strip the
// netfilter rules from the built-ins.
func TestBuiltinProfileWinsOverRegistered(t *testing.T) {
	p := newTestProvider(t)
	p.RegisterProfile("Standard HTTPS/QUIC", []string{"--override"})

	resolved, err := p.resolveProfile("Standard HTTPS/QUIC")
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Filters) == 0 {
		t.Error("built-in profile lost its packet filters")
	}
	if slices.Contains(resolved.Args, "--override") {
		t.Error("registered args overrode the built-in profile")
	}

	if got := len(p.GetProfiles()); got != len(profileOrder) {
		t.Errorf("re-registering a built-in added a duplicate entry: %d profiles", got)
	}
}

func TestResolveUnknownProfile(t *testing.T) {
	p := newTestProvider(t)
	if _, err := p.resolveProfile("No Such Profile"); err == nil {
		t.Error("expected an error for an unknown profile")
	}
}

func TestEveryBuiltinProfileHasFiltersAndArgs(t *testing.T) {
	for _, name := range profileOrder {
		profile, ok := builtinProfiles[name]
		if !ok {
			t.Errorf("%q is listed in profileOrder but has no definition", name)
			continue
		}
		if len(profile.Filters) == 0 {
			t.Errorf("%q queues no traffic: nfqws would start and see nothing", name)
		}
		if len(profile.Args) == 0 {
			t.Errorf("%q passes no desync arguments to nfqws", name)
		}
	}

	if len(profileOrder) != len(builtinProfiles) {
		t.Errorf("profileOrder lists %d profiles but %d are defined; the extras would be invisible in the UI",
			len(profileOrder), len(builtinProfiles))
	}
}

func TestLogRingBufferIsBounded(t *testing.T) {
	p := newTestProvider(t)
	for i := 0; i < maxLogLines*2; i++ {
		p.addLog("line")
	}
	if got := len(p.GetLogs()); got > maxLogLines {
		t.Errorf("log buffer grew to %d lines, cap is %d", got, maxLogLines)
	}
}

// GetLogs must hand back a copy: returning the live slice let a caller iterate
// it while the engine goroutine appended, which is a data race.
func TestGetLogsReturnsCopy(t *testing.T) {
	p := newTestProvider(t)
	p.addLog("original")

	logs := p.GetLogs()
	logs[len(logs)-1] = "mutated"

	if got := p.GetLogs(); got[len(got)-1] != "original" {
		t.Error("GetLogs exposed the provider's internal slice")
	}
}

func TestRuleSpecQueuesWithBypass(t *testing.T) {
	spec := strings.Join(ruleSpec(packetFilter{Proto: "tcp", Ports: "443", HandshakeOnly: true}), " ")

	// Without --queue-bypass an nfqws crash blackholes every matched packet
	// and takes the user's connection down with it.
	if !strings.Contains(spec, "--queue-bypass") {
		t.Errorf("rule is missing --queue-bypass: %s", spec)
	}
	if !strings.Contains(spec, "--queue-num "+nfqueueNum) {
		t.Errorf("rule targets the wrong queue: %s", spec)
	}
	if !strings.Contains(spec, "connbytes") {
		t.Errorf("handshake-only rule should limit connbytes: %s", spec)
	}
	if !strings.Contains(spec, "-m mark ! --mark "+fwmarkMatch) {
		t.Errorf("rule must skip already-processed packets or it loops: %s", spec)
	}
}

// connbytes is a TCP-oriented match; applying it to UDP would drop the QUIC
// rules that the YouTube and Discord profiles depend on.
func TestRuleSpecOmitsConnbytesForUDP(t *testing.T) {
	spec := strings.Join(ruleSpec(packetFilter{Proto: "udp", Ports: "443", HandshakeOnly: true}), " ")
	if strings.Contains(spec, "connbytes") {
		t.Errorf("UDP rule should not use connbytes: %s", spec)
	}
}

func TestNftPortSet(t *testing.T) {
	cases := map[string]string{
		"443":                  "{ 443 }",
		"80,443":               "{ 80, 443 }",
		"50000:65535":          "{ 50000-65535 }",
		"443,3478,50000:65535": "{ 443, 3478, 50000-65535 }",
	}

	for in, want := range cases {
		if got := nftPortSet(in); got != want {
			t.Errorf("nftPortSet(%q) = %q, want %q", in, got, want)
		}
	}
}

// Every port list in a built-in profile has to survive translation to nft
// syntax, otherwise the nftables backend silently rejects the whole ruleset.
func TestBuiltinPortsTranslateToNft(t *testing.T) {
	for name, profile := range builtinProfiles {
		for _, f := range profile.Filters {
			got := nftPortSet(f.Ports)
			if strings.Contains(got, ":") {
				t.Errorf("%s: iptables range syntax leaked into nft rule: %s", name, got)
			}
			if !strings.HasPrefix(got, "{ ") || !strings.HasSuffix(got, " }") {
				t.Errorf("%s: malformed nft set literal: %s", name, got)
			}
		}
	}
}

//go:build darwin

package providers

import (
	"strings"
	"testing"
)

// TestMacOSProfilesHaveValidPfRules verifies that every built-in macOS profile
// uses pf rules that are valid on macOS (route-to / rdr pass — NOT divert-packet
// which is Linux-only and causes pfctl to reject the ruleset).
func TestMacOSProfilesHaveValidPfRules(t *testing.T) {
	for name, profile := range macBuiltinProfiles {
		if len(profile.PfRules) == 0 {
			t.Errorf("profile %q has no pf rules", name)
			continue
		}
		for _, rule := range profile.PfRules {
			if strings.Contains(rule, "divert-packet") {
				t.Errorf("profile %q contains divert-packet (Linux-only): %s", name, rule)
			}
			if strings.Contains(rule, "divert-to") {
				t.Errorf("profile %q contains divert-to (Linux-only): %s", name, rule)
			}
		}
	}
}

// TestMacOSProfilesHaveTpwsArgs verifies that every profile's tpws args include
// --bind-addr=127.0.0.1 so tpws listens locally (required for route-to redirect).
func TestMacOSProfilesHaveTpwsArgs(t *testing.T) {
	for name, profile := range macBuiltinProfiles {
		hasBind := false
		for _, arg := range profile.Args {
			if strings.HasPrefix(arg, "--bind-addr") {
				hasBind = true
				break
			}
		}
		if !hasBind {
			t.Errorf("profile %q is missing --bind-addr=127.0.0.1 in tpws args", name)
		}
	}
}

// TestMacOSProfilesHaveDPIArgs verifies that every profile passes at least one
// desync argument for tpws (--split-pos, --disorder, --oob, --tlsrec, --hostcase).
func TestMacOSProfilesHaveDPIArgs(t *testing.T) {
	for name, profile := range macBuiltinProfiles {
		hasDPI := false
		for _, arg := range profile.Args {
			if strings.HasPrefix(arg, "--split-pos") ||
				strings.HasPrefix(arg, "--disorder") ||
				strings.HasPrefix(arg, "--oob") ||
				strings.HasPrefix(arg, "--tlsrec") ||
				strings.HasPrefix(arg, "--hostcase") ||
				strings.HasPrefix(arg, "--domcase") ||
				strings.HasPrefix(arg, "--dpi-desync") {
				hasDPI = true
				break
			}
		}
		if !hasDPI {
			t.Errorf("profile %q has no desync args — would run tpws as a plain proxy with no DPI bypass", name)
		}
	}
}

// TestMacOSProfileOrderComplete verifies that every entry in macProfileOrder
// exists in macBuiltinProfiles (no dangling references in the UI list).
func TestMacOSProfileOrderComplete(t *testing.T) {
	for _, name := range macProfileOrder {
		if _, ok := macBuiltinProfiles[name]; !ok {
			t.Errorf("macProfileOrder references %q which is not in macBuiltinProfiles", name)
		}
	}
}

// TestMacOSProviderGetProfiles verifies GetProfiles returns all built-in profiles.
func TestMacOSProviderGetProfiles(t *testing.T) {
	p := NewZapretMacOSProvider("").(*ZapretMacOSProvider)
	got := p.GetProfiles()
	if len(got) < len(macProfileOrder) {
		t.Errorf("GetProfiles returned %d profiles, want at least %d", len(got), len(macProfileOrder))
	}
	names := make(map[string]bool, len(got))
	for _, n := range got {
		names[n] = true
	}
	for _, want := range macProfileOrder {
		if !names[want] {
			t.Errorf("GetProfiles is missing profile %q", want)
		}
	}
}

// TestMacOSProviderRegisterProfile verifies custom profiles are discoverable.
func TestMacOSProviderRegisterProfile(t *testing.T) {
	p := NewZapretMacOSProvider("").(*ZapretMacOSProvider)
	p.RegisterProfile("My Custom", []string{"--filter-tcp=443", "--dpi-desync=fake"})

	profiles := p.GetProfiles()
	found := false
	for _, n := range profiles {
		if n == "My Custom" {
			found = true
		}
	}
	if !found {
		t.Error("registered custom profile 'My Custom' not returned by GetProfiles")
	}
}

// TestMacOSProviderResolveCustomProfile verifies custom profiles get tpws-compatible
// pf rules (route-to / rdr pass, not divert-packet).
func TestMacOSProviderResolveCustomProfile(t *testing.T) {
	p := NewZapretMacOSProvider("").(*ZapretMacOSProvider)
	p.RegisterProfile("Custom Test", []string{"--filter-tcp=443", "--dpi-desync=fake"})

	profile, err := p.resolveProfile("Custom Test")
	if err != nil {
		t.Fatalf("resolveProfile error: %v", err)
	}
	if len(profile.PfRules) == 0 {
		t.Fatal("custom profile has no pf rules")
	}
	for _, rule := range profile.PfRules {
		if strings.Contains(rule, "divert-packet") {
			t.Errorf("custom profile pf rule uses divert-packet: %s", rule)
		}
	}
	hasBind := false
	for _, arg := range profile.Args {
		if strings.HasPrefix(arg, "--bind-addr") {
			hasBind = true
		}
	}
	if !hasBind {
		t.Error("custom profile is missing --bind-addr in tpws args")
	}
}

// TestMacOSProviderInitialStatus verifies the provider starts in Stopped state.
func TestMacOSProviderInitialStatus(t *testing.T) {
	p := NewZapretMacOSProvider("")
	if got := p.GetStatus(); got != StatusStopped {
		t.Errorf("initial status = %v, want Stopped", got)
	}
}

// TestTpwsPfRules verifies the generated pf rules contain the expected patterns.
func TestTpwsPfRules(t *testing.T) {
	rules := tpwsPfRules("80,443")
	if len(rules) == 0 {
		t.Fatal("tpwsPfRules returned no rules")
	}

	hasRouteTO := false
	hasRdr := false
	hasQUICBlock := false

	for _, r := range rules {
		if strings.Contains(r, "route-to") {
			hasRouteTO = true
		}
		if strings.HasPrefix(r, "rdr pass") {
			hasRdr = true
		}
		if strings.Contains(r, "block drop") && strings.Contains(r, "udp") {
			hasQUICBlock = true
		}
		if strings.Contains(r, "divert-packet") {
			t.Errorf("tpwsPfRules generated divert-packet rule (Linux-only): %s", r)
		}
	}

	if !hasRouteTO {
		t.Error("tpwsPfRules missing route-to rule (required to redirect outgoing TCP to loopback)")
	}
	if !hasRdr {
		t.Error("tpwsPfRules missing rdr pass rule (required to redirect loopback traffic to tpws port)")
	}
	if !hasQUICBlock {
		t.Error("tpwsPfRules missing QUIC block rule (UDP port 443 must be blocked so browsers use TCP)")
	}
}

func TestMacOSProviderResolveProfileAliases(t *testing.T) {
	p := NewZapretMacOSProvider("").(*ZapretMacOSProvider)

	aliases := map[string]string{
		"ultimate":    "Ultimate Bypass (Multi-Strategy)",
		"ULTIMATE":    "Ultimate Bypass (Multi-Strategy)",
		"youtube":     "YouTube QUIC Aggressive",
		"discord":     "Discord Voice Optimized",
		"telegram":    "Telegram API Bypass",
		"https":       "Standard HTTPS/QUIC",
		"split":       "HTTP + HTTPS Split",
		"recommended": "Ultimate Bypass (Multi-Strategy)",
	}

	for alias, expectedName := range aliases {
		prof, err := p.resolveProfile(alias)
		if err != nil {
			t.Errorf("resolveProfile(%q) failed: %v", alias, err)
			continue
		}
		expectedProf, _ := p.resolveProfile(expectedName)
		if len(prof.Args) != len(expectedProf.Args) {
			t.Errorf("resolveProfile(%q) did not match %q args length (%d != %d)", alias, expectedName, len(prof.Args), len(expectedProf.Args))
		}
	}
}

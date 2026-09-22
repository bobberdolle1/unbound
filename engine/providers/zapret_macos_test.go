//go:build darwin

package providers

import (
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
)

// TestMacOSProfilesKeepPFOutOfSOCKSPath verifies that SOCKS-mode profiles
// carry no transparent-routing configuration. tpws --socks requires a SOCKS
// handshake, so PF rdr/route-to rules must never target its listener.
func TestMacOSProfilesKeepPFOutOfSOCKSPath(t *testing.T) {
	for name, profile := range macBuiltinProfiles {
		if !strings.Contains(strings.Join(profile.Args, " "), "--bind-addr=127.0.0.1") {
			t.Errorf("profile %q is missing the loopback SOCKS bind address", name)
		}
	}
}

// TestMacOSProfilesHaveTpwsArgs verifies that every profile's tpws args include
// --bind-addr=127.0.0.1 for the local SOCKS listener.
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

// TestMacOSProviderResolveCustomProfile verifies custom profiles remain
// SOCKS-compatible and do not gain an implicit PF transparent redirect.
func TestMacOSProviderResolveCustomProfile(t *testing.T) {
	p := NewZapretMacOSProvider("").(*ZapretMacOSProvider)
	p.RegisterProfile("Custom Test", []string{"--filter-tcp=443", "--dpi-desync=fake"})

	profile, err := p.resolveProfile("Custom Test")
	if err != nil {
		t.Fatalf("resolveProfile error: %v", err)
	}
	if profile.BlockQUIC {
		t.Fatal("custom profile unexpectedly blocks unrelated UDP/443 traffic")
	}
	if !strings.Contains(strings.Join(profile.Args, " "), "--bind-addr=127.0.0.1") {
		t.Error("custom profile is missing --bind-addr=127.0.0.1")
	}
}

// TestMacOSProviderInitialStatus verifies the provider starts in Stopped state.
func TestMacOSProviderInitialStatus(t *testing.T) {
	p := NewZapretMacOSProvider("")
	if got := p.GetStatus(); got != StatusStopped {
		t.Errorf("initial status = %v, want Stopped", got)
	}
}

// TestQUICBlockIsProfileSpecific verifies that only profiles which explicitly
// need TCP fallback can block UDP/443 at the PF layer.
func TestQUICBlockIsProfileSpecific(t *testing.T) {
	for name, profile := range macBuiltinProfiles {
		wantBlock := name == "Ultimate Bypass (Multi-Strategy)" || name == "YouTube QUIC Aggressive"
		if profile.BlockQUIC != wantBlock {
			t.Errorf("profile %q BlockQUIC = %t, want %t", name, profile.BlockQUIC, wantBlock)
		}
	}
}

func TestMacOSProviderResolveProfileAliases(t *testing.T) {
	p := NewZapretMacOSProvider("").(*ZapretMacOSProvider)

	aliases := map[string]string{
		"ultimate":    "Ultimate Bypass (Multi-Strategy)",
		"ULTIMATE":    "Ultimate Bypass (Multi-Strategy)",
		"youtube":     "YouTube QUIC Aggressive",
		"discord":     "Discord TCP Bypass (Web / Gateway)",
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

func TestCheckTpwsPortAvailableReportsActionableCollision(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	port := strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
	err = checkTpwsPortAvailable(port)
	var collision *SocksPortInUseError
	if !errors.As(err, &collision) {
		t.Fatalf("checkTpwsPortAvailable() error = %v, want SocksPortInUseError", err)
	}
	if collision.Code != "SOCKS_PORT_IN_USE" {
		t.Errorf("collision code = %q, want SOCKS_PORT_IN_USE", collision.Code)
	}
	if collision.Port != listener.Addr().(*net.TCPAddr).Port {
		t.Errorf("collision port = %d, want %d", collision.Port, listener.Addr().(*net.TCPAddr).Port)
	}
}

func TestRemoveLegacyUnboundPFDeclarations(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "removes exact legacy declarations",
			input: "rdr-anchor \"com.apple/*\"\nrdr-anchor \"com.unbound.zapret\"\nanchor \"com.apple/*\"\nanchor \"com.unbound.zapret\"\n",
			want:  "rdr-anchor \"com.apple/*\"\nanchor \"com.apple/*\"\n",
		},
		{name: "absent is no-op", input: "anchor \"com.apple/*\"\n", want: "anchor \"com.apple/*\"\n"},
		{name: "preserves unrelated anchors", input: "anchor \"com.example.proxy\"\nrdr-anchor \"com.apple/*\"\n", want: "anchor \"com.example.proxy\"\nrdr-anchor \"com.apple/*\"\n"},
		{name: "preserves similar names", input: "anchor \"com.unbound.zapret2\"\n", want: "anchor \"com.unbound.zapret2\"\n"},
		{name: "preserves similar suffix names", input: "anchor \"com.unbound.zapret-other\"\n", want: "anchor \"com.unbound.zapret-other\"\n"},
		{name: "removes duplicates", input: "anchor \"com.unbound.zapret\"\nanchor \"com.unbound.zapret\"\n", want: ""},
		{name: "fails closed on unexpected declaration", input: "anchor \"com.unbound.zapret\" all\n", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := removeLegacyUnboundPFDeclarations([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %t", err, tt.wantErr)
			}
			if !tt.wantErr && string(got) != tt.want {
				t.Errorf("result = %q, want %q", got, tt.want)
			}
		})
	}
}

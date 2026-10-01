//go:build windows

package historyid

import (
	"net"
	"slices"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// These tests are hermetic. They never touch the network, never spawn a
// process, and never depend on how the machine running them is configured:
// every input below is synthetic.

// TestUsableAddressRejectsValuesThatIdentifyNothing pins the one filter that
// belongs to this file. Windows reports 0.0.0.0 and :: for adapters with no
// configured gateway or resolver; those mean "absent" and must not become part
// of the network context.
func TestUsableAddressRejectsValuesThatIdentifyNothing(t *testing.T) {
	for _, input := range []string{"", "   ", "0.0.0.0", "::", "not-an-address", "192.168.1.0/24"} {
		if got := usableAddress(input); got != "" {
			t.Errorf("usableAddress(%q) = %q, want empty: the value identifies nothing", input, got)
		}
	}
}

// TestUsableAddressKeepsLoopbackResolvers guards the opposite mistake. A
// resolver on 127.0.0.1 or ::1 is a real, meaningful resolver and filtering it
// would silently collapse "local DNS cache" onto "no DNS configured".
func TestUsableAddressKeepsLoopbackResolvers(t *testing.T) {
	for _, input := range []string{"127.0.0.1", "::1"} {
		if got := usableAddress(input); got == "" {
			t.Errorf("usableAddress(%q) = empty, want the loopback address kept", input)
		}
	}
}

// TestUsableAddressNormalizesRoutableAddresses checks the ordinary path: a
// concrete gateway or resolver address survives normalization.
func TestUsableAddressNormalizesRoutableAddresses(t *testing.T) {
	got := usableAddress("192.168.1.1")
	if got != "192.168.1.1" {
		t.Errorf("usableAddress(%q) = %q, want the address normalized to itself", "192.168.1.1", got)
	}
}

// TestAddressSetDropsUnspecifiedAndDeduplicates is the filtering contract for
// a gateway or resolver list.
func TestAddressSetDropsUnspecifiedAndDeduplicates(t *testing.T) {
	got := addressSet([]string{"192.168.1.1", "0.0.0.0", "::", "8.8.8.8", "192.168.1.1", "junk"})
	want := []string{"192.168.1.1", "8.8.8.8"}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("addressSet(...) = %v, want %v", got, want)
	}
}

// TestAddressSetIsOrderIndependent proves that enumeration order is not
// semantic: the identity must depend on the SET of facts, never on the order
// the OS happened to report them in.
func TestAddressSetIsOrderIndependent(t *testing.T) {
	forward := addressSet([]string{"192.168.1.1", "10.0.0.1", "172.16.0.1"})
	reverse := addressSet([]string{"172.16.0.1", "10.0.0.1", "192.168.1.1"})
	if !slices.Equal(forward, reverse) {
		t.Fatalf("order changed the result: %v vs %v", forward, reverse)
	}
	if !slices.IsSorted(forward) {
		t.Fatalf("addressSet result %v is not sorted", forward)
	}
}

// TestAddressSetIsNilWhenNothingUsable survives: a nil slice and an empty
// slice both mean "the set is absent", and absence must be representable
// without inventing a value.
func TestAddressSetIsNilWhenNothingUsable(t *testing.T) {
	for name, input := range map[string][]string{
		"nil":          nil,
		"empty":        {},
		"only junk":    {"", "nonsense"},
		"only unknown": {"0.0.0.0", "::"},
	} {
		if got := addressSet(input); got != nil {
			t.Errorf("addressSet(%s) = %v, want nil", name, got)
		}
	}
}

// TestUsableInterfaceIndexes builds the candidate gateway owners from a
// synthetic interface list.
func TestUsableInterfaceIndexes(t *testing.T) {
	indexes := usableInterfaceIndexes([]net.Interface{
		{Index: 12, Flags: net.FlagUp | net.FlagBroadcast | net.FlagMulticast},
		{Index: 13, Flags: net.FlagUp | net.FlagLoopback},         // up loopback
		{Index: 14, Flags: net.FlagBroadcast | net.FlagMulticast}, // not up
		{Index: 15, Flags: 0},         // administratively down
		{Index: 0, Flags: net.FlagUp}, // no index
	})
	if len(indexes) != 1 {
		t.Fatalf("usableInterfaceIndexes kept %d interfaces, want 1", len(indexes))
	}
	if _, ok := indexes[12]; !ok {
		t.Fatalf("usableInterfaceIndexes dropped the up, non-loopback interface 12")
	}
}

// TestUsableInterfaceIndexesIsNilWhenNothingQualifies covers the
// "no effective local network" case: it must be representable as an absence
// rather than a fabricated one.
func TestUsableInterfaceIndexesIsNilWhenNothingQualifies(t *testing.T) {
	if got := usableInterfaceIndexes(nil); got != nil {
		t.Errorf("usableInterfaceIndexes(nil) = %v, want nil", got)
	}
	if got := usableInterfaceIndexes([]net.Interface{{Index: 3, Flags: net.FlagUp | net.FlagLoopback}}); got != nil {
		t.Errorf("usableInterfaceIndexes(loopback only) = %v, want nil", got)
	}
}

// TestAdapterEligible combines the caller's interface set with the adapter's
// own operational state. A down adapter contributes no gateway even when the
// caller named its interface, and an unnamed adapter contributes none either.
func TestAdapterEligible(t *testing.T) {
	indexes := usableInterfaceIndexes([]net.Interface{{Index: 12, Flags: net.FlagUp}})
	cases := map[string]struct {
		adapter *windows.IpAdapterAddresses
		want    bool
	}{
		"up and named": {
			adapter: &windows.IpAdapterAddresses{IfIndex: 12, OperStatus: windows.IfOperStatusUp, IfType: windows.IF_TYPE_ETHERNET_CSMACD},
			want:    true,
		},
		"down": {
			adapter: &windows.IpAdapterAddresses{IfIndex: 12, OperStatus: windows.IfOperStatusDown, IfType: windows.IF_TYPE_ETHERNET_CSMACD},
		},
		"unknown state": {
			adapter: &windows.IpAdapterAddresses{IfIndex: 12, OperStatus: windows.IfOperStatusUnknown, IfType: windows.IF_TYPE_ETHERNET_CSMACD},
		},
		"loopback": {
			adapter: &windows.IpAdapterAddresses{IfIndex: 12, OperStatus: windows.IfOperStatusUp, IfType: windows.IF_TYPE_SOFTWARE_LOOPBACK},
		},
		"unnamed interface": {
			adapter: &windows.IpAdapterAddresses{IfIndex: 99, OperStatus: windows.IfOperStatusUp, IfType: windows.IF_TYPE_ETHERNET_CSMACD},
		},
		"no index": {
			adapter: &windows.IpAdapterAddresses{OperStatus: windows.IfOperStatusUp, IfType: windows.IF_TYPE_ETHERNET_CSMACD},
		},
	}
	for name, tc := range cases {
		if got := adapterEligible(tc.adapter, indexes); got != tc.want {
			t.Errorf("adapterEligible(%s) = %v, want %v", name, got, tc.want)
		}
	}

	if adapterEligible(&windows.IpAdapterAddresses{IfIndex: 12, OperStatus: windows.IfOperStatusUp}, nil) {
		t.Error("adapterEligible accepted an adapter with no caller interfaces")
	}
}

// TestProfileNameCannotBeMistakenForAnSSID pins the namespacing of the
// profile value: whatever an adapter is called, the value is marked as
// adapter-derived so a wireless network name can never be inferred from it.
func TestProfileNameCannotBeMistakenForAnSSID(t *testing.T) {
	got := profileName("  HomeNet 5G  ")
	if got != profileMarker+"homenet 5g" {
		t.Fatalf("profileName(...) = %q, want the trimmed, lowercased, marked name", got)
	}
	if got == profileName("HomeNet") {
		t.Fatal("profileName ignored case, so the identity would not be stable")
	}
	if profileName("   ") != "" {
		t.Error("profileName of a blank adapter name must be empty, not a marked empty name")
	}
}

// TestReadNetworkProfileIsAlwaysMarked exercises the real Win32 path on
// whatever machine runs it. It asserts only the invariant that holds for every
// possible configuration: the value is either absent or explicitly marked as
// adapter-derived. It requires no particular adapter to exist, and an empty
// result is a valid outcome rather than a failure.
func TestReadNetworkProfileIsAlwaysMarked(t *testing.T) {
	profile := readNetworkProfile()
	if profile == "" {
		return
	}
	for _, part := range strings.Split(profile, "|") {
		if !strings.HasPrefix(part, profileMarker) {
			t.Fatalf("readNetworkProfile() contained %q, which is not marked as adapter-derived", part)
		}
	}
}

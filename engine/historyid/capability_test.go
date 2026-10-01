package historyid

import (
	"regexp"
	"strings"
	"testing"

	"unbound/engine/backendcap"
	"unbound/engine/strategyir"
)

var capabilityFormat = regexp.MustCompile(`^capability-v1-[0-9a-f]{64}$`)

func mustCapabilityFingerprint(t *testing.T, capabilities backendcap.Capabilities) CapabilityFingerprint {
	t.Helper()
	fingerprint, err := CapabilityFingerprintFor(capabilities)
	if err != nil {
		t.Fatalf("CapabilityFingerprintFor(%q) returned error: %v", capabilities.Backend, err)
	}
	return fingerprint
}

// reversed returns a reversed copy, leaving the input untouched. Reversing is
// the cheapest shuffle that provably changes the order of every set.
func reversed[T any](values []T) []T {
	if values == nil {
		return nil
	}
	out := make([]T, len(values))
	for i, value := range values {
		out[len(values)-1-i] = value
	}
	return out
}

// reorderedCapabilities returns the same advertised capabilities with every set
// in the opposite order. It is a deep copy: the input must not be disturbed.
func reorderedCapabilities(capabilities backendcap.Capabilities) backendcap.Capabilities {
	out := capabilities
	out.Profile.Transports = reversed(capabilities.Profile.Transports)
	out.Profile.ApplicationProtocols = reversed(capabilities.Profile.ApplicationProtocols)
	out.Profile.IPFamilies = reversed(capabilities.Profile.IPFamilies)
	out.Profile.Operations = reversed(capabilities.Profile.Operations)
	out.Profile.PositionAnchors = reversed(capabilities.Profile.PositionAnchors)
	out.Profile.FakePayloadRefs = reversed(capabilities.Profile.FakePayloadRefs)
	out.Profile.RangeDirections = reversed(capabilities.Profile.RangeDirections)
	out.Profile.RangeCounters = reversed(capabilities.Profile.RangeCounters)
	out.Profile.LuaModules = reversed(capabilities.Profile.LuaModules)
	out.Profile.LuaFunctions = reversed(capabilities.Profile.LuaFunctions)
	out.Capture.Transports = reversed(capabilities.Capture.Transports)
	out.Capture.Directions = reversed(capabilities.Capture.Directions)
	out.Capture.IPFamilies = reversed(capabilities.Capture.IPFamilies)
	return out
}

// emptySetCapabilities replaces every set with an explicit empty, NON nil
// slice. Most of these fields carry omitempty, so this is exactly the case that
// would change the marshalled bytes of the same semantic value.
func emptySetCapabilities(capabilities backendcap.Capabilities) backendcap.Capabilities {
	out := capabilities
	out.Profile.Transports = []strategyir.Transport{}
	out.Profile.ApplicationProtocols = []strategyir.ApplicationProtocol{}
	out.Profile.IPFamilies = []strategyir.IPFamily{}
	out.Profile.Operations = []strategyir.OperationKind{}
	out.Profile.PositionAnchors = []strategyir.PositionAnchor{}
	out.Profile.FakePayloadRefs = []string{}
	out.Profile.RangeDirections = []strategyir.RangeDirection{}
	out.Profile.RangeCounters = []strategyir.RangeCounter{}
	out.Profile.LuaModules = []string{}
	out.Profile.LuaFunctions = []string{}
	out.Capture.Transports = []backendcap.CaptureTransport{}
	out.Capture.Directions = []strategyir.Direction{}
	out.Capture.IPFamilies = []strategyir.IPFamily{}
	return out
}

func TestCapabilityFingerprintFormatIsVersionedLowercaseHex(t *testing.T) {
	for _, backend := range []backendcap.Backend{
		backendcap.Zapret2Windows,
		backendcap.Zapret2Linux,
		backendcap.Zapret1TPWSDarwin,
		backendcap.NativeWindows,
		backendcap.Backend(""),
	} {
		fingerprint := mustCapabilityFingerprint(t, backendcap.Get(backend))
		if !capabilityFormat.MatchString(string(fingerprint)) {
			t.Fatalf("fingerprint for %q = %q, want capability-v1- followed by 64 lowercase hex", backend, fingerprint)
		}
		if !fingerprint.Available() {
			t.Fatalf("fingerprint for %q = %q reports unavailable", backend, fingerprint)
		}
	}
}

func TestCapabilityFingerprintIgnoresSetOrder(t *testing.T) {
	for _, backend := range []backendcap.Backend{
		backendcap.Zapret2Windows,
		backendcap.Zapret2Linux,
		backendcap.Zapret1TPWSDarwin,
	} {
		capabilities := backendcap.Get(backend)
		stable := mustCapabilityFingerprint(t, capabilities)
		shuffled := mustCapabilityFingerprint(t, reorderedCapabilities(capabilities))
		if stable != shuffled {
			t.Fatalf("fingerprint for %q depends on set order: %q != %q", backend, stable, shuffled)
		}
		// Reordering must not have disturbed the source value.
		if again := mustCapabilityFingerprint(t, capabilities); again != stable {
			t.Fatalf("reordering mutated the source capabilities for %q: %q != %q", backend, again, stable)
		}
	}
}

func TestCapabilityFingerprintIgnoresRepeatedMembers(t *testing.T) {
	capabilities := backendcap.Get(backendcap.Zapret2Windows)
	duplicated := capabilities
	duplicated.Profile.Transports = []strategyir.Transport{
		strategyir.TransportQUIC, strategyir.TransportTCP, strategyir.TransportUDP,
		strategyir.TransportTCP, strategyir.TransportQUIC,
	}
	baseline := mustCapabilityFingerprint(t, capabilities)
	if got := mustCapabilityFingerprint(t, duplicated); got != baseline {
		t.Fatalf("duplicate members changed the fingerprint: %q != %q", got, baseline)
	}
}

func TestCapabilityFingerprintTreatsNilAndEmptySetsIdentically(t *testing.T) {
	absent := backendcap.Capabilities{Backend: backendcap.NativeLinux}
	present := emptySetCapabilities(absent)
	nilFingerprint := mustCapabilityFingerprint(t, absent)
	emptyFingerprint := mustCapabilityFingerprint(t, present)
	if nilFingerprint != emptyFingerprint {
		t.Fatalf("nil and empty sets produced different fingerprints: %q != %q", nilFingerprint, emptyFingerprint)
	}
	// A single empty member is still a real advertisement and must not collapse
	// into the absent case.
	single := absent
	single.Profile.LuaModules = []string{""}
	if got := mustCapabilityFingerprint(t, single); got == nilFingerprint {
		t.Fatal("a set with one empty member collided with an absent set")
	}
}

func TestCapabilityFingerprintChangesWithASingleCapability(t *testing.T) {
	baseline := backendcap.Get(backendcap.Zapret2Windows)
	baselineFingerprint := mustCapabilityFingerprint(t, baseline)

	quicOff := baseline
	quicOff.Profile.QUIC = false

	extraOperation := baseline
	extraOperation.Profile.Operations = append(
		append([]strategyir.OperationKind(nil), baseline.Profile.Operations...),
		strategyir.OperationDisorder,
	)

	fewerCaptureTransports := baseline
	fewerCaptureTransports.Capture.Transports = []backendcap.CaptureTransport{backendcap.CaptureTransportTCP}

	differentCaptureKind := baseline
	differentCaptureKind.Capture.BackendKind = backendcap.CaptureNFQUEUE

	renamedLuaFunction := baseline
	renamedLuaFunction.Profile.LuaFunctions = append([]string(nil), baseline.Profile.LuaFunctions...)
	renamedLuaFunction.Profile.LuaFunctions[0] = "not-a-declared-function"

	changed := map[string]backendcap.Capabilities{
		"quic_disabled":            quicOff,
		"extra_operation":          extraOperation,
		"fewer_capture_transports": fewerCaptureTransports,
		"different_capture_kind":   differentCaptureKind,
		"renamed_lua_function":     renamedLuaFunction,
	}
	for name, capabilities := range changed {
		if got := mustCapabilityFingerprint(t, capabilities); got == baselineFingerprint {
			t.Fatalf("%s did not change the fingerprint %q", name, baselineFingerprint)
		}
	}
}

func TestCapabilityFingerprintDistinguishesBackends(t *testing.T) {
	backends := []backendcap.Backend{
		backendcap.Zapret2Windows,
		backendcap.Zapret2Linux,
		backendcap.Zapret1TPWSDarwin,
		backendcap.NativeWindows,
		backendcap.NativeLinux,
		backendcap.NativeDarwin,
		backendcap.Backend(""),
	}
	seen := make(map[CapabilityFingerprint]backendcap.Backend, len(backends))
	for _, backend := range backends {
		fingerprint := mustCapabilityFingerprint(t, backendcap.Get(backend))
		if previous, clash := seen[fingerprint]; clash {
			t.Fatalf("backends %q and %q share fingerprint %q", previous, backend, fingerprint)
		}
		seen[fingerprint] = backend
	}
}

func TestCapabilityFingerprintIsDeterministicForEmptyAdvertisement(t *testing.T) {
	first := mustCapabilityFingerprint(t, backendcap.Capabilities{})
	second := mustCapabilityFingerprint(t, backendcap.Capabilities{})
	if first != second {
		t.Fatalf("empty advertisement is not deterministic: %q != %q", first, second)
	}
	if !first.Available() {
		t.Fatalf("empty advertisement produced an unavailable fingerprint %q", first)
	}
}

func TestCapabilityFingerprintRejectsUnencodableValues(t *testing.T) {
	cases := map[string]func(*backendcap.Capabilities){
		"line_break_in_set_member": func(c *backendcap.Capabilities) {
			c.Profile.Transports = []strategyir.Transport{"TCP\nUDP"}
		},
		"comma_in_set_member": func(c *backendcap.Capabilities) {
			c.Profile.FakePayloadRefs = []string{"a,b"}
		},
		"count_separator_in_set_member": func(c *backendcap.Capabilities) {
			c.Profile.LuaFunctions = []string{"a|b"}
		},
		"line_break_in_backend_id": func(c *backendcap.Capabilities) {
			c.Backend = "zapret2/\r\nwindows"
		},
		"line_break_in_capture_kind": func(c *backendcap.Capabilities) {
			c.Capture.BackendKind = "WINDIVERT\nNFQUEUE"
		},
	}
	for name, mutate := range cases {
		capabilities := backendcap.Get(backendcap.Zapret2Windows)
		mutate(&capabilities)
		fingerprint, err := CapabilityFingerprintFor(capabilities)
		if err == nil {
			t.Fatalf("%s: expected an error, got fingerprint %q", name, fingerprint)
		}
		if fingerprint != "" {
			t.Fatalf("%s: error returned a fingerprint %q", name, fingerprint)
		}
	}
}

func TestCapabilityCanonicalFormIsFullyOrderedAndComplete(t *testing.T) {
	canonical, err := capabilityCanonical(backendcap.Get(backendcap.Zapret2Windows))
	if err != nil {
		t.Fatalf("capabilityCanonical returned error: %v", err)
	}
	rendered := string(canonical)
	for _, expected := range []string{
		"capability-source-v1\n",
		"schema=" + capabilitySchema + "\n",
		"backend=" + string(backendcap.Zapret2Windows) + "\n",
		"profile.transports=3|QUIC,TCP,UDP\n",
		"profile.quic=true\n",
		"capture.backend_kind=WINDIVERT\n",
		"capture.ip_families=2|IPv4,IPv6\n",
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("canonical capability form is missing %q:\n%s", expected, rendered)
		}
	}
	// A field that the backend does not advertise is written as an explicitly
	// empty set, never omitted.
	emptyCanonical, err := capabilityCanonical(backendcap.Capabilities{Backend: backendcap.NativeDarwin})
	if err != nil {
		t.Fatalf("capabilityCanonical returned error: %v", err)
	}
	for _, expected := range []string{
		"profile.lua_modules=0|\n",
		"profile.fake_payload_refs=0|\n",
		"capture.transports=0|\n",
		"profile.quic=false\n",
		"capture.backend_kind=\n",
	} {
		if !strings.Contains(string(emptyCanonical), expected) {
			t.Fatalf("canonical capability form is missing %q:\n%s", expected, emptyCanonical)
		}
	}
}

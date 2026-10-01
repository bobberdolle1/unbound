package historyid

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"unbound/engine/backendcap"
)

// capabilitySchema versions the canonical encoding below. It is hashed as
// source material, not merely prefixed onto the output, so a future re-encoding
// can never collide with a fingerprint persisted under the current one.
const capabilitySchema = "backendcap-capabilities-v1"

// CapabilityFingerprintFor derives the opaque capability fingerprint from the
// backend's advertised static capabilities.
//
// It answers exactly one question: does this backend advertise the same
// semantic ability set? That is a different question from the one
// BackendFingerprint answers. Two builds of the same upstream bundle advertise
// the same abilities, and two different bundles may advertise the same ones, so
// reuse requires BOTH identities to match, never one or the other.
//
// WHAT IS NOT IN HERE. Provenance (upstream tag and commit) is excluded on
// purpose: it is implementation identity, not advertised ability, and the
// backend fingerprint already carries it. Timestamps, DNS, remote edges, the
// current target, process id, queue number and temp asset paths are excluded
// because they are host or run state, not static backend description.
func CapabilityFingerprintFor(capabilities backendcap.Capabilities) (CapabilityFingerprint, error) {
	canonical, err := capabilityCanonical(capabilities)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return CapabilityFingerprint(formatIdentity(capabilityPrefix, sum[:])), nil
}

// capabilityCanonical renders the advertised capability set into a stable byte
// string. It is hand written and line oriented for the same reasons as
// contextSource.canonical: no struct tags, no omitempty ambiguity, and no
// possibility of a map iteration order leaking into the digest.
//
// EVERY SLICE IS NORMALIZED BEFORE HASHING. The capability structs carry
// omitempty on most slice fields, which means a nil slice and an empty slice
// marshal to DIFFERENT bytes for the same semantic value. Hashing the structs
// as they were built would therefore make the fingerprint depend on an accident
// of construction rather than on what the backend advertises. Every slice is
// sorted and de-duplicated first, and every set is written with an explicit
// element count, so "no such capability" has exactly one representation.
//
// ORDER IS NOT SEMANTIC. Transports, application protocols, IP families,
// operations, position anchors, fake payload references, range directions,
// range counters, Lua modules, Lua functions and the capture sets are unordered
// sets. Two capability values that list the same members in a different order
// must produce byte identical canonical form, and they do.
func capabilityCanonical(capabilities backendcap.Capabilities) ([]byte, error) {
	profile := capabilities.Profile
	capture := capabilities.Capture

	var b strings.Builder
	b.WriteString("capability-source-v1\n")
	if err := writeScalar(&b, "schema", capabilitySchema); err != nil {
		return nil, err
	}
	if err := writeScalar(&b, "backend", string(capabilities.Backend)); err != nil {
		return nil, err
	}
	sets := []struct {
		name   string
		values []string
	}{
		{"profile.transports", sortedSet(profile.Transports)},
		{"profile.application_protocols", sortedSet(profile.ApplicationProtocols)},
		{"profile.ip_families", sortedSet(profile.IPFamilies)},
		{"profile.operations", sortedSet(profile.Operations)},
		{"profile.position_anchors", sortedSet(profile.PositionAnchors)},
		{"profile.fake_payload_refs", sortedSet(profile.FakePayloadRefs)},
		{"profile.range_directions", sortedSet(profile.RangeDirections)},
		{"profile.range_counters", sortedSet(profile.RangeCounters)},
		{"profile.lua_modules", sortedSet(profile.LuaModules)},
		{"profile.lua_functions", sortedSet(profile.LuaFunctions)},
		{"capture.transports", sortedSet(capture.Transports)},
		{"capture.directions", sortedSet(capture.Directions)},
		{"capture.ip_families", sortedSet(capture.IPFamilies)},
	}
	scalars := []struct {
		name  string
		value string
	}{
		{"profile.multi_protocol_payload_filter", strconv.FormatBool(profile.MultiProtocolPayloadFilter)},
		{"profile.port_ranges", strconv.FormatBool(profile.PortRanges)},
		{"profile.managed_hostlists", strconv.FormatBool(profile.ManagedHostlists)},
		{"profile.auto_hostlists", strconv.FormatBool(profile.AutoHostlists)},
		{"profile.ip_set_references", strconv.FormatBool(profile.IPSetReferences)},
		{"profile.quic", strconv.FormatBool(profile.QUIC)},
		{"profile.fake_payloads", strconv.FormatBool(profile.FakePayloads)},
		{"profile.fake_repeat", strconv.FormatBool(profile.FakeRepeat)},
		{"profile.fake_ttl", strconv.FormatBool(profile.FakeTTL)},
		{"profile.fake_sequence_offset", strconv.FormatBool(profile.FakeSequenceOffset)},
		{"profile.fake_acknowledgment_offset", strconv.FormatBool(profile.FakeAcknowledgmentOffset)},
		{"profile.fake_tcp_md5", strconv.FormatBool(profile.FakeTCPMD5)},
		{"profile.fake_tcp_timestamp", strconv.FormatBool(profile.FakeTCPTimestamp)},
		{"capture.backend_kind", string(capture.BackendKind)},
	}
	for _, entry := range sets {
		if err := writeSet(&b, entry.name, entry.values); err != nil {
			return nil, err
		}
	}
	for _, entry := range scalars {
		if err := writeScalar(&b, entry.name, entry.value); err != nil {
			return nil, err
		}
	}
	return []byte(b.String()), nil
}

// writeSet renders a sorted, de-duplicated set. The element count is written
// explicitly so that an empty set, a one element set and a set whose single
// member is the empty string can never render to the same bytes.
func writeSet(b *strings.Builder, name string, values []string) error {
	for _, value := range values {
		if err := setElementEncodable(name, value); err != nil {
			return err
		}
	}
	b.WriteString(name)
	b.WriteByte('=')
	b.WriteString(strconv.Itoa(len(values)))
	b.WriteByte('|')
	b.WriteString(strings.Join(values, ","))
	b.WriteByte('\n')
	return nil
}

// writeScalar renders a single value. Every scalar is always written, including
// the empty one, so a field can never be confused with an absent field.
func writeScalar(b *strings.Builder, name, value string) error {
	if err := scalarEncodable(name, value); err != nil {
		return err
	}
	b.WriteString(name)
	b.WriteByte('=')
	b.WriteString(value)
	b.WriteByte('\n')
	return nil
}

// scalarEncodable rejects a value that would break the line oriented canonical
// form and thereby let two different capability sets hash to the same value.
func scalarEncodable(name, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("capability field %s contains a line break", name)
	}
	return nil
}

// setElementEncodable additionally rejects the set separators, so no member can
// imitate the encoding of a different set.
func setElementEncodable(name, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("capability field %s contains a line break", name)
	}
	if strings.ContainsAny(value, ",|") {
		return fmt.Errorf("capability field %s contains a set separator", name)
	}
	return nil
}

// sortedSet renders any slice of string like values as a sorted, de-duplicated
// set of strings. A nil slice and an empty slice both mean "advertises nothing"
// and both produce an empty, non nil result.
func sortedSet[T ~string](values []T) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = string(value)
	}
	sort.Strings(out)
	unique := out[:0]
	for i, value := range out {
		if i == 0 || value != out[i-1] {
			unique = append(unique, value)
		}
	}
	return unique
}

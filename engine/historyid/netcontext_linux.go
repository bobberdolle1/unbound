//go:build linux

package historyid

import (
	"bytes"
	"net"
	"os"
	"strconv"
	"strings"
)

// Linux network facts are read from procfs and /etc only. Nothing here shells
// out to ip, nmcli, resolvectl or any other helper: a shell-out is a second
// process whose behaviour depends on PATH, locale, D-Bus state and can block
// indefinitely, which would make identity derivation hang. procfs is a kernel
// interface, always present when the kernel is Linux, and always read-only to
// us.
//
// Nothing in this file mutates machine state: it opens files O_RDONLY and
// parses text. The values it produces stay in memory as HMAC input only.

// procNetRoutePaths are tried in order. /proc/net is a symlink into the
// network namespace's procfs, and /proc/self/net is the namespace-explicit
// spelling of the same file; both normally resolve identically.
var procNetRoutePaths = [...]string{"/proc/net/route", "/proc/self/net/route"}

const resolvConfPath = "/etc/resolv.conf"

// readGateways returns the default-route gateway addresses observed for the
// given interfaces, read-only from procfs.
func readGateways(ifaces []net.Interface) []string {
	var raw []byte
	for _, path := range procNetRoutePaths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		raw = data
		break
	}
	if raw == nil {
		// Absent is acceptable: no gateways simply contributes nothing to the
		// identity. It must not fail the whole identity.
		return nil
	}

	allowed := make(map[string]struct{}, len(ifaces))
	for i := range ifaces {
		allowed[ifaces[i].Name] = struct{}{}
	}

	var out []string
	// parseProcNetRoutePairs already normalized each gateway; keep only the
	// interfaces the caller asked about.
	for _, pair := range parseProcNetRoutePairs(raw) {
		if _, ok := allowed[pair.iface]; !ok {
			continue
		}
		out = append(out, pair.gateway)
	}
	return sortedUnique(out)
}

// routeGateway is one default-route row from /proc/net/route.
type routeGateway struct {
	iface   string
	gateway string
}

// parseProcNetRoute returns the gateways of the default routes described by a
// /proc/net/route dump, as dotted-quad strings. It is the pure half of
// readGateways, so it can be exercised with synthetic input.
func parseProcNetRoute(data []byte) []string {
	pairs := parseProcNetRoutePairs(data)
	if len(pairs) == 0 {
		return nil
	}
	out := make([]string, 0, len(pairs))
	for i := range pairs {
		out = append(out, pairs[i].gateway)
	}
	// The kernel's row order is not semantic, so the set is canonicalized here
	// rather than relying on the caller.
	return sortAndDedup(out)
}

// parseProcNetRoutePairs parses a /proc/net/route dump.
//
// Format: one header line, then one route per line of whitespace-separated
// fields, the first five being
//
//	Iface  Destination  Gateway  Flags  RefCnt  Use  Metric  Mask  MTU ...
//
// Destination and Gateway are 8 hex digits holding a 32-bit address in HOST
// byte order on a little-endian machine, which is what every Linux port of
// /proc/net/route emits: the four hex byte pairs are little-endian, so
// "0102A8C0" is 192.168.2.1, not 1.2.168.192.
//
// Only the default route (Destination == 0) counts as a gateway. A zero
// Gateway means the route is on-link: there is no next hop to observe, and
// "0.0.0.0" is never a gateway address.
func parseProcNetRoutePairs(data []byte) []routeGateway {
	lines := bytes.Split(data, []byte{'\n'})
	var out []routeGateway
	for _, line := range lines {
		fields := strings.Fields(string(line))
		if len(fields) < 3 {
			continue
		}
		iface := fields[0]
		if fields[1] != "00000000" {
			continue
		}
		if fields[2] == "00000000" {
			// On-link default route: no gateway exists.
			continue
		}
		addr, ok := parseRouteHexIPv4(fields[2])
		if !ok {
			continue
		}
		normalized := normalizeAddress(addr)
		if normalized == "" {
			continue
		}
		out = append(out, routeGateway{iface: iface, gateway: normalized})
	}
	return out
}

// parseRouteHexIPv4 decodes the 8 hex digits of a /proc/net/route address,
// which are the 4 address bytes in little-endian order.
func parseRouteHexIPv4(field string) (string, bool) {
	if len(field) != 8 {
		return "", false
	}
	v, err := strconv.ParseUint(field, 16, 32)
	if err != nil {
		return "", false
	}
	// Little-endian: least significant byte first.
	return net.IPv4(byte(v), byte(v>>8), byte(v>>16), byte(v>>24)).String(), true
}

// readResolvers returns the configured DNS resolver addresses, read-only.
func readResolvers() []string {
	data, err := os.ReadFile(resolvConfPath)
	if err != nil {
		return nil
	}
	return parseResolvConf(data)
}

// parseResolvConf is the pure half of readResolvers: it returns the
// nameserver addresses of a resolv.conf document.
//
// A nameserver value may carry a zone (a link-local address with a scope, or a
// nameserver bound to an interface), as in "fe80::1%eth0"; the zone is dropped
// because it identifies the host's interface naming, not the resolver, and it
// would make the identity depend on an interface name that OS enumeration order
// already excludes from the canonical form. Loopback resolvers such as the
// 127.0.0.53 systemd-resolved stub are kept: they are a real fact about this
// machine's configuration, and excluding them would make the identity depend on
// which resolver implementation happens to be installed.
func parseResolvConf(data []byte) []string {
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if hash := strings.IndexByte(line, '#'); hash >= 0 {
			line = line[:hash]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] != "nameserver" || len(fields) < 2 {
			continue
		}
		value := strings.TrimSpace(fields[1])
		if zone := strings.IndexByte(value, '%'); zone >= 0 {
			value = value[:zone]
		}
		if value == "" {
			continue
		}
		normalized := normalizeAddress(value)
		if normalized == "" {
			continue
		}
		out = append(out, normalized)
	}
	return sortedUnique(out)
}

// readNetworkProfile returns an OS network profile identity, or "".
//
// Linux has no cheap, stable, shell-free equivalent of the Windows
// NetworkListManager profile. The strong signal, the NetworkManager connection
// profile name, is only reachable through nmcli or D-Bus, and the systemd
// resolved state through resolvectl; both are separate processes that can block
// indefinitely, and neither is present on a host that does not use those
// components. Guessing a value instead would be exactly the fabricated-fallback
// behaviour this package must never have.
//
// The gateway and resolver sets already distinguish the networks that matter,
// so the absent profile is reported as absent rather than invented.
func readNetworkProfile() string {
	return ""
}

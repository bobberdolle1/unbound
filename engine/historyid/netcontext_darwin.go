//go:build darwin

package historyid

import (
	"net"
	"os"
	"strings"
)

// Darwin facts are read from the filesystem only. Nothing here shells out to
// netstat, route or scutil: a shell-out is a second process whose behaviour
// depends on PATH and locale, and the routing socket (AF_ROUTE) is privileged,
// so neither is a dependable read-only observation.
//
// Nothing in this file mutates machine state: it opens files O_RDONLY and
// parses text. The values it produces stay in memory as HMAC input only.

const resolvConfPath = "/etc/resolv.conf"

// readGateways returns the default-route gateway addresses observed for the
// given interfaces, read-only.
//
// On Darwin the default route's gateway lives only in the kernel routing
// database, reachable through the privileged routing socket. There is no
// unprivileged file that contains it. Rather than fabricate a gateway or shell
// out, the set is reported as absent: an absent gateway can only make history
// reuse less likely, never more.
func readGateways(_ []net.Interface) []string {
	return nil
}

// readResolvers returns the configured DNS resolver addresses, read-only.
//
// Unlike a netconfig-managed host, Darwin has no resolver multiplexer to ask:
// the file is the configuration, so reading it is both complete and free.
func readResolvers() []string {
	data, err := os.ReadFile(resolvConfPath)
	if err != nil {
		return nil
	}
	return parseResolvConfText(data)
}

// parseResolvConfText returns the nameserver addresses of a resolv.conf
// document. A value may carry a scope zone, as in "fe80::1%en0"; the zone is
// dropped because it names a local interface rather than the resolver.
func parseResolvConfText(data []byte) []string {
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if hash := strings.IndexByte(line, '#'); hash >= 0 {
			line = line[:hash]
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "nameserver" {
			continue
		}
		value := fields[1]
		if zone := strings.IndexByte(value, '%'); zone >= 0 {
			value = value[:zone]
		}
		normalized := normalizeAddress(value)
		if normalized == "" {
			continue
		}
		out = append(out, normalized)
	}
	return sortAndDedup(out)
}

// readNetworkProfile returns an OS network profile identity, or "".
//
// Darwin's location and network configuration are reachable only through
// SystemConfiguration's dynamic store, which is reachable only through a
// process that talks to it. Guessing a value would be exactly the fabricated
// fallback this package must never have, so the profile is reported as absent.
func readNetworkProfile() string {
	return ""
}

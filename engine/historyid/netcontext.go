package historyid

import (
	"context"
	"net"
	"runtime"
	"strings"
)

// NetworkContextProvider produces the opaque local network context identity.
//
// READ-ONLY CONTRACT. A provider observes and nothing else. It never changes
// DNS, routes, proxies, interface configuration, Wi-Fi, the firewall or a VPN;
// it never connects or disconnects anything; it never restarts or signals a
// network service. Reading local network configuration must be unobservable to
// the network and to the user's configuration; an identity that only exists to
// make historical reuse more conservative must never be able to change anything.
type NetworkContextProvider interface {
	Identity(ctx context.Context) (NetworkContextIdentity, error)
}

// SystemNetworkContextProvider derives the identity from the live local
// network configuration of this machine.
type SystemNetworkContextProvider struct {
	// Key is the local, randomly generated HMAC key. It never leaves the host.
	Key []byte
}

var _ NetworkContextProvider = SystemNetworkContextProvider{}

// Identity observes the local network context and reduces it to an opaque
// identity. The observed facts exist only as HMAC input and are never returned,
// logged or persisted.
func (p SystemNetworkContextProvider) Identity(ctx context.Context) (NetworkContextIdentity, error) {
	if err := ctx.Err(); err != nil {
		return "", &ErrUnavailable{Identity: "context", Reason: "cancelled before observation"}
	}
	source, err := readContextSource()
	if err != nil {
		return "", err
	}
	return ContextKey(p.Key, source)
}

// readContextSource collects and canonicalizes the local network facts.
//
// Every field is a set, because OS enumeration order is not semantic. An empty
// gateway, resolver or profile set means "absent", which is a legitimate state:
// it makes the identity less likely to match a historical one, never more.
func readContextSource() (contextSource, error) {
	source := contextSource{Platform: runtime.GOOS}

	ifaces, err := net.Interfaces()
	if err != nil {
		return source, &ErrUnavailable{Identity: "context", Reason: "local interfaces unavailable"}
	}

	// The interface set is the minimum evidence of an effective local network.
	// The index is included because it is part of what the OS reports about the
	// interface, and it does not leak a raw network fact: it is an OS-local
	// counter, not an address, and it never leaves this package in clear text.
	var names []string
	for i := range ifaces {
		iface := &ifaces[i]
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if !hasGlobalUnicast(*iface) {
			continue
		}
		names = append(names, iface.Name+"|"+itoa(iface.Index))
	}
	source.DefaultInterface = sortedUnique(names)

	source.Gateway = sortedUnique(readGateways(ifaces))
	source.Resolver = sortedUnique(readResolvers())
	source.Profile = readNetworkProfile()

	if len(source.DefaultInterface) == 0 {
		return source, &ErrUnavailable{
			Identity: "context",
			Reason:   "no active non-loopback interface with a global unicast address",
		}
	}
	return source, nil
}

// hasGlobalUnicast reports whether the interface carries at least one routable
// address. Loopback and link-local addresses are excluded: they say nothing
// about the network this host is actually attached to.
func hasGlobalUnicast(iface net.Interface) bool {
	addrs, err := iface.Addrs()
	if err != nil {
		return false
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		if ipNet.IP.IsGlobalUnicast() && !ipNet.IP.IsLinkLocalUnicast() {
			return true
		}
	}
	return false
}

// itoa renders a non-negative int without pulling in strconv for one call.
func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var digits [20]byte
	i := len(digits)
	for v > 0 {
		i--
		digits[i] = byte('0' + v%10)
		v /= 10
	}
	return string(digits[i:])
}

// normalizeAddress canonicalizes a host or address string so that the same
// resolver or gateway always produces the same canonical form. A value that is
// not an IP is kept (lowercased) rather than discarded: a hostname is still a
// real configuration fact, and dropping it would silently weaken the identity.
func normalizeAddress(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return host
}

// sortAndDedup reduces values to the canonical set form used by contextSource.
func sortAndDedup(values []string) []string { return sortedUnique(values) }

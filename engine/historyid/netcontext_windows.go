//go:build windows

package historyid

import (
	"net"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// WINDOWS NETWORK FACTS.
//
// Everything below is a read-only call into iphlpapi through
// golang.org/x/sys/windows. There is deliberately NO subprocess of any kind:
// no PowerShell, no netsh, no cmd. docs/release/v0.7.0-acceptance.md records
// ENVIRONMENT_SPECIFIC_NETTCPIP_PROVIDER_HANG, where the NetTCPIP PowerShell
// providers hang indefinitely on some lab hosts. A desktop application that
// blocks forever on a network-state query is a far worse failure than a
// context identity that is simply absent, and an absent identity only makes
// history reuse more conservative.
//
// Nothing here mutates machine state: no route, DNS, proxy, interface, Wi-Fi,
// firewall or VPN change is requested, and no network service is restarted.
//
// Every string this file produces is derived from an address or an adapter
// name that only ever exists in memory as HMAC input. Nothing is logged and
// nothing is persisted.

// Enumeration flag sets. Each is as narrow as the caller needs, because a
// narrower request is a smaller buffer and less data in this process. Note
// what is deliberately NOT skipped: GAA_FLAG_SKIP_DNS_SERVER is left off for
// resolvers, and GAA_FLAG_SKIP_FRIENDLY_NAME is left off for the profile.
const (
	gatewayFlags = windows.GAA_FLAG_SKIP_UNICAST |
		windows.GAA_FLAG_SKIP_ANYCAST |
		windows.GAA_FLAG_SKIP_MULTICAST |
		windows.GAA_FLAG_INCLUDE_GATEWAYS

	resolverFlags = windows.GAA_FLAG_SKIP_UNICAST |
		windows.GAA_FLAG_SKIP_ANYCAST |
		windows.GAA_FLAG_SKIP_MULTICAST

	profileFlags = windows.GAA_FLAG_SKIP_UNICAST |
		windows.GAA_FLAG_SKIP_ANYCAST |
		windows.GAA_FLAG_SKIP_MULTICAST |
		windows.GAA_FLAG_SKIP_DNS_SERVER
)

// profileMarker namespaces an adapter-derived value. A wireless network name
// and an adapter name are both plausible strings; the marker makes the former
// impossible to confuse with the latter even in a debug dump.
const profileMarker = "adapter:"

// walkAdapters performs the required two-call size/allocate/fetch dance for
// GetAdaptersAddresses and invokes visit once per adapter.
//
// The returned IP_ADAPTER_ADDRESSES block is a linked list of structures whose
// AdapterName, FriendlyName, Description and every nested First* pointer point
// INTO the single flat buffer this function allocates. The buffer is Go heap
// memory that is NOT explicitly freed, but it must stay reachable for as long
// as any pointer into it is read, and it must not be moved or collected
// mid-walk. Both are guaranteed by keeping the slice alive across the whole
// traversal (deferred runtime.KeepAlive) and by requiring visit to copy every
// string it retains: ip.String() and windows.UTF16PtrToString both allocate, so
// nothing handed back to a caller ever aliases the buffer.
//
// walkAdapters reports whether the enumeration succeeded. Failure is not an
// error the caller must handle specially: on this platform a missing gateway
// or resolver list is an acceptable absence, never a fabricated value.
func walkAdapters(flags uint32, visit func(*windows.IpAdapterAddresses)) bool {
	var size uint32
	// First call: no buffer, so this only reports the required size. Anything
	// other than ERROR_BUFFER_OVERFLOW means there is nothing to enumerate
	// (ERROR_NO_DATA) or the call failed outright.
	if err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, flags, 0, nil, &size); err != windows.ERROR_BUFFER_OVERFLOW {
		return false
	}
	if size == 0 {
		return false
	}

	buf := make([]byte, size)
	defer runtime.KeepAlive(buf)
	if err := windows.GetAdaptersAddresses(windows.AF_UNSPEC, flags, 0, (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])), &size); err != nil {
		return false
	}

	for adapter := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buf[0])); adapter != nil; adapter = adapter.Next {
		// A zero length marks a malformed list; trusting it would loop forever.
		if adapter.Length == 0 {
			break
		}
		visit(adapter)
	}
	return true
}

// socketAddressText renders one SOCKET_ADDRESS, or "" when it is not an IPv4
// or IPv6 address. The result is freshly allocated; it does not alias the
// enumeration buffer.
func socketAddressText(addr *windows.SocketAddress) string {
	if addr.Sockaddr == nil {
		return ""
	}
	ip := addr.IP()
	if ip == nil {
		return ""
	}
	return ip.String()
}

// usableAddress normalizes text into the canonical form used everywhere in
// this package, or returns "" when the value cannot identify anything.
//
// The unspecified addresses are dropped: Windows reports 0.0.0.0 and :: for
// adapters that have no configured gateway or resolver, and those mean
// "absent", not "the gateway is 0.0.0.0". Keeping them would let a machine
// with no gateway at all hash differently from one that was never asked.
func usableAddress(text string) string {
	text = normalizeAddress(text)
	if text == "" {
		return ""
	}
	ip := net.ParseIP(text)
	if ip == nil || ip.IsUnspecified() {
		return ""
	}
	return text
}

// addressSet turns raw enumerated text into the canonical sorted set this
// package uses, dropping unusable entries. Order of enumeration is never
// semantic, so the result depends only on the SET of values. An input with
// nothing usable in it yields nil, which the canonical form treats as an
// explicitly absent field.
func addressSet(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if usable := usableAddress(value); usable != "" {
			out = append(out, usable)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return sortAndDedup(out)
}

// usableInterfaceIndexes reduces the caller's interface set to the indices
// that may own a gateway: up and not loopback. It returns nil when nothing
// qualifies, which means no gateway can be attributed to a real local network.
func usableInterfaceIndexes(ifaces []net.Interface) map[uint32]struct{} {
	indexes := make(map[uint32]struct{}, len(ifaces))
	for _, ifc := range ifaces {
		if ifc.Index <= 0 {
			continue
		}
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		indexes[uint32(ifc.Index)] = struct{}{}
	}
	if len(indexes) == 0 {
		return nil
	}
	return indexes
}

// adapterEligible reports whether an enumerated adapter may contribute a
// gateway address: it is up, it is not the software loopback, and the caller
// actually named its interface.
func adapterEligible(adapter *windows.IpAdapterAddresses, indexes map[uint32]struct{}) bool {
	if adapter == nil || len(indexes) == 0 {
		return false
	}
	if adapter.IfIndex == 0 || adapter.OperStatus != windows.IfOperStatusUp {
		return false
	}
	if adapter.IfType == windows.IF_TYPE_SOFTWARE_LOOPBACK {
		return false
	}
	_, named := indexes[adapter.IfIndex]
	return named
}

// readGateways returns the default-route gateway addresses of the given
// interfaces, IPv4 and IPv6 alike.
//
// Without shell access this is the only honest source of "where does my
// traffic actually go" on Windows: PowerShell's NetTCPIP providers are known
// to hang in this environment (see ENVIRONMENT_SPECIFIC_NETTCPIP_PROVIDER_HANG
// in docs/release/v0.7.0-acceptance.md), and a hang in a desktop app is worse
// than a missing gateway.
//
// Returns nil if the enumeration fails or yields nothing usable. Absence is
// acceptable and simply makes the network context identity weaker; it is
// never replaced with a fabricated address.
func readGateways(ifaces []net.Interface) []string {
	indexes := usableInterfaceIndexes(ifaces)
	if len(indexes) == 0 {
		return nil
	}
	var gateways []string
	if !walkAdapters(gatewayFlags, func(adapter *windows.IpAdapterAddresses) {
		if !adapterEligible(adapter, indexes) {
			return
		}
		for gateway := adapter.FirstGatewayAddress; gateway != nil; gateway = gateway.Next {
			gateways = append(gateways, socketAddressText(&gateway.Address))
		}
	}) {
		return nil
	}
	return addressSet(gateways)
}

// readResolvers returns the configured DNS resolver addresses, from the same
// enumeration, as a canonical sorted set.
//
// Returns nil on failure or when nothing is configured. Absence is acceptable.
func readResolvers() []string {
	var servers []string
	if !walkAdapters(resolverFlags, func(adapter *windows.IpAdapterAddresses) {
		if adapter.OperStatus != windows.IfOperStatusUp {
			return
		}
		if adapter.IfType == windows.IF_TYPE_SOFTWARE_LOOPBACK {
			return
		}
		for dns := adapter.FirstDnsServerAddress; dns != nil; dns = dns.Next {
			servers = append(servers, socketAddressText(&dns.Address))
		}
	}) {
		return nil
	}
	return addressSet(servers)
}

// profileName is the adapter-derived identity contributed to the network
// context. It describes HOW this machine is attached, which is stable across
// reboots and across which network is joined, and it is not a network name.
func profileName(name string) string {
	trimmed := strings.ToLower(strings.TrimSpace(name))
	if trimmed == "" {
		return ""
	}
	return profileMarker + trimmed
}

// readNetworkProfile returns an opaque, locally stable description of the
// adapter topology, or "".
//
// It is built from the FriendlyName (falling back to Description) of the
// up, non-loopback adapters, marked so it can never be read as a wireless
// network name. Wireless adapters are excluded outright: Windows connection
// profiles and hotspot adapters can carry a name that IS the SSID, and the
// point of this field is to notice that the machine's local setup changed, not
// to record which network was joined.
//
// Returns "" if the enumeration fails or yields nothing usable. That must not
// fail the identity: the canonical form encodes an empty profile explicitly,
// so a machine that cannot report one is still identifiable by its other
// facts.
func readNetworkProfile() string {
	var names []string
	walkAdapters(profileFlags, func(adapter *windows.IpAdapterAddresses) {
		if adapter.OperStatus != windows.IfOperStatusUp {
			return
		}
		switch adapter.IfType {
		case windows.IF_TYPE_SOFTWARE_LOOPBACK, windows.IF_TYPE_IEEE80211, windows.IF_TYPE_TUNNEL:
			return
		}
		name := windows.UTF16PtrToString(adapter.FriendlyName)
		if name == "" {
			name = windows.UTF16PtrToString(adapter.Description)
		}
		if token := profileName(name); token != "" {
			names = append(names, token)
		}
	})
	sorted := sortAndDedup(names)
	if len(sorted) == 0 {
		return ""
	}
	return strings.Join(sorted, "|")
}

package engine

import "sync"

var (
	secureDNSMu sync.RWMutex

	// SecureDNSServers are the resolvers Unbound switches to when the user enables
	// Secure DNS. Cloudflare is used because it answers on both plain UDP/53 and
	// DNS-over-HTTPS from the same addresses, so the setting keeps working whether
	// or not the platform supports DoH.
	//
	// Prefer GetSecureDNSServers() for concurrent reads.
	SecureDNSServers = []string{"1.1.1.1", "1.0.0.1"}
)

// DNSProviders contains pre-configured public DNS resolvers.
var DNSProviders = map[string][]string{
	"Cloudflare": {"1.1.1.1", "1.0.0.1"},
	"Google":     {"8.8.8.8", "8.8.4.4"},
	"Quad9":      {"9.9.9.9", "149.112.112.112"},
	"AdGuard":    {"94.140.14.14", "94.140.15.15"},
}

// GetSecureDNSServers returns a thread-safe copy of the active DNS resolvers.
func GetSecureDNSServers() []string {
	secureDNSMu.RLock()
	defer secureDNSMu.RUnlock()
	out := make([]string, len(SecureDNSServers))
	copy(out, SecureDNSServers)
	return out
}

// SetDNSProvider changes the active resolvers to the requested provider.
func SetDNSProvider(name string) {
	secureDNSMu.Lock()
	defer secureDNSMu.Unlock()
	if servers, ok := DNSProviders[name]; ok {
		copied := make([]string, len(servers))
		copy(copied, servers)
		SecureDNSServers = copied
	}
}

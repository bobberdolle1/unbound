package engine

// SecureDNSServers are the resolvers Unbound switches to when the user enables
// Secure DNS. Cloudflare is used because it answers on both plain UDP/53 and
// DNS-over-HTTPS from the same addresses, so the setting keeps working whether
// or not the platform supports DoH.
//
// These were previously hardcoded as string literals in four separate places
// (app.go's setter, its getter, the README and the tray), which meant changing
// the resolver required finding all four.
var SecureDNSServers = []string{"1.1.1.1", "1.0.0.1"}

// DNSProviders contains pre-configured public DNS resolvers.
var DNSProviders = map[string][]string{
	"Cloudflare": {"1.1.1.1", "1.0.0.1"},
	"Google":     {"8.8.8.8", "8.8.4.4"},
	"Quad9":      {"9.9.9.9", "149.112.112.112"},
	"AdGuard":    {"94.140.14.14", "94.140.15.15"},
}

// SetDNSProvider changes the active resolvers to the requested provider.
func SetDNSProvider(name string) {
	if servers, ok := DNSProviders[name]; ok {
		SecureDNSServers = servers
	}
}

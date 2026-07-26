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

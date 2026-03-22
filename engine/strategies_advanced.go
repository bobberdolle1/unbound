package engine

type AdvancedProfile struct {
	Name        string
	Description string
	Args        []string
	Category    string
	Techniques  []string
}

func GetAdvancedProfiles(luaDir string) []AdvancedProfile {
	return []AdvancedProfile{
		{
			Name:        "Unbound Ultimate (God Mode)",
			Description: "Universal multi-strategy with TLS/QUIC/UDP bypass and stateful tracking",
			Category:    "universal",
			Techniques:  []string{"multisplit", "tls_payload", "quic_payload", "udp_bypass", "conntrack", "autottl", "seqovl"},
			Args: []string{
				"--blob=tls_google:tls_clienthello_www_google_com.bin",
				"--blob=quic_google:quic_initial_www_google_com.bin",
				"--filter-tcp=443", "--payload=tls_client_hello", 
				"--lua-desync=fake:blob=tls_google:ttl=4:repeats=11:tls_mod=rnd,dupsid,sni=www.google.com", 
				"--lua-desync=multidisorder:pos=1,midsld",
				"--new", 
				"--filter-udp=443,50000-65535", "--payload=quic_initial,discord_ip_discovery",
				"--lua-desync=fake:blob=quic_google:ttl=4:repeats=11",
				"--lua-desync=multisplit:pos=1",
				"--new",
				"--filter-udp=50000-65535", "--payload=unknown",
				"--lua-desync=udplen:increment=2",
			},
		},
	}
}

func GetProfilesByCategory(category string) []AdvancedProfile {
	allProfiles := GetAdvancedProfiles("")
	filtered := make([]AdvancedProfile, 0)
	
	for _, p := range allProfiles {
		if p.Category == category {
			filtered = append(filtered, p)
		}
	}
	
	return filtered
}

func GetProfileCategories() []string {
	return []string{
		"universal",
		"aggressive",
		"smart",
		"experimental",
		"stealth",
		"deep",
		"chaos",
		"handshake",
		"quic",
		"http",
		"stateful",
	}
}

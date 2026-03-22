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
				"--filter-tcp=443",
				"--hostlist-auto=autodetect.txt",
				"--hostlist-auto-fail-threshold=2",
				"--hostlist-auto-fail-time=30",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=fake:ttl=3:tcp_md5:badseq:blob=fake_default_tls",
				"--lua-desync=fake:ttl=4:tcp_md5:badseq:blob=fake_default_tls",
				"--lua-desync=multisplit:pos=sni,sni+1,sni+2:badseq",
				"--new",
				"--filter-udp=443",
				"--hostlist-auto=autodetect.txt",
				"--payload=quic_initial",
				"--lua-desync=fake:ttl=3:repeats=15:blob=fake_default_quic",
				"--lua-desync=multisplit:pos=1,2,3",
				"--new",
				"--filter-udp=50000-65535",
				"--payload=discord_ip_discovery,stun,unknown",
				"--lua-desync=fake:ttl=4:repeats=3:blob=0x00000000000000000000000000000000",
				"--lua-desync=udplen:increment=2",
				"--new",
				"--filter-tcp=80,5222,5223,5228,8888",
				"--out-range=-d10",
				"--payload=http_req,mtproto_initial",
				"--lua-desync=fake:ttl=4:tcp_md5:blob=fake_default_http",
				"--lua-desync=multisplit:pos=1,2",
			},
		},
		{
			Name:        "Stealth Mode (Minimal Footprint)",
			Description: "Low-profile bypass with minimal packet manipulation",
			Category:    "stealth",
			Techniques:  []string{"multisplit", "single_fake", "low_ttl"},
			Args: []string{
				"--filter-tcp=443",
				"--hostlist-auto=autodetect.txt",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=fake:ttl=6:blob=fake_default_tls",
				"--lua-desync=multisplit:pos=sni",
				"--new",
				"--filter-udp=443",
				"--payload=quic_initial",
				"--lua-desync=fake:ttl=6:blob=fake_default_quic",
			},
		},
		{
			Name:        "Chaos Engineering (Max Entropy)",
			Description: "Extreme randomization and disorder for advanced DPI",
			Category:    "chaos",
			Techniques:  []string{"multidisorder", "triple_fake", "badseq", "tcp_md5"},
			Args: []string{
				"--filter-tcp=443",
				"--hostlist-auto=autodetect.txt",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=fake:ttl=2:tcp_md5:badseq:blob=fake_default_tls",
				"--lua-desync=fake:ttl=3:tcp_md5:badseq:blob=fake_default_tls",
				"--lua-desync=fake:ttl=4:tcp_md5:badseq:blob=fake_default_tls",
				"--lua-desync=multidisorder:pos=sni,sni+1,sni+2,midsld:badseq",
				"--new",
				"--filter-udp=443",
				"--payload=quic_initial",
				"--lua-desync=fake:ttl=2:repeats=20:blob=fake_default_quic",
				"--lua-desync=multisplit:pos=1,2,3,4",
			},
		},
		{
			Name:        "QUIC Specialist",
			Description: "Optimized for QUIC/HTTP3 traffic (YouTube, Google services)",
			Category:    "quic",
			Techniques:  []string{"quic_initial", "fake_flood", "multisplit"},
			Args: []string{
				"--filter-udp=443",
				"--hostlist-auto=autodetect.txt",
				"--payload=quic_initial",
				"--lua-desync=fake:ttl=3:repeats=25:blob=fake_default_quic",
				"--lua-desync=multisplit:pos=1,2,3",
				"--new",
				"--filter-tcp=443",
				"--hostlist-auto=autodetect.txt",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=fake:ttl=4:blob=fake_default_tls",
				"--lua-desync=multisplit:pos=sni:badseq",
			},
		},
		{
			Name:        "Deep Packet Inspection Killer",
			Description: "Multi-layer attack against stateful DPI with connection tracking",
			Category:    "deep",
			Techniques:  []string{"multidisorder", "badseq", "tcp_md5", "repeats"},
			Args: []string{
				"--filter-tcp=443",
				"--hostlist-auto=autodetect.txt",
				"--out-range=-d10",
				"--payload=tls_client_hello",
				"--lua-desync=fake:ttl=3:tcp_md5:badseq:blob=fake_default_tls",
				"--lua-desync=fake:ttl=4:tcp_md5:badseq:blob=fake_default_tls",
				"--lua-desync=multidisorder:pos=sni,sni+1,midsld:badseq",
				"--new",
				"--filter-udp=443,50000-65535",
				"--payload=quic_initial,discord_ip_discovery,stun",
				"--lua-desync=fake:ttl=3:repeats=15:blob=fake_default_quic",
				"--lua-desync=multisplit:pos=1,2",
			},
		},
		{
			Name:        "HTTP/MTProto Focus",
			Description: "Specialized for Telegram and HTTP-based services",
			Category:    "http",
			Techniques:  []string{"http_req", "mtproto_initial", "multisplit"},
			Args: []string{
				"--filter-tcp=80,443,5222,5223,5228,8888",
				"--hostlist-auto=autodetect.txt",
				"--out-range=-d10",
				"--payload=http_req,mtproto_initial,tls_client_hello",
				"--lua-desync=fake:ttl=4:tcp_md5:blob=fake_default_http",
				"--lua-desync=fake:ttl=4:tcp_md5:blob=fake_default_tls",
				"--lua-desync=multisplit:pos=1,sni",
				"--new",
				"--filter-udp=443,8888",
				"--payload=quic_initial",
				"--lua-desync=fake:ttl=4:repeats=10:blob=fake_default_quic",
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
